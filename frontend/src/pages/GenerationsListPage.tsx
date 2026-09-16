import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { listGenerations } from '@/api/client'
import type { GroundednessResult, IndexedGeneration } from '@/api/types'
import { Badge } from '@/components/ui/badge'

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

function GenerationRow({ generation }: { generation: IndexedGeneration }) {
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
        {generation.language && <Badge variant="secondary">{generation.language}</Badge>}
        {/* A row with no record is a Generation nothing tracked: the files
            are real, but the Application link, groundedness and language
            were never written down. */}
        {!recorded && <Badge variant="outline">Not tracked</Badge>}
        {generation.groundedness && <GroundednessSummary result={generation.groundedness} />}
      </div>
    </li>
  )
}

export default function GenerationsListPage() {
  const [generations, setGenerations] = useState<IndexedGeneration[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    listGenerations()
      .then((g) => setGenerations(g ?? []))
      .catch((e) => setError(e.message))
  }, [])

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
            <GenerationRow key={generation.slug} generation={generation} />
          ))}
        </ul>
      )}
    </>
  )
}
