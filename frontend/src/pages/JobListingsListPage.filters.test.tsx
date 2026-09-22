import { describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import JobListingsListPage from './JobListingsListPage'
import { listingSummaryWithApplication } from '@/test/fixtures'
import { currentSearch, renderPage } from '@/test/render'
import { recordedRequests, server } from '@/test/server'

// The filters' whole point is the query the backend actually receives, so
// these tests assert on the query string @/api/client built — unmocked —
// rather than on the arguments it was called with.
function showList() {
  server.use(
    http.get('/api/job-listings', () =>
      HttpResponse.json([listingSummaryWithApplication({ id: 'acme', company: 'Acme' })]),
    ),
  )
}

async function lastListQuery(): Promise<string> {
  const listRequests = (await recordedRequests()).filter((r) => r.path === '/api/job-listings')
  return listRequests[listRequests.length - 1].search
}

async function listRequestCount(): Promise<number> {
  return (await recordedRequests()).filter((r) => r.path === '/api/job-listings').length
}

function open(at = '/jobs') {
  showList()
  return renderPage(<JobListingsListPage />, { at, pattern: '/jobs' })
}

async function chooseFromSelect(user: UserEvent, selectName: string, option: string) {
  await user.click(await screen.findByRole('combobox', { name: selectName }))
  await user.click(await screen.findByRole('option', { name: option }))
}

describe('Job Listing filters', () => {
  it('StatusFilter_Applied_SendsItAsTheBackendsStatusQueryParam', async () => {
    const { user } = open()
    await screen.findByText('Acme')

    await chooseFromSelect(user, 'Status', 'Interviewing')

    await waitFor(async () => expect(await lastListQuery()).toBe('?status=interviewing'))
  })

  it('CompanyFilter_Typed_SendsItAsTheBackendsCompanyQueryParam', async () => {
    const { user } = open()
    await screen.findByText('Acme')

    await user.type(screen.getByLabelText('Company'), 'acme')

    await waitFor(async () => expect(await lastListQuery()).toBe('?company=acme'))
  })

  it('SavedDateFilters_Applied_SendThemAsSavedFromAndSavedToQueryParams', async () => {
    const { user } = open()
    await screen.findByText('Acme')

    await user.type(screen.getByLabelText('Saved from'), '2026-01-01')
    await user.type(screen.getByLabelText('Saved to'), '2026-02-01')

    await waitFor(async () =>
      expect(await lastListQuery()).toBe('?savedFrom=2026-01-01&savedTo=2026-02-01'),
    )
  })

  it('Filters_Applied_AreReflectedInTheUrlSoTheViewCanBeBookmarked', async () => {
    const { user } = open()
    await screen.findByText('Acme')

    await chooseFromSelect(user, 'Status', 'Sent')

    await waitFor(() => expect(currentSearch()).toBe('?status=sent'))
  })

  it('Filters_RestoredFromTheUrl_AreSentOnTheFirstRequest', async () => {
    open('/jobs?status=sent&company=acme')

    await screen.findByText('Acme')
    expect(await lastListQuery()).toBe('?status=sent&company=acme')
  })

  it('ClearFilters_Clicked_DropsEveryFilterFromTheRequestAndTheUrl', async () => {
    const { user } = open('/jobs?status=sent&company=acme')
    await screen.findByText('Acme')

    await user.click(screen.getByRole('button', { name: 'Clear filters' }))

    await waitFor(async () => expect(await lastListQuery()).toBe(''))
    expect(currentSearch()).toBe('')
  })
})

describe('RAL Range sort and filter', () => {
  // Typing a bound must not fire a request per keystroke: the RAL filter is
  // applied explicitly, unlike the URL-backed filters above.
  it('RALFilter_BoundsTypedButNotApplied_SendsNoFurtherRequest', async () => {
    const { user } = open()
    await screen.findByText('Acme')
    const before = await listRequestCount()

    await user.type(screen.getByLabelText('Min RAL'), '40000')
    await user.type(screen.getByLabelText('Max RAL'), '60000')

    expect(await listRequestCount()).toBe(before)
  })

  it('RALFilter_Applied_SendsTheBoundsAndCurrencyAsQueryParams', async () => {
    const { user } = open()
    await screen.findByText('Acme')

    await user.type(screen.getByLabelText('Min RAL'), '40000')
    await user.type(screen.getByLabelText('Max RAL'), '60000')
    await user.click(screen.getByRole('button', { name: 'Apply RAL filter' }))

    await waitFor(async () =>
      expect(await lastListQuery()).toBe('?ral_min=40000&ral_max=60000&ral_currency=EUR'),
    )
  })

  it('RALFilter_MinAboveMax_ReportsItLocallyAndSendsNothing', async () => {
    const { user } = open()
    await screen.findByText('Acme')
    const before = await listRequestCount()

    await user.type(screen.getByLabelText('Min RAL'), '60000')
    await user.type(screen.getByLabelText('Max RAL'), '40000')
    await user.click(screen.getByRole('button', { name: 'Apply RAL filter' }))

    expect(await screen.findByText('Min RAL must not exceed Max RAL.')).toBeInTheDocument()
    expect(await listRequestCount()).toBe(before)
  })

  it('RALSort_Applied_SendsTheSortAndOrderTheBackendUnderstands', async () => {
    const { user } = open()
    await screen.findByText('Acme')

    await chooseFromSelect(user, 'Sort by RAL Range', 'RAL: high to low')

    await waitFor(async () => expect(await lastListQuery()).toBe('?sort=ral&order=desc'))
  })

  it('RALFilterCleared_AfterBeingApplied_ReturnsToTheUnfilteredRequest', async () => {
    const { user } = open()
    await screen.findByText('Acme')

    await user.type(screen.getByLabelText('Min RAL'), '40000')
    await user.click(screen.getByRole('button', { name: 'Apply RAL filter' }))
    await waitFor(async () => expect(await lastListQuery()).toContain('ral_min=40000'))

    await user.click(screen.getByRole('button', { name: 'Clear RAL filter' }))

    await waitFor(async () => expect(await lastListQuery()).toBe(''))
    expect(currentSearch()).toBe('')
  })

  // The RAL sort and applied bounds share the URL with the other filters, so
  // returning from a Job Listing's own page restores them too (issue #94).
  it('RALFilterAndSort_Applied_AreReflectedInTheUrlSoTheViewSurvivesADetour', async () => {
    const { user } = open()
    await screen.findByText('Acme')

    await chooseFromSelect(user, 'Sort by RAL Range', 'RAL: low to high')
    await user.type(screen.getByLabelText('Min RAL'), '40000')
    await user.type(screen.getByLabelText('Max RAL'), '60000')
    await user.click(screen.getByRole('button', { name: 'Apply RAL filter' }))

    await waitFor(() =>
      expect(currentSearch()).toBe('?ralSort=asc&ralMin=40000&ralMax=60000&ralCurrency=EUR'),
    )
  })

  it('RALFilterAndSort_RestoredFromTheUrl_AreSentOnTheFirstRequestAndShownInTheInputs', async () => {
    open('/jobs?status=sent&ralSort=desc&ralMin=40000&ralCurrency=EUR')

    await screen.findByText('Acme')
    expect(await lastListQuery()).toBe(
      `?status=sent&sort=ral&order=desc&ral_min=40000&ral_max=${Number.MAX_SAFE_INTEGER}&ral_currency=EUR`,
    )
    expect(screen.getByLabelText('Min RAL')).toHaveValue(40000)
    expect(screen.getByRole('button', { name: 'Clear RAL filter' })).toBeInTheDocument()
  })

  it('ClearFilters_WithARALFilterApplied_LeavesTheRALFilterInPlace', async () => {
    const { user } = open('/jobs?status=sent&ralMin=40000&ralCurrency=EUR')
    await screen.findByText('Acme')

    await user.click(screen.getByRole('button', { name: 'Clear filters' }))

    await waitFor(() => expect(currentSearch()).toBe('?ralMin=40000&ralCurrency=EUR'))
  })
})

// Filtering by where the role is (issue #206, story 58): everything in one
// place, or everything remote.
describe('Location filter', () => {
  it('LocationFilter_Typed_IsSentAndWrittenToTheUrl', async () => {
    const { user } = open()
    await screen.findByText('Acme')

    await user.type(screen.getByLabelText('Location'), 'Milan')

    await waitFor(() => expect(currentSearch()).toBe('?location=Milan'))
    await waitFor(async () => expect(await lastListQuery()).toBe('?location=Milan'))
  })

  it('LocationFilter_RestoredFromTheUrl_IsSentOnTheFirstRequestAndShownInTheInput', async () => {
    open('/jobs?location=Remote')

    await screen.findByText('Acme')
    expect(await lastListQuery()).toBe('?location=Remote')
    expect(screen.getByLabelText('Location')).toHaveValue('Remote')
  })

  it('ClearFilters_WithALocationFilter_ClearsItToo', async () => {
    const { user } = open('/jobs?location=Milan&status=sent')
    await screen.findByText('Acme')

    await user.click(screen.getByRole('button', { name: 'Clear filters' }))

    await waitFor(() => expect(currentSearch()).toBe(''))
  })
})
