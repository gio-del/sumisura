import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import type { ImportPreview } from '@/api/types'
import ImportPage from '@/pages/ImportPage'
import { renderPage } from '@/test/render'
import { server } from '@/test/server'

const preview: ImportPreview = {
  skipped: 0,
  drafts: [
    {
      entry: {
        id: 'experience/acme-corp',
        type: 'experience',
        employer: 'Acme Corp',
        role: 'Backend Engineer',
        start: '2019-03',
        end: '2021',
        tags: ['Go'],
        bullets: ['Built a Go HTTP API', 'Led a team of twelve'],
      },
      notInText: ['Led a team of twelve'],
    },
  ],
}

async function proposeFrom(text: string) {
  server.use(http.post('/api/master-data/import/preview', () => HttpResponse.json(preview)))
  const view = renderPage(<ImportPage />, { at: '/import', pattern: '/import' })
  await view.user.click(screen.getByLabelText('CV text'))
  await view.user.paste(text)
  await view.user.click(screen.getByRole('button', { name: 'Propose Entries' }))
  return view
}

describe('ImportPage', () => {
  it('ImportPage_PreviewsDraftsFlaggingBulletsNotInTheText', async () => {
    await proposeFrom('Acme Corp 2019-2021')
    expect(await screen.findByText('Backend Engineer at Acme Corp')).toBeInTheDocument()
    expect(screen.getByText('not in the text')).toBeInTheDocument()
  })

  it('ImportPage_SavesADraftOnlyWhenTheUserApprovesIt', async () => {
    let saved = 0
    server.use(
      http.post('/api/master-data/import/entries', async () => {
        saved++
        return HttpResponse.json({ ...preview.drafts[0].entry, version: 'v1' }, { status: 201 })
      }),
    )
    const view = await proposeFrom('Acme Corp 2019-2021')
    await screen.findByText('Backend Engineer at Acme Corp')
    expect(saved).toBe(0)

    await view.user.click(screen.getByRole('button', { name: 'Save Backend Engineer at Acme Corp' }))

    expect(await screen.findByRole('link', { name: 'experience/acme-corp' })).toBeInTheDocument()
    expect(saved).toBe(1)
  })

  it('ImportPage_ShowsTheConflictWhenTheIdAlreadyExists', async () => {
    server.use(
      http.post('/api/master-data/import/entries', () => new HttpResponse('an entry with this id already exists', { status: 409 })),
    )
    const view = await proposeFrom('Acme Corp 2019-2021')
    await view.user.click(await screen.findByRole('button', { name: 'Save Backend Engineer at Acme Corp' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('already exists')
  })
})
