import { describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import JobListingsListPage from './JobListingsListPage'
import { listingSummaryWithApplication } from '@/test/fixtures'
import { currentSearch, renderPage } from '@/test/render'
import { server } from '@/test/server'

// Tabs and the collapsible filters panel (issue #308).

function open(at = '/jobs') {
  server.use(
    http.get('/api/job-listings', () => HttpResponse.json([listingSummaryWithApplication({ id: 'acme', company: 'Acme' })])),
  )
  return renderPage(<JobListingsListPage />, { at, pattern: '/jobs' })
}

describe('Job Listings tabs and filters panel', () => {
  it('FiltersButton_ToggledRepeatedly_OpensAndClosesThePanel', async () => {
    const { user } = open()
    await screen.findByText('Acme')
    const button = screen.getByRole('button', { name: 'Filters · 0' })
    expect(screen.queryByLabelText('Company')).not.toBeInTheDocument()
    for (let i = 0; i < 3; i++) {
      await user.click(button)
      expect(screen.getByLabelText('Company')).toBeInTheDocument()
      expect(button).toHaveAttribute('aria-expanded', 'true')
      await user.click(button)
      expect(screen.queryByLabelText('Company')).not.toBeInTheDocument()
    }
  })

  it('FiltersButton_Count_CountsPanelFiltersButNotTheTabs', async () => {
    open('/jobs?status=sent&company=acme&archived=only&priority=high&ralMin=1')
    await screen.findByText('Acme')
    // status, company and RAL; the Archived tab is not a filter
    expect(screen.getByRole('button', { name: 'Filters · 3' })).toBeInTheDocument()
  })

  it('Tabs_Chosen_WriteTheViewToTheUrl', async () => {
    const { user } = open()
    await screen.findByText('Acme')
    expect(screen.getByRole('tab', { name: 'Active' })).toHaveAttribute('aria-selected', 'true')

    await user.click(screen.getByRole('tab', { name: 'High priority' }))
    await waitFor(() => expect(currentSearch()).toBe('?priority=high'))
    expect(screen.getByRole('tab', { name: 'High priority' })).toHaveAttribute('aria-selected', 'true')

    await user.click(screen.getByRole('tab', { name: 'Archived' }))
    await waitFor(() => expect(currentSearch()).toBe('?archived=only'))

    await user.click(screen.getByRole('tab', { name: 'Active' }))
    await waitFor(() => expect(currentSearch()).toBe(''))
  })
})
