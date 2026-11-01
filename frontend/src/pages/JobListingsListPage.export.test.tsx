import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import JobListingsListPage from './JobListingsListPage'
import { renderPage } from '@/test/render'
import { server } from '@/test/server'

describe('JobListingsListPage export', () => {
  it('JobListingsListPage_Loaded_OffersTheFullBackupZipAndSaysWhatItHolds', async () => {
    server.use(http.get('/api/job-listings', () => HttpResponse.json([])))

    renderPage(<JobListingsListPage />, { at: '/jobs', pattern: '/jobs' })

    const link = await screen.findByRole('link', { name: 'Export backup' })
    // No scope parameter: the unscoped export includes the Master Data.
    expect(link).toHaveAttribute('href', '/api/export')
    expect(link).toHaveAttribute('download')
    expect(link).toHaveAttribute('title', expect.stringContaining('Master Data'))
  })
})
