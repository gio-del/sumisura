import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import JobListingDetailPage from './JobListingDetailPage'
import type { JobListingWithApplication } from '@/api/types'
import { APPLICATION_VERSION, JOB_LISTING_VERSION, application, generationRecord, listingWithApplication } from '@/test/fixtures'
import { currentPath, currentSearch, renderApp, renderPage } from '@/test/render'
import { recordedRequests, requestsTo, server, versionHeadersTo } from '@/test/server'

const DETAIL_PATH = '/api/job-listings/acme'

function serveDetail(record: JobListingWithApplication) {
  server.use(http.get(DETAIL_PATH, () => HttpResponse.json(record)))
}

/** open deep-links straight to the Job Listing's page, as a bookmark does. */
function open(record: JobListingWithApplication) {
  serveDetail(record)
  return renderPage(<JobListingDetailPage />, { at: '/jobs/acme', pattern: '/jobs/:id' })
}

async function chooseFromSelect(user: UserEvent, selectName: string, option: string) {
  await user.click(await screen.findByRole('combobox', { name: selectName }))
  await user.click(await screen.findByRole('option', { name: option }))
}

describe('JobListingDetailPage', () => {
  it('JobListingDetailPage_DeepLinked_FetchesItsOwnRecordByIdAndHeadsThePageWithIt', async () => {
    open(listingWithApplication({ id: 'acme', company: 'Acme', title: 'Backend Engineer' }))

    expect(await screen.findByRole('heading', { level: 1, name: 'Backend Engineer — Acme' })).toBeInTheDocument()
    const requests = await recordedRequests()
    expect(requests).toEqual([{ method: 'GET', path: DETAIL_PATH, search: '', body: undefined }])
  })

  it('JobListingDetailPage_WithCompanyLogo_ShowsTheLogo', async () => {
    const { container } = open(listingWithApplication({ logo: 'acme.png' }))

    await screen.findByRole('heading', { level: 1, name: 'Acme' })
    expect(container.querySelector('img')).toHaveAttribute('src', '/api/job-listings/acme/logo')
  })

  it('JobListingDetailPage_WithoutCompanyLogo_ShowsNoImageAtAll', async () => {
    const { container } = open(listingWithApplication())

    await screen.findByRole('heading', { level: 1, name: 'Acme' })
    expect(container.querySelector('img')).toBeNull()
  })

  // A page that exists to show one Job Listing has no rows competing for
  // space, so the Job Description is simply there — no expand toggle — with
  // the Markdown treatment issue #19 settled.
  it('JobDescription_Shown_RendersInlineAsMarkdownWithNoToggle', async () => {
    open(
      listingWithApplication({
        jobDescription: 'We are hiring.\n\n**Requirements**\n\n- Go\n- [Postgres](https://postgres.example)',
      }),
    )

    const heading = await screen.findByRole('heading', { name: 'Job Description' })
    const section = heading.closest('section') as HTMLElement
    expect(within(section).getByText('Requirements').tagName).toBe('STRONG')
    expect(within(section).getAllByRole('listitem').map((li) => li.textContent)).toEqual(['Go', 'Postgres'])
    expect(within(section).getByRole('link', { name: 'Postgres' })).toHaveAttribute(
      'href',
      'https://postgres.example',
    )
    expect(screen.queryByRole('button', { name: /description/i })).not.toBeInTheDocument()
  })

  it('JobDescription_ManuallyPastedWithoutMarkdown_KeepsItsLineBreaks', async () => {
    open(listingWithApplication({ jobDescription: 'Line one\nLine two' }))

    const heading = await screen.findByRole('heading', { name: 'Job Description' })
    expect((heading.closest('section') as HTMLElement).querySelector('br')).not.toBeNull()
  })

  it('PostingLink_JobListingHasASourceUrl_LinksToTheOriginalPosting', async () => {
    open(listingWithApplication({ url: 'https://acme.example/jobs/1' }))

    expect(await screen.findByRole('link', { name: 'View posting' })).toHaveAttribute(
      'href',
      'https://acme.example/jobs/1',
    )
  })

  it('JobListingDetailPage_UnknownId_SaysTheJobListingWasNotFound', async () => {
    server.use(http.get(DETAIL_PATH, () => new HttpResponse('job listing not found', { status: 404 })))
    renderPage(<JobListingDetailPage />, { at: '/jobs/acme', pattern: '/jobs/:id' })

    expect(await screen.findByRole('heading', { name: 'Job Listing not found' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '← Back to Job Listings' })).toHaveAttribute('href', '/jobs')
  })

  it('JobListingDetailPage_RequestFails_ShowsTheError', async () => {
    server.use(http.get(DETAIL_PATH, () => new HttpResponse('data directory unreadable', { status: 500 })))
    renderPage(<JobListingDetailPage />, { at: '/jobs/acme', pattern: '/jobs/:id' })

    expect(await screen.findByRole('alert')).toHaveTextContent('data directory unreadable')
    expect(screen.queryByRole('heading', { name: 'Job Listing not found' })).not.toBeInTheDocument()
  })

  it('BackLink_ArrivedByDeepLink_FallsBackToTheUnfilteredList', async () => {
    open(listingWithApplication())

    expect(await screen.findByRole('link', { name: '← Back to Job Listings' })).toHaveAttribute('href', '/jobs')
  })

  it('GenerateAction_Shown_LinksToTheJobListingsGenerationPage', async () => {
    open(listingWithApplication())

    expect(await screen.findByRole('link', { name: 'Generate CV' })).toHaveAttribute('href', '/jobs/acme/generate')
  })
})

