import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import JobListingsListPage from './JobListingsListPage'
import type { JobListingSummaryWithApplication } from '@/api/types'
import { JOB_LISTING_VERSION, jobListing, listingSummaryWithApplication } from '@/test/fixtures'
import { currentSearch, openFilters, renderPage } from '@/test/render'
import { requestsTo, server, versionHeadersTo } from '@/test/server'

// Archiving (issue #98) is a view concern: the list asks the backend for one
// of three archived views, and each row can be archived or unarchived in
// place without a confirmation dialog.

const ACME = { id: 'acme', company: 'Acme' }
const GLOBEX = { id: 'globex', company: 'Globex', archived: true }

// serveListByView answers GET /api/job-listings the way the backend does:
// archived=exclude (or absent) leaves archived rows out, only keeps just
// them, all keeps both.
function serveListByView(rows: JobListingSummaryWithApplication[]) {
  server.use(
    http.get('/api/job-listings', ({ request }) => {
      const view = new URL(request.url).searchParams.get('archived') ?? 'exclude'
      const keep = rows.filter((r) =>
        view === 'all' ? true : view === 'only' ? r.jobListing.archived : !r.jobListing.archived,
      )
      return HttpResponse.json(keep)
    }),
  )
}

async function lastListQuery(): Promise<string> {
  const listRequests = await requestsTo('/api/job-listings')
  return listRequests[listRequests.length - 1].search
}

function open(at = '/jobs', rows = [listingSummaryWithApplication(ACME), listingSummaryWithApplication(GLOBEX)]) {
  serveListByView(rows)
  return renderPage(<JobListingsListPage />, { at, pattern: '/jobs' })
}

describe('Archived view control', () => {
  it('ArchivedView_Default_IsActiveAndSendsNoArchivedParam', async () => {
    open()

    expect(await screen.findByText('Acme')).toBeInTheDocument()
    expect(screen.queryByText('Globex')).not.toBeInTheDocument()
    await openFilters()
    expect(screen.getByRole('combobox', { name: 'Show' })).toHaveTextContent('Active')
    expect(await lastListQuery()).toBe('')
  })

  it('ArchivedView_ArchivedChosen_RequestsOnlyArchivedAndWritesItToTheUrl', async () => {
    const { user } = open()
    await screen.findByText('Acme')
    await openFilters()

    await user.click(screen.getByRole('combobox', { name: 'Show' }))
    await user.click(await screen.findByRole('option', { name: 'Archived' }))

    expect(await screen.findByText('Globex')).toBeInTheDocument()
    expect(screen.queryByText('Acme')).not.toBeInTheDocument()
    expect(await lastListQuery()).toBe('?archived=only')
    expect(currentSearch()).toBe('?archived=only')
  })

  it('ArchivedView_AllRestoredFromTheUrl_ComposesWithTheOtherFiltersOnTheFirstRequest', async () => {
    open('/jobs?status=saved&archived=all&ralSort=desc')

    expect(await screen.findByText('Globex')).toBeInTheDocument()
    expect(screen.getByText('Acme')).toBeInTheDocument()
    await openFilters()
    expect(screen.getByRole('combobox', { name: 'Show' })).toHaveTextContent('All')
    expect(await lastListQuery()).toBe('?status=saved&archived=all&sort=ral&order=desc')
  })

  it('ArchivedView_ActiveChosenAgain_DropsTheParamFromTheUrlAndRequest', async () => {
    const { user } = open('/jobs?archived=only')
    await screen.findByText('Globex')
    await openFilters()

    await user.click(screen.getByRole('combobox', { name: 'Show' }))
    await user.click(await screen.findByRole('option', { name: 'Active' }))

    expect(await screen.findByText('Acme')).toBeInTheDocument()
    expect(await lastListQuery()).toBe('')
    expect(currentSearch()).toBe('')
  })

  it('ClearFilters_WithAnArchivedView_LeavesTheViewInPlace', async () => {
    const { user } = open('/jobs?status=saved&archived=only')
    await screen.findByText('Globex')

    await user.click(screen.getByRole('button', { name: 'Clear filters' }))

    await waitFor(() => expect(currentSearch()).toBe('?archived=only'))
  })

  it('ArchivedView_EmptyArchivedView_SaysSoRatherThanLookingLikeAFailure', async () => {
    open('/jobs?archived=only', [listingSummaryWithApplication(ACME)])

    expect(await screen.findByText('No archived Job Listings.')).toBeInTheDocument()
  })

  it('ArchivedView_EmptyActiveView_PointsAtTheArchivedView', async () => {
    open('/jobs', [listingSummaryWithApplication(GLOBEX)])

    expect(
      await screen.findByText('No active Job Listings. Archived ones are under Show: Archived.'),
    ).toBeInTheDocument()
  })
})

