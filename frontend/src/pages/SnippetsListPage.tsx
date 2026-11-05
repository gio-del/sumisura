import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { listSnippets } from '@/api/client'
import type { Snippet } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ListFilters, NoResults, useListFilters } from '@/components/ListFilters'
import { allTags, hasAllTags, matchesQuery } from '@/lib/listFilter'

export default function SnippetsListPage() {
  const [snippets, setSnippets] = useState<Snippet[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const filters = useListFilters(['kind'])

  useEffect(() => {
    listSnippets()
      .then((s) => setSnippets(s ?? []))
      .catch((e) => setError(e.message))
  }, [])

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!snippets) return <p>Loading…</p>

  const visible = snippets.filter(
    (s) =>
      hasAllTags(s.tags, filters.tags) &&
      (!filters.extra.kind || s.kind === filters.extra.kind) &&
      matchesQuery(filters.q, [s.kind, s.lang, ...s.tags, s.body]),
  )

  return (
    <>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="mb-0">Cover Letter Snippets</h1>
        <Button asChild>
          <Link to="/snippets/new">+ New Snippet</Link>
        </Button>
      </div>

      {snippets.length === 0 && (
        <p>No snippets yet. Without any, Cover Letter drafting falls back to fresh prose.</p>
      )}

      {snippets.length > 0 && (
        <ListFilters
          label="Search snippets"
          q={filters.q}
          onQuery={(v) => filters.set('q', v)}
          tagChoices={allTags(snippets)}
          chosenTags={filters.tags}
          onToggleTag={filters.toggleTag}
          kindChoices={[...new Set(snippets.map((s) => s.kind))].sort()}
          chosenKind={filters.extra.kind}
          onKind={(k) => filters.set('kind', k)}
        />
      )}

      {snippets.length > 0 && visible.length === 0 && <NoResults onClear={filters.clear} />}

      <ul className="flex flex-col gap-2">
        {visible.map((snippet) => (
          <li key={snippet.id} className="rounded-xl border border-border bg-card px-4 py-3">
            <Link to={`/snippets/${snippet.id}`} className="font-medium no-underline hover:underline">
              {snippet.kind}
            </Link>
            <p className="mb-0">
              {snippet.body.slice(0, 80)}
              {snippet.body.length > 80 ? '…' : ''}
            </p>
            <div className="mt-1 flex flex-wrap items-center gap-1">
              {snippet.lang && <Badge>{snippet.lang}</Badge>}
              {snippet.tags.map((tag) => (
                <Badge variant="secondary" key={tag}>
                  {tag}
                </Badge>
              ))}
              {snippet.lastUsedAt ? (
                <span className="text-xs text-muted-foreground">
                  Last used {new Date(snippet.lastUsedAt).toLocaleDateString()}
                </span>
              ) : (
                <Badge variant="outline">Never used</Badge>
              )}
            </div>
          </li>
        ))}
      </ul>
    </>
  )
}
