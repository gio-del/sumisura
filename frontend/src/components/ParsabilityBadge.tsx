import type { ATSReport } from '@/api/types'
import { cn } from '@/lib/utils'

const statusLabel: Record<ATSReport['status'], string> = {
  ok: 'ATS check: OK',
  warning: 'ATS check: Warning',
  unavailable: 'ATS check: Unavailable',
}

const statusBorderClass: Record<ATSReport['status'], string> = {
  ok: 'border-border',
  warning: 'border-unresolved',
  unavailable: 'border-border',
}

// Non-blocking signal only (PRD "PDF ATS-parsability check", story 7) —
// this never gates approval the way Visual Review itself does. A pass
// shows a plain label; a warning is expandable to the missing fields/
// ordering violations that made the extracted text layer look off; an
// unavailable check (pdftotext missing/erroring) says so without implying
// anything about the PDF itself (story 10).
export default function ParsabilityBadge({ result, label }: { result: ATSReport; label: string }) {
  const border = statusBorderClass[result.status]

  if (result.status === 'ok') {
    return (
      <div className={cn('mb-2 inline-block rounded-xl border bg-card px-4 py-2', border)}>
        <strong className="font-semibold">
          {label}: {statusLabel.ok}
        </strong>
      </div>
    )
  }

  if (result.status === 'unavailable') {
    return (
      <div className={cn('mb-2 inline-block rounded-xl border bg-card px-4 py-2', border)}>
        <strong className="font-semibold">
          {label}: {statusLabel.unavailable}
        </strong>
        <div className="text-sm text-muted-foreground">
          The check couldn't run{result.reason ? `: ${result.reason}` : '.'}
        </div>
      </div>
    )
  }

  const hasDetail = (result.missingFields?.length ?? 0) > 0 || (result.orderingViolations?.length ?? 0) > 0

  return (
    <details className={cn('mb-2 inline-block rounded-xl border bg-card px-4 py-2', border)}>
      <summary className="cursor-pointer font-semibold">
        {label}: {statusLabel.warning}
      </summary>
      {hasDetail && (
        <div className="mt-2 text-sm">
          {result.missingFields && result.missingFields.length > 0 && (
            <div>
              <div className="text-muted-foreground">Missing from the extracted text:</div>
              <ul className="ml-4 list-disc">
                {result.missingFields.map((field) => (
                  <li key={field}>{field}</li>
                ))}
              </ul>
            </div>
          )}
          {result.orderingViolations && result.orderingViolations.length > 0 && (
            <div>
              <div className="text-muted-foreground">Out of order:</div>
              <ul className="ml-4 list-disc">
                {result.orderingViolations.map((violation) => (
                  <li key={violation}>{violation}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </details>
  )
}
