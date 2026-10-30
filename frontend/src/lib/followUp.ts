// formatFollowUp renders an Application's followUpAt (YYYY-MM-DD) as dd/mm.
export function formatFollowUp(followUpAt: string): string {
  const [, month, day] = followUpAt.split('-')
  return `${day}/${month}`
}