describe('Archiving a row', () => {
  it('ArchiveButton_Clicked_ArchivesWithoutConfirmationAndDropsTheRowFromTheActiveView', async () => {
    let archived = false
    server.use(
      http.post('/api/job-listings/acme/archive', () => {
        archived = true
        return HttpResponse.json({ jobListing: jobListing({ ...ACME, archived: true }) })
      }),
    )
    const { user } = open()
    const row = (await screen.findByText('Acme')).closest('li')!

    await user.click(within(row).getByRole('button', { name: 'Archive' }))

    await waitFor(() => expect(screen.queryByText('Acme')).not.toBeInTheDocument())
    expect(archived).toBe(true)
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(await requestsTo('/api/job-listings/acme/archive')).toMatchObject([{ method: 'POST' }])
  })

  it('UnarchiveButton_ClickedInTheAllView_KeepsTheRowAndOffersArchiveAgain', async () => {
    server.use(
      http.post('/api/job-listings/globex/unarchive', () =>
        HttpResponse.json({ jobListing: jobListing({ ...GLOBEX, archived: false }) }),
      ),
    )
    const { user } = open('/jobs?archived=all')
    const row = (await screen.findByText('Globex')).closest('li')!
    expect(within(row).getByText('Archived')).toBeInTheDocument()

    await user.click(within(row).getByRole('button', { name: 'Unarchive' }))

    expect(await within(row).findByRole('button', { name: 'Archive' })).toBeInTheDocument()
    expect(within(row).queryByText('Archived')).not.toBeInTheDocument()
  })

  // In the All view the row stays and flips, so it must also adopt the fresh
  // Job Listing token the toggle answered with, or its next toggle would
  // present a spent one and be refused (issue #89).
  it('ArchiveToggle_InTheAllView_PresentsTheRowsVersionThenTheFreshOne', async () => {
    server.use(
      http.post('/api/job-listings/acme/archive', () =>
        HttpResponse.json({ jobListing: jobListing({ ...ACME, archived: true, version: 'job-listing-v2' }) }),
      ),
      http.post('/api/job-listings/acme/unarchive', () =>
        HttpResponse.json({ jobListing: jobListing({ ...ACME, archived: false, version: 'job-listing-v3' }) }),
      ),
    )
    const { user } = open('/jobs?archived=all')
    const row = (await screen.findByText('Acme')).closest('li')!

    await user.click(within(row).getByRole('button', { name: 'Archive' }))
    await user.click(await within(row).findByRole('button', { name: 'Unarchive' }))

    expect(await within(row).findByRole('button', { name: 'Archive' })).toBeInTheDocument()
    expect(versionHeadersTo('/api/job-listings/acme/archive').map((r) => r.ifMatch)).toEqual([JOB_LISTING_VERSION])
    expect(versionHeadersTo('/api/job-listings/acme/unarchive').map((r) => r.ifMatch)).toEqual(['job-listing-v2'])
  })

  it('ArchiveButton_ChangedOnDisk_KeepsTheRowAndReloadReReadsTheList', async () => {
    server.use(
      http.post('/api/job-listings/acme/archive', () => new HttpResponse('changed on disk', { status: 409 })),
    )
    const { user } = open()
    const row = (await screen.findByText('Acme')).closest('li')!

    await user.click(within(row).getByRole('button', { name: 'Archive' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('This Job Listing changed on disk since you opened it.')
    expect(screen.getByText('Acme')).toBeInTheDocument()

    await user.click(within(alert).getByRole('button', { name: 'Reload the current version' }))

    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
    expect(await requestsTo('/api/job-listings')).toHaveLength(2)
  })

  it('ArchivedRow_InTheArchivedView_IsDeEmphasizedButKeepsItsStatusMove', async () => {
    open('/jobs?archived=only')
    const row = (await screen.findByText('Globex')).closest('li')!

    expect(row).toHaveAttribute('data-archived', 'true')
    expect(within(row).getByText('Archived')).toBeInTheDocument()
    expect(within(row).getByRole('combobox', { name: 'Move Globex to a new status' })).toBeEnabled()
    expect(within(row).getByRole('button', { name: 'Unarchive' })).toBeInTheDocument()
    expect(within(row).queryByRole('button', { name: 'Archive' })).not.toBeInTheDocument()
  })

  it('ArchiveButton_RequestFails_ShowsTheErrorInlineAndKeepsTheRow', async () => {
    server.use(
      http.post('/api/job-listings/acme/archive', () => new HttpResponse('disk full', { status: 500 })),
    )
    const { user } = open()
    const row = (await screen.findByText('Acme')).closest('li')!

    await user.click(within(row).getByRole('button', { name: 'Archive' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('disk full')
    expect(screen.getByText('Acme')).toBeInTheDocument()
  })
})
