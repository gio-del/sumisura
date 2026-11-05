import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import EntriesListPage from './EntriesListPage'
import SnippetsListPage from './SnippetsListPage'
import { entry } from '@/test/fixtures'
import { currentSearch, renderPage } from '@/test/render'
import { server } from '@/test/server'

function seedEntries() {
  server.use(
    http.get('/api/entries', () =>
      HttpResponse.json([
        entry({ id: 'a', client: 'Alpha', employer: 'Globex', tags: ['go', 'sql'] }),
        entry({ id: 'b', client: 'Beta', employer: 'Globex', tags: ['go'] }),
      ]),
    ),
  )
}

describe('Entries filters', () => {
  it('SearchTagsAndClear_NarrowThenRestoreTheList', async () => {
    seedEntries()
    const { user } = renderPage(<EntriesListPage />, { at: '/entries' , pattern: '/entries' })
    await screen.findByText('Alpha')

    await user.type(screen.getByRole('searchbox'), 'BETA')
    expect(screen.queryByText('Alpha')).toBeNull()
    expect(screen.getByText('Beta')).toBeTruthy()
    expect(currentSearch()).toBe('?q=BETA')

    await user.clear(screen.getByRole('searchbox'))
    await user.click(screen.getByRole('button', { name: 'sql' }))
    expect(screen.queryByText('Beta')).toBeNull()
    expect(currentSearch()).toBe('?tag=sql')

    await user.click(screen.getByRole('button', { name: 'go' }))
    expect(currentSearch()).toBe('?tag=sql&tag=go')
    await user.type(screen.getByRole('searchbox'), 'zzz')
    await user.click(await screen.findByText('Azzera filtri'))
    expect(await screen.findByText('Beta')).toBeTruthy()
    expect(currentSearch()).toBe('')
  })

  it('UrlRestoresTheFilter', async () => {
    seedEntries()
    renderPage(<EntriesListPage />, { at: '/entries?tag=sql', pattern: '/entries' })
    await screen.findByText('Alpha')
    expect(screen.queryByText('Beta')).toBeNull()
  })
})

describe('Snippets filters', () => {
  it('KindAndBodySearch', async () => {
    server.use(
      http.get('/api/snippets', () =>
        HttpResponse.json([
          { id: 's1', kind: 'opening', tags: ['x'], body: 'Hello pipelines', version: 'v' },
          { id: 's2', kind: 'closing', tags: [], body: 'Thanks', version: 'v' },
        ]),
      ),
    )
    const { user } = renderPage(<SnippetsListPage />, { at: '/snippets', pattern: '/snippets' })
    await screen.findByText('Thanks')
    await user.type(screen.getByRole('searchbox'), 'pipelines')
    expect(screen.queryByText('Thanks')).toBeNull()
    await user.clear(screen.getByRole('searchbox'))
    await user.click(screen.getByRole('button', { name: 'closing' }))
    expect(screen.queryByText(/Hello/)).toBeNull()
    expect(currentSearch()).toBe('?kind=closing')
  })
})
