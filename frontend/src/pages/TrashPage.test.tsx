import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import type { TrashItem } from '@/api/types'
import { entry } from '@/test/fixtures'
import { renderApp } from '@/test/render'
import { server } from '@/test/server'

const ENTRY_PATH = '/api/master-data/entries/experience/acme-backend'

describe('Trash', () => {
  it('Trash_DeleteAnEntryThenRestoreIt_ListsItInTheTrashAndBringsItBack', async () => {
    const trash: TrashItem[] = []
    server.use(
      http.get(ENTRY_PATH, () => HttpResponse.json(entry({ id: 'experience/acme-backend' }))),
      http.get('/api/master-data/entries', () => HttpResponse.json([])),
      http.delete(ENTRY_PATH, () => {
        trash.push({
          id: 'experience~acme-backend-20260102T030405.000000000Z',
          kind: 'entry',
          originalId: 'experience/acme-backend',
          label: 'Globex',
          trashedAt: '2026-01-02T03:04:05Z',
          content: '---\nemployer: Globex\n---\n',
        })
        return new HttpResponse(null, { status: 204 })
      }),
      http.get('/api/master-data/trash', () => HttpResponse.json({ items: trash })),
      http.post('/api/master-data/trash/:id/restore', ({ params }) => {
        trash.splice(0, trash.length)
        return HttpResponse.json({ id: 'experience/acme-backend', kind: 'entry', restoredFrom: params.id })
      }),
    )

    const first = renderApp({ at: '/entries/experience/acme-backend' })
    await first.user.click(await screen.findByRole('button', { name: 'Delete' }))
    await first.user.click(await screen.findByRole('button', { name: 'Yes, delete' }))
    await screen.findByRole('heading', { name: 'Master Data' })
    expect(trash).toHaveLength(1)
    first.unmount()

    const second = renderApp({ at: '/trash' })
    expect(await screen.findByText(/experience\/acme-backend, trashed/)).toBeInTheDocument()
    await second.user.click(screen.getByRole('button', { name: 'Restore' }))

    expect(await screen.findByText('The trash is empty.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'experience/acme-backend' })).toHaveAttribute(
      'href',
      '/entries/experience/acme-backend',
    )
  })

  it('Trash_DeletePermanently_RemovesTheItem', async () => {
    let items: TrashItem[] = [
      {
        id: 'cover-letter-snippets~closing-20260102T030405.000000000Z',
        kind: 'snippet',
        originalId: 'closing',
        label: 'closing',
        trashedAt: '2026-01-02T03:04:05Z',
        content: 'Thanks.',
      },
    ]
    server.use(
      http.get('/api/master-data/trash', () => HttpResponse.json({ items })),
      http.delete('/api/master-data/trash/:id', () => {
        items = []
        return new HttpResponse(null, { status: 204 })
      }),
    )

    const { user } = renderApp({ at: '/trash' })
    await user.click(await screen.findByRole('button', { name: 'Delete permanently' }))

    expect(await screen.findByText('The trash is empty.')).toBeInTheDocument()
  })
})
