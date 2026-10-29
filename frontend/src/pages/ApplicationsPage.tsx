import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { isConflict, jobListingLogoUrl, listApplications, updateApplicationStatus } from '@/api/client'
import type { ApplicationGroups, ApplicationStatus, JobListingSummaryWithApplication } from '@/api/types'
import ApplicationStatusControl from '@/components/ApplicationStatusControl'
import ConflictAlert from '@/components/ConflictAlert'
import RALBadge from '@/components/RALBadge'
import StaleBadge from '@/components/StaleBadge'
import { formatFollowUp } from '@/lib/followUp'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { methodKindLabel } from '@/lib/applicationMethod'
import { statusLabel } from '@/lib/applicationStatus'
import { cn, jobListingHeading } from '@/lib/utils'

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

function applicationsCount(count: number): string {
  return `${count} ${count === 1 ? 'Application' : 'Applications'}`
}

function generationsLabel(count: number): string {
  if (count === 0) return 'No Generations yet'
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
  async function handleStatusChange(id: string, status: ApplicationStatus) {
    setStatusError(null)
    setConflict(false)
    setUpdatingId(id)
    const row = data?.groups.flatMap((group) => group.items).find((item) => item.application.id === id)
    try {
      await updateApplicationStatus(id, status, row?.application.version)
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

  return (
    <>
      <h1>Applications</h1>

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
          <div className="flex flex-col gap-8">
            {data.groups.map((group) => (
              <section key={group.status} aria-label={`${statusLabel[group.status]}, ${applicationsCount(group.count)}`}>
                <h2 className="mb-3 flex items-center gap-2">
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
                        onMove={(next) => handleStatusChange(item.application.id, next)}
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
  onMove: (next: ApplicationStatus) => void
}) {
  const { jobListing, application } = item
  const generationCount = application.generations?.length ?? 0
  const methodUnresolved = application.method.kind === 'unresolved'

  return (
    <li className="rounded-xl border border-border bg-card px-4 py-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="flex min-w-0 items-center gap-2">
          {jobListing.logo && (
            <img src={jobListingLogoUrl(jobListing.id)} alt="" className="h-8 w-8 shrink-0 rounded object-contain" />
          )}
          <Link
            to={`/jobs/${encodeURIComponent(jobListing.id)}`}
            className="break-words font-semibold no-underline hover:underline"
          >
            {jobListingHeading(jobListing)}
          </Link>
        </span>
        <div className="flex flex-wrap items-center gap-2">
          {application.isStale && <StaleBadge followUpAt={application.followUpAt} />}
          {!application.isStale && application.followUpAt && (
            <Badge variant="outline">Follow-up {formatFollowUp(application.followUpAt)}</Badge>
          )}
          <ApplicationStatusControl
            company={jobListing.company}
            status={application.status}
            disabled={updating}
            onMove={onMove}
          />
        </div>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
        <span className={cn(methodUnresolved && 'font-medium text-unresolved')}>
          {methodUnresolved ? 'Method unresolved' : `Method: ${methodKindLabel[application.method.kind]}`}
        </span>
        <span className="text-muted-foreground">{application.contact ? 'Contact recorded' : 'No Contact'}</span>
        <span className="text-muted-foreground">{generationsLabel(generationCount)}</span>
        <span className="text-muted-foreground">{statusChangedLabel(application.statusUpdatedAt)}</span>
        {generationCount === 0 && (
          <Button asChild size="sm" variant="outline" className="ml-auto">
            <Link to={`/jobs/${encodeURIComponent(jobListing.id)}/generate`}>Generate CV</Link>
          </Button>
        )}
      </div>
      <div className="mt-2">
        <RALBadge ral={jobListing.ral} />
      </div>
    </li>
  )
}
