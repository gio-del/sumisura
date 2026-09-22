import { describe, expect, it } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import JobListingFieldsEditor from './JobListingFieldsEditor'
import type { JobListing } from '@/api/types'
import { requestsTo, server, versionHeadersTo } from '@/test/server'

// Correcting a Job Listing (issue #206, stories 61-63, 69, 70). A bad
// capture is something the user fixes, not something that lives in their
// records forever.

const LISTING: JobListing = {
  schemaVersion: 2,
  id: 'acme',
  title: 'Backend Enginer',
  company: 'Acme',
  location: 'Milan',
  url: 'https://www.linkedin.com/jobs/view/4012345678/',
  source: 'manual',
  savedAt: '2026-09-20T09:12:44.102Z',
  jobDescription: 'Build Go services.',
  ral: { source: 'n/a' },
  freshnessStatus: 'not-yet-checked',
  archived: false,
  version: 'v1',
}

function renderEditor(listing: Partial<JobListing> = {}, onChange: (updated: JobListing) => void = () => {}) {
  const user = userEvent.setup()
  render(<JobListingFieldsEditor jobListing={{ ...LISTING, ...listing }} onChange={onChange} onReload={async () => {}} />)
  return { user }
}

function serveCorrection(overrides: Partial<JobListing> = {}) {
  server.use(
    http.patch('/api/job-listings/acme', async ({ request }) => {
      const body = (await request.json()) as Partial<JobListing>
      return HttpResponse.json({ jobListing: { ...LISTING, ...body, ...overrides, version: 'v2' } })
    }),
  )
}

describe('Correcting the text fields', () => {
  it('Correct_Fields_ShownAsTheyAreUntilEditingStarts', () => {
    renderEditor()

    expect(screen.getByText(/Backend Enginer/)).toBeInTheDocument()
    expect(screen.getByText(/Milan/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Job Title')).not.toBeInTheDocument()
  })

  it('Correct_Title_SendsOnlyWhatChanged', async () => {
    serveCorrection()
    const { user } = renderEditor()

    await user.click(screen.getByRole('button', { name: /correct details/i }))
    const title = screen.getByLabelText('Job Title')
    await user.clear(title)
    await user.type(title, 'Backend Engineer')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    const sent = await requestsTo('/api/job-listings/acme')
    expect(sent).toHaveLength(1)
    expect(sent[0].body).toEqual({ title: 'Backend Engineer', company: 'Acme', location: 'Milan' })
  })

  it('Correct_Company_IsSent', async () => {
    serveCorrection()
    const { user } = renderEditor()

    await user.click(screen.getByRole('button', { name: /correct details/i }))
    const company = screen.getByLabelText('Company')
    await user.clear(company)
    await user.type(company, 'Acme Rockets')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    const sent = await requestsTo('/api/job-listings/acme')
    expect((sent[0].body as { company: string }).company).toBe('Acme Rockets')
  })

  it('Correct_Location_IsSent', async () => {
    serveCorrection()
    const { user } = renderEditor()

    await user.click(screen.getByRole('button', { name: /correct details/i }))
    const location = screen.getByLabelText('Location')
    await user.clear(location)
    await user.type(location, 'Remote (EU)')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    const sent = await requestsTo('/api/job-listings/acme')
    expect((sent[0].body as { location: string }).location).toBe('Remote (EU)')
  })

  // Story 70: corrected fields show up immediately, without a reload.
  it('Correct_Saved_ShowsTheNewValueWithoutReloading', async () => {
    serveCorrection()
    let latest: JobListing | null = null
    const { user } = renderEditor({}, (updated) => {
      latest = updated
    })

    await user.click(screen.getByRole('button', { name: /correct details/i }))
    const title = screen.getByLabelText('Job Title')
    await user.clear(title)
    await user.type(title, 'Backend Engineer')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(latest).not.toBeNull())
    expect(latest!.title).toBe('Backend Engineer')
    expect(latest!.version).toBe('v2')
  })

  // Story 69: an edit is refused if someone changed the record meanwhile.
  it('Correct_RecordChangedMeanwhile_ShowsConflictAndKeepsTheDraft', async () => {
    server.use(http.patch('/api/job-listings/acme', () => new HttpResponse('changed on disk', { status: 409 })))
    const { user } = renderEditor()

    await user.click(screen.getByRole('button', { name: /correct details/i }))
    const title = screen.getByLabelText('Job Title')
    await user.clear(title)
    await user.type(title, 'Backend Engineer')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(/reload/i)
    // The draft stays, so the correction isn't lost to the conflict.
    expect(screen.getByLabelText('Job Title')).toHaveValue('Backend Engineer')
  })

  it('Correct_Save_PresentsTheVersionItRead', async () => {
    serveCorrection()
    const { user } = renderEditor()

    await user.click(screen.getByRole('button', { name: /correct details/i }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    const headers = await versionHeadersTo('/api/job-listings/acme')
    expect(headers[0].ifMatch).toBe('v1')
  })

  it('Correct_Cancelled_SendsNothingAndRestoresTheOriginal', async () => {
    const { user } = renderEditor()

    await user.click(screen.getByRole('button', { name: /correct details/i }))
    const title = screen.getByLabelText('Job Title')
    await user.clear(title)
    await user.type(title, 'Something else')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(screen.getByText(/Backend Enginer/)).toBeInTheDocument()
    expect(await requestsTo('/api/job-listings/acme')).toHaveLength(0)
  })
})

describe('Entering a RAL Range by hand', () => {
  it('RAL_Entered_IsSentAsAPlainFigure', async () => {
    serveCorrection({ ral: { min: 55000, max: 65000, currency: 'EUR', source: 'manual' } })
    const { user } = renderEditor()

    await user.click(screen.getByRole('button', { name: /enter the ral range/i }))
    await user.type(screen.getByLabelText('Minimum'), '55000')
    await user.type(screen.getByLabelText('Maximum'), '65000')
    await user.click(screen.getByRole('button', { name: 'Save RAL Range' }))

    const sent = await requestsTo('/api/job-listings/acme')
    expect(sent[0].body).toEqual({ ral: { min: 55000, max: 65000, currency: 'EUR' } })
  })

  // Story 65: the source is the backend's to set, never the client's.
  it('RAL_Entered_NeverClaimsASource', async () => {
    serveCorrection({ ral: { min: 55000, max: 65000, currency: 'EUR', source: 'manual' } })
    const { user } = renderEditor()

    await user.click(screen.getByRole('button', { name: /enter the ral range/i }))
    await user.type(screen.getByLabelText('Minimum'), '55000')
    await user.type(screen.getByLabelText('Maximum'), '65000')
    await user.click(screen.getByRole('button', { name: 'Save RAL Range' }))

    const sent = await requestsTo('/api/job-listings/acme')
    expect((sent[0].body as { ral: Record<string, unknown> }).ral.source).toBeUndefined()
  })

  it('RAL_MaxBelowMin_IsRefusedBeforeAnythingIsSent', async () => {
    const { user } = renderEditor()

    await user.click(screen.getByRole('button', { name: /enter the ral range/i }))
    await user.type(screen.getByLabelText('Minimum'), '65000')
    await user.type(screen.getByLabelText('Maximum'), '55000')
    await user.click(screen.getByRole('button', { name: 'Save RAL Range' }))

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(await requestsTo('/api/job-listings/acme')).toHaveLength(0)
  })
})
