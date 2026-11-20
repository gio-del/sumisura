import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import type { SetupCheck } from '@/api/types'
import SetupCheckPage from '@/pages/SetupCheckPage'
import TodayPage from '@/pages/TodayPage'
import { renderPage } from '@/test/render'
import { server } from '@/test/server'

const missingTypst: SetupCheck = {
  ready: false,
  items: [
    { id: 'typst', status: 'missing', required: true, hint: 'typst is not on PATH. See troubleshooting.md.' },
    { id: 'python', status: 'missing', required: false, hint: 'Optional: python3 is not installed.' },
  ],
}

describe('SetupCheckPage', () => {
  it('SetupCheckPage_ShowsTheHintOfAMissingItem', async () => {
    server.use(http.get('/api/setup-check', () => HttpResponse.json(missingTypst)))
    renderPage(<SetupCheckPage />, { at: '/setup', pattern: '/setup' })

    expect(await screen.findByText(/typst is not on PATH/)).toBeInTheDocument()
    expect(screen.getByText('Missing')).toBeInTheDocument()
    expect(screen.getByText('Missing (optional)')).toBeInTheDocument()
  })

  it('TodayPage_WarnsOnlyWhenARequiredItemIsMissing', async () => {
    server.use(
      http.get('/api/setup-check', () => HttpResponse.json(missingTypst)),
      http.get('/api/today', () =>
        HttpResponse.json({ followUps: [], interviews: [], stale: [], trackedBoards: 0, pendingCaptures: 0 }),
      ),
    )
    renderPage(<TodayPage />, { at: '/', pattern: '/' })
    expect(await screen.findByRole('link', { name: 'Check the installation' })).toBeInTheDocument()
  })

  it('TodayPage_ShowsNoWarningWhenReady', async () => {
    server.use(
      http.get('/api/setup-check', () => HttpResponse.json({ ready: true, items: [] })),
      http.get('/api/today', () =>
        HttpResponse.json({ followUps: [], interviews: [], stale: [], trackedBoards: 0, pendingCaptures: 0 }),
      ),
    )
    renderPage(<TodayPage />, { at: '/', pattern: '/' })
    await screen.findByText('Nothing needs your attention today.')
    expect(screen.queryByRole('link', { name: 'Check the installation' })).toBeNull()
  })
})
