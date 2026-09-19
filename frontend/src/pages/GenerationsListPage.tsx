import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { atsReportPath, deleteGeneration, listGenerations } from '@/api/client'
import type { GroundednessResult, IndexedGeneration } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

// formatDate renders the row's date, tolerating the empty createdAt an
// output directory gets when its name carries no -yyyymmdd-hhmmss stamp.
function formatDate(createdAt: string): string {
  if (!createdAt) return 'Unknown date'
  const d = new Date(createdAt)
  return Number.isNaN(d.getTime()) ? 'Unknown date' : d.toLocaleString()
}

// GroundednessSummary counts the bullets that carried at least one flag —
// the same signal Text Review shows, reduced to one line.
function GroundednessSummary({ result }: { result: GroundednessResult }) {
  const flagged = (result.bullets ?? []).filter((b) => b.flags.length > 0).length
  const coverLetterFlags = (result.coverLetter ?? []).length
  if (flagged === 0 && coverLetterFlags === 0) {
    return <span className="text-sm text-muted-foreground">Groundedness: no flags</span>
  }
  return (
    <span className="text-sm text-muted-foreground">
      Groundedness: {flagged} bullet{flagged === 1 ? '' : 's'} flagged
      {coverLetterFlags > 0 ? `, ${coverLetterFlags} in the cover letter` : ''}
    </span>
  )
}

// DeleteControl asks once, on the row itself. Deleting clears the files in
// output/<slug>/; a recorded Generation keeps its record, so the row stays
// with its files reported missing, and an unrecorded one disappears.
function DeleteControl({ generation, onDeleted }: { generation: IndexedGeneration; onDeleted: () => void }) {
  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleDelete() {
    setDeleting(true)
    setError(null)
    try {
      await deleteGeneration(generation.slug)
      onDeleted()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setDeleting(false)
      setConfirming(false)
    }
  }

  if (error) {
    return (
      <span role="alert" className="text-sm font-medium text-destructive">
        {error}
      </span>
    )
  }

  if (!confirming) {
    return (
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="ml-auto"
        onClick={() => setConfirming(true)}
      >
        Delete files
      </Button>
    )
  }

  return (
    <span className="ml-auto flex items-center gap-2">
      <span className="text-sm text-muted-foreground">
        {generation.recorded ? 'Delete the files? The record stays.' : 'Delete the files?'}
      </span>
      <Button type="button" variant="destructive" size="sm" disabled={deleting} onClick={handleDelete}>
        {deleting ? 'Deleting…' : 'Delete'}
      </Button>
      <Button type="button" variant="outline" size="sm" disabled={deleting} onClick={() => setConfirming(false)}>
        Cancel
      </Button>
    </span>
  )
}

function GenerationRow({ generation, onDeleted }: { generation: IndexedGeneration; onDeleted: () => void }) {
  const { slug, company, jobTitle, applicationId, recorded, hasCv, hasCoverLetter } = generation
  return (
    <li className="rounded-xl border border-border bg-card px-4 py-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <span className="font-medium">
          {recorded && applicationId ? (
            <Link to={`/jobs/${applicationId}`} className="no-underline hover:underline">
              {company}
              {jobTitle ? ` — ${jobTitle}` : ''}
            </Link>
          ) : (
            'Default Mode'
          )}
        </span>
        <span className="text-sm text-muted-foreground">{formatDate(generation.createdAt)}</span>
      </div>

      <p className="mb-0 font-mono text-xs text-muted-foreground">{slug}</p>

      <div className="mt-2 flex flex-wrap items-center gap-2">
        {hasCv ? (
          <a href={`/api/generations/${slug}/cv.pdf`} target="_blank" rel="noreferrer">
            CV
          </a>
        ) : (
          <span className="text-sm text-muted-foreground">CV file missing</span>
        )}
        {hasCoverLetter && (
          <a href={`/api/generations/${slug}/cover-letter.pdf`} target="_blank" rel="noreferrer">
            Cover Letter
          </a>
        )}
        {generation.hasAtsReport && <Link to={atsReportPath(slug)}>ATS Report</Link>}
        {generation.language && <Badge variant="secondary">{generation.language}</Badge>}
        {/* A row with no record is a Generation nothing tracked: the files
            are real, but the Application link, groundedness and language
            were never written down. */}
        {!recorded && <Badge variant="outline">Not tracked</Badge>}
        {generation.groundedness && <GroundednessSummary result={generation.groundedness} />}
        {(hasCv || hasCoverLetter) && <DeleteControl generation={generation} onDeleted={onDeleted} />}
      </div>
    </li>
  )
}

export default function GenerationsListPage() {
  const [generations, setGenerations] = useState<IndexedGeneration[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  // reload re-reads the index after a delete rather than patching state:
  // deleting an unrecorded Generation removes its row, while deleting a
  // recorded one keeps the row and flips its files to missing — the backend
  // decides which, not the page.
  const reload = useCallback(() => {
    listGenerations()
      .then((g) => setGenerations(g ?? []))
      .catch((e) => setError(e.message))
  }, [])

  useEffect(() => {
    reload()
  }, [reload])

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!generations) return <p>Loading…</p>

  return (
    <>
      <h1>Generated CVs</h1>

      {generations.length === 0 ? (
        <p>
          Nothing generated yet. Run <Link to="/generate">Generate</Link>, or the{' '}
          <code>/sumisura:tailor-cv</code> skill.
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {generations.map((generation) => (
            <GenerationRow key={generation.slug} generation={generation} onDeleted={reload} />
          ))}
        </ul>
      )}
    </>
  )
}
