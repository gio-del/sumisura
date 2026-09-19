import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import type { PendingCapture, SaveJobListingResult } from '@/api/types'
import PendingCapturesPage from '@/pages/PendingCapturesPage'
import { listingWithApplication } from '@/test/fixtures'
import { renderPage } from '@/test/render'
import { requestsTo, server } from '@/test/server'

function capture(overrides: Partial<PendingCapture> = {}): PendingCapture {
  return {
    schemaVersion: 1,
    id: 'linkedin-d74bc7f6d6aa',
    url: 'https://www.linkedin.com/jobs/view/4012345678/',
    postingKey: 'linkedin:4012345678',
    provider: 'linkedin',
    title: 'Backend Engineer',
    company: 'Hooli',
    savedAt: '2026-09-17T10:00:00Z',
    ...overrides,
  }
}

function showInbox(captures: PendingCapture[]) {
  server.use(http.get('/api/pending-captures', () => HttpResponse.json(captures)))
  return renderPage(<PendingCapturesPage />, { at: '/inbox', pattern: '/inbox' })
}

// The To complete inbox (issue #182): what was shared from a phone, and the
// two ways out of it — completed into a Job Listing, or dismissed.
describe('PendingCapturesPage', () => {
  it('PendingCapturesPage_Empty_ExplainsHowLinksArrive', async () => {
    showInbox([])

    expect(await screen.findByText(/Nothing waiting/)).toBeInTheDocument()
  })

  it('PendingCapturesPage_ListsCapturesWithHintsProviderAndLink', async () => {
    showInbox([capture(), capture({ id: 'other-0123456789ab', provider: 'other', title: undefined, company: undefined, url: 'https://www.example.com/careers/42' })])

    const hinted = await screen.findByRole('listitem', { name: 'Backend Engineer · Hooli' })
    expect(within(hinted).getByText('LinkedIn')).toBeInTheDocument()
    expect(within(hinted).getByRole('link')).toHaveAttribute('href', 'https://www.linkedin.com/jobs/view/4012345678/')
    const bare = screen.getByRole('listitem', { name: 'example.com' })
    expect(within(bare).getByText('Link')).toBeInTheDocument()
  })

  it('PendingCapturesPage_Complete_SendsConfirmedFieldsAndLinksTheJobListing', async () => {
    const saved: SaveJobListingResult = { ...listingWithApplication(), duplicateWarning: undefined }
    server.use(http.post('/api/pending-captures/linkedin-d74bc7f6d6aa/complete', () => HttpResponse.json(saved, { status: 201 })))
    const { user } = showInbox([capture()])

    const item = await screen.findByRole('listitem', { name: 'Backend Engineer · Hooli' })
    await user.click(within(item).getByRole('button', { name: 'Complete' }))
    expect(within(item).getByLabelText('Company')).toHaveValue('Hooli')
    await user.clear(within(item).getByLabelText('Job Title (optional)'))
    await user.type(within(item).getByLabelText('Job Title (optional)'), 'Senior Backend Engineer')
    await user.type(within(item).getByLabelText('Job Description'), 'Build backends.')
    await user.click(within(item).getByRole('button', { name: 'Save Job Listing' }))

    const status = await screen.findByRole('status')
    expect(within(status).getByRole('link')).toHaveAttribute('href', `/jobs/${saved.jobListing.id}`)
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
    const [req] = await requestsTo('/api/pending-captures/linkedin-d74bc7f6d6aa/complete')
    expect(req.body).toEqual({ company: 'Hooli', title: 'Senior Backend Engineer', jobDescription: 'Build backends.' })
  })

  it('PendingCapturesPage_CompleteRejected_ShowsErrorAndKeepsItem', async () => {
    server.use(
      http.post('/api/pending-captures/linkedin-d74bc7f6d6aa/complete', () =>
        new HttpResponse('validation failed: company is required', { status: 400 }),
      ),
    )
    const { user } = showInbox([capture()])

    const item = await screen.findByRole('listitem', { name: 'Backend Engineer · Hooli' })
    await user.click(within(item).getByRole('button', { name: 'Complete' }))
    await user.type(within(item).getByLabelText('Job Description'), 'Build backends.')
    await user.click(within(item).getByRole('button', { name: 'Save Job Listing' }))

    expect(await within(item).findByRole('alert')).toHaveTextContent('company is required')
    expect(screen.getByRole('listitem', { name: 'Backend Engineer · Hooli' })).toBeInTheDocument()
  })

  it('PendingCapturesPage_Dismiss_DeletesAndRemovesItem', async () => {
    server.use(http.delete('/api/pending-captures/linkedin-d74bc7f6d6aa', () => new HttpResponse(null, { status: 204 })))
    const { user } = showInbox([capture()])

    const item = await screen.findByRole('listitem', { name: 'Backend Engineer · Hooli' })
    await user.click(within(item).getByRole('button', { name: 'Dismiss' }))

    expect(await screen.findByText(/Nothing waiting/)).toBeInTheDocument()
    expect((await requestsTo('/api/pending-captures/linkedin-d74bc7f6d6aa')).map((r) => r.method)).toEqual(['DELETE'])
  })

  it('PendingCapturesPage_Suggest_FillsOnlyEmptyFields', async () => {
    server.use(
      http.post('/api/pending-captures/linkedin-d74bc7f6d6aa/hints', () =>
        HttpResponse.json({ company: 'Hooli Inc.', title: 'Platform Engineer' }),
      ),
    )
    const { user } = showInbox([capture({ title: undefined })])

    const item = await screen.findByRole('listitem', { name: 'Hooli' })
    await user.click(within(item).getByRole('button', { name: 'Complete' }))
    await user.type(within(item).getByLabelText('Job Description'), 'Build backends.')
    await user.click(within(item).getByRole('button', { name: 'Suggest Company and Title' }))

    expect(await within(item).findByDisplayValue('Platform Engineer')).toBeInTheDocument()
    expect(within(item).getByLabelText('Company')).toHaveValue('Hooli')
  })

  it('PendingCapturesPage_SuggestFails_SaysSoAndStillSaves', async () => {
    const saved: SaveJobListingResult = { ...listingWithApplication(), duplicateWarning: undefined }
    server.use(
      http.post('/api/pending-captures/linkedin-d74bc7f6d6aa/hints', () => new HttpResponse('overloaded', { status: 502 })),
      http.post('/api/pending-captures/linkedin-d74bc7f6d6aa/complete', () => HttpResponse.json(saved, { status: 201 })),
    )
    const { user } = showInbox([capture({ title: undefined, company: undefined })])

    const item = await screen.findByRole('listitem', { name: 'linkedin.com' })
    await user.click(within(item).getByRole('button', { name: 'Complete' }))
    await user.type(within(item).getByLabelText('Job Description'), 'Build backends.')
    await user.click(within(item).getByRole('button', { name: 'Suggest Company and Title' }))

    expect(await within(item).findByText(/Couldn.t suggest them this time/)).toBeInTheDocument()
    await user.type(within(item).getByLabelText('Company'), 'Hooli')
    await user.click(within(item).getByRole('button', { name: 'Save Job Listing' }))
    expect(await screen.findByRole('status')).toHaveTextContent('Saved as a Job Listing')
  })
})
