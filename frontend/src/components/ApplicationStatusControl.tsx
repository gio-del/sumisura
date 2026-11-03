import { useState } from 'react'
import { CLOSE_REASONS, CLOSE_REASON_LABELS } from '@/api/types'
import type { ApplicationStatus, CloseReason } from '@/api/types'
import { allowedNextStatuses, needsConfirmation, statusLabel } from '@/lib/applicationStatus'
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

// NO_REASON stands for "no close reason" in a Select, whose items cannot
// carry an empty value.
export const NO_REASON = 'none'

/**
 * ApplicationStatusControl is the one "Move to…" control, rendered by both
 * the Job Listings list (fast triage across many records) and the Job
 * Listing detail page (issue #94). The allowed-next-Status map, the rule for
 * which moves need confirming and the confirmation dialog itself live here
 * so the two pages cannot drift into confirming a move on one page and not
 * the other. Sending the move is the caller's job: each page updates its own
 * state and surfaces its own errors.
 */
export default function ApplicationStatusControl({
  company,
  status,
  disabled,
  onMove,
}: {
  company: string
  status: ApplicationStatus
  disabled?: boolean
  onMove: (next: ApplicationStatus, closeReason?: CloseReason) => void
}) {
  const [pendingStatus, setPendingStatus] = useState<ApplicationStatus | null>(null)
  const [pendingReason, setPendingReason] = useState<CloseReason | ''>('')
  const nextStatuses = allowedNextStatuses[status]

  if (nextStatuses.length === 0) return null

  function handleSelect(next: ApplicationStatus) {
    if (needsConfirmation(status, next)) {
      setPendingReason('')
      setPendingStatus(next)
      return
    }
    onMove(next)
  }

  return (
    <>
      <Select value="" onValueChange={(value) => handleSelect(value as ApplicationStatus)} disabled={disabled}>
        <SelectTrigger size="sm" aria-label={`Move ${company} to a new status`}>
          <SelectValue placeholder="Move to…" />
        </SelectTrigger>
        <SelectContent>
          {nextStatuses.map((next) => (
            <SelectItem key={next} value={next}>
              {statusLabel[next]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <AlertDialog open={pendingStatus !== null} onOpenChange={(open) => !open && setPendingStatus(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pendingStatus === 'rejected'
                ? 'Mark this Application Rejected?'
                : pendingStatus === 'withdrawn'
                  ? 'Mark this Application Withdrawn?'
                  : 'Reopen this Application to Interviewing?'}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {pendingStatus === 'rejected'
                ? `${company} will be marked Rejected. You can Reopen it back to Interviewing later if this turns out to be premature.`
                : pendingStatus === 'withdrawn'
                  ? `${company} will be marked Withdrawn. You can Reopen it back to Interviewing later if you change your mind.`
                  : `${company} will move back to Interviewing.`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          {(pendingStatus === 'rejected' || pendingStatus === 'withdrawn') && (
            <Select value={pendingReason || NO_REASON} onValueChange={(value) => setPendingReason(value === NO_REASON ? '' : (value as CloseReason))}>
              <SelectTrigger aria-label="Close reason">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NO_REASON}>No reason</SelectItem>
                {CLOSE_REASONS.map((reason) => (
                  <SelectItem key={reason} value={reason}>
                    {CLOSE_REASON_LABELS[reason]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault()
                if (pendingStatus) onMove(pendingStatus, pendingReason || undefined)
                setPendingStatus(null)
              }}
            >
              {pendingStatus === 'rejected'
                ? 'Yes, mark Rejected'
                : pendingStatus === 'withdrawn'
                  ? 'Yes, mark Withdrawn'
                  : 'Yes, reopen'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
