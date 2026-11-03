import { useState } from 'react'
import { compareGenerations } from '@/api/client'
import type { CompareText, GenerationComparison, GenerationRecord } from '@/api/types'
import { Button } from '@/components/ui/button'

const UNAVAILABLE = 'Data not available'

function formatDate(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString()
}

// Before/after pair, stacked so a phone never scrolls sideways: long text
// wraps inside each block.
function Pair({ a, b }: { a: string; b: string }) {
  return (
    <div className="flex flex-col gap-1">
      <p className="m-0 rounded border-l-4 border-destructive/60 bg-muted/40 px-2 py-1 break-words whitespace-pre-wrap">
        <span className="sr-only">A: </span>
        {a || <em className="text-muted-foreground">(not kept)</em>}
      </p>
      <p className="m-0 rounded border-l-4 border-primary/60 bg-muted/40 px-2 py-1 break-words whitespace-pre-wrap">
        <span className="sr-only">B: </span>
        {b || <em className="text-muted-foreground">(not kept)</em>}
      </p>
    </div>
  )
}

function TextSection({ title, text, missing }: { title: string; text?: CompareText; missing: boolean }) {
  return (
    <section aria-label={title} className="flex flex-col gap-1">
      <h3 className="my-0 text-base">{title}</h3>
      {!text ? (
        <p className="m-0 text-muted-foreground">{missing ? UNAVAILABLE : 'None'}</p>
      ) : text.changed ? (
        <Pair a={text.a} b={text.b} />
      ) : (
        <p className="m-0 text-muted-foreground">Same in both.</p>
      )}
    </section>
  )
}

function EntryList({ title, items }: { title: string; items: { entryId: string; title: string }[] }) {
  return (
    <section aria-label={title} className="flex flex-col gap-1">
      <h3 className="my-0 text-base">{title}</h3>
      {items.length === 0 ? (
        <p className="m-0 text-muted-foreground">None</p>
      ) : (
        <ul className="m-0 pl-5">
          {items.map((e) => (
            <li key={e.entryId}>{e.title}</li>
          ))}
        </ul>
      )}
    </section>
  )
}

function Result({ result }: { result: GenerationComparison }) {
  const selectionMissing = !result.a.selectionAvailable || !result.b.selectionAvailable
  return (
    <div aria-label="Comparison" role="region" className="mt-3 flex flex-col gap-4 text-sm">
      {selectionMissing ? (
        <p role="status" className="m-0 text-muted-foreground">
          {UNAVAILABLE}: {[result.a, result.b].filter((s) => !s.selectionAvailable).map((s) => s.slug).join(', ')} has
          no selection.json.
        </p>
      ) : (
        <>
          <EntryList title="Entries only in B" items={result.entriesAdded} />
          <EntryList title="Entries only in A" items={result.entriesRemoved} />
          <section aria-label="Rewritten bullets" className="flex flex-col gap-2">
            <h3 className="my-0 text-base">Rewritten bullets</h3>
            {result.rewrittenChanges.length === 0 && <p className="m-0 text-muted-foreground">No differences.</p>}
            {result.rewrittenChanges.map((c) => (
              <div key={`${c.entryId}#${c.sourceIndex}`} className="flex flex-col gap-1">
                <span className="font-medium">{c.title}</span>
                <Pair a={c.a} b={c.b} />
              </div>
            ))}
          </section>
          <TextSection title="Summary" text={result.summary} missing={false} />
          <section aria-label="Tag row" className="flex flex-col gap-1">
            <h3 className="my-0 text-base">Tag row</h3>
            {result.tagChanges.length === 0 && <p className="m-0 text-muted-foreground">No differences.</p>}
            {result.tagChanges.map((c) => (
              <div key={c.entryId} className="flex flex-col gap-1">
                <span className="font-medium">{c.title}</span>
                <Pair a={c.a.join(', ')} b={c.b.join(', ')} />
              </div>
            ))}
          </section>
        </>
      )}
      <TextSection
        title="Cover Letter"
        text={result.coverLetter}
        missing={!result.a.coverLetterAvailable || !result.b.coverLetterAvailable}
      />
    </div>
  )
}

/**
 * GenerationCompare picks two of an Application's Generations and shows a
 * read-only textual comparison of their output directories (issue #263).
 * Nothing here calls Claude.
 */
export default function GenerationCompare({ generations }: { generations: GenerationRecord[] }) {
  const [a, setA] = useState(generations[generations.length - 2]?.slug ?? '')
  const [b, setB] = useState(generations[generations.length - 1]?.slug ?? '')
  const [result, setResult] = useState<GenerationComparison | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  if (generations.length < 2) return null

  async function run() {
    setLoading(true)
    setError(null)
    setResult(null)
    try {
      setResult(await compareGenerations(a, b))
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  const options = generations.map((g) => (
    <option key={g.slug} value={g.slug}>
      {formatDate(g.createdAt)} · {g.slug}
    </option>
  ))
  const selectClass = 'w-full rounded-md border border-input bg-background px-2 py-1 text-sm'

  return (
    <div className="mt-4">
      <h3 className="mt-0 mb-2 text-base">Compare Generations</h3>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
        <label className="flex flex-1 flex-col gap-1 text-sm">
          Generation A
          <select className={selectClass} value={a} onChange={(e) => setA(e.target.value)}>
            {options}
          </select>
        </label>
        <label className="flex flex-1 flex-col gap-1 text-sm">
          Generation B
          <select className={selectClass} value={b} onChange={(e) => setB(e.target.value)}>
            {options}
          </select>
        </label>
        <Button type="button" size="sm" variant="outline" disabled={loading || a === b} onClick={run}>
          {loading ? 'Comparing…' : 'Compare'}
        </Button>
      </div>
      {error && (
        <p role="alert" className="mt-2 text-sm font-medium text-destructive">
          {error}
        </p>
      )}
      {result && <Result result={result} />}
    </div>
  )
}
