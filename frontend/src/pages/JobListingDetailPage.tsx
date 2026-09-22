import { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkBreaks from 'remark-breaks'
import ApplicationMethodEditor from '@/components/ApplicationMethodEditor'
import ApplicationNotes from '@/components/ApplicationNotes'
import ApplicationStatusBadges from '@/components/ApplicationStatusBadges'
import ApplicationStatusControl from '@/components/ApplicationStatusControl'
import ApplyGuidance from '@/components/ApplyGuidance'
import ConflictAlert from '@/components/ConflictAlert'
import FreshnessBadge from '@/components/FreshnessBadge'
import JobListingFieldsEditor from '@/components/JobListingFieldsEditor'
import NeedsAttentionBadge from '@/components/NeedsAttentionBadge'
import RALBadge from '@/components/RALBadge'
import StaleEntriesNotice from '@/components/StaleEntriesNotice'
import {
  ApiError,
  atsReportPath,
  checkJobListingFreshness,
  deleteJobListing,
  generationFileUrl,
  getJobListing,
  isConflict,
  jobListingLogoUrl,
  resolveJobListing,
  setJobListingArchived,
  updateApplicationContact,
  updateApplicationMethod,
  updateApplicationStatus,
} from '@/api/client'
import type { ApplicationMethod, ApplicationStatus, Contact, GenerationRecord, JobListingWithApplication } from '@/api/types'
import { GENERATION_FILES_GONE_NOTE, useGenerationOnDisk } from '@/components/useGenerationOnDisk'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { jobListingHeading } from '@/lib/utils'

const JOB_LISTINGS_PATH = '/jobs'

// Links to a Generation's files, or — since output/ is derived and may have
// been cleared (ADR-0008) — a plain note that they're no longer on disk.
function GenerationFileLinks({ generation }: { generation: GenerationRecord }) {
  const onDisk = useGenerationOnDisk(generation.slug)
  // The ATS Report lives on the record, so it stays reachable after the
  // files are gone (issue #198).
  const atsLink = generation.atsReports && (
    <>
      {' · '}
      <Link to={atsReportPath(generation.slug)}>ATS Report</Link>
    </>
  )

  if (onDisk === false) {
    return (
      <>
        <FilesGoneNote />
        {atsLink}
      </>
    )
  }

  return (
    <>
      <a href={generationFileUrl(generation.slug, 'cv.pdf')} target="_blank" rel="noreferrer">
        CV
      </a>
      {generation.coverLetterPath && (
        <>
          {' · '}
          <a href={generationFileUrl(generation.slug, 'cover-letter.pdf')} target="_blank" rel="noreferrer">
            Cover Letter
          </a>
        </>
      )}
      {atsLink}
    </>
  )
}

function FilesGoneNote() {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="underline decoration-dotted">files no longer on disk</span>
      </TooltipTrigger>
      <TooltipContent>{GENERATION_FILES_GONE_NOTE}</TooltipContent>
    </Tooltip>
  )
}

/**
 * JobListingDetailLocationState is what a Job Listings list row puts in
 * navigation state when it links here: the list's own location, filters
 * and RAL sort included, so the back link returns to the list as it was.
 */
export interface JobListingDetailLocationState {
  from?: string
}

