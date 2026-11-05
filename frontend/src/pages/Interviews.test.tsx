import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import JobListingDetailPage from './JobListingDetailPage'
import type { JobListingWithApplication } from '@/api/types'
import { application, listingWithApplication } from '@/test/fixtures'
import { renderPage } from '@/test/render'
import { requestsTo, server } from '@/test/server'

function open(record: JobListingWithApplication) {
  server.use(http.get('/api/job-listings/acme', () => HttpResponse.json(record)))
  return renderPage(<JobListingDetailPage />, { at: '/jobs/acme', pattern: '/jobs/:id' })
}

describe('interviews', () => {
  it('InterviewEditor_AddingOne_SendsTheWholeList', async () => {
    const { user } = open(listingWithApplication({}, { status: 'interviewing' }))
    server.use(
      http.patch('/api/applications/acme/interviews', () =>
        HttpResponse.json(application({ status: 'interviewing', interviews: [{ at: '2099-01-20T14:30', note: 'tecnico' }] })),
      ),
    )

    await user.click(await screen.findByRole('button', { name: 'Add interview' }))
    await user.type(screen.getByLabelText('Interview date and time'), '2099-01-20T14:30')
    await user.type(screen.getByLabelText('Interview note'), 'tecnico')
    await user.click(screen.getByRole('button', { name: 'Save interview' }))

    expect((await requestsTo('/api/applications/acme/interviews'))[0].body).toEqual({
      interviews: [{ at: '2099-01-20T14:30', note: 'tecnico' }],
    })
    expect(await screen.findByText('20/01 14:30')).toBeInTheDocument()
  })

  it('InterviewEditor_EditingAndDeleting_ReplacesAndClears', async () => {
    const { user } = open(
      listingWithApplication({}, { status: 'interviewing', interviews: [{ at: '2099-01-20T14:30', note: 'tecnico' }] }),
    )
    server.use(
      http.patch('/api/applications/acme/interviews', () => HttpResponse.json(application({ status: 'interviewing' }))),
    )

    await user.click(await screen.findByRole('button', { name: 'Edit' }))
    const note = screen.getByLabelText('Interview note')
    await user.clear(note)
    await user.type(note, 'con il CTO')
    await user.click(screen.getByRole('button', { name: 'Save interview' }))
    expect((await requestsTo('/api/applications/acme/interviews'))[0].body).toEqual({
      interviews: [{ at: '2099-01-20T14:30', note: 'con il CTO' }],
    })
  })

  it('InterviewEditor_Deleting_SendsAnEmptyList', async () => {
    const { user } = open(
      listingWithApplication({}, { status: 'interviewing', interviews: [{ at: '2099-01-20T14:30' }] }),
    )
    server.use(
      http.patch('/api/applications/acme/interviews', () => HttpResponse.json(application({ status: 'interviewing' }))),
    )

    await user.click(await screen.findByRole('button', { name: 'Delete' }))

    expect((await requestsTo('/api/applications/acme/interviews'))[0].body).toEqual({ interviews: [] })
    expect(await screen.findByText('none')).toBeInTheDocument()
  })
})
