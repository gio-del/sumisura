import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { exportApplicationsCsvUrl, followUpsIcsUrl, isConflict, jobListingLogoUrl, listApplications, updateApplicationStatus } from '@/api/client'
import type { Application, ApplicationGroups, ApplicationStatus, CloseReason, JobListingSummaryWithApplication } from '@/api/types'
import type { RALRange } from '@/api/types'
import ApplicationStatusControl from '@/components/ApplicationStatusControl'
import ConflictAlert from '@/components/ConflictAlert'
import RALBadge from '@/components/RALBadge'
import StaleBadge from '@/components/StaleBadge'
import { formatFollowUp, nextInterview, formatInterview } from '@/lib/followUp'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { methodKindLabel } from '@/lib/applicationMethod'
import { statusBadgeClass, statusLabel } from '@/lib/applicationStatus'
import { cn, jobListingHeading } from '@/lib/utils'

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

function applicationsCount(count: number): string {
  return `${count} ${count === 1 ? 'Application' : 'Applications'}`
}

function generationsLabel(count: number): string {
  return `${count} ${count === 1 ? 'Generation' : 'Generations'}`
}

function statusChangedLabel(statusUpdatedAt?: string): string {
  const changedAt = statusUpdatedAt ? new Date(statusUpdatedAt) : null
  if (!changedAt || Number.isNaN(changedAt.getTime())) return 'Status change date unknown'
  return `Status changed ${changedAt.toLocaleDateString()}`
}

/**
 * ApplicationsPage is the Applications view (issue #95): every tracked
 * Application under its Status, answering "what is waiting on me right now".
 * It complements the Stats page rather than repeating it — Stats is the
 * pipeline in aggregate, this is the specific Applications in it. Grouping
 * and order come from GET /api/applications as-is; the page never regroups
 * or re-sorts, so the two views cannot disagree about where an Application
 * stands. Everything a row cannot show is one link away on the Job Listing's
 * own page.
 */
export default function ApplicationsPage() {
  const [data, setData] = useState<ApplicationGroups | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [statusError, setStatusError] = useState<string | null>(null)
  const [updatingId, setUpdatingId] = useState<string | null>(null)
  // conflict is a row Status move the backend refused with 409 because the
  // Application changed on disk since this view was loaded (issue #89).
  const [conflict, setConflict] = useState(false)
  const [reloading, setReloading] = useState(false)
  // tab is the Status shown on a phone, one column at a time; null means the
  // first group that has something in it.
  const [tab, setTab] = useState<ApplicationStatus | null>(null)

  useEffect(() => {
    listApplications()
      .then(setData)
      .catch((e) => setError(e.message))
  }, [])

  // A move goes through the same per-Application PATCH every page uses, and
  // the backend stays the authority on whether it is legal. Only once it has
  // persisted is the view re-read, so the row lands where the server's
  // grouping puts it and a failed move never shows as a moved row. The move
  // presents the row's Application token, so a row the skill or another tab
  // moved on since this view loaded is refused rather than moved backwards.
  async function handleStatusChange(id: string, status: ApplicationStatus, closeReason?: CloseReason) {
    setStatusError(null)
    setConflict(false)
    setUpdatingId(id)
    const row = data?.groups.flatMap((group) => group.items).find((item) => item.application.id === id)
    try {
      await updateApplicationStatus(id, status, row?.application.version, closeReason)
    } catch (err) {
      if (isConflict(err)) {
        setConflict(true)
      } else {
        setStatusError(errorMessage(err))
      }
      setUpdatingId(null)
      return
    }
    try {
      setData(await listApplications())
    } catch (err) {
      setStatusError(`Status changed, but the Applications view could not refresh: ${errorMessage(err)}`)
    } finally {
      setUpdatingId(null)
    }
  }

  async function handleReloadAfterConflict() {
    setReloading(true)
    try {
      setData(await listApplications())
      setConflict(false)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setReloading(false)
    }
  }

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!data) return <p>Loading…</p>

  const activeStatus = tab ?? (data.groups.find((g) => g.count > 0) ?? data.groups[0])?.status

  return (
    <>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="mb-0">Applications</h1>
        <div className="flex gap-2">
          <Button asChild variant="outline">
            <a href={followUpsIcsUrl()} download>
              Add to calendar
            </a>
          </Button>
          <Button asChild variant="outline">
            <a href={exportApplicationsCsvUrl()} download>
              Export CSV
            </a>
          </Button>
        </div>
      </div>

      {conflict && (
        <ConflictAlert
          record="Application"
          action="change"
          keepsEdits={false}
          onReload={handleReloadAfterConflict}
          reloading={reloading}
        />
      )}
      {statusError && (
        <p role="alert" className="mb-4 font-medium text-destructive">
          {statusError}
        </p>
      )}

      {data.total === 0 ? (
        <p>
          No tracked Applications yet. Every saved Job Listing gets one —{' '}
          <Link to="/jobs/new">Save a Job Listing</Link> to start your pipeline.
        </p>
      ) : (
        <>
          <p className="mb-6 text-sm text-muted-foreground">
            Stale Applications first, then the ones waiting longest since their last Status change. Archived Job
            Listings are left out; find them under <Link to="/jobs?archived=only">Job Listings</Link>.
          </p>
          <div role="tablist" aria-label="Status" className="-mx-4 mb-4 flex gap-2 overflow-x-auto px-4 pb-1 md:hidden">
            {data.groups.map((group) => (
              <button
                key={group.status}
                type="button"
                role="tab"
                id={`tab-${group.status}`}
                aria-selected={group.status === activeStatus}
                onClick={() => setTab(group.status)}
                className={cn(
                  'inline-flex min-h-11 shrink-0 cursor-pointer items-center gap-2 rounded-full border px-4 text-sm font-medium',
                  group.status === activeStatus
                    ? 'border-primary bg-primary text-primary-foreground'
                    : 'border-border bg-card text-foreground',
                )}
              >
                {statusLabel[group.status]}
                <span className="tabular-nums opacity-80">{group.count}</span>
              </button>
            ))}
          </div>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4 2xl:grid-cols-7">
            {data.groups.map((group) => (
              <section
                key={group.status}
                aria-label={`${statusLabel[group.status]}, ${applicationsCount(group.count)}`}
                className={cn('min-w-0', group.status === activeStatus ? 'block' : 'hidden md:block')}
              >
                <h2 className="mb-3 hidden items-center gap-2 md:flex">
                  {statusLabel[group.status]}
                  <Badge variant="secondary" aria-hidden>
                    {group.count}
                  </Badge>
                </h2>
                {group.items.length === 0 ? (
                  <p className="text-sm text-muted-foreground">No Applications in {statusLabel[group.status]}.</p>
                ) : (
                  <ul className="flex flex-col gap-3">
                    {group.items.map((item) => (
                      <ApplicationRow
                        key={item.jobListing.id}
                        item={item}
                        updating={updatingId === item.application.id}
                        onMove={(next, closeReason) => handleStatusChange(item.application.id, next, closeReason)}
                      />
                    ))}
                  </ul>
                )}
              </section>
            ))}
          </div>
        </>
      )}
    </>
  )
}

