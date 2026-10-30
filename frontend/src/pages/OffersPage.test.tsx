import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import OffersPage from './OffersPage'
import type { Offer } from '@/api/types'
import { renderPage } from '@/test/render'
import { server } from '@/test/server'

function offer(id: string, overrides: Partial<Offer> = {}): Offer {
  return {
    id,
    company: id,
    ral: { source: 'stated', min: 40000, max: 50000, currency: 'EUR' },
    offeredAt: '2026-02-01T10:00:00Z',
    notes: [],
    ...overrides,
  }
}

function showOffers(offers: Offer[]) {
  server.use(http.get('/api/applications/offers', () => HttpResponse.json(offers)))
  return renderPage(<OffersPage />, { at: '/offers', pattern: '/offers' })
}

function companiesIn(region: HTMLElement): string[] {
  return within(region)
    .getAllByRole('link')
    .map((a) => a.textContent ?? '')
}

describe('OffersPage', () => {
  it('OffersPage_NoOffers_ShowsEmptyStateNotAnError', async () => {
    showOffers([])
    expect(await screen.findByText(/No offers yet/)).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('OffersPage_Loaded_SortsByHighestRalThenByDate', async () => {
    showOffers([
      offer('low', { ral: { source: 'stated', min: 30000, max: 40000, currency: 'EUR' }, offeredAt: '2026-02-01T10:00:00Z' }),
      offer('high', { ral: { source: 'stated', min: 60000, max: 70000, currency: 'EUR' }, offeredAt: '2026-02-02T10:00:00Z' }),
      offer('mid', { ral: { source: 'stated', min: 40000, max: 50000, currency: 'EUR' }, offeredAt: '2026-03-01T10:00:00Z' }),
    ])
    const eur = await screen.findByRole('region', { name: 'EUR' })
    expect(companiesIn(eur)).toEqual(['high', 'mid', 'low'])

    const select = screen.getByLabelText('Sort by')
    await userEvent.selectOptions(select, 'date')
    expect(companiesIn(screen.getByRole('region', { name: 'EUR' }))).toEqual(['mid', 'high', 'low'])
  })

  it('OffersPage_MixedCurrencies_GroupsThemAndSaysTheyAreNotComparable', async () => {
    showOffers([
      offer('eur', { ral: { source: 'stated', min: 1, max: 100, currency: 'EUR' } }),
      offer('usd', { ral: { source: 'stated', min: 1, max: 900000, currency: 'USD' } }),
    ])
    expect(await screen.findByRole('region', { name: 'EUR' })).toBeInTheDocument()
    expect(companiesIn(screen.getByRole('region', { name: 'EUR' }))).toEqual(['eur'])
    expect(companiesIn(screen.getByRole('region', { name: 'USD' }))).toEqual(['usd'])
    expect(screen.getByRole('note')).toHaveTextContent(/not converted and not comparable/)
  })

  it('OffersPage_Conflict_ShowsBothDeclaredFigures', async () => {
    showOffers([
      offer('c', {
        ral: {
          source: 'conflict',
          descriptionStated: { min: 40000, max: 50000, currency: 'EUR' },
          listingStated: { min: 70000, max: 80000, currency: 'EUR' },
        },
      }),
    ])
    expect(await screen.findByText(/Job Description: EUR 40,000/)).toBeInTheDocument()
    expect(screen.getByText(/Listing: EUR 70,000/)).toBeInTheDocument()
  })
})
