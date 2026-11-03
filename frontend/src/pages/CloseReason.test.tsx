import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import type { UserEvent } from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import JobListingDetailPage from './JobListingDetailPage'
import StatsPage from './StatsPage'
import type { ApplicationStats, JobListingWithApplication } from '@/api/types'
import { application, listingWithApplication } from '@/test/fixtures'
import { renderPage } from '@/test/render'
import { requestsTo, server } from '@/test/server'

async function chooseFromSelect(user: UserEvent, selectName: string, option: string) {
  await user.click(await screen.findByRole('combobox', { name: selectName }))
  await user.click(await screen.findByRole('option', { name: option }))
}

function open(record: JobListingWithApplication) {
  server.use(http.get('/api/job-listings/acme', () => HttpResponse.json(record)))
  return renderPage(<JobListingDetailPage />, { at: '/jobs/acme', pattern: '/jobs/:id' })
}

describe('close reason', () => {
  it('StatusMove_ToRejectedWithAReason_SendsItAlongsideTheStatus', async () => {
    const { user } = open(listingWithApplication({}, { status: 'interviewing' }))
    server.use(
      http.patch('/api/applications/acme/status', () =>
        HttpResponse.json(application({ status: 'rejected', closeReason: 'rejected-after-interview' })),
      ),
    )

    await chooseFromSelect(user, 'Move Acme to a new status', 'Rejected')
    await chooseFromSelect(user, 'Close reason', 'Rejected after interview')
    await user.click(await screen.findByRole('button', { name: 'Yes, mark Rejected' }))

    expect((await requestsTo('/api/applications/acme/status'))[0].body).toEqual({
      status: 'rejected',
      closeReason: 'rejected-after-interview',
    })
    expect(await screen.findByText('Rejected after interview', { selector: '[data-slot="badge"]' })).toBeInTheDocument()
  })

  it('CloseReasonEditor_OnAClosedApplication_SetsTheReason', async () => {
    const { user } = open(listingWithApplication({}, { status: 'withdrawn' }))
    server.use(
      http.patch('/api/applications/acme/close-reason', () =>
        HttpResponse.json(application({ status: 'withdrawn', closeReason: 'salary-mismatch' })),
      ),
    )

    await chooseFromSelect(user, 'Close reason', 'Salary mismatch')

    expect((await requestsTo('/api/applications/acme/close-reason'))[0].body).toEqual({ closeReason: 'salary-mismatch' })
    expect(await screen.findByRole('combobox', { name: 'Close reason' })).toHaveTextContent('Salary mismatch')
  })

  it('CloseReasonEditor_OnAnOpenApplication_IsNotOffered', async () => {
    open(listingWithApplication({}, { status: 'sent' }))

    await screen.findByRole('heading', { level: 1, name: 'Acme' })
    expect(screen.queryByRole('combobox', { name: 'Close reason' })).not.toBeInTheDocument()
  })

  it('StatsPage_WithClosedApplications_ShowsTheCountPerReason', async () => {
    const stats: ApplicationStats = {
      total: 3,
      counts: [],
      conversions: [],
      timeInStage: [],
      closeReasons: [
        { reason: 'no-response', count: 2 },
        { reason: 'rejected-after-screening', count: 0 },
        { reason: 'rejected-after-interview', count: 0 },
        { reason: 'salary-mismatch', count: 1 },
        { reason: 'position-closed', count: 0 },
        { reason: 'other', count: 0 },
      ],
    }
    server.use(http.get('/api/applications/stats', () => HttpResponse.json(stats)))
    renderPage(<StatsPage />, { at: '/stats', pattern: '/stats' })

    const section = (await screen.findByRole('heading', { name: 'Close reasons' })).closest('section') as HTMLElement
    expect(Array.from(section.querySelectorAll('li')).map((li) => li.textContent)).toEqual([
      'No response2',
      'Rejected after screening0',
      'Rejected after interview0',
      'Salary mismatch1',
      'Position closed0',
      'Other0',
    ])
  })
})
