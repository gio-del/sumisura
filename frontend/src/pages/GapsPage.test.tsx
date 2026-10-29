import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import type { GapsReport } from '@/api/types'
import GapsPage from '@/pages/GapsPage'
import { renderPage } from '@/test/render'
import { server } from '@/test/server'

function showGaps(report: GapsReport) {
  server.use(http.get('/api/master-data/gaps', () => HttpResponse.json(report)))
  return renderPage(<GapsPage />, { at: '/gaps', pattern: '/gaps' })
}

describe('GapsPage', () => {
  it('GapsPage_ListsUncoveredTermsBeforeCoveredOnesWithLinksToTheListings', async () => {
    showGaps({
      gaps: [
        { term: 'terraform', count: 2, listingIds: ['alpha', 'beta'], covered: false },
        { term: 'go', count: 3, listingIds: ['alpha', 'beta', 'gamma'], covered: true },
      ],
    })

    const uncovered = await screen.findByRole('region', { name: 'Not covered' })
    const covered = screen.getByRole('region', { name: 'Covered' })
    expect(within(uncovered).getByText('terraform')).toBeInTheDocument()
    expect(within(uncovered).getByRole('link', { name: 'alpha' })).toHaveAttribute('href', '/jobs/alpha')
    expect(within(covered).getByText('go')).toBeInTheDocument()
    expect(uncovered.compareDocumentPosition(covered) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('GapsPage_ShowsAnEmptyStateWithoutRecurringTerms', async () => {
    showGaps({ gaps: [] })
    expect(await screen.findByText('No recurring terms yet.')).toBeInTheDocument()
  })
})