function ApplicationRow({
  item,
  updating,
  onMove,
}: {
  item: JobListingSummaryWithApplication
  updating: boolean
  onMove: (next: ApplicationStatus, closeReason?: CloseReason) => void
}) {
  const { jobListing, application } = item
  const generationCount = application.generations?.length ?? 0
  const methodUnresolved = application.method.kind === 'unresolved'

  const interview = nextInterviewOf(application)

  return (
    <li className="min-w-0 rounded-xl border border-border bg-card p-3">
      <div className="flex items-start gap-2">
        {jobListing.logo && (
          <img src={jobListingLogoUrl(jobListing.id)} alt="" className="h-8 w-8 shrink-0 rounded object-contain" />
        )}
        <Link
          to={`/jobs/${encodeURIComponent(jobListing.id)}`}
          className="min-w-0 break-words font-semibold no-underline hover:underline"
        >
          {jobListingHeading(jobListing)}
        </Link>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-2">
        <Badge data-status={application.status} className={statusBadgeClass[application.status]}>
          {statusLabel[application.status]}
        </Badge>
        {application.isStale && <StaleBadge followUpAt={application.followUpAt} />}
        {!application.isStale && application.followUpAt && (
          <Badge variant="outline">Follow-up {formatFollowUp(application.followUpAt)}</Badge>
        )}
        {interview && <Badge variant="outline">Interview {formatInterview(interview.at)}</Badge>}
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
        <span className={cn(methodUnresolved && 'font-medium text-unresolved')}>
          {methodUnresolved ? 'Method unresolved' : `Method: ${methodKindLabel[application.method.kind]}`}
        </span>
        {application.contact && <span className="text-muted-foreground">Contact recorded</span>}
        {generationCount > 0 && <span className="text-muted-foreground">{generationsLabel(generationCount)}</span>}
        <span className="text-muted-foreground">{statusChangedLabel(application.statusUpdatedAt)}</span>
      </div>
      {hasRALFigure(jobListing.ral) && (
        <div className="mt-2 [&>div]:mb-0">
          <RALBadge ral={jobListing.ral} />
        </div>
      )}
      <div className="mt-3 flex items-center gap-2">
        <Button asChild className="min-w-0 flex-1">
          {generationCount === 0 ? (
            <Link to={`/jobs/${encodeURIComponent(jobListing.id)}/generate`}>Generate CV</Link>
          ) : (
            <Link to={`/jobs/${encodeURIComponent(jobListing.id)}`}>Open</Link>
          )}
        </Button>
        <ApplicationStatusControl
          menu
          company={jobListing.company}
          status={application.status}
          disabled={updating}
          onMove={onMove}
        />
      </div>
    </li>
  )
}

// hasRALFigure: the RAL box only appears when there is something to say — a
// figure, or a conflict between the two sources.
function hasRALFigure(ral: RALRange): boolean {
  if (ral.source === 'conflict') return true
  return ral.source !== 'n/a' && ral.min != null && ral.max != null
}

// nextInterviewOf is the next interview of an Application still open (not
// Rejected, Withdrawn or Offer), the one the list shows.
function nextInterviewOf(application: Application) {
  if (application.status === 'rejected' || application.status === 'withdrawn' || application.status === 'offer') return undefined
  return nextInterview(application.interviews)
}
