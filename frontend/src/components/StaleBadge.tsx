import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { formatFollowUp } from '@/lib/followUp'

// StaleBadge is the one marker for an Application whose isStale flag is set
// (issue #37): shown on Job Listing rows, the detail page and the
// Applications view (issue #95) alike. With a follow-up day set (issue #240)
// the tooltip names it instead of the fixed 14 days.
export default function StaleBadge({ followUpAt }: { followUpAt?: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Badge variant="outline" className="border-accent text-accent">
          Follow-up overdue
        </Badge>
      </TooltipTrigger>
      <TooltipContent>
        {followUpAt ? `Follow-up was due ${formatFollowUp(followUpAt)}` : 'No Status change in over 14 days'}
      </TooltipContent>
    </Tooltip>
  )
}
