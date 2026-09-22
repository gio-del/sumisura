import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import JobListingCreatePage from './JobListingCreatePage'
import { renderPage } from '@/test/render'
import { requestsTo, server } from '@/test/server'

// One Job Listing per posting (issue #206, stories 5 and 8): the manual save
// form refuses a posting already tracked, names the record that holds it and
// makes checking it one click — and one more click to bring it back when it
// turned out to be archived.

const EXISTING = {
  id: 'acme',
  title: 'Senior Backend Engineer',
  company: 'Acme',
  savedAt: '2026-09-14T08:30:00Z',
  archived: false,
}

function serveRefusal(existing: typeof EXISTING) {
  server.use(
    http.post('/api/job-listings', () =>
      HttpResponse.json(
        { reason: 'duplicate-posting', message: 'This posting is already saved as a Job Listing.', existing },
        { status: 409 },
      ),
    ),
  )
}

async function submitADuplicate(existing = EXISTING) {
  serveRefusal(existing)
  const { user } = renderPage(<JobListingCreatePage />, { at: '/jobs/new', pattern: '/jobs/new' })

  await user.type(screen.getByLabelText('Company'), 'Acme')
  await user.type(screen.getByLabelText('Posting URL (optional)'), 'https://www.linkedin.com/jobs/view/4012345678/')
  await user.type(screen.getByLabelText('Job Description (paste text)'), 'A backend role.')
  await user.click(screen.getByRole('button', { name: 'Save Job Listing' }))
  return { user }
}

describe('A posting already tracked', () => {
  it('DuplicatePosting_Refused_NamesTheExistingListingAndWhenItWasSaved', async () => {
    await submitADuplicate()

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('already saved')
    expect(alert).toHaveTextContent('Senior Backend Engineer')
    expect(alert).toHaveTextContent('Acme')
    expect(alert).toHaveTextContent(new Date(EXISTING.savedAt).toLocaleDateString())
  })

  it('DuplicatePosting_Refused_LinksStraightToTheExistingListing', async () => {
    await submitADuplicate()

    const link = await screen.findByRole('link', { name: /existing Job Listing|Open it/i })
    expect(link).toHaveAttribute('href', '/jobs/acme')
  })

  it('DuplicatePosting_Refused_DoesNotShowTheRawErrorBody', async () => {
    await submitADuplicate()

    await screen.findByRole('alert')
    expect(screen.queryByText(/duplicate-posting/)).not.toBeInTheDocument()
  })

  it('DuplicatePosting_LiveMatch_OffersNoUnarchive', async () => {
    await submitADuplicate()

    await screen.findByRole('alert')
    expect(screen.queryByRole('button', { name: /bring it back|unarchive/i })).not.toBeInTheDocument()
  })

  it('DuplicatePosting_ArchivedMatch_OffersToBringItBack', async () => {
    const { user } = await submitADuplicate({ ...EXISTING, archived: true })

    const alert = await screen.findByRole('alert')
    // Story 7: the refusal itself reads the same; only the offer is extra.
    expect(alert).toHaveTextContent('already saved')

    server.use(
      http.post('/api/job-listings/acme/unarchive', () =>
        HttpResponse.json({ jobListing: { id: 'acme', company: 'Acme', archived: false } }),
      ),
    )
    await user.click(screen.getByRole('button', { name: /bring it back/i }))

    const unarchived = await requestsTo('/api/job-listings/acme/unarchive')
    expect(unarchived).toHaveLength(1)
    expect(await screen.findByText(/brought back|back in your Job Listings/i)).toBeInTheDocument()
  })
})
