import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import type { UsageReport } from '@/api/types'
import UsagePage from '@/pages/UsagePage'
import { renderPage } from '@/test/render'
import { server } from '@/test/server'

function showUsage(report: UsageReport) {
  server.use(http.get('/api/master-data/usage', () => HttpResponse.json(report)))
  return renderPage(<UsagePage />, { at: '/usage', pattern: '/usage' })
}

describe('UsagePage', () => {
  it('UsagePage_ListsNeverChosenFirstAndFlipsOnDemand', async () => {
    showUsage({
      countedGenerations: 4,
      legacySkipped: 1,
      entries: [
        { id: 'projects/x', label: 'Never Co', generations: 0, share: 0 },
        { id: 'experience/y', label: 'Often Co', generations: 2, share: 0.5, lastUsedAt: '2025-03-01T00:00:00Z' },
      ],
      snippets: [],
    })

    const entries = await screen.findByRole('region', { name: 'Entries' })
    expect(within(entries).getAllByRole('link').map((l) => l.textContent)).toEqual(['Never Co', 'Often Co'])
    expect(within(entries).getByText('Never chosen')).toBeInTheDocument()
    expect(within(entries).getByText(/2 Generations \(50%\)/)).toBeInTheDocument()
    expect(screen.getByText(/1 older Generation recorded without usage data is left out/)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Least used first' }))
    expect(within(entries).getAllByRole('link').map((l) => l.textContent)).toEqual(['Often Co', 'Never Co'])
  })

  it('UsagePage_ShowsAnEmptyTextWithoutGenerations', async () => {
    showUsage({
      countedGenerations: 0,
      legacySkipped: 0,
      entries: [{ id: 'projects/x', label: 'Never Co', generations: 0, share: 0 }],
      snippets: [],
    })
    expect(await screen.findByText('No Generations to measure yet.')).toBeInTheDocument()
    expect(screen.queryByText(/NaN/)).not.toBeInTheDocument()
  })
})
