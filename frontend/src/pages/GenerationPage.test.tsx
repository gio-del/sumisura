import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import GenerationPage from './GenerationPage'
import type { Application, GenerateResult, RenderResult, SelectedEntry } from '@/api/types'
import {
  application,
  generateResult,
  generationEntries,
  listingWithApplication,
  renderResult,
} from '@/test/fixtures'
import { renderPage } from '@/test/render'
import { recordedRequests, requestsTo, server } from '@/test/server'

const RENDER_PATH = '/api/generations/render'
const RECORD_PATH = '/api/applications/acme/generations'

interface Backend {
  generate?: GenerateResult
  render?: RenderResult | { error: string }
  record?: Application | { error: string }
}

/**
 * standUp declares the endpoints the page touches. Every response is faked at
 * the HTTP boundary and nothing below it, so the real @/api/client assembles
 * the paths and bodies asserted on below.
 */
function standUp(backend: Backend = {}) {
  server.use(
    http.get('/api/master-data/entries', () => HttpResponse.json(generationEntries())),
    // The detail endpoint answers with the {jobListing, application} pair
    // (issue #94); the Generation page reads its Job Listing out of it.
    http.get('/api/job-listings/acme', () =>
      HttpResponse.json(listingWithApplication({ company: 'Acme' })),
    ),
    http.post('/api/generations', () => HttpResponse.json(backend.generate ?? generateResult())),
    http.post(RENDER_PATH, () => {
      const result = backend.render ?? renderResult()
      return 'error' in result ? new HttpResponse(result.error, { status: 500 }) : HttpResponse.json(result)
    }),
    http.post(RECORD_PATH, () => {
      const result = backend.record ?? application()
      return 'error' in result ? new HttpResponse(result.error, { status: 500 }) : HttpResponse.json(result)
    }),
  )
}

/** openTextReview runs a Generation and waits for the Text Review checkpoint. */
async function openTextReview(options: { tracked?: boolean } = {}) {
  const tracked = options.tracked ?? true
  const page = tracked
    ? renderPage(<GenerationPage />, { at: '/jobs/acme/generate', pattern: '/jobs/:id/generate' })
    : renderPage(<GenerationPage />, { at: '/generate', pattern: '/generate' })
  await page.user.click(await screen.findByRole('button', { name: 'Start Generation' }))
  await screen.findByRole('heading', { name: /Text Review/ })
  return page
}

async function approveTextReview(user: UserEvent) {
  await user.click(screen.getByRole('button', { name: 'Approve Text Review' }))
}

/** renderedSelection is the Selection the app actually sent to Render. */
async function renderedSelection(): Promise<SelectedEntry[]> {
  const [request] = await requestsTo(RENDER_PATH)
  return (request.body as { selection: { entries: SelectedEntry[] } }).selection.entries
}

async function recordedBody(): Promise<Record<string, unknown>> {
  const [request] = await requestsTo(RECORD_PATH)
  return request.body as Record<string, unknown>
}

describe('Text Review decisions reaching Render', () => {
  it('Render_InvalidGenerationName_RejectsItLocallyWithoutSendingAnything', async () => {
    standUp()
    const { user } = await openTextReview()

    await user.clear(screen.getByLabelText('Name this Generation'))
    await user.type(screen.getByLabelText('Name this Generation'), 'Acme Corp')
    await approveTextReview(user)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Name must be lowercase kebab-case, e.g. "acme-corp".',
    )
    expect(await requestsTo(RENDER_PATH)).toHaveLength(0)
  })

  it('Render_EntryExcludedAtTextReview_LeavesThatEntryOutOfWhatIsRendered', async () => {
    standUp()
    const { user } = await openTextReview()

    await user.click(screen.getByRole('checkbox', { name: 'Pathfinder' }))
    await approveTextReview(user)

    await screen.findByRole('heading', { name: 'Visual Review' })
    expect((await renderedSelection()).map((e) => e.entryId)).toEqual(['globex-backend'])
  })

  it('Render_BulletExcludedAtTextReview_LeavesThatBulletOutOfWhatIsRendered', async () => {
    standUp()
    const { user } = await openTextReview()

    await user.click(screen.getByRole('checkbox', { name: 'Owned deploys end to end.' }))
    await approveTextReview(user)

    await screen.findByRole('heading', { name: 'Visual Review' })
    const selection = await renderedSelection()
    expect(selection[0].bullets.map((b) => b.rewritten)).toEqual(['Built a Go HTTP API.'])
    expect(selection[1].bullets).toHaveLength(2)
  })

  it('Render_EveryBulletOfAnEntryExcluded_DropsThatEntryEntirely', async () => {
    standUp()
    const { user } = await openTextReview()

    await user.click(screen.getByRole('checkbox', { name: 'Wrote a route planner.' }))
    await user.click(screen.getByRole('checkbox', { name: 'Published it as open source.' }))
    await approveTextReview(user)

    await screen.findByRole('heading', { name: 'Visual Review' })
    expect((await renderedSelection()).map((e) => e.entryId)).toEqual(['globex-backend'])
  })

  it('Render_EditedBullet_SendsTheCorrectedTextRatherThanTheRewrite', async () => {
    standUp()
    const { user } = await openTextReview()

    const editor = screen.getAllByRole('textbox').find((t) => (t as HTMLTextAreaElement).value === 'Built a Go HTTP API.')!
    await user.clear(editor)
    await user.type(editor, 'Built a Go HTTP API serving 2k rps.')
    await approveTextReview(user)

    await screen.findByRole('heading', { name: 'Visual Review' })
    const selection = await renderedSelection()
    expect(selection[0].bullets[0].rewritten).toBe('Built a Go HTTP API serving 2k rps.')
  })
})

