import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import GenerationsListPage from './GenerationsListPage'
import type { IndexedGeneration } from '@/api/types'
import { renderPage } from '@/test/render'
import { server } from '@/test/server'

function showGenerations(generations: IndexedGeneration[]) {
  server.use(http.get('/api/generations', () => HttpResponse.json(generations)))
  return renderPage(<GenerationsListPage />, { at: '/generations', pattern: '/generations' })
}

function recorded(overrides: Partial<IndexedGeneration> = {}): IndexedGeneration {
  return {
    slug: 'acme-corp-20260911-143022',
    createdAt: '2026-09-11T14:30:22Z',
    recorded: true,
    applicationId: 'acme-corp',
    company: 'Acme Corp',
    jobTitle: 'Senior Data Engineer',
    cvPath: 'output/acme-corp-20260911-143022/cv.pdf',
    language: 'en',
    hasCv: true,
    hasCoverLetter: true,
    hasAtsReport: false,
    ...overrides,
  }
}

function unrecorded(overrides: Partial<IndexedGeneration> = {}): IndexedGeneration {
  return {
    slug: 'default-20260916-062819',
    createdAt: '2026-09-16T06:28:19Z',
    recorded: false,
    hasCv: true,
    hasCoverLetter: false,
    hasAtsReport: false,
    ...overrides,
  }
}

// The page answers "what have I generated so far" (issue #173) across both
// halves of the index: Generations recorded against an Application, and
// output directories nothing ever recorded.
describe('GenerationsListPage', () => {
  it('GenerationsListPage_RecordedGeneration_LinksToItsApplication', async () => {
    showGenerations([recorded()])

    const row = await screen.findByRole('listitem')
    const link = within(row).getByRole('link', { name: /Acme Corp/ })
    expect(link).toHaveAttribute('href', '/jobs/acme-corp')
    expect(link).toHaveTextContent('Senior Data Engineer')
    expect(within(row).getByRole('link', { name: 'CV' })).toHaveAttribute(
      'href',
      '/api/generations/acme-corp-20260911-143022/cv.pdf',
    )
    expect(within(row).getByRole('link', { name: 'Cover Letter' })).toBeInTheDocument()
  })

  it('GenerationsListPage_UnrecordedGeneration_IsShownAsDefaultModeAndNotTracked', async () => {
    showGenerations([unrecorded()])

    const row = await screen.findByRole('listitem')
    expect(row).toHaveTextContent('Default Mode')
    expect(row).toHaveTextContent('Not tracked')
    expect(within(row).getByRole('link', { name: 'CV' })).toBeInTheDocument()
    expect(within(row).queryByRole('link', { name: 'Cover Letter' })).not.toBeInTheDocument()
  })

  // Records may outlive their files: the row stays, without a dead link.
  it('GenerationsListPage_RecordWithNoFilesLeft_SaysSoInsteadOfOfferingADeadLink', async () => {
    showGenerations([recorded({ hasCv: false, hasCoverLetter: false })])

    const row = await screen.findByRole('listitem')
    expect(row).toHaveTextContent('CV file missing')
    expect(within(row).queryByRole('link', { name: 'CV' })).not.toBeInTheDocument()
  })

  // An output directory whose name carries no timestamp has no date to show,
  // and the mtime would be a different fact.
  it('GenerationsListPage_GenerationWithNoDate_SaysUnknownRatherThanInventingOne', async () => {
    showGenerations([unrecorded({ slug: 'hand-made', createdAt: '' })])

    expect(await screen.findByText('Unknown date')).toBeInTheDocument()
  })

  // Deleting clears output/<slug>/. The page re-reads the index rather than
  // guessing what is left: an unrecorded Generation disappears, a recorded
  // one stays with its files missing.
  it('GenerationsListPage_Delete_AsksFirstThenDeletesAndReloads', async () => {
    const user = userEvent.setup()
    let deleted: string | null = null
    server.use(
      http.delete('/api/generations/:slug', ({ params }) => {
        deleted = String(params.slug)
        return new HttpResponse(null, { status: 204 })
      }),
      http.get('/api/generations', () => HttpResponse.json(deleted ? [] : [unrecorded()])),
    )
    renderPage(<GenerationsListPage />, { at: '/generations', pattern: '/generations' })

    await user.click(await screen.findByRole('button', { name: 'Delete files' }))
    expect(screen.getByText('Delete the files?')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    expect(await screen.findByText(/Nothing generated yet/)).toBeInTheDocument()
    expect(deleted).toBe('default-20260916-062819')
  })

  it('GenerationsListPage_DeleteCancelled_DeletesNothing', async () => {
    const user = userEvent.setup()
    let called = false
    server.use(
      http.delete('/api/generations/:slug', () => {
        called = true
        return new HttpResponse(null, { status: 204 })
      }),
    )
    showGenerations([unrecorded()])

    await user.click(await screen.findByRole('button', { name: 'Delete files' }))
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(screen.getByRole('button', { name: 'Delete files' })).toBeInTheDocument()
    expect(called).toBe(false)
  })

  // A recorded Generation keeps its record, so the confirmation says so
  // rather than implying the whole Generation is being erased.
  it('GenerationsListPage_DeleteOnARecordedGeneration_SaysTheRecordStays', async () => {
    const user = userEvent.setup()
    showGenerations([recorded()])

    await user.click(await screen.findByRole('button', { name: 'Delete files' }))

    expect(screen.getByText('Delete the files? The record stays.')).toBeInTheDocument()
  })

  // Nothing to delete: the files are already gone.
  it('GenerationsListPage_RecordWithNoFiles_OffersNoDeleteButton', async () => {
    showGenerations([recorded({ hasCv: false, hasCoverLetter: false })])

    await screen.findByRole('listitem')
    expect(screen.queryByRole('button', { name: 'Delete files' })).not.toBeInTheDocument()
  })

  it('GenerationsListPage_NothingGeneratedYet_PointsAtTheGeneratePage', async () => {
    showGenerations([])

    expect(await screen.findByText(/Nothing generated yet/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Generate' })).toHaveAttribute('href', '/generate')
  })
})
