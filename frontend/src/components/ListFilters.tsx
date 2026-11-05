import { Link, useSearchParams } from 'react-router-dom'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'

// The filter lives in the URL (?q=&tag=&tag=) so it survives a detour into
// one record and back (issue #294).
export interface ListFilterState {
  q: string
  tags: string[]
  /** Extra single-valued filters by param name (e.g. kind). */
  extra: Record<string, string>
  active: boolean
}

export function useListFilters(extraKeys: string[] = []): ListFilterState & {
  set: (key: string, value: string) => void
  toggleTag: (tag: string) => void
  clear: () => void
} {
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const tags = params.getAll('tag')
  const extra: Record<string, string> = {}
  for (const k of extraKeys) extra[k] = params.get(k) ?? ''
  const active = Boolean(q || tags.length || Object.values(extra).some(Boolean))

  function set(key: string, value: string) {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value) next.set(key, value)
        else next.delete(key)
        return next
      },
      { replace: true },
    )
  }
  function toggleTag(tag: string) {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        const cur = next.getAll('tag')
        next.delete('tag')
        for (const t of cur.includes(tag) ? cur.filter((c) => c !== tag) : [...cur, tag]) next.append('tag', t)
        return next
      },
      { replace: true },
    )
  }
  function clear() {
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        for (const k of ['q', 'tag', ...extraKeys]) next.delete(k)
        return next
      },
      { replace: true },
    )
  }
  return { q, tags, extra, active, set, toggleTag, clear }
}

interface Props {
  label: string
  q: string
  onQuery: (q: string) => void
  /** Clickable chips: tags, or kinds when used for the extra filter. */
  tagChoices: string[]
  chosenTags: string[]
  onToggleTag: (tag: string) => void
  kindChoices?: string[]
  chosenKind?: string
  onKind?: (kind: string) => void
}

export function ListFilters(p: Props) {
  return (
    <div className="mb-4 flex flex-col gap-2">
      <Input
        type="search"
        aria-label={p.label}
        placeholder={p.label}
        value={p.q}
        onChange={(e) => p.onQuery(e.target.value)}
      />
      {p.kindChoices && p.kindChoices.length > 0 && (
        <div className="flex flex-wrap gap-1" role="group" aria-label="Filter by kind">
          {p.kindChoices.map((k) => (
            <button
              type="button"
              key={k}
              aria-pressed={p.chosenKind === k}
              onClick={() => p.onKind?.(p.chosenKind === k ? '' : k)}
            >
              <Badge variant={p.chosenKind === k ? 'default' : 'outline'}>{k}</Badge>
            </button>
          ))}
        </div>
      )}
      {p.tagChoices.length > 0 && (
        <div className="flex flex-wrap gap-1" role="group" aria-label="Filter by tag">
          {p.tagChoices.map((t) => (
            <button type="button" key={t} aria-pressed={p.chosenTags.includes(t)} onClick={() => p.onToggleTag(t)}>
              <Badge variant={p.chosenTags.includes(t) ? 'default' : 'secondary'}>{t}</Badge>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

export function NoResults({ onClear }: { onClear: () => void }) {
  return (
    <p>
      No matches.{' '}
      <Link
        to="#"
        onClick={(e) => {
          e.preventDefault()
          onClear()
        }}
      >
        Azzera filtri
      </Link>
    </p>
  )
}