// backTarget is the back link's destination: the list location the row link
// carried when there is one, the bare list for a deep link, a bookmark or a
// reload — so arriving either way is never a dead end (issue #94 stories
// 4-7). Anything that is not a Job Listings list location is ignored rather
// than followed.
function backTarget(state: unknown): string {
  const from = (state as JobListingDetailLocationState | null)?.from
  if (typeof from === 'string' && (from === JOB_LISTINGS_PATH || from.startsWith(`${JOB_LISTINGS_PATH}?`))) {
    return from
  }
  return JOB_LISTINGS_PATH
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

/**
 * JobListingDetailPage is one Job Listing and its Application at their own
 * address, /jobs/:id (issue #94): the place a single Job Listing is worked
 * on. It fetches its own record by id — never receiving one through
 * navigation state — so a deep link and a click from the list take exactly
 * the same path, and every mutation replaces that one record in place.
 */
export default function JobListingDetailPage() {
  const { id = '' } = useParams()
  const location = useLocation()
  const navigate = useNavigate()
  const backTo = backTarget(location.state)

  const [record, setRecord] = useState<JobListingWithApplication | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [notFound, setNotFound] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [updatingStatus, setUpdatingStatus] = useState(false)
  const [resolving, setResolving] = useState(false)
  const [checkingFreshness, setCheckingFreshness] = useState(false)
  const [archiving, setArchiving] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  // fixRequest is the popover asking one of the editors below to open
  // (issue #206, stories 74-76). It carries a timestamp so asking twice
  // for the same field opens it again rather than being swallowed as an
  // unchanged value.
  const [fixRequest, setFixRequest] = useState<{ field: 'ral' | 'method'; at: number } | null>(null)
  // conflict is a Status change or delete the backend refused with 409
  // because the Application or Job Listing changed on disk since this page
  // read them (issue #89). Method and Contact edits show their own, next to
  // the draft they preserve.
  const [conflict, setConflict] = useState<{ record: string; action: 'change' | 'delete' } | null>(null)
  const [reloading, setReloading] = useState(false)

  useEffect(() => {
    setRecord(null)
    setLoadError(null)
    setNotFound(false)
    setActionError(null)
    setConflict(null)
    getJobListing(id)
      .then(setRecord)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 404) {
          setNotFound(true)
          return
        }
        setLoadError(errorMessage(err))
      })
  }, [id])

  const backLink = (
    <p className="mb-4 inline-block text-sm">
      <Link to={backTo} className="no-underline hover:underline">
        ← Back to Job Listings
      </Link>
    </p>
  )

  if (notFound)
    return (
      <>
        {backLink}
        <h1>Job Listing not found</h1>
        <p>There is no Job Listing with id “{id}”. It may have been deleted since this link was saved.</p>
      </>
    )
  if (loadError)
    return (
      <>
        {backLink}
        <p role="alert" className="font-medium text-destructive">
          {loadError}
        </p>
      </>
    )
  if (!record) return <p>Loading…</p>

  const { jobListing, application } = record
  const heading = jobListingHeading(jobListing)
  const generations = application.generations ?? []

  // reloadRecord re-reads the Job Listing and its Application — the explicit
  // "reload the current version" action after a conflict, which refreshes
  // both version tokens (issue #89).
  async function reloadRecord() {
    setRecord(await getJobListing(id))
  }

  async function handleReloadAfterConflict() {
    setReloading(true)
    try {
      await reloadRecord()
      setConflict(null)
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setNotFound(true)
        return
      }
      setActionError(errorMessage(err))
    } finally {
      setReloading(false)
    }
  }

  async function handleStatusChange(status: ApplicationStatus) {
    setActionError(null)
    setConflict(null)
    setUpdatingStatus(true)
    try {
      const updated = await updateApplicationStatus(id, status, application.version)
      setRecord((prev) => (prev ? { ...prev, application: updated } : prev))
    } catch (err) {
      if (isConflict(err)) {
        setConflict({ record: 'Application', action: 'change' })
      } else {
        setActionError(errorMessage(err))
      }
    } finally {
      setUpdatingStatus(false)
    }
  }

  async function handleMethodChange(method: ApplicationMethod) {
    const updated = await updateApplicationMethod(id, method, application.version)
    setRecord((prev) => (prev ? { ...prev, application: updated } : prev))
  }

  async function handleContactChange(contact: Contact) {
    const updated = await updateApplicationContact(id, contact, application.version)
    setRecord((prev) => (prev ? { ...prev, application: updated } : prev))
  }

  async function handleResolve() {
    setActionError(null)
    setResolving(true)
    try {
      setRecord(await resolveJobListing(id))
    } catch (err) {
      setActionError(errorMessage(err))
    } finally {
      setResolving(false)
    }
  }

  async function handleCheckFreshness() {
    setActionError(null)
    setCheckingFreshness(true)
    try {
      const updated = await checkJobListingFreshness(id)
      setRecord((prev) => (prev ? { ...prev, jobListing: updated } : prev))
    } catch (err) {
      setActionError(errorMessage(err))
    } finally {
      setCheckingFreshness(false)
    }
  }

  // Archive/Unarchive (issue #98) needs no confirmation: it deletes nothing
  // and is undone by the same button, unlike Delete below.
  async function handleArchiveToggle() {
    setActionError(null)
    setConflict(null)
    setArchiving(true)
    try {
      const updated = await setJobListingArchived(id, !jobListing.archived, jobListing.version)
      setRecord((prev) => (prev ? { ...prev, jobListing: updated } : prev))
    } catch (err) {
      if (isConflict(err)) {
        setConflict({ record: 'Job Listing', action: 'change' })
      } else {
        setActionError(errorMessage(err))
      }
    } finally {
      setArchiving(false)
    }
  }

  async function handleConfirmDelete() {
    setDeleteError(null)
    setConflict(null)
    setDeleting(true)
    try {
      await deleteJobListing(id, jobListing.version, application.version)
      navigate(backTo)
    } catch (err) {
      if (isConflict(err)) {
        // Deleting also destroys the Application, so a Status that moved
        // on since the page was loaded refuses it too (story 19).
        setConflict({ record: 'Job Listing or its Application', action: 'delete' })
      } else {
        setDeleteError(errorMessage(err))
      }
      setDeleting(false)
      setDeleteOpen(false)
    }
  }

  return (
    <>
      {backLink}

      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <span className="flex min-w-0 items-center gap-3">
          {jobListing.logo && (
            <img src={jobListingLogoUrl(jobListing.id)} alt="" className="h-10 w-10 shrink-0 rounded object-contain" />
          )}
          <h1 className="mb-0 break-words">{heading}</h1>
        </span>
        <div className="flex flex-wrap items-center gap-2">
          {jobListing.archived && <Badge variant="outline">Archived</Badge>}
          <ApplicationStatusBadges
            application={application}
            needsAttention={
              <NeedsAttentionBadge
                ral={jobListing.ral}
                method={application.method}
                onRetry={handleResolve}
                onEnterRAL={() => setFixRequest({ field: 'ral', at: Date.now() })}
                onSetMethod={() => setFixRequest({ field: 'method', at: Date.now() })}
                retrying={resolving}
              />
            }
          />
          <ApplicationStatusControl
            company={jobListing.company}
            status={application.status}
            disabled={updatingStatus}
            onMove={handleStatusChange}
          />
        </div>
      </div>

      {conflict && (
        <ConflictAlert
          record={conflict.record}
          action={conflict.action}
          keepsEdits={false}
          onReload={handleReloadAfterConflict}
          reloading={reloading}
        />
      )}
      {actionError && (
        <p role="alert" className="mb-4 font-medium text-destructive">
          {actionError}
        </p>
      )}
      {deleteError && (
        <p role="alert" className="mb-4 font-medium text-destructive">
          {deleteError}
        </p>
      )}

      <section className="flex flex-col gap-3 rounded-xl border border-border bg-card px-4 py-3">
        <p className="mb-0 text-sm text-muted-foreground">
          Saved {new Date(jobListing.savedAt).toLocaleDateString()}
          {jobListing.url && (
            <>
              {' · '}
              <a href={jobListing.url} target="_blank" rel="noreferrer">
                View posting
              </a>
            </>
          )}
        </p>
        {jobListing.url && (
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <FreshnessBadge status={jobListing.freshnessStatus} />
            {jobListing.freshnessCheckedAt && (
              <span className="text-muted-foreground">
                Checked {new Date(jobListing.freshnessCheckedAt).toLocaleString()}
              </span>
            )}
            <Tooltip>
              <TooltipTrigger asChild>
                <Button size="sm" variant="outline" onClick={handleCheckFreshness} disabled={checkingFreshness}>
                  {checkingFreshness ? 'Checking…' : 'Check freshness'}
                </Button>
              </TooltipTrigger>
              <TooltipContent>Re-fetches the source URL to see if the posting is still live</TooltipContent>
            </Tooltip>
          </div>
        )}
        <JobListingFieldsEditor
          jobListing={jobListing}
          onChange={(updated) => setRecord((prev) => (prev ? { ...prev, jobListing: updated } : prev))}
          onReload={reloadRecord}
          openRALEntry={fixRequest?.field === 'ral' ? fixRequest.at : undefined}
        />
        <RALBadge ral={jobListing.ral} />
        <ApplicationMethodEditor
          method={application.method}
          onSave={handleMethodChange}
          onReload={reloadRecord}
          openEditor={fixRequest?.field === 'method' ? fixRequest.at : undefined}
        />
        <ApplyGuidance jobListing={jobListing} application={application} onSaveContact={handleContactChange}
          onReload={reloadRecord}
        />
      </section>

      <ApplicationNotes
        applicationId={application.id}
        version={application.version}
        notes={application.notes}
        onApplicationChange={(updated) => setRecord((prev) => (prev ? { ...prev, application: updated } : prev))}
        onReload={reloadRecord}
      />

      <section className="mt-6">
        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
          <h2 className="my-0">Generations</h2>
          <Button asChild size="sm" variant="outline">
            <Link to={`/jobs/${jobListing.id}/generate`}>{generations.length ? 'Regenerate CV' : 'Generate CV'}</Link>
          </Button>
        </div>
        {generations.length === 0 ? (
          <p className="text-sm text-muted-foreground">No Generations yet.</p>
        ) : (
          <ol aria-label="Generation history" className="flex flex-col gap-1 pl-0 text-sm">
            {generations
              .map((generation, index) => ({ generation, index }))
              .reverse()
              .map(({ generation, index }) => (
                <li key={generation.slug + generation.createdAt} className="list-none">
                  {formatDate(generation.createdAt)}
                  {index === generations.length - 1 && ' (latest)'}
                  {' · '}
                  <GenerationFileLinks generation={generation} />
                </li>
              ))}
          </ol>
        )}
        <StaleEntriesNotice generations={application.generations} />
      </section>

      {jobListing.jobDescription && (
        <section className="mt-6">
          <h2 className="mt-0 mb-2">Job Description</h2>
          <div
            className="overflow-x-auto rounded-lg border border-border bg-muted/40 p-3 text-sm break-words
              [&_a]:underline [&_ol]:list-decimal [&_ol]:pl-5 [&_p+p]:mt-2 [&_p+ul]:mt-2 [&_strong]:font-semibold
              [&_ul]:list-disc [&_ul]:pl-5"
          >
            <ReactMarkdown remarkPlugins={[remarkBreaks]}>{jobListing.jobDescription}</ReactMarkdown>
          </div>
        </section>
      )}

      <div className="mt-6 flex justify-end gap-2">
        <Button variant="outline" onClick={handleArchiveToggle} disabled={archiving || deleting}>
          {jobListing.archived ? 'Unarchive' : 'Archive'}
        </Button>
        <Button variant="outline" onClick={() => setDeleteOpen(true)} disabled={deleting}>
          Delete
        </Button>
      </div>

      <AlertDialog open={deleteOpen} onOpenChange={(open) => !deleting && setDeleteOpen(open)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {heading}?</AlertDialogTitle>
            <AlertDialogDescription>
              This will also remove its Application (Status, Method, Contact, Notes, and Generation history). This
              action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={(e) => {
                e.preventDefault()
                handleConfirmDelete()
              }}
              disabled={deleting}
            >
              {deleting ? 'Deleting…' : 'Yes, delete'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
