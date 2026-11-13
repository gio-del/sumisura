import type { ReactNode } from 'react'
import { CLOSE_REASON_LABELS } from '@/api/types'
import type { Application } from '@/api/types'
import StaleBadge from '@/components/StaleBadge'
import { Badge } from '@/components/ui/badge'
import { formatFollowUp } from '@/lib/followUp'
import { statusBadgeClass, statusLabel } from '@/lib/applicationStatus'

// ApplicationStatusBadges is the at-a-glance state of a Job Listing's
// Application — needs-attention, Status and follow-up-overdue — rendered
// identically on its list row and its detail page (issue #94).
//
// needsAttention is passed in rather than derived here: on the detail page
// it is the popover that names each cause and offers its fix (issue #206),
// while a list row, which has no resolve controls, shows the plain badge.
export default function ApplicationStatusBadges({
  application,
  needsAttention,
}: {
  application: Application
  needsAttention: ReactNode
}) {
  return (
    <>
      {needsAttention}
      <Badge data-status={application.status} className={statusBadgeClass[application.status]}>
        {statusLabel[application.status]}
      </Badge>
      {application.closeReason && <Badge variant="outline">{CLOSE_REASON_LABELS[application.closeReason]}</Badge>}
      {application.isStale && <StaleBadge followUpAt={application.followUpAt} />}
      {!application.isStale && application.followUpAt && (
        <Badge variant="outline">Follow-up {formatFollowUp(application.followUpAt)}</Badge>
      )}
    </>
  )
}
