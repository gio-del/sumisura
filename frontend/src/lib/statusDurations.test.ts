import { describe, expect, it } from 'vitest'
import { statusDurations } from '@/lib/statusDurations'

const now = new Date('2026-11-05T12:00:00Z')

describe('statusDurations', () => {
  it('ThreeSteps_GivesWholeDaysPerStepAndDaysUntilNowForTheCurrent', () => {
    const rows = statusDurations(
      [
        { status: 'saved', changedAt: '2026-10-20T12:00:00Z' },
        { status: 'sent', changedAt: '2026-10-25T12:00:00Z' },
        { status: 'interviewing', changedAt: '2026-11-01T18:00:00Z' },
      ],
      now,
    )
    expect(rows.map((r) => r.days)).toEqual([5, 7, 3])
    expect(rows.map((r) => r.current)).toEqual([false, false, true])
  })

  it('LegacySavedOnly_IsOneCurrentRow', () => {
    const rows = statusDurations([{ status: 'saved', changedAt: '2026-11-03T00:00:00Z' }], now)
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({ days: 2, current: true })
  })

  it('SameDay_IsZeroDays', () => {
    const rows = statusDurations(
      [
        { status: 'saved', changedAt: '2026-11-05T08:00:00Z' },
        { status: 'tailoring', changedAt: '2026-11-05T09:00:00Z' },
      ],
      now,
    )
    expect(rows.map((r) => r.days)).toEqual([0, 0])
  })

  it('AbsentOrEmpty_GivesNoRows', () => {
    expect(statusDurations(undefined, now)).toEqual([])
    expect(statusDurations([], now)).toEqual([])
  })

  it('UnorderedInput_IsSortedOldestFirst', () => {
    const rows = statusDurations(
      [
        { status: 'sent', changedAt: '2026-11-01T12:00:00Z' },
        { status: 'saved', changedAt: '2026-10-30T12:00:00Z' },
      ],
      now,
    )
    expect(rows.map((r) => r.change.status)).toEqual(['saved', 'sent'])
  })
})