describe('Recording the Generation against an Application', () => {
  // The Generate -> Text Review -> Render -> record order is the pipeline
  // CONTEXT.md describes; asserting it as a sequence is what stops a refactor
  // quietly reordering or dropping a step.
  it('Generation_TrackedApplication_RunsGenerateThenRenderThenRecordInThatOrder', async () => {
    standUp()
    const { user } = await openTextReview()

    await approveTextReview(user)
    await screen.findByRole('heading', { name: 'Visual Review' })

    const posts = (await recordedRequests()).filter((r) => r.method === 'POST').map((r) => r.path)
    expect(posts).toEqual(['/api/generations', RENDER_PATH, RECORD_PATH])
  })

  it('Generation_DefaultMode_RecordsNothingAgainstAnyApplication', async () => {
    standUp()
    const { user } = await openTextReview({ tracked: false })

    await approveTextReview(user)
    await screen.findByRole('heading', { name: 'Visual Review' })

    const posts = (await recordedRequests()).filter((r) => r.method === 'POST').map((r) => r.path)
    expect(posts).toEqual(['/api/generations', RENDER_PATH])
  })

  it('RecordedGeneration_SuccessfulRender_CarriesTheEntryIdsSelectionChose', async () => {
    standUp()
    const { user } = await openTextReview()

    await user.click(screen.getByRole('checkbox', { name: 'Pathfinder' }))
    await approveTextReview(user)
    await screen.findByRole('heading', { name: 'Visual Review' })

    expect(await recordedBody()).toMatchObject({
      slug: 'acme',
      cvPath: 'output/acme/cv.pdf',
      entryIds: ['globex-backend'],
    })
  })

  it('RecordedGeneration_SuccessfulRender_CarriesUsageLanguageAndGroundedness', async () => {
    standUp({
      generate: generateResult({
        language: 'it',
        groundedness: { bullets: [{ entryId: 'globex-backend', sourceIndex: 0, flags: [] }] },
      }),
    })
    const { user } = await openTextReview()

    await approveTextReview(user)
    await screen.findByRole('heading', { name: 'Visual Review' })

    expect(await recordedBody()).toMatchObject({
      language: 'it',
      usage: { inputTokens: 1200, outputTokens: 340, estimatedCostUsd: 0.0421 },
      groundedness: { bullets: [{ entryId: 'globex-backend', sourceIndex: 0, flags: [] }] },
    })
  })

  // Issue #198: the ATS Report Render returned is kept on the record, and
  // Visual Review links to its full view.
  it('RecordedGeneration_SuccessfulRender_CarriesTheATSReports', async () => {
    const atsReports = { cv: { status: 'warning' as const, missingFields: ['Phone'], extractedText: 'Jane Doe' } }
    standUp({ render: renderResult({ atsReports }) })
    const { user } = await openTextReview()

    await approveTextReview(user)
    await screen.findByRole('heading', { name: 'Visual Review' })

    expect(await recordedBody()).toMatchObject({ atsReports })
    expect(screen.getByRole('link', { name: 'Open the ATS Report ↗' })).toHaveAttribute('href', '/generations/acme/ats')
  })

  // sourceSnippetIds records which Cover Letter Snippets were used. A run
  // that produced no Cover Letter must not contribute to that tracking —
  // absent has to keep meaning "no Snippet used", not "we did not check".
  it('RecordedGeneration_CoverLetterProduced_CarriesItsSourceSnippetIds', async () => {
    standUp({
      generate: generateResult({
        coverLetter: { body: 'Dear Acme,', sourceSnippetIds: ['opening-warm', 'closing-standard'] },
      }),
      render: renderResult({ coverLetterPath: 'output/acme/cover-letter.pdf' }),
    })
    const { user } = await openTextReview()

    await approveTextReview(user)
    await screen.findByRole('heading', { name: 'Visual Review' })

    expect(await recordedBody()).toMatchObject({
      coverLetterPath: 'output/acme/cover-letter.pdf',
      sourceSnippetIds: ['opening-warm', 'closing-standard'],
    })
  })

  it('RecordedGeneration_NoCoverLetterRendered_OmitsSourceSnippetIdsEntirely', async () => {
    standUp({
      generate: generateResult({
        coverLetter: { body: 'Dear Acme,', sourceSnippetIds: ['opening-warm'] },
      }),
      render: renderResult(),
    })
    const { user } = await openTextReview()

    await approveTextReview(user)
    await screen.findByRole('heading', { name: 'Visual Review' })

    expect(await recordedBody()).not.toHaveProperty('sourceSnippetIds')
  })

  it('Generation_RenderFails_SurfacesTheErrorAndRecordsNothing', async () => {
    standUp({ render: { error: 'typst exited with status 1' } })
    const { user } = await openTextReview()

    await approveTextReview(user)

    expect(await screen.findByRole('alert')).toHaveTextContent('typst exited with status 1')
    expect(screen.queryByRole('heading', { name: 'Visual Review' })).not.toBeInTheDocument()
    expect(await requestsTo(RECORD_PATH)).toHaveLength(0)
  })

  // A bookkeeping failure must never cost the user the PDF they waited for.
  it('Generation_RecordFails_KeepsTheRenderedPdfOnScreenAndSaysRecordingFailed', async () => {
    standUp({ record: { error: 'application not found' } })
    const { user } = await openTextReview()

    await approveTextReview(user)

    expect(await screen.findByRole('heading', { name: 'Visual Review' })).toBeInTheDocument()
    expect(screen.getByTitle('Tailored CV preview')).toHaveAttribute(
      'src',
      '/api/generations/acme/cv.pdf',
    )
    expect(screen.getByRole('link', { name: 'Download CV (PDF)' })).toBeInTheDocument()
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Rendered, but failed to link this Generation to the Application: application not found',
    )
  })
})
