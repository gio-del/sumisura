import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import StatusTimeline from '@/components/StatusTimeline'
import type { Application } from '@/api/types'

const base = { schemaVersion: 1, id: 'a', jobListingId: 'j', method: 'unknown', isStale: false, version: 'v' }

describe('StatusTimeline', () => {
  it('SavedSentInterviewing_ShowsThreeRowsWithDurations', () => {
    const application = {
      ...base,
      status: 'interviewing',
      statusHistory: [
        { status: 'saved', changedAt: '2026-10-20T12:00:00Z' },
        { status: 'sent', changedAt: '2026-10-25T12:00:00Z' },
        { status: 'interviewing', changedAt: '2026-11-01T12:00:00Z' },
      ],
    } as unknown as Application
    render(<StatusTimeline application={application} now={new Date('2026-11-05T12:00:00Z')} />)
    const items = within(screen.getByRole('list', { name: 'Status history' })).getAllByRole('listitem')
    expect(items).toHaveLength(3)
    expect(items[0]).toHaveTextContent('Saved')
    expect(items[0]).toHaveTextContent('5 days')
    expect(items[1]).toHaveTextContent('7 days')
    expect(items[2]).toHaveTextContent('for 4 days')
  })

  it('RejectedWithReason_ShowsTheReasonOnTheCurrentRow', () => {
    const application = {
      ...base,
      status: 'rejected',
      closeReason: 'no-response',
      statusHistory: [{ status: 'rejected', changedAt: '2026-11-04T12:00:00Z' }],
    } as unknown as Application
    render(<StatusTimeline application={application} now={new Date('2026-11-05T12:00:00Z')} />)
    expect(screen.getByRole('listitem')).toHaveTextContent('No response')
    expect(screen.getByRole('listitem')).toHaveTextContent('for 1 day')
  })

  it('NoHistory_RendersNothing', () => {
    const { container } = render(<StatusTimeline application={{ ...base, status: 'sent' } as unknown as Application} />)
    expect(container).toBeEmptyDOMElement()
  })
})
