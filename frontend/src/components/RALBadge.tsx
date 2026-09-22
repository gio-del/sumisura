import type { RALFigure, RALRange } from '@/api/types'
import { cn } from '@/lib/utils'

const sourceLabel: Record<RALRange['source'], string> = {
  stated: 'Stated in the Job Description',
  estimated: "Claude's estimate — not a fact",
  'n/a': 'Not found',
  unresolved: "Couldn't check — retry",
  conflict: 'Conflict — Job Description and listing disagree',
  manual: 'Entered by you',
}

const sourceBorderClass: Record<RALRange['source'], string> = {
  stated: 'border-ral-stated',
  estimated: 'border-ral-estimated',
  'n/a': 'border-border',
  unresolved: 'border-unresolved',
  conflict: 'border-ral-conflict',
  manual: 'border-ral-manual',
}

function formatFigure(figure: RALFigure): string {
  return figure.min === figure.max
    ? `${figure.currency} ${figure.min.toLocaleString()}`.trim()
    : `${figure.currency} ${figure.min.toLocaleString()} – ${figure.max.toLocaleString()}`.trim()
}

// Always labels the RAL Range's source, so the user is never misled into
// treating a guess as a fact (story 8, CONTEXT.md's RAL Range entry).
// Conflict has no auto-picked figure to show — both sources' own figures
// are shown side by side instead, purely informational (no "pick a
// winner" control, per ADR-0014).
export default function RALBadge({ ral }: { ral: RALRange }) {
  const border = sourceBorderClass[ral.source]

  if (ral.source === 'conflict' && ral.descriptionStated && ral.listingStated) {
    return (
      <div className={cn('mb-4 inline-block rounded-xl border bg-card px-4 py-2', border)}>
        <strong className="font-semibold">RAL Range: Conflict</strong>
        <div className="text-sm text-muted-foreground">{sourceLabel.conflict}</div>
        <div className="mt-1 text-sm">
          <div>Job Description: {formatFigure(ral.descriptionStated)}</div>
          <div>Listing: {formatFigure(ral.listingStated)}</div>
        </div>
      </div>
    )
  }

  const figure =
    ral.source === 'n/a' || ral.min == null || ral.max == null
      ? null
      : ral.min === ral.max
        ? `${ral.currency ?? ''} ${ral.min.toLocaleString()}`.trim()
        : `${ral.currency ?? ''} ${ral.min.toLocaleString()} – ${ral.max.toLocaleString()}`.trim()

  return (
    <div className={cn('mb-4 inline-block rounded-xl border bg-card px-4 py-2', border)}>
      <strong className="font-semibold">RAL Range: {figure ?? 'Not available'}</strong>
      <div className="text-sm text-muted-foreground">{sourceLabel[ral.source]}</div>
    </div>
  )
}
