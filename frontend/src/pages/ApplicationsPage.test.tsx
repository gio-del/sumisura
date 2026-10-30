import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { MemoryRouter } from 'react-router-dom'
import ApplicationsPage from './ApplicationsPage'
import type { ApplicationGroups, ApplicationStatus } from '@/api/types'
import AppNav from '@/components/AppNav'
import { TooltipProvider } from '@/components/ui/tooltip'
import {
  APPLICATION_VERSION,
  application,
  applicationGroups,
  generationRecord,
  listingSummaryWithApplication,
  listingWithApplication,
} from '@/test/fixtures'
import { currentPath, renderApp, renderPage } from '@/test/render'
import { recordedRequests, requestsTo, server, versionHeadersTo } from '@/test/server'

function showApplications(groups: ApplicationGroups) {
  server.use(http.get('/api/applications', () => HttpResponse.json(groups)))
  return renderPage(<ApplicationsPage />, { at: '/applications', pattern: '/applications' })
}

/** group is the section the page renders for one Status. */
function group(label: string): HTMLElement {
  return screen.getByRole('region', { name: new RegExp(`^${label}, `) })
}

// The Applications view answers "what is waiting on me right now" (issue
// #95). Grouping and ordering are the backend's (tracking.GroupApplications
// covers them); the page must render the groups it is given, in the order
// given, and each row must be enough to recognise and act on.
describe('ApplicationsPage', () => {
  it('ApplicationsPage_Loaded_OffersTheCsvExportAsADownload', async () => {
    showApplications(applicationGroups({ saved: [listingSummaryWithApplication({ id: 'acme', company: 'Acme' })] }))

    const link = await screen.findByRole('link', { name: 'Export CSV' })
    expect(link).toHaveAttribute('href', '/api/applications/export.csv')
    expect(link).toHaveAttribute('download')
  })

  it('ApplicationsPage_Loaded_ShowsEveryStatusGroupInTheBackendsOrderWithCounts', async () => {
    showApplications(
      applicationGroups({
        saved: [listingSummaryWithApplication({ id: 'acme', company: 'Acme' })],
        sent: [
          listingSummaryWithApplication({ id: 'globex', company: 'Globex' }, { status: 'sent' }),
          listingSummaryWithApplication({ id: 'initech', company: 'Initech' }, { status: 'sent' }),
        ],
      }),
    )

    expect(await screen.findByRole('heading', { level: 1, name: 'Applications' })).toBeInTheDocument()
    expect(screen.getAllByRole('region').map((r) => r.getAttribute('aria-label'))).toEqual([
      'Saved, 1 Application',
      'Tailoring, 0 Applications',
      'Sent, 2 Applications',
      'Interviewing, 0 Applications',
      'Offer, 0 Applications',
      'Rejected, 0 Applications',
      'Withdrawn, 0 Applications',
    ])
    expect(within(group('Interviewing')).getByText('No Applications in Interviewing.')).toBeInTheDocument()

    // Rows keep the backend's order within a group.
    const sentRows = within(group('Sent')).getAllByRole('listitem')
    expect(sentRows.map((row) => within(row).getByRole('link', { name: /Globex|Initech/ }).textContent)).toEqual([
      'Globex',
      'Initech',
    ])

    const requests = await recordedRequests()
    expect(requests).toHaveLength(1)
    expect(requests[0]).toMatchObject({ method: 'GET', path: '/api/applications', search: '' })
  })

  it('Row_Shown_IdentifiesTheApplicationAndItsStateAtAGlance', async () => {
    showApplications(
      applicationGroups({
        sent: [
          listingSummaryWithApplication(
            {
              id: 'acme',
              company: 'Acme',
              title: 'Backend Engineer',
              ral: { min: 45000, max: 55000, currency: 'EUR', source: 'stated' },
            },
            {
              status: 'sent',
              isStale: true,
              statusUpdatedAt: '2026-01-10T10:00:00Z',
              method: { kind: 'email', value: 'jobs@acme.example' },
              contact: { name: 'Jane Recruiter', email: 'jane@acme.example' },
              generations: [
                generationRecord({ slug: 'acme-1', createdAt: '2026-01-08T10:00:00Z', cvPath: 'output/acme-1/cv.pdf' }),
                generationRecord({ slug: 'acme-2', createdAt: '2026-01-09T10:00:00Z', cvPath: 'output/acme-2/cv.pdf' }),
              ],
            },
          ),
        ],
      }),
    )

    const row = await screen.findByRole('listitem')
    expect(within(row).getByRole('link', { name: 'Backend Engineer — Acme' })).toHaveAttribute('href', '/jobs/acme')
    expect(within(row).getByText('Follow-up overdue')).toBeInTheDocument()
    expect(within(row).getByText('Method: Email')).toBeInTheDocument()
    expect(within(row).getByText('Contact recorded')).toBeInTheDocument()
    expect(within(row).getByText('2 Generations')).toBeInTheDocument()
    expect(within(row).getByText('RAL Range: EUR 45,000 – 55,000')).toBeInTheDocument()
    expect(within(row).getByText(`Status changed ${new Date('2026-01-10T10:00:00Z').toLocaleDateString()}`)).toBeInTheDocument()
    // Already tailored: the next action is not generating another CV.
    expect(within(row).queryByRole('link', { name: 'Generate CV' })).not.toBeInTheDocument()
  })

  it('Row_OlderRecordWithNothingOptional_RendersCleanlyAndOffersGeneration', async () => {
    showApplications(
      applicationGroups({
        interviewing: [
          listingSummaryWithApplication(
            { id: 'legacy', company: 'Legacy Corp' },
            { status: 'interviewing', method: { kind: 'unresolved' } },
          ),
        ],
      }),
    )

    const row = await screen.findByRole('listitem')
    expect(within(row).getByRole('link', { name: 'Legacy Corp' })).toHaveAttribute('href', '/jobs/legacy')
    expect(within(row).queryByRole('img')).not.toBeInTheDocument()
    expect(within(row).queryByText('Follow-up overdue')).not.toBeInTheDocument()
    expect(within(row).getByText('Method unresolved')).toBeInTheDocument()
    expect(within(row).getByText('No Contact')).toBeInTheDocument()
    expect(within(row).getByText('No Generations yet')).toBeInTheDocument()
    expect(within(row).getByText('Status change date unknown')).toBeInTheDocument()
    expect(within(row).getByRole('link', { name: 'Generate CV' })).toHaveAttribute('href', '/jobs/legacy/generate')
  })

  it('Row_MethodOther_ReadsAsAMethodRatherThanUnresolved', async () => {
    showApplications(
      applicationGroups({ saved: [listingSummaryWithApplication({}, { method: { kind: 'other' }, generations: [] })] }),
    )

    const row = await screen.findByRole('listitem')
    expect(within(row).getByText('Method: Other')).toBeInTheDocument()
    expect(within(row).queryByText('Method unresolved')).not.toBeInTheDocument()
    expect(within(row).getByText('No Generations yet')).toBeInTheDocument()
  })

  it('ApplicationsPage_NoApplications_ShowsAnEmptyStatePointingAtSavingAJobListing', async () => {
    showApplications(applicationGroups())

    expect(await screen.findByText(/No tracked Applications yet/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Save a Job Listing' })).toHaveAttribute('href', '/jobs/new')
    expect(screen.queryAllByRole('region')).toHaveLength(0)
  })

  it('ApplicationsPage_RequestFails_ShowsTheErrorRatherThanAnEmptyPipeline', async () => {
    server.use(http.get('/api/applications', () => new HttpResponse('data directory unreadable', { status: 500 })))

    renderPage(<ApplicationsPage />, { at: '/applications', pattern: '/applications' })

    expect(await screen.findByRole('alert')).toHaveTextContent('data directory unreadable')
    expect(screen.queryByText(/No tracked Applications yet/)).not.toBeInTheDocument()
    expect(screen.queryAllByRole('region')).toHaveLength(0)
  })

  it('ApplicationsRoute_RowLink_OpensTheJobListingDetailPage', async () => {
    server.use(
      http.get('/api/applications', () =>
        HttpResponse.json(applicationGroups({ saved: [listingSummaryWithApplication({ id: 'acme', company: 'Acme' })] })),
      ),
      http.get('/api/job-listings/acme', () => HttpResponse.json(listingWithApplication({ id: 'acme', company: 'Acme' }))),
    )
    const { user } = renderApp({ at: '/applications' })

    await user.click(await screen.findByRole('link', { name: 'Acme' }))

    expect(currentPath()).toBe('/jobs/acme')
  })
})

describe('Primary nav', () => {
  it('AppNav_ApplicationsEntry_SitsBetweenJobListingsAndStats', () => {
    server.use(http.get('/api/usage', () => HttpResponse.json({ estimatedCostUsd: 0 })))
    render(
      <TooltipProvider>
        <MemoryRouter>
          <AppNav />
        </MemoryRouter>
      </TooltipProvider>,
    )

    const labels = within(screen.getByRole('navigation', { name: 'Primary' }))
      .getAllByRole('link')
      .map((link) => link.textContent)
    const applications = labels.indexOf('Applications')
    expect(applications).toBeGreaterThan(labels.indexOf('Job Listings'))
    expect(applications).toBeLessThan(labels.indexOf('Stats'))
    expect(screen.getByRole('link', { name: 'Applications' })).toHaveAttribute('href', '/applications')
  })
})

// A row moves its Application with the same constrained control the Job
// Listings list and detail page use (its full confirmation matrix is covered
// in ApplicationStatusControl.test.tsx), then the row lands in its new group
// from a fresh, server-ordered snapshot.
describe('Moving an Application from its row', () => {
  const STATUS_PATH = '/api/applications/acme/status'

  /** serveStatefully answers GET /api/applications with Acme in its current Status. */
  function serveStatefully(initial: ApplicationStatus) {
    let current = initial
    server.use(
      http.get('/api/applications', () =>
        HttpResponse.json(
          applicationGroups({
            [current]: [listingSummaryWithApplication({ id: 'acme', company: 'Acme' }, { status: current })],
          }),
        ),
      ),
      http.patch(STATUS_PATH, async ({ request }) => {
        const { status } = (await request.json()) as { status: ApplicationStatus }
        current = status
        return HttpResponse.json(application({ status }))
      }),
    )
  }

  async function moveTo(user: UserEvent, status: string) {
    await user.click(await screen.findByRole('combobox', { name: 'Move Acme to a new status' }))
    await user.click(await screen.findByRole('option', { name: status }))
  }

  it('StatusControl_Sent_OffersOnlyTheStateMachinesNextMoves', async () => {
    serveStatefully('sent')
    const { user } = renderPage(<ApplicationsPage />, { at: '/applications', pattern: '/applications' })

    await user.click(await screen.findByRole('combobox', { name: 'Move Acme to a new status' }))

    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual(['Interviewing', 'Rejected', 'Withdrawn'])
  })

  it('StatusMove_NoConfirmationNeeded_MovesTheRowIntoItsNewGroupWithoutAReload', async () => {
    serveStatefully('sent')
    const { user } = renderPage(<ApplicationsPage />, { at: '/applications', pattern: '/applications' })

    await moveTo(user, 'Interviewing')

    expect(await screen.findByRole('region', { name: 'Interviewing, 1 Application' })).toBeInTheDocument()
    expect(within(group('Interviewing')).getByRole('link', { name: 'Acme' })).toBeInTheDocument()
    expect(within(group('Sent')).queryByRole('link', { name: 'Acme' })).not.toBeInTheDocument()
    expect(await requestsTo(STATUS_PATH)).toEqual([
      { method: 'PATCH', path: STATUS_PATH, search: '', body: { status: 'interviewing' } },
    ])
    expect(await requestsTo('/api/applications')).toHaveLength(2)
  })

  it('StatusMove_ToRejected_AsksForConfirmationBeforeSendingAnything', async () => {
    serveStatefully('sent')
    const { user } = renderPage(<ApplicationsPage />, { at: '/applications', pattern: '/applications' })

    await moveTo(user, 'Rejected')

    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText('Mark this Application Rejected?')).toBeInTheDocument()
    expect(await requestsTo(STATUS_PATH)).toHaveLength(0)

    await user.click(within(dialog).getByRole('button', { name: 'Yes, mark Rejected' }))

    expect(await screen.findByRole('region', { name: 'Rejected, 1 Application' })).toBeInTheDocument()
    expect(await requestsTo(STATUS_PATH)).toEqual([
      { method: 'PATCH', path: STATUS_PATH, search: '', body: { status: 'rejected' } },
    ])
  })

  // The row move rewrites the Application file, so it presents the row's
  // Application token from the grouped view (issue #89, extended to the
  // Applications page).
  it('StatusMove_Sent_PresentsTheRowsApplicationVersion', async () => {
    serveStatefully('sent')
    const { user } = renderPage(<ApplicationsPage />, { at: '/applications', pattern: '/applications' })

    await moveTo(user, 'Interviewing')

    expect(await screen.findByRole('region', { name: 'Interviewing, 1 Application' })).toBeInTheDocument()
    expect(versionHeadersTo(STATUS_PATH)).toEqual([
      { method: 'PATCH', path: STATUS_PATH, ifMatch: APPLICATION_VERSION, applicationIfMatch: undefined },
    ])
  })

  it('StatusMove_ChangedOnDisk_LeavesTheRowAndReloadShowsWhereItNowIs', async () => {
    serveStatefully('sent')
    server.use(http.patch(STATUS_PATH, () => new HttpResponse('changed on disk', { status: 409 })))
    const { user } = renderPage(<ApplicationsPage />, { at: '/applications', pattern: '/applications' })

    await moveTo(user, 'Interviewing')

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('This Application changed on disk since you opened it.')
    expect(within(group('Sent')).getByRole('link', { name: 'Acme' })).toBeInTheDocument()
    expect(await requestsTo('/api/applications')).toHaveLength(1)

    server.use(
      http.get('/api/applications', () =>
        HttpResponse.json(
          applicationGroups({
            offer: [listingSummaryWithApplication({ id: 'acme', company: 'Acme' }, { status: 'offer' })],
          }),
        ),
      ),
    )
    await user.click(within(alert).getByRole('button', { name: 'Reload the current version' }))

    expect(await screen.findByRole('region', { name: 'Offer, 1 Application' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('StatusMove_UpdateFails_LeavesTheRowWhereItWasAndSaysWhy', async () => {
    serveStatefully('sent')
    server.use(http.patch(STATUS_PATH, () => new HttpResponse('invalid status transition', { status: 400 })))
    const { user } = renderPage(<ApplicationsPage />, { at: '/applications', pattern: '/applications' })

    await moveTo(user, 'Interviewing')

    expect(await screen.findByRole('alert')).toHaveTextContent('invalid status transition')
    expect(within(group('Sent')).getByRole('link', { name: 'Acme' })).toBeInTheDocument()
    expect(group('Interviewing')).toHaveAttribute('aria-label', 'Interviewing, 0 Applications')
    expect(await requestsTo('/api/applications')).toHaveLength(1)
  })
})
