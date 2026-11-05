import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { deleteGeneration, getDiskUsage } from '@/api/client'
import type { DiskUsage } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

function formatDate(createdAt: string): string {
  const d = new Date(createdAt)
  return !createdAt || Number.isNaN(d.getTime()) ? 'Unknown date' : d.toLocaleString()
}

// GenerationSpacePage shows what output/ occupies (issue #285) and proposes
// superseded Generations. Nothing is deleted without an explicit confirmation,
// and deletion reuses the per-Generation call: records stay.
export default function GenerationSpacePage() {
  const [days, setDays] = useState(90)
  const [usage, setUsage] = useState<DiskUsage | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const load = useCallback(() => {
    getDiskUsage(days)
      .then((u) => {
        setUsage(u)
        setSelected(new Set())
      })
      .catch((e) => setError(e.message))
  }, [days])

  useEffect(() => {
    load()
  }, [load])

  function toggle(slug: string) {
    setConfirming(false)
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(slug)) next.delete(slug)
      else next.add(slug)
      return next
    })
  }

  async function handleDelete() {
    setDeleting(true)
    setError(null)
    try {
      for (const slug of selected) await deleteGeneration(slug)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
    setDeleting(false)
    setConfirming(false)
    load()
  }

  if (!usage) {
    return error ? (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    ) : (
      <p>Loading…</p>
    )
  }

  const superseded = usage.generations.filter((g) => g.superseded)
  const selectedBytes = usage.generations.filter((g) => selected.has(g.slug)).reduce((n, g) => n + g.bytes, 0)

  return (
    <>
      <h1>Space</h1>
      <p>
        <Link to="/generations">Back to Generated CVs</Link>
      </p>
      <p>
        <strong>output/</strong> uses {formatBytes(usage.totalBytes)} across {usage.generations.length} Generation
        {usage.generations.length === 1 ? '' : 's'}.
      </p>
      {error && (
        <p role="alert" className="font-medium text-destructive">
          {error}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <label htmlFor="older-than-days">Closed Applications count as old after (days)</label>
        <input
          id="older-than-days"
          type="number"
          min={0}
          value={days}
          onChange={(e) => setDays(Math.max(0, Number(e.target.value) || 0))}
          className="w-20 rounded border border-border px-2 py-1"
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => setSelected(new Set(superseded.map((g) => g.slug)))}
        >
          Select superseded ({superseded.length})
        </Button>
      </div>

      {usage.generations.length === 0 ? (
        <p>Nothing in output/.</p>
      ) : (
        <ul className="mt-3 flex flex-col gap-2">
          {usage.generations.map((g) => (
            <li key={g.slug} className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-card px-4 py-3">
              <input
                type="checkbox"
                aria-label={`Select ${g.slug}`}
                checked={selected.has(g.slug)}
                onChange={() => toggle(g.slug)}
              />
              <span className="font-mono text-xs">{g.slug}</span>
              <span className="text-sm text-muted-foreground">
                {g.applicationId ? `${g.company}${g.jobTitle ? ` — ${g.jobTitle}` : ''}` : 'Default Mode'}
              </span>
              <span className="text-sm text-muted-foreground">{formatDate(g.createdAt)}</span>
              <span className="ml-auto text-sm">{formatBytes(g.bytes)}</span>
              {g.superseded && <Badge variant="secondary">Superseded</Badge>}
            </li>
          ))}
        </ul>
      )}

      {selected.size > 0 && (
        <div className="mt-3 flex items-center gap-2">
          {confirming ? (
            <>
              <span className="text-sm text-muted-foreground">
                Delete the files of {selected.size} Generation{selected.size === 1 ? '' : 's'} ({formatBytes(selectedBytes)})?
                Recorded Generations keep their record.
              </span>
              <Button type="button" variant="destructive" size="sm" disabled={deleting} onClick={handleDelete}>
                {deleting ? 'Deleting…' : 'Confirm delete'}
              </Button>
              <Button type="button" variant="outline" size="sm" disabled={deleting} onClick={() => setConfirming(false)}>
                Cancel
              </Button>
            </>
          ) : (
            <Button type="button" variant="destructive" size="sm" onClick={() => setConfirming(true)}>
              Delete selected ({selected.size})
            </Button>
          )}
        </div>
      )}
    </>
  )
}
