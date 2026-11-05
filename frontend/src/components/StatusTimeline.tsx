import { CLOSE_REASON_LABELS } from '@/api/types'
import type { Application } from '@/api/types'
import { statusLabel } from '@/lib/applicationStatus'
import { statusDurations } from '@/lib/statusDurations'

function plural(days: number): string {
  return `${days} ${days === 1 ? 'day' : 'days'}`
}

// StatusTimeline is the read-only «Cronologia» of an Application: one line per
// Status it passed through, with the date and the whole days spent there
// (issue #288). Nothing is shown when the history was never recorded.
export default function StatusTimeline({ application, now = new Date() }: { application: Application; now?: Date }) {
  const rows = statusDurations(application.statusHistory, now)
  if (rows.length === 0) return null
  const closed = application.status === 'rejected' || application.status === 'withdrawn'
  return (
    <section className="mt-6" aria-labelledby="status-timeline-heading">
      <h2 id="status-timeline-heading" className="mb-2 mt-0">
        History
      </h2>
      <ol aria-label="Status history" className="flex flex-col gap-1 pl-0 text-sm">
        {rows.map(({ change, days, current }) => (
          <li key={change.status + change.changedAt} className="list-none">
            <span className="font-medium">{statusLabel[change.status]}</span>
            {' · '}
            {new Date(change.changedAt).toLocaleDateString()}
            {' · '}
            {current ? `for ${plural(days)}` : plural(days)}
            {current && closed && application.closeReason && (
              <span className="text-muted-foreground"> · {CLOSE_REASON_LABELS[application.closeReason]}</span>
            )}
          </li>
        ))}
      </ol>
    </section>
  )
}
