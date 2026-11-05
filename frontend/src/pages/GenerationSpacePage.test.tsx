import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import GenerationSpacePage from './GenerationSpacePage'
import type { DiskUsage } from '@/api/types'
import { renderPage } from '@/test/render'
import { server } from '@/test/server'

describe('GenerationSpacePage', () => {
  it('GenerationSpacePage_SelectSupersededAndConfirm_DeletesAndRowDisappears', async () => {
    let usage: DiskUsage = {
      totalBytes: 3072,
      generations: [
        { slug: 'acme-20260902-100000', bytes: 1024, createdAt: '2026-09-02T10:00:00Z', applicationId: 'acme', company: 'Acme', superseded: false },
        { slug: 'acme-20260901-100000', bytes: 2048, createdAt: '2026-09-01T10:00:00Z', applicationId: 'acme', company: 'Acme', superseded: true },
      ],
    }
    const deleted: string[] = []
    server.use(
      http.get('/api/generations/disk-usage', () => HttpResponse.json(usage)),
      http.delete('/api/generations/:slug', ({ params }) => {
        deleted.push(String(params.slug))
        usage = { totalBytes: 1024, generations: usage.generations.filter((g) => g.slug !== params.slug) }
        return new HttpResponse(null, { status: 204 })
      }),
    )
    renderPage(<GenerationSpacePage />, { at: '/generations/space', pattern: '/generations/space' })

    expect(await screen.findByText(/3\.0 KB/)).toBeInTheDocument()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /Select superseded \(1\)/ }))
    await user.click(screen.getByRole('button', { name: /Delete selected \(1\)/ }))
    expect(deleted).toEqual([])
    await user.click(screen.getByRole('button', { name: 'Confirm delete' }))

    await waitFor(() => expect(screen.getAllByRole('listitem')).toHaveLength(1))
    expect(deleted).toEqual(['acme-20260901-100000'])
    expect(within(screen.getByRole('listitem')).getByText('acme-20260902-100000')).toBeInTheDocument()
  })
})
