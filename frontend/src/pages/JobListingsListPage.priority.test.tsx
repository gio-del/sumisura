import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import type { UserEvent } from '@testing-library/user-event'
import JobListingsListPage from './JobListingsListPage'
import type { JobListingSummaryWithApplication } from '@/api/types'
import { JOB_LISTING_VERSION, jobListing, listingSummaryWithApplication } from '@/test/fixtures'
import { currentSearch, renderPage } from '@/test/render'
import { requestsTo, server, versionHeadersTo } from '@/test/server'

// Priority (issue #280) is the user's own note of how much they want a Job
// Listing: set from the row, filtered on, and sorted high first.

const ACME = { id: 'acme', company: 'Acme', priority: 'low' as const }
const GLOBEX = { id: 'globex', company: 'Globex', priority: 'high' as const }
const INITECH = { id: 'initech', company: 'Initech' }

// serveList answers the way the backend does: priority filters exactly, and
// sort=priority puts high first and undecided last.
function serveList(rows: JobListingSummaryWithApplication[]) {
  const rank = { high: 0, medium: 1, low: 2 } as const
  server.use(
    http.get('/api/job-listings', ({ request }) => {
      const params = new URL(request.url).searchParams
      let out = rows.filter((r) => !params.get('priority') || r.jobListing.priority === params.get('priority'))
      if (params.get('sort') === 'priority') {
        out = [...out].sort(
          (a, b) =>
            (a.jobListing.priority ? rank[a.jobListing.priority] : 3) -
            (b.jobListing.priority ? rank[b.jobListing.priority] : 3),
        )
      }
      return HttpResponse.json(out)
    }),
  )
}

async function chooseFromSelect(user: UserEvent, selectName: string, option: string) {
  await user.click(await screen.findByRole('combobox', { name: selectName }))
  await user.click(await screen.findByRole('option', { name: option }))
}

function open(at = '/jobs') {
  serveList([
    listingSummaryWithApplication(ACME),
    listingSummaryWithApplication(INITECH),
    listingSummaryWithApplication(GLOBEX),
  ])
  return renderPage(<JobListingsListPage />, { at, pattern: '/jobs' })
}

function shownCompanies(): string[] {
  return screen
    .getAllByRole('listitem')
    .map((li) => ['Acme', 'Globex', 'Initech'].find((c) => within(li).queryByText(c)))
    .filter((c): c is string => Boolean(c))
}

describe('Priority on the Job Listings list', () => {
  it('Row_SettingAPriority_PatchesWithTheVersionTokenAndShowsIt', async () => {
    server.use(
      http.patch('/api/job-listings/initech', () =>
        HttpResponse.json({ jobListing: jobListing({ id: 'initech', priority: 'medium', version: 'job-listing-v2' }) }),
      ),
    )
    const { user } = open()
    const row = (await screen.findByText('Initech')).closest('li')!

    await chooseFromSelect(user, 'Priority for Initech', 'Priority: medium')

    await waitFor(() => expect(within(row).getByRole('combobox', { name: 'Priority for Initech' })).toHaveTextContent('medium'))
    const bodies = (await requestsTo('/api/job-listings/initech')).filter((r) => r.method === 'PATCH')
    expect(bodies.map((r) => r.body)).toEqual([{ priority: 'medium' }])
    expect(versionHeadersTo('/api/job-listings/initech').map((r) => r.ifMatch)).toEqual([JOB_LISTING_VERSION])
  })

  it('Row_ClearingAPriority_SendsAnEmptyValue', async () => {
    let sent: unknown
    server.use(
      http.patch('/api/job-listings/globex', async ({ request }) => {
        sent = await request.json()
        return HttpResponse.json({ jobListing: jobListing({ id: 'globex' }) })
      }),
    )
    const { user } = open()
    await screen.findByText('Globex')

    await chooseFromSelect(user, 'Priority for Globex', 'Priority: not set')

    await waitFor(() => expect(sent).toEqual({ priority: '' }))
  })

  it('Row_ChangedOnDisk_ShowsTheConflictAlert', async () => {
    server.use(http.patch('/api/job-listings/acme', () => new HttpResponse('changed on disk', { status: 409 })))
    const { user } = open()
    await screen.findByText('Acme')

    await chooseFromSelect(user, 'Priority for Acme', 'Priority: high')

    expect(await screen.findByRole('alert')).toHaveTextContent('This Job Listing changed on disk since you opened it.')
  })

  it('PriorityFilter_Chosen_SendsItAndWritesItToTheUrl', async () => {
    const { user } = open()
    await screen.findByText('Acme')

    await chooseFromSelect(user, 'Priority', 'High')

    await waitFor(() => expect(shownCompanies()).toEqual(['Globex']))
    expect(currentSearch()).toBe('?priority=high')
    const listRequests = await requestsTo('/api/job-listings')
    expect(listRequests[listRequests.length - 1].search).toBe('?priority=high')
  })

  it('HighFirst_Toggled_OrdersHighBeforeLowAndUndecidedLast', async () => {
    const { user } = open()
    await screen.findByText('Acme')
    expect(shownCompanies()).toEqual(['Acme', 'Initech', 'Globex'])

    await user.click(screen.getByRole('button', { name: 'High priority first' }))

    await waitFor(() => expect(shownCompanies()).toEqual(['Globex', 'Acme', 'Initech']))
    expect(currentSearch()).toBe('?prioritySort=1')
    const listRequests = await requestsTo('/api/job-listings')
    expect(listRequests[listRequests.length - 1].search).toBe('?sort=priority')
  })
})
