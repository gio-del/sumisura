import type { StatusChange } from '@/api/types'

const DAY_MS = 24 * 60 * 60 * 1000

export interface StatusDuration {
  change: StatusChange
  // days is the whole days spent in this Status: until the next change, or
  // until now for the current (last) one.
  days: number
  current: boolean
}

// statusDurations turns an Application's Status history into one row per
// step, oldest first, with whole days spent in each (issue #288). Pure: now
// is passed in. A bad date counts as 0 days rather than throwing.
export function statusDurations(history: StatusChange[] | undefined, now: Date): StatusDuration[] {
  const sorted = [...(history ?? [])].sort((a, b) => Date.parse(a.changedAt) - Date.parse(b.changedAt))
  return sorted.map((change, i) => {
    const current = i === sorted.length - 1
    const end = current ? now.getTime() : Date.parse(sorted[i + 1].changedAt)
    const days = Math.floor((end - Date.parse(change.changedAt)) / DAY_MS)
    return { change, days: Number.isFinite(days) ? Math.max(0, days) : 0, current }
  })
}
