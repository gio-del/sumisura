import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
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

  it('GenerationsListPage_NothingGeneratedYet_PointsAtTheGeneratePage', async () => {
    showGenerations([])

    expect(await screen.findByText(/Nothing generated yet/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Generate' })).toHaveAttribute('href', '/generate')
  })
})