describe('Job Listing detail routing', () => {
  it('Route_JobsNew_StillResolvesToTheCreatePageRatherThanADetailPage', async () => {
    renderApp({ at: '/jobs/new' })

    expect(await screen.findByRole('button', { name: 'Save Job Listing' })).toBeInTheDocument()
    expect(await recordedRequests()).toEqual([])
  })

  it('Route_JobsId_ResolvesToTheDetailPage', async () => {
    serveDetail(listingWithApplication({ title: 'Backend Engineer' }))
    renderApp({ at: '/jobs/acme' })

    expect(await screen.findByRole('heading', { level: 1, name: 'Backend Engineer — Acme' })).toBeInTheDocument()
  })
})

describe('Job Listing freshness on its page', () => {
  it('FreshnessBadge_LiveUrl_ShowsWhenItWasLastChecked', async () => {
    open(
      listingWithApplication({
        url: 'https://acme.example/jobs/1',
        freshnessStatus: 'live',
        freshnessCheckedAt: '2026-02-01T09:30:00Z',
      }),
    )

    expect(await screen.findByText('Live')).toBeInTheDocument()
    expect(screen.getByText(/^Checked /)).toBeInTheDocument()
  })

  it('FreshnessBadge_NoSourceUrl_ShowsNoFreshnessControlAtAll', async () => {
    open(listingWithApplication({ url: undefined }))

    await screen.findByRole('heading', { level: 1, name: 'Acme' })
    expect(screen.queryByRole('button', { name: 'Check freshness' })).not.toBeInTheDocument()
    expect(screen.queryByText('Not yet checked')).not.toBeInTheDocument()
  })

  it('FreshnessCheck_ReturnsUnreachable_ReplacesTheBadgeWithTheNewStatus', async () => {
    const record = listingWithApplication({ url: 'https://acme.example/jobs/1', freshnessStatus: 'not-yet-checked' })
    const { user } = open(record)
    server.use(
      http.post('/api/job-listings/acme/check-freshness', () =>
        HttpResponse.json({
          jobListing: { ...record.jobListing, freshnessStatus: 'unreachable', freshnessCheckedAt: '2026-02-02T08:00:00Z' },
        }),
      ),
    )

    await user.click(await screen.findByRole('button', { name: 'Check freshness' }))

    expect(await screen.findByText('Unreachable')).toBeInTheDocument()
    expect(screen.queryByText('Not yet checked')).not.toBeInTheDocument()
    expect(await requestsTo(DETAIL_PATH)).toHaveLength(1)
  })
})

