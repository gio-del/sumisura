import type { Interview } from '@/api/types'

// formatFollowUp renders an Application's followUpAt (YYYY-MM-DD) as dd/mm.
export function formatFollowUp(followUpAt: string): string {
  const [, month, day] = followUpAt.split('-')
  return `${day}/${month}`
}

// formatInterview renders an Interview's at (YYYY-MM-DDTHH:MM) as dd/mm HH:MM.
export function formatInterview(at: string): string {
  const [date, time] = at.split('T')
  return `${formatFollowUp(date)} ${time}`
}

// nextInterview is the soonest interview not before now (local time), if any.
export function nextInterview(interviews: Interview[] | undefined, now: Date = new Date()): Interview | undefined {
  const pad = (n: number) => String(n).padStart(2, '0')
  const current = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}T${pad(now.getHours())}:${pad(now.getMinutes())}`
  return (interviews ?? []).find((iv) => iv.at >= current)
}
