import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import CoverageSection from './CoverageSection'
import { renderPage } from '@/test/render'
import { requestsTo, server } from '@/test/server'

const PATH = '/api/job-listings/acme/coverage'

describe('CoverageSection', () => {
  it('CoverageSection_Closed_MakesNoRequest', async () => {
    renderPage(<CoverageSection jobListingId="acme" />)
    expect(await screen.findByRole('button', { name: 'Show coverage' })).toBeInTheDocument()
    expect(await requestsTo(PATH)).toEqual([])
  })

  it('CoverageSection_Opened_ListsCoveredTermsWithEntriesAndUncoveredOnesWithAnAddLink', async () => {
    server.use(
      http.get(PATH, () =>
        HttpResponse.json({ covered: [{ term: 'go', entryIds: ['experience/a', 'projects/b'] }], uncovered: ['terraform'] }),
      ),
    )
    const { user } = renderPage(<CoverageSection jobListingId="acme" />)
    await user.click(await screen.findByRole('button', { name: 'Show coverage' }))

    expect(await screen.findByText('terraform')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'experience/a' })).toHaveAttribute('href', '/entries/experience/a')
    expect(screen.getByRole('link', { name: 'projects/b' })).toHaveAttribute('href', '/entries/projects/b')
    expect(screen.getByRole('link', { name: 'Add an Entry' })).toHaveAttribute('href', '/entries/new')
  })

  it('CoverageSection_NothingToCompare_ShowsAnEmptyStateNotAnError', async () => {
    server.use(http.get(PATH, () => HttpResponse.json({ covered: [], uncovered: [] })))
    const { user } = renderPage(<CoverageSection jobListingId="acme" />)
    await user.click(await screen.findByRole('button', { name: 'Show coverage' }))

    expect(await screen.findByText(/no Job Description/)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('CoverageSection_RequestFails_ShowsTheError', async () => {
    server.use(http.get(PATH, () => new HttpResponse('boom', { status: 500 })))
    const { user } = renderPage(<CoverageSection jobListingId="acme" />)
    await user.click(await screen.findByRole('button', { name: 'Show coverage' }))

    expect(await screen.findByRole('alert')).toBeInTheDocument()
  })
})