describe('Job Listing RAL Range and resolution on its page', () => {
  it('RALRange_Stated_ShowsTheRangeWithItsSourceLabelled', async () => {
    open(listingWithApplication({ ral: { min: 45000, max: 55000, currency: 'EUR', source: 'stated' } }))

    expect(await screen.findByText('RAL Range: EUR 45,000 – 55,000')).toBeInTheDocument()
    expect(screen.getByText('Stated in the Job Description')).toBeInTheDocument()
  })

  it('Resolve_UnresolvedRAL_RetriesAndShowsTheResolvedRecordInPlace', async () => {
    const { user } = open(listingWithApplication({ ral: { source: 'unresolved' } }))
    server.use(
      http.post('/api/job-listings/acme/resolve', () =>
        HttpResponse.json(
          listingWithApplication({ ral: { min: 45000, max: 55000, currency: 'EUR', source: 'estimated' } }),
        ),
      ),
    )

    // The retry now lives beside the cause it fixes, inside the
    // "Needs attention" popover (issue #206).
    await user.click(await screen.findByRole('button', { name: /needs attention/i }))
    await user.click(await screen.findByRole('button', { name: 'Retry' }))

    expect(await screen.findByText('RAL Range: EUR 45,000 – 55,000')).toBeInTheDocument()
    expect(screen.queryByText('Needs attention')).not.toBeInTheDocument()
    expect(await requestsTo('/api/job-listings/acme/resolve')).toHaveLength(1)
  })

  it('Resolve_Fails_SurfacesTheError', async () => {
    const { user } = open(listingWithApplication({ ral: { source: 'unresolved' } }))
    server.use(
      http.post('/api/job-listings/acme/resolve', () => new HttpResponse('claude unavailable', { status: 502 })),
    )

    await user.click(await screen.findByRole('button', { name: /needs attention/i }))
    await user.click(await screen.findByRole('button', { name: 'Retry' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('claude unavailable')
  })
})

describe('Application Method and Contact on the Job Listing page', () => {
  it('ApplicationMethod_Corrected_SendsTheCorrectionAndShowsItInPlace', async () => {
    const { user } = open(listingWithApplication())
    server.use(
      http.patch('/api/applications/acme/method', () =>
        HttpResponse.json(application({ method: { kind: 'other', value: 'Referral via Jane' } })),
      ),
    )

    await user.click(await screen.findByRole('button', { name: 'Correct' }))
    await chooseFromSelect(user, 'Application method', 'Other')
    const value = screen.getByPlaceholderText('URL or email address')
    await user.clear(value)
    await user.type(value, 'Referral via Jane')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(
      await screen.findByText(
        (_, el) => el?.tagName === 'P' && Boolean(el.textContent?.startsWith('Apply via Other: Referral via Jane')),
      ),
    ).toBeInTheDocument()
    expect(await requestsTo('/api/applications/acme/method')).toEqual([
      {
        method: 'PATCH',
        path: '/api/applications/acme/method',
        search: '',
        body: { kind: 'other', value: 'Referral via Jane' },
      },
    ])
  })

  // A conflict keeps the correction on screen; only the explicit reload
  // replaces it with what is on disk (issue #89, stories 6, 9-10 and 16).
  it('ApplicationMethod_ChangedOnDisk_KeepsTheDraftUntilTheUserReloads', async () => {
    const { user } = open(listingWithApplication())
    server.use(
      http.patch('/api/applications/acme/method', () => new HttpResponse('changed on disk', { status: 409 })),
    )

    await user.click(await screen.findByRole('button', { name: 'Correct' }))
    const value = screen.getByPlaceholderText('URL or email address')
    await user.clear(value)
    await user.type(value, 'https://acme.example/careers')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('This Application changed on disk since you opened it.')
    expect(screen.getByPlaceholderText('URL or email address')).toHaveValue('https://acme.example/careers')
    expect(versionHeadersTo('/api/applications/acme/method').map((r) => r.ifMatch)).toEqual([APPLICATION_VERSION])

    serveDetail(listingWithApplication({}, { method: { kind: 'email', value: 'jobs@acme.example' } }))
    await user.click(within(alert).getByRole('button', { name: 'Reload the current version' }))

    expect(
      await screen.findByText(
        (_, el) => el?.tagName === 'P' && Boolean(el.textContent?.startsWith('Apply via Email: jobs@acme.example')),
      ),
    ).toBeInTheDocument()
    expect(screen.queryByPlaceholderText('URL or email address')).not.toBeInTheDocument()
    expect(await requestsTo(DETAIL_PATH)).toHaveLength(2)
  })

  // A Contact is only ever saved on explicit confirmation — never on entry.
  it('Contact_EnteredForAnEmailApplication_IsSavedOnlyOnceConfirmed', async () => {
    const email = { kind: 'email' as const, value: 'jobs@acme.example' }
    const confirmed = { name: 'Jane Doe', email: 'jobs@acme.example' }
    const { user } = open(listingWithApplication({}, { method: email }))
    server.use(
      http.patch('/api/applications/acme/contact', () =>
        HttpResponse.json(application({ method: email, contact: confirmed })),
      ),
      http.get('/api/applications/acme/mailto', () => HttpResponse.json({ uri: 'mailto:jobs@acme.example' })),
    )

    await user.click(await screen.findByRole('button', { name: 'Enter manually' }))
    await user.type(screen.getByPlaceholderText('Contact name'), 'Jane Doe')
    expect(await requestsTo('/api/applications/acme/contact')).toHaveLength(0)

    await user.click(screen.getByRole('button', { name: 'Confirm & Save' }))

    expect(await screen.findByText('Jane Doe')).toBeInTheDocument()
    expect(await requestsTo('/api/applications/acme/contact')).toEqual([
      { method: 'PATCH', path: '/api/applications/acme/contact', search: '', body: confirmed },
    ])
  })
})

describe('Generation history on the Job Listing page', () => {
  it('GenerationHistory_SeveralGenerations_LinksEveryTailoredCVAndCoverLetter', async () => {
    open(
      listingWithApplication(
        {},
        {
          generations: [
            generationRecord({ slug: 'acme-1', createdAt: '2026-01-10T10:00:00Z', cvPath: 'output/acme-1/cv.pdf' }),
            generationRecord({
              slug: 'acme-2',
              createdAt: '2026-01-20T10:00:00Z',
              cvPath: 'output/acme-2/cv.pdf',
              coverLetterPath: 'output/acme-2/cover-letter.pdf',
            }),
          ],
        },
      ),
    )

    const history = await screen.findByRole('list', { name: 'Generation history' })
    const links = within(history)
      .getAllByRole('link')
      .map((a) => [a.textContent, a.getAttribute('href')])
    expect(links).toEqual([
      ['CV', '/api/generations/acme-2/cv.pdf'],
      ['Cover Letter', '/api/generations/acme-2/cover-letter.pdf'],
      ['CV', '/api/generations/acme-1/cv.pdf'],
    ])
    expect(screen.getByRole('link', { name: 'Regenerate CV' })).toBeInTheDocument()
  })

  // The notice depends on the detail endpoint attaching stale-Entry
  // information (issue #94's backend half); here it is rendered from it.
  it('StaleEntriesNotice_LatestGenerationDrewOnEditedEntries_NamesThem', async () => {
    open(
      listingWithApplication(
        {},
        {
          generations: [
            generationRecord({
              slug: 'acme-1',
              createdAt: '2026-01-10T10:00:00Z',
              cvPath: 'output/acme-1/cv.pdf',
              staleEntries: ['Globex — Backend Engineer'],
            }),
          ],
        },
      ),
    )

    expect(
      await screen.findByText(/Latest CV may be outdated — edited in Master Data since generation: Globex — Backend Engineer/),
    ).toBeInTheDocument()
  })

  it('StaleEntriesNotice_NothingEditedSinceGeneration_ShowsNoNotice', async () => {
    open(
      listingWithApplication(
        {},
        { generations: [generationRecord({ slug: 'acme-1', createdAt: '2026-01-10T10:00:00Z', cvPath: 'output/acme-1/cv.pdf' })] },
      ),
    )

    await screen.findByRole('list', { name: 'Generation history' })
    expect(screen.queryByText(/may be outdated/)).not.toBeInTheDocument()
  })
})

// Deleting a Job Listing takes its Application — Status, Method, Contact and
// the whole Generation history — with it, so it is confirmation-gated and the
// dialog names what is about to go. It now happens on the focused page.
describe('Job Listing deletion', () => {
  it('Delete_Requested_AsksForConfirmationNamingTheJobListingThenReturnsToTheList', async () => {
    serveDetail(listingWithApplication({ title: 'Backend Engineer' }))
    server.use(
      http.delete(DETAIL_PATH, () => new HttpResponse(null, { status: 204 })),
      http.get('/api/job-listings', () => HttpResponse.json([])),
    )
    const { user } = renderApp({ at: '/jobs/acme' })

    await user.click(await screen.findByRole('button', { name: 'Delete' }))

    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText('Delete Backend Engineer — Acme?')).toBeInTheDocument()
    expect(
      within(dialog).getByText(/also remove its Application \(Status, Method, Contact, Notes, and Generation history\)/),
    ).toBeInTheDocument()
    expect(await requestsTo(DETAIL_PATH)).toEqual([{ method: 'GET', path: DETAIL_PATH, search: '', body: undefined }])

    await user.click(within(dialog).getByRole('button', { name: 'Yes, delete' }))

    expect(
      await screen.findByText('No active Job Listings. Archived ones are under Show: Archived.'),
    ).toBeInTheDocument()
    expect(currentPath()).toBe('/jobs')
    expect(currentSearch()).toBe('')
    expect((await requestsTo(DETAIL_PATH)).map((r) => r.method)).toEqual(['GET', 'DELETE'])
  })

  // The delete removes two files, so it presents two tokens: the Job
  // Listing's as If-Match, its Application's as Application-If-Match.
  it('Delete_Confirmed_PresentsBothTheJobListingAndApplicationVersions', async () => {
    serveDetail(listingWithApplication())
    server.use(
      http.delete(DETAIL_PATH, () => new HttpResponse(null, { status: 204 })),
      http.get('/api/job-listings', () => HttpResponse.json([])),
    )
    const { user } = renderApp({ at: '/jobs/acme' })

    await user.click(await screen.findByRole('button', { name: 'Delete' }))
    await user.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Yes, delete' }))

    await waitFor(() => expect(currentPath()).toBe('/jobs'))
    expect(versionHeadersTo(DETAIL_PATH).filter((r) => r.method === 'DELETE')).toEqual([
      { method: 'DELETE', path: DETAIL_PATH, ifMatch: JOB_LISTING_VERSION, applicationIfMatch: APPLICATION_VERSION },
    ])
  })

  // Story 19: a Status that moved on elsewhere refuses the delete, and the
  // page stays put with the record intact rather than navigating away.
  it('Delete_ChangedOnDisk_StaysOnThePageAndOffersAReload', async () => {
    const { user } = open(listingWithApplication())
    server.use(http.delete(DETAIL_PATH, () => new HttpResponse('changed on disk', { status: 409 })))

    await user.click(await screen.findByRole('button', { name: 'Delete' }))
    await user.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Yes, delete' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('This Job Listing or its Application changed on disk since you opened it.')
    expect(alert).toHaveTextContent('Your delete was not applied.')
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
    expect(screen.getByRole('heading', { level: 1, name: 'Acme' })).toBeInTheDocument()

    serveDetail(listingWithApplication({}, { status: 'interviewing' }))
    await user.click(within(alert).getByRole('button', { name: 'Reload the current version' }))

    expect(await screen.findByText('Interviewing')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('Delete_ConfirmationCancelled_DeletesNothingAndStaysOnThePage', async () => {
    const { user } = open(listingWithApplication())

    await user.click(await screen.findByRole('button', { name: 'Delete' }))
    await user.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Cancel' }))

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1, name: 'Acme' })).toBeInTheDocument()
    expect((await requestsTo(DETAIL_PATH)).map((r) => r.method)).toEqual(['GET'])
  })

  it('Delete_Fails_SurfacesTheErrorAndStaysOnThePage', async () => {
    const { user } = open(listingWithApplication())
    server.use(http.delete(DETAIL_PATH, () => new HttpResponse('permission denied', { status: 500 })))

    await user.click(await screen.findByRole('button', { name: 'Delete' }))
    await user.click(within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Yes, delete' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('permission denied')
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
    expect(screen.getByRole('heading', { level: 1, name: 'Acme' })).toBeInTheDocument()
  })
})

// Archiving (issue #98) sits next to Delete as its non-destructive
// alternative: no confirmation, the Job Listing stays on its page, and its
// Status stays movable.
describe('Job Listing archiving', () => {
  it('Archive_Clicked_ArchivesWithoutConfirmationAndOffersUnarchive', async () => {
    const record = listingWithApplication({ title: 'Backend Engineer' }, { status: 'rejected' })
    const { user } = open(record)
    server.use(
      http.post(`${DETAIL_PATH}/archive`, () =>
        HttpResponse.json({ jobListing: { ...record.jobListing, archived: true } }),
      ),
    )

    await user.click(await screen.findByRole('button', { name: 'Archive' }))

    expect(await screen.findByRole('button', { name: 'Unarchive' })).toBeInTheDocument()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(screen.getByText('Archived')).toBeInTheDocument()
    expect(screen.getByText('Rejected')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
    expect((await requestsTo(`${DETAIL_PATH}/archive`)).map((r) => r.method)).toEqual(['POST'])
  })

  it('Unarchive_OnAnArchivedJobListing_UnarchivesAndKeepsItsStatusMovable', async () => {
    const record = listingWithApplication({ archived: true })
    const { user } = open(record)
    server.use(
      http.post(`${DETAIL_PATH}/unarchive`, () =>
        HttpResponse.json({ jobListing: { ...record.jobListing, archived: false } }),
      ),
    )
    expect(await screen.findByRole('combobox', { name: 'Move Acme to a new status' })).toBeEnabled()

    await user.click(screen.getByRole('button', { name: 'Unarchive' }))

    expect(await screen.findByRole('button', { name: 'Archive' })).toBeInTheDocument()
    expect(screen.queryByText('Archived')).not.toBeInTheDocument()
  })

  // Archiving rewrites the Job Listing file, so it presents the Job
  // Listing's token and adopts the fresh one it answers with (issue #89).
  it('Archive_Toggled_PresentsTheJobListingVersionAndAdoptsTheFreshOne', async () => {
    const record = listingWithApplication()
    const { user } = open(record)
    server.use(
      http.post(`${DETAIL_PATH}/archive`, () =>
        HttpResponse.json({ jobListing: { ...record.jobListing, archived: true, version: 'job-listing-v2' } }),
      ),
      http.post(`${DETAIL_PATH}/unarchive`, () =>
        HttpResponse.json({ jobListing: { ...record.jobListing, archived: false, version: 'job-listing-v3' } }),
      ),
    )

    await user.click(await screen.findByRole('button', { name: 'Archive' }))
    await user.click(await screen.findByRole('button', { name: 'Unarchive' }))

    expect(await screen.findByRole('button', { name: 'Archive' })).toBeInTheDocument()
    expect(versionHeadersTo(`${DETAIL_PATH}/archive`).map((r) => r.ifMatch)).toEqual([JOB_LISTING_VERSION])
    expect(versionHeadersTo(`${DETAIL_PATH}/unarchive`).map((r) => r.ifMatch)).toEqual(['job-listing-v2'])
  })

  it('Archive_ChangedOnDisk_ShowsTheConflictAndReloadRefreshesTheRecord', async () => {
    const { user } = open(listingWithApplication())
    server.use(http.post(`${DETAIL_PATH}/archive`, () => new HttpResponse('changed on disk', { status: 409 })))

    await user.click(await screen.findByRole('button', { name: 'Archive' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('This Job Listing changed on disk since you opened it.')
    expect(screen.getByRole('button', { name: 'Archive' })).toBeInTheDocument()

    serveDetail(listingWithApplication({ archived: true }))
    await user.click(within(alert).getByRole('button', { name: 'Reload the current version' }))

    expect(await screen.findByRole('button', { name: 'Unarchive' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('Archive_Fails_SurfacesTheErrorInline', async () => {
    const { user } = open(listingWithApplication())
    server.use(http.post(`${DETAIL_PATH}/archive`, () => new HttpResponse('disk full', { status: 500 })))

    await user.click(await screen.findByRole('button', { name: 'Archive' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('disk full')
    expect(screen.getByRole('button', { name: 'Archive' })).toBeInTheDocument()
  })
})

// The "Needs attention" popover's manual fixes open the editors that
// already exist, rather than being a second way to write the same fields
// (issue #206, stories 75, 76).
describe('Fixing what needs attention', () => {
  it('NeedsAttention_EnterTheRALMyself_OpensTheRALForm', async () => {
    const { user } = open(listingWithApplication({ ral: { source: 'unresolved' } }))

    await user.click(await screen.findByRole('button', { name: /needs attention/i }))
    await user.click(await screen.findByRole('button', { name: /enter it myself/i }))

    expect(await screen.findByLabelText('Minimum')).toBeInTheDocument()
    expect(screen.getByLabelText('Maximum')).toBeInTheDocument()
  })

  it('NeedsAttention_SetTheMethodMyself_OpensTheApplicationMethodEditor', async () => {
    const { user } = open(listingWithApplication({}, { method: { kind: 'unresolved' } }))

    await user.click(await screen.findByRole('button', { name: /needs attention/i }))
    await user.click(await screen.findByRole('button', { name: /set it myself/i }))

    expect(await screen.findByRole('combobox', { name: 'Application method' })).toBeInTheDocument()
  })

  // Story 78: once both are answered the badge is gone entirely.
  it('NeedsAttention_NothingUnresolved_ShowsNoBadge', async () => {
    open(listingWithApplication({ ral: { min: 1, max: 2, currency: 'EUR', source: 'estimated' } }))

    expect(await screen.findByRole('heading', { level: 1 })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /needs attention/i })).not.toBeInTheDocument()
  })
})
