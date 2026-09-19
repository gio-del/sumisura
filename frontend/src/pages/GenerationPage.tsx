import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import BulletDiff from '@/components/BulletDiff'
import ParsabilityBadge from '@/components/ParsabilityBadge'
import RALBadge from '@/components/RALBadge'
import {
  atsReportPath,
  createGeneration,
  generationFileUrl,
  getJobListing,
  listEntries,
  previewGeneration,
  recordApplicationGeneration,
  renderGeneration,
} from '@/api/client'
import type {
  Entry,
  GenerateResult,
  GenerationUsage,
  GroundednessFlag,
  GroundednessResult,
  JobListing,
  RALRange,
  RenderResult,
  SelectedBullet,
  SelectedEntry,
} from '@/api/types'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn, jobListingHeading } from '@/lib/utils'

const SLUG_RE = /^[a-z0-9]+(-[a-z0-9]+)*$/

// Matches backend/internal/generation.SupportedLanguages (issue #41's PRD).
const LANGUAGE_LABELS: Record<string, string> = { en: 'English', it: 'Italian' }

interface EditableBullet extends SelectedBullet {
  included: boolean
}

interface EditableEntry {
  entryId: string
  reason: string
  included: boolean
  bullets: EditableBullet[]
}

function toEditable(entries: SelectedEntry[]): EditableEntry[] {
  return entries.map((e) => ({
    entryId: e.entryId,
    reason: e.reason,
    included: true,
    bullets: e.bullets.map((b) => ({ ...b, included: true })),
  }))
}

function bulletFlags(groundedness: GroundednessResult | null, entryId: string, sourceIndex: number): GroundednessFlag[] {
  return groundedness?.bullets?.find((b) => b.entryId === entryId && b.sourceIndex === sourceIndex)?.flags ?? []
}

const GROUNDEDNESS_REASON_LABEL: Record<GroundednessFlag['reason'], string> = {
  'no-source-match': 'no matching source bullet found',
  'numeric-mismatch': 'contains a number/detail not present in source',
}

function GroundednessBadge({ flags }: { flags: GroundednessFlag[] }) {
  if (flags.length === 0) return null
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="ml-2 cursor-help rounded-full border border-amber-500/50 bg-amber-500/10 px-2 py-0.5 text-xs font-medium text-amber-700 dark:text-amber-400">
          {flags.length} flagged
        </span>
      </TooltipTrigger>
      <TooltipContent>
        <ul className="list-disc pl-4">
          {flags.map((flag, i) => (
            <li key={i}>
              "{flag.sentence}" — {GROUNDEDNESS_REASON_LABEL[flag.reason]}
            </li>
          ))}
        </ul>
      </TooltipContent>
    </Tooltip>
  )
}

function entryLabel(entry: Entry | undefined, entryId: string): string {
  if (!entry) return entryId
  if (entry.type === 'experience') {
    return entry.client ? `${entry.employer} — ${entry.client}` : (entry.employer ?? entryId)
  }
  return entry.name ?? entryId
}

