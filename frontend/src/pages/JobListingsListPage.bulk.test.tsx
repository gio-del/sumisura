import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import JobListingsListPage from './JobListingsListPage'
import { jobListing, listingSummaryWithApplication } from '@/test/fixtures'
import { renderPage } from '@/test/render'
import { requestsTo, server } from '@/test/server'

// Bulk archive (issue #303): select Saved rows, archive them in one command,
// a conflict on one row does not stop the others.

const daysAgo = (n: number) => new Date(Date.now() - n * 24 * 60 * 60 * 1000).toISOString()

const OLD = listingSummaryWithApplication({ id: 'old', company: 'Oldco' }, { id: 'old', status: 'saved', statusUpdatedAt: daysAgo(45) })
const FRESH = listingSummaryWithApplication({ id: 'fresh', company: 'Freshco' }, { id: 'fresh', status: 'saved', statusUpdatedAt: daysAgo(3) })
const SENT = listingSummaryWithApplication({ id: 'sent', company: 'Sentco' }, { id: 'sent', status: 'sent', statusUpdatedAt: daysAgo(90) })

function open(rows = [OLD, FRESH, SENT]) {
  server.use(http.get('/api/job-listings', () => HttpResponse.json(rows)))
  return renderPage(<JobListingsListPage />, { at: '/jobs', pattern: '/jobs' })
}

function serveArchive(id: string, company: string) {
  server.use(
    http.post(`/api/job-listings/${id}/archive`, () =>
      HttpResponse.json({ jobListing: jobListing({ id, company, archived: true }) }),
    ),
  )
}

describe('Bulk archive', () => {
  it('Checkbox_OnlyOnSavedRows', async () => {
    open()
    await screen.findByText('Oldco')

    expect(screen.getByRole('checkbox', { name: /Select Oldco/ })).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: /Select Freshco/ })).toBeInTheDocument()
    expect(screen.queryByRole('checkbox', { name: /Select Sentco/ })).not.toBeInTheDocument()
  })

  it('SelectStale_SelectsOnlySavedRowsPastTheThreshold', async () => {
    const { user } = open()
    await screen.findByText('Oldco')

    await user.click(screen.getByRole('button', { name: /Select Saved for over 30 days/ }))

    expect(screen.getByRole('checkbox', { name: /Select Oldco/ })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: /Select Freshco/ })).not.toBeChecked()
    expect(screen.getByRole('button', { name: 'Archive selected (1)' })).toBeEnabled()
  })

  it('ArchiveSelected_ArchivesEveryChosenRowAndReportsTheCount', async () => {
    serveArchive('old', 'Oldco')
    serveArchive('fresh', 'Freshco')
    const { user } = open()
    await screen.findByText('Oldco')

    await user.click(screen.getByRole('checkbox', { name: /Select Oldco/ }))
    await user.click(screen.getByRole('checkbox', { name: /Select Freshco/ }))
    await user.click(screen.getByRole('button', { name: 'Archive selected (2)' }))

    expect(await screen.findByText('Archived 2 Job Listings.')).toBeInTheDocument()
    expect(screen.queryByText('Oldco')).not.toBeInTheDocument()
    expect(screen.queryByText('Freshco')).not.toBeInTheDocument()
    expect(screen.getByText('Sentco')).toBeInTheDocument()
    expect(await requestsTo('/api/job-listings/old/archive')).toHaveLength(1)
    expect(await requestsTo('/api/job-listings/fresh/archive')).toHaveLength(1)
  })

  it('ArchiveSelected_ConflictOnOneRow_DoesNotStopTheOthers', async () => {
    server.use(http.post('/api/job-listings/old/archive', () => new HttpResponse('changed', { status: 409 })))
    serveArchive('fresh', 'Freshco')
    const { user } = open()
    await screen.findByText('Oldco')

    await user.click(screen.getByRole('checkbox', { name: /Select Oldco/ }))
    await user.click(screen.getByRole('checkbox', { name: /Select Freshco/ }))
    await user.click(screen.getByRole('button', { name: 'Archive selected (2)' }))

    expect(await screen.findByText('Archived 1 Job Listing, 1 not archived.')).toBeInTheDocument()
    const alert = await screen.findByRole('alert')
    expect(within(alert).getByRole('button', { name: 'Reload the current version' })).toBeInTheDocument()
    expect(screen.getByText('Oldco')).toBeInTheDocument()
    expect(screen.queryByText('Freshco')).not.toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Archive selected (1)' })).toBeInTheDocument())
  })
})
