import type { ApplicationMethod, RALRange } from '@/api/types'
import { type AttentionCauseID, attentionCauses } from '@/lib/attentionCauses'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

// "Needs attention" used to be one word fired by two unrelated conditions
// (`ral.source === 'unresolved' || method.kind === 'unresolved'`), with no
// indication of which, what it meant, or what to do — and for an
// unresolved RAL Range there was no manual fix anywhere in the app. It is
// now a starting point: it names each cause actually firing in one plain
// sentence, and offers the fix beside it (issue #206, stories 71-78).

export default function NeedsAttentionBadge({
  ral,
  method,
  onRetry,
  onEnterRAL,
  onSetMethod,
  retrying,
}: {
  ral: RALRange
  method: ApplicationMethod
  // onRetry is the resolve call the app already had: it retries whichever
  // of the two is still unresolved.
  onRetry: () => void
  // onEnterRAL opens the manual RAL Range entry, so a retry that keeps
  // failing isn't a dead end (story 75).
  onEnterRAL: () => void
  // onSetMethod opens the Application Method editor the page already has.
  onSetMethod: () => void
  retrying: boolean
}) {
  const causes = attentionCauses(ral, method)
  if (causes.length === 0) return null

  const fixes: Record<AttentionCauseID, { label: string; onClick: () => void }> = {
    ral: { label: 'Enter it myself', onClick: onEnterRAL },
    method: { label: 'Set it myself', onClick: onSetMethod },
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button type="button" className="cursor-pointer">
          <Badge variant="outline" className="border-unresolved text-unresolved">
            Needs attention
          </Badge>
        </button>
      </PopoverTrigger>
      <PopoverContent>
        <p className="mt-0 mb-2 text-sm font-semibold">
          {causes.length === 1 ? 'One thing needs attention' : `${causes.length} things need attention`}
        </p>
        <ul className="m-0 flex list-none flex-col gap-3 p-0">
          {causes.map((cause) => (
            <li key={cause.id} className="flex flex-col gap-1">
              <span className="text-sm font-medium">{cause.title}</span>
              <span className="text-sm text-muted-foreground">{cause.explanation}</span>
              <span className="flex flex-wrap gap-2 pt-1">
                <Button size="sm" variant="outline" onClick={onRetry} disabled={retrying}>
                  {retrying ? 'Retrying…' : 'Retry'}
                </Button>
                <Button size="sm" variant="ghost" onClick={fixes[cause.id].onClick}>
                  {fixes[cause.id].label}
                </Button>
              </span>
            </li>
          ))}
        </ul>
      </PopoverContent>
    </Popover>
  )
}
