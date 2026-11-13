import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AppNav from '@/components/AppNav'
import { server } from '@/test/server'

function setDesktop(matches: boolean) {
  vi.stubGlobal(
    'matchMedia',
    vi.fn().mockImplementation(() => ({
      matches,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  )
}

function renderNav(path = '/') {
  server.use(http.get('/api/usage', () => HttpResponse.json({ estimatedCostUsd: 1.5 })))
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppNav />
    </MemoryRouter>,
  )
}

const ALL = [
  'Today', 'Job Listings', 'To complete', 'Applications', 'Offers', 'Browse ATS Boards',
  'Master Data', 'Profile', 'Cover Letter Snippets', 'Import', 'Trash',
  'Generate', 'Generated CVs', 'Gaps', 'Tag Lint', 'Stats',
]

describe('AppNav', () => {
  beforeEach(() => setDesktop(true))
  afterEach(() => vi.unstubAllGlobals())

  it('Desktop_Sidebar_ListsEveryPageInThreeGroups', async () => {
    renderNav()
    const nav = screen.getByRole('navigation', { name: 'Primary' })
    const labels = within(nav).getAllByRole('link').map((l) => l.textContent).filter((t) => t)
    for (const l of ALL) expect(labels).toContain(l)
    expect(within(nav).getAllByRole('heading').map((h) => h.textContent)).toEqual(['Job search', 'My data', 'Tools'])
    expect(await within(nav).findByText(/\$1\.50/)).toBeInTheDocument()
  })

  it('Phone_BottomBar_HasFourAreasAndMore_NothingLost', async () => {
    setDesktop(false)
    const user = userEvent.setup()
    renderNav('/applications')
    const nav = screen.getByRole('navigation', { name: 'Primary' })
    expect(within(nav).getAllByRole('link').map((l) => l.textContent)).toEqual([
      'Applications', 'Job Listings', 'Generate', 'Master Data',
    ])
    expect(screen.getByRole('link', { name: 'Applications' })).toHaveAttribute('aria-current', 'page')

    const more = screen.getByRole('button', { name: 'More' })
    expect(more).toHaveAttribute('aria-expanded', 'false')
    await user.click(more)
    expect(more).toHaveAttribute('aria-expanded', 'true')
    const menu = document.getElementById('more-menu') as HTMLElement
    const labels = within(menu).getAllByRole('link').map((l) => l.textContent)
    const bottom = ['Applications', 'Job Listings', 'Generate', 'Master Data']
    expect([...labels, ...bottom].sort()).toEqual([...ALL].sort())

    await user.click(more)
    expect(more).toHaveAttribute('aria-expanded', 'false')
    expect(document.getElementById('more-menu')).toBeNull()
    await user.click(more)
    expect(document.getElementById('more-menu')).not.toBeNull()
    await user.keyboard('{Escape}')
    expect(document.getElementById('more-menu')).toBeNull()
  })

  it('Phone_ChoosingAnItemInMore_ClosesTheMenu', async () => {
    setDesktop(false)
    const user = userEvent.setup()
    renderNav('/')
    await user.click(screen.getByRole('button', { name: 'More' }))
    await user.click(screen.getByRole('link', { name: 'Trash' }))
    expect(document.getElementById('more-menu')).toBeNull()
    expect(screen.getByRole('button', { name: 'More' })).toHaveAttribute('aria-expanded', 'false')
  })
})
