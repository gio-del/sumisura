import { describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { MemoryRouter } from 'react-router-dom'
import AppRoutes from '@/AppRoutes'
import type { AddPendingCaptureResult } from '@/api/types'
import AccessGate from '@/components/AccessGate'
import SharePage from '@/pages/SharePage'
import { listingWithApplication } from '@/test/fixtures'
import { renderPage } from '@/test/render'
import { requestsTo, server } from '@/test/server'

const LINKEDIN_SHARE =
  '/share?title=&text=' + encodeURIComponent('Check out this job at Hooli: https://www.linkedin.com/jobs/view/4012345678/')

function answer(result: AddPendingCaptureResult, status = 201) {
  server.use(http.post('/api/pending-captures', () => HttpResponse.json(result, { status })))
}

function openShare(at = LINKEDIN_SHARE) {
  return renderPage(<SharePage />, { at, pattern: '/share' })
}

// The Web Share Target (issue #185): what a job shared to the installed app
// sends, and each outcome the user is told about.
describe('SharePage', () => {
  it('SharePage_SendsWhatWasSharedOnceAndConfirmsPending', async () => {
    answer({
      outcome: 'pending',
      message: 'Saved to To complete.',
      pendingCapture: {
        schemaVersion: 1,
        id: 'linkedin-d74bc7f6d6aa',
        url: 'https://www.linkedin.com/jobs/view/4012345678/',
        postingKey: 'linkedin:4012345678',
        provider: 'linkedin',
        company: 'Hooli',
        savedAt: '2026-09-17T10:00:00Z',
      },
    })
    openShare()

    const status = await screen.findByText('Saved to To complete.')
    expect(within(status.closest('[role=status]') as HTMLElement).getByRole('link')).toHaveAttribute('href', '/inbox')
    expect(screen.getByText('Hooli')).toBeInTheDocument()
    const requests = await requestsTo('/api/pending-captures')
    expect(requests).toHaveLength(1)
    expect(requests[0].body).toEqual({ text: 'Check out this job at Hooli: https://www.linkedin.com/jobs/view/4012345678/' })
  })

  // Issue #200: the LinkedIn app shares only the link, so the share screen
  // lets the job be finished right there, with Company and Job Title read
  // out of the pasted description.
  it('SharePage_BareLink_FinishOnThePhoneWithSuggestedCompanyAndTitle', async () => {
    const saved = { ...listingWithApplication(), duplicateWarning: undefined }
    answer({
      outcome: 'pending',
      message: 'Saved to To complete.',
      pendingCapture: {
        schemaVersion: 1,
        id: 'linkedin-73a7fd2fa6a3',
        url: 'https://www.linkedin.com/jobs/view/4459189120/',
        postingKey: 'linkedin:4459189120',
        provider: 'linkedin',
        savedAt: '2026-09-19T07:34:01Z',
      },
    })
    server.use(
      http.post('/api/pending-captures/linkedin-73a7fd2fa6a3/hints', () =>
        HttpResponse.json({ company: 'Qonto', title: 'Analytics Engineer' }),
      ),
      http.post('/api/pending-captures/linkedin-73a7fd2fa6a3/complete', () => HttpResponse.json(saved, { status: 201 })),
    )
    const { user } = openShare('/share?url=' + encodeURIComponent('https://www.linkedin.com/jobs/view/4459189120/'))

    const finish = await screen.findByRole('region', { name: 'Finish now' })
    expect(within(finish).getByRole('link', { name: 'Open the posting ↗' })).toHaveAttribute(
      'href',
      'https://www.linkedin.com/jobs/view/4459189120/',
    )
    expect(within(finish).getByRole('link', { name: 'Later' })).toHaveAttribute('href', '/inbox')
    await user.click(within(finish).getByLabelText('Job Description'))
    await user.paste('Qonto is hiring an Analytics Engineer in Milan.')

    expect(await within(finish).findByDisplayValue('Qonto')).toBeInTheDocument()
    expect(within(finish).getByLabelText('Job Title (optional)')).toHaveValue('Analytics Engineer')
    const [hints] = await requestsTo('/api/pending-captures/linkedin-73a7fd2fa6a3/hints')
    expect(hints.body).toEqual({ jobDescription: 'Qonto is hiring an Analytics Engineer in Milan.' })

    await user.click(within(finish).getByRole('button', { name: 'Save Job Listing' }))
    expect(await screen.findByText('Saved as a Job Listing.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open the Job Listing →' })).toHaveAttribute('href', `/jobs/${saved.jobListing.id}`)
    const [completed] = await requestsTo('/api/pending-captures/linkedin-73a7fd2fa6a3/complete')
    expect(completed.body).toEqual({
      company: 'Qonto',
      title: 'Analytics Engineer',
      jobDescription: 'Qonto is hiring an Analytics Engineer in Milan.',
    })
  })

  it('SharePage_ATSPostingSaved_LinksTheJobListing', async () => {
    answer({ outcome: 'job-listing', message: 'Saved as a Job Listing.', jobListingId: 'hooli' })
    openShare('/share?url=' + encodeURIComponent('https://boards.greenhouse.io/hooli/jobs/4567890'))

    await screen.findByText('Saved as a Job Listing.')
    expect(screen.getByRole('link', { name: 'Open the Job Listing →' })).toHaveAttribute('href', '/jobs/hooli')
    const [req] = await requestsTo('/api/pending-captures')
    expect(req.body).toEqual({ url: 'https://boards.greenhouse.io/hooli/jobs/4567890' })
  })

  it('SharePage_AlreadyTracked_LinksTheExistingJobListing', async () => {
    answer({ outcome: 'already-tracked', message: 'Already tracked as a Job Listing.', jobListingId: 'hooli' }, 200)
    openShare()

    await screen.findByText('Already tracked as a Job Listing.')
    expect(screen.getByRole('link', { name: 'Open the Job Listing →' })).toHaveAttribute('href', '/jobs/hooli')
  })

  it('SharePage_Failure_ShowsErrorAndRetries', async () => {
    let calls = 0
    server.use(
      http.post('/api/pending-captures', () => {
        calls++
        return calls === 1
          ? new HttpResponse('validation failed: no http(s) link found in what was shared', { status: 400 })
          : HttpResponse.json({ outcome: 'already-pending', message: 'Already waiting in To complete.' }, { status: 200 })
      }),
    )
    const { user } = openShare()

    expect(await screen.findByRole('alert')).toHaveTextContent('no http(s) link found')
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByText('Already waiting in To complete.')).toBeInTheDocument()
    expect(calls).toBe(2)
  })

  it('SharePage_DeviceWithoutAccess_EntersTokenThenCompletesTheShare', async () => {
    server.use(
      http.get('/api/auth/status', () => HttpResponse.json({ required: true, authenticated: false })),
      http.post('/api/auth/session', () => new HttpResponse(null, { status: 204 })),
    )
    answer({ outcome: 'already-pending', message: 'Already waiting in To complete.' }, 200)
    const user = userEvent.setup()
    render(
      <AccessGate>
        <MemoryRouter initialEntries={[LINKEDIN_SHARE]}>
          <AppRoutes />
        </MemoryRouter>
      </AccessGate>,
    )

    await user.type(await screen.findByLabelText('Access token'), 's3cret')
    expect(await requestsTo('/api/pending-captures')).toHaveLength(0)
    await user.click(screen.getByRole('button', { name: 'Continue' }))

    expect(await screen.findByText('Already waiting in To complete.')).toBeInTheDocument()
    expect(await requestsTo('/api/pending-captures')).toHaveLength(1)
  })
})