export default function GenerationPage() {
  const { id: jobListingId } = useParams<{ id?: string }>()
  const [jobListing, setJobListing] = useState<JobListing | null>(null)
  const [entriesById, setEntriesById] = useState<Map<string, Entry>>(new Map())
  const [jobDescription, setJobDescription] = useState('')
  const [jobDescriptionUrl, setJobDescriptionUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [mode, setMode] = useState<'default' | 'tailored' | null>(null)
  const [editable, setEditable] = useState<EditableEntry[] | null>(null)
  const [coverLetter, setCoverLetter] = useState<string | null>(null)
  const [coverLetterSnippetIds, setCoverLetterSnippetIds] = useState<string[] | undefined>(undefined)
  const [ral, setRal] = useState<RALRange | null>(null)
  const [usage, setUsage] = useState<GenerationUsage | null>(null)
  const [language, setLanguage] = useState<string | null>(null)
  const [languageChanging, setLanguageChanging] = useState(false)
  const [groundedness, setGroundedness] = useState<GroundednessResult | null>(null)
  const [slug, setSlug] = useState(jobListingId ?? 'default')
  const [rendering, setRendering] = useState(false)
  const [renderError, setRenderError] = useState<string | null>(null)
  const [render, setRender] = useState<RenderResult | null>(null)
  const [linkError, setLinkError] = useState<string | null>(null)
  const [previewResult, setPreviewResult] = useState<GenerateResult | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)
  const [previewError, setPreviewError] = useState<string | null>(null)

  useEffect(() => {
    listEntries()
      .then((entries) => setEntriesById(new Map(entries.map((e) => [e.id, e]))))
      .catch(() => {
        // Labels fall back to raw entryIds if Master Data can't be loaded.
      })
  }, [])

  useEffect(() => {
    if (!jobListingId) return
    getJobListing(jobListingId)
      .then(({ jobListing: listing }) => {
        setJobListing(listing)
        setJobDescription(listing.jobDescription)
      })
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
  }, [jobListingId])

  async function handleStart(languageOverride?: string) {
    setError(null)
    setLoading(true)
    setRender(null)
    setRenderError(null)
    setPreviewResult(null)
    try {
      const result = await createGeneration({
        jobDescription: jobDescription.trim() || undefined,
        jobDescriptionUrl: jobDescriptionUrl.trim() || undefined,
        languageOverride,
      })
      setMode(result.mode)
      setEditable(toEditable(result.selection.entries))
      setCoverLetter(result.coverLetter?.body ?? null)
      setCoverLetterSnippetIds(result.coverLetter?.sourceSnippetIds)
      setRal(result.ral ?? null)
      setUsage(result.usage ?? null)
      setLanguage(result.language)
      setGroundedness(result.groundedness ?? null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  async function handlePreview() {
    setPreviewError(null)
    setPreviewLoading(true)
    setPreviewResult(null)
    try {
      const result = await previewGeneration({
        jobDescription: jobDescription.trim() || undefined,
        jobDescriptionUrl: jobDescriptionUrl.trim() || undefined,
      })
      setPreviewResult(result)
    } catch (err) {
      setPreviewError(err instanceof Error ? err.message : String(err))
    } finally {
      setPreviewLoading(false)
    }
  }

  // Re-runs Selection+Rewrite (and the Cover Letter, if any) in the
  // corrected language rather than translating the already-generated text
  // in place, so phrasing stays natural (issue #41's PRD, stories 4-5).
  async function handleLanguageChange(next: string) {
    setLanguageChanging(true)
    try {
      await handleStart(next)
    } finally {
      setLanguageChanging(false)
    }
  }

  function updateBullet(entryId: string, sourceIndex: number, rewritten: string) {
    setEditable((prev) =>
      prev
        ? prev.map((e) =>
            e.entryId !== entryId
              ? e
              : { ...e, bullets: e.bullets.map((b) => (b.sourceIndex === sourceIndex ? { ...b, rewritten } : b)) },
          )
        : prev,
    )
  }

  function toggleBullet(entryId: string, sourceIndex: number) {
    setEditable((prev) =>
      prev
        ? prev.map((e) =>
            e.entryId !== entryId
              ? e
              : {
                  ...e,
                  bullets: e.bullets.map((b) =>
                    b.sourceIndex === sourceIndex ? { ...b, included: !b.included } : b,
                  ),
                }
          )
        : prev,
    )
  }

  function toggleEntry(entryId: string) {
    setEditable((prev) =>
      prev ? prev.map((e) => (e.entryId !== entryId ? e : { ...e, included: !e.included })) : prev,
    )
  }

  async function handleRender() {
    if (!editable) return
    if (!SLUG_RE.test(slug)) {
      setRenderError('Name must be lowercase kebab-case, e.g. "acme-corp".')
      return
    }

    const selection: SelectedEntry[] = editable
      .filter((e) => e.included)
      .map((e) => ({
        entryId: e.entryId,
        reason: e.reason,
        bullets: e.bullets
          .filter((b) => b.included)
          .map((b) => ({ sourceIndex: b.sourceIndex, source: b.source, rewritten: b.rewritten })),
      }))
      .filter((e) => e.bullets.length > 0)

    setRenderError(null)
    setLinkError(null)
    setRendering(true)
    try {
      const result = await renderGeneration({
        slug,
        selection: { entries: selection },
        coverLetter: coverLetter !== null ? { body: coverLetter } : undefined,
        language: language ?? undefined,
        jobDescription: jobDescription.trim() || undefined,
      })
      setRender(result)

      if (jobListingId) {
        try {
          await recordApplicationGeneration(jobListingId, {
            slug: result.slug,
            cvPath: result.cvPath,
            coverLetterPath: result.coverLetterPath,
            sourceSnippetIds: result.coverLetterPath ? coverLetterSnippetIds : undefined,
            usage: usage ?? undefined,
            language: language ?? undefined,
            groundedness: groundedness ?? undefined,
            atsReports: result.atsReports,
            entryIds: selection.map((e) => e.entryId),
          })
        } catch (err) {
          setLinkError(err instanceof Error ? err.message : String(err))
        }
      }
    } catch (err) {
      setRenderError(err instanceof Error ? err.message : String(err))
    } finally {
      setRendering(false)
    }
  }

  return (
    <>
      <h1>{jobListing ? `Generate a Tailored CV for ${jobListingHeading(jobListing)}` : 'Generate a Tailored CV'}</h1>

      <section>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="job-description">Job Description (paste text)</FieldLabel>
            <Textarea
              id="job-description"
              rows={6}
              value={jobDescription}
              onChange={(e) => setJobDescription(e.target.value)}
              placeholder="Paste the job description here…"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="job-description-url">…or a URL to fetch it from</FieldLabel>
            <Input
              id="job-description-url"
              type="url"
              value={jobDescriptionUrl}
              onChange={(e) => setJobDescriptionUrl(e.target.value)}
              placeholder="https://…"
            />
          </Field>
        </FieldGroup>
        <p>Leave both blank to run Default Mode (a general-purpose CV from your most representative Entries).</p>
        <div className="mt-6 flex gap-3">
          <Button onClick={() => handleStart()} disabled={loading}>
            {loading ? 'Generating…' : 'Start Generation'}
          </Button>
          <Button variant="outline" onClick={handlePreview} disabled={previewLoading}>
            {previewLoading ? 'Previewing…' : 'Preview Selection'}
          </Button>
        </div>
      </section>

      {error && (
        <p role="alert" className="font-medium text-destructive">
          {error}
        </p>
      )}

      {previewError && (
        <p role="alert" className="font-medium text-destructive">
          {previewError}
        </p>
      )}

      {previewResult && (
        <section className="rounded-xl border-2 border-dashed border-primary/50 bg-primary/5 p-5">
          <h2 className="flex items-center gap-2">
            Selection Preview
            <span className="rounded-full bg-primary/15 px-2 py-0.5 text-xs font-semibold uppercase tracking-wide text-primary">
              Preview only — nothing saved
            </span>
          </h2>
          <p className="text-muted-foreground">
            A cheap, read-only look at which Entries and bullets Selection would pick — no Rewrite, no Cover
            Letter, no RAL Range, and nothing recorded against this Application. Start a real Generation above
            to act on it.
          </p>
          {previewResult.selection.entries.map((entry) => {
            const label = entryLabel(entriesById.get(entry.entryId), entry.entryId)
            return (
              <div key={entry.entryId} className="mt-4 rounded-lg border border-border bg-card p-4">
                <h3 className="mb-0">{label}</h3>
                <p className="mt-1 text-muted-foreground italic">{entry.reason}</p>
                <ul className="mt-2 flex flex-col gap-1">
                  {entry.bullets.map((bullet) => (
                    <li key={bullet.sourceIndex} className="border-t border-border pt-1 first:border-t-0 first:pt-0">
                      {bullet.source}
                    </li>
                  ))}
                </ul>
              </div>
            )
          })}
        </section>
      )}

      {editable && (
        <section>
          <h2>Text Review {mode === 'default' && '(Default Mode)'}</h2>
          <p>Review Selection and Rewrite before anything is rendered. Edit any bullet, or exclude one entirely.</p>

          {ral && <RALBadge ral={ral} />}
          {usage && usage.estimatedCostUsd > 0 && (
            <p className="text-sm text-muted-foreground">
              Estimated Claude API cost for this Generation: ${usage.estimatedCostUsd.toFixed(4)} (
              {usage.inputTokens + usage.outputTokens} tokens
              {usage.webSearchUses ? `, ${usage.webSearchUses} web search${usage.webSearchUses === 1 ? '' : 'es'}` : ''}
              )
            </p>
          )}

          {language && (
            <Field className="mb-4 max-w-64">
              <FieldLabel htmlFor="generation-language">Language</FieldLabel>
              <Select value={language} onValueChange={handleLanguageChange} disabled={languageChanging || loading}>
                <SelectTrigger id="generation-language" size="sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {Object.entries(LANGUAGE_LABELS).map(([code, label]) => (
                    <SelectItem key={code} value={code}>
                      {label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FieldDescription>
                {languageChanging
                  ? 'Regenerating Selection and Rewrite in the new language…'
                  : 'Correct this if the detected language is wrong — Selection and Rewrite re-run in the chosen language.'}
              </FieldDescription>
            </Field>
          )}

          {editable.map((entry) => {
            const label = entryLabel(entriesById.get(entry.entryId), entry.entryId)
            return (
              <div key={entry.entryId} className="mb-4 rounded-xl border border-border bg-card p-5">
                <h3 className="mb-0">
                  <label className="flex items-center gap-2 font-semibold">
                    <Checkbox checked={entry.included} onCheckedChange={() => toggleEntry(entry.entryId)} />
                    {label}
                  </label>
                </h3>
                <p className="mt-2 text-muted-foreground italic">{entry.reason}</p>
                {entry.included && (
                  <ul className="mt-3 flex flex-col gap-3">
                    {entry.bullets.map((bullet) => (
                      <li
                        key={bullet.sourceIndex}
                        className={cn(
                          'border-t border-border pt-2 first:border-t-0 first:pt-0',
                          !bullet.included && 'opacity-50',
                        )}
                      >
                        <label className="flex items-start gap-2">
                          <Checkbox
                            className="mt-0.5"
                            checked={bullet.included}
                            onCheckedChange={() => toggleBullet(entry.entryId, bullet.sourceIndex)}
                          />
                          <span className={cn(bullet.included ? '' : 'line-through')}>
                            <BulletDiff source={bullet.source} rewritten={bullet.rewritten} />
                            <GroundednessBadge flags={bulletFlags(groundedness, entry.entryId, bullet.sourceIndex)} />
                          </span>
                        </label>
                        {bullet.included && (
                          <Textarea
                            className="mt-2"
                            rows={2}
                            value={bullet.rewritten}
                            onChange={(e) => updateBullet(entry.entryId, bullet.sourceIndex, e.target.value)}
                          />
                        )}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )
          })}

          {coverLetter !== null && (
            <div className="mb-4 rounded-xl border border-border bg-card p-5">
              <h3>
                Cover Letter
                <GroundednessBadge flags={groundedness?.coverLetter ?? []} />
              </h3>
              <Textarea rows={12} value={coverLetter} onChange={(e) => setCoverLetter(e.target.value)} />
            </div>
          )}

          <Field>
            <FieldLabel htmlFor="generation-slug">Name this Generation</FieldLabel>
            <Input
              id="generation-slug"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              placeholder="acme-corp"
              aria-describedby="generation-slug-hint"
            />
            <FieldDescription id="generation-slug-hint">
              Lowercase kebab-case, e.g. "acme-corp". Reusing a name is fine: each render is saved to its own
              folder, named with this prefix plus a UTC timestamp, so earlier renders are never overwritten.
            </FieldDescription>
          </Field>

          {renderError && (
            <p role="alert" className="font-medium text-destructive">
              {renderError}
            </p>
          )}

          <div className="mt-6 flex gap-3">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button onClick={handleRender} disabled={rendering}>
                  {rendering ? 'Rendering…' : render ? 'Re-render' : 'Approve Text Review'}
                </Button>
              </TooltipTrigger>
              <TooltipContent>Renders the Tailored CV PDF from your edits above</TooltipContent>
            </Tooltip>
          </div>
        </section>
      )}

      {render && (
        <section>
          <h2>Visual Review</h2>
          {jobListingId && !linkError && (
            <p>
              Linked to <strong>{jobListing ? jobListingHeading(jobListing) : jobListingId}</strong>'s Application.
            </p>
          )}
          {linkError && (
            <p role="alert" className="font-medium text-destructive">
              Rendered, but failed to link this Generation to the Application: {linkError}
            </p>
          )}
          {render.cvPageCount !== 1 && (
            <p role="alert" className="font-medium text-destructive">
              The CV rendered to {render.cvPageCount} pages — it should be one. Trim a bullet or Entry above and
              re-render.
            </p>
          )}
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <ParsabilityBadge result={render.atsReports.cv} label="CV" />
            {render.atsReports.coverLetter && (
              <ParsabilityBadge result={render.atsReports.coverLetter} label="Cover Letter" />
            )}
            <Link to={atsReportPath(render.slug)} target="_blank" className="text-sm">
              Open the ATS Report ↗
            </Link>
          </div>
          <iframe
            className="block h-[min(800px,75vh)] w-full rounded-xl border border-border"
            title="Tailored CV preview"
            src={generationFileUrl(render.slug, 'cv.pdf')}
          />
          <p>
            <a href={generationFileUrl(render.slug, 'cv.pdf')} download={`${render.slug}-cv.pdf`}>
              Download CV (PDF)
            </a>
            {render.coverLetterPath && (
              <>
                {' · '}
                <a href={generationFileUrl(render.slug, 'cover-letter.pdf')} download={`${render.slug}-cover-letter.pdf`}>
                  Download Cover Letter (PDF)
                </a>
                {' · '}
                <a href={generationFileUrl(render.slug, 'cover-letter.txt')} download={`${render.slug}-cover-letter.txt`}>
                  Download Cover Letter (Text)
                </a>
              </>
            )}
          </p>
        </section>
      )}
    </>
  )
}
