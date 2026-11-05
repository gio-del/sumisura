import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { duplicateDraft, readDuplicateDraft } from '@/lib/duplicateEntry'
import { entry } from '@/test/fixtures'
import { currentPath, renderApp } from '@/test/render'
import { recordedRequests, server } from '@/test/server'

const ENTRY_PATH = '/api/master-data/entries/experience/acme-backend'

describe('duplicateDraft', () => {
  it('drops identity and suffixes the title', () => {
    const d = duplicateDraft(entry({ id: 'experience/acme-backend', version: 'v1', title: 'Billing', role: 'Dev' }))
    expect(d).not.toHaveProperty('id')
    expect(d).not.toHaveProperty('version')
    expect(d.title).toBe('Billing (copia)')
  })

  it('readDuplicateDraft_NoState_ReturnsNull', () => {
    expect(readDuplicateDraft(null)).toBeNull()
    expect(readDuplicateDraft({})).toBeNull()
  })
})

describe('Duplicate an Entry', () => {
  it('opens the create form prefilled and writes nothing until saved', async () => {
    server.use(
      http.get(ENTRY_PATH, () =>
        HttpResponse.json(
          entry({ id: 'experience/acme-backend', employer: 'Acme', role: 'Backend Engineer', tags: ['go', 'sql'], bullets: ['One', 'Two'] }),
        ),
      ),
    )
    const { user } = renderApp({ at: '/entries/experience/acme-backend' })

    await user.click(await screen.findByRole('button', { name: 'Duplica' }))

    expect(await screen.findByRole('heading', { name: 'New Entry' })).toBeInTheDocument()
    expect(screen.getByLabelText('Employer')).toHaveValue('Acme')
    expect(screen.getByLabelText('Project title')).toHaveValue('Backend Engineer (copia)')
    expect(screen.getByLabelText('Tags (comma-separated)')).toHaveValue('go, sql')
    expect(screen.getByLabelText('Bullets (one per line)')).toHaveValue('One\nTwo')
    expect(currentPath()).toBe('/entries/new')
    expect((await recordedRequests()).filter((r) => r.method !== 'GET')).toHaveLength(0)

    server.use(http.post('/api/master-data/entries', () => HttpResponse.json(entry({ id: 'experience/acme-backend-2' }))))
    await user.click(screen.getByRole('button', { name: 'Create' }))
    await screen.findByRole('button', { name: 'Duplica' })
    const posts = (await recordedRequests()).filter((r) => r.method === 'POST')
    expect(posts).toHaveLength(1)
  })

  it('opening the create form directly shows it blank', async () => {
    const { user: _u } = renderApp({ at: '/entries/new' })
    expect(await screen.findByLabelText('Employer')).toHaveValue('')
  })
})
