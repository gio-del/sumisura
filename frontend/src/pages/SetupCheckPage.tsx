import { useEffect, useState } from 'react'
import { getSetupCheck } from '@/api/client'
import type { SetupCheck, SetupCheckItem, SetupCheckStatus } from '@/api/types'

const titles: Record<string, string> = {
  typst: 'Typst',
  claudeKey: 'Claude API key',
  profile: 'Profile',
  entries: 'Master Data Entries',
  template: 'Templates',
  output: 'Output folder',
  python: 'Python 3.10+',
  cleanSkill: 'Prose clean-up skill',
  lan: 'LAN access token',
}

const statusText: Record<SetupCheckStatus, string> = {
  ok: 'OK',
  missing: 'Missing',
  notNeeded: 'Not needed',
}

function Row({ item }: { item: SetupCheckItem }) {
  return (
    <li className="flex flex-col gap-1 border-b py-3">
      <div className="flex flex-wrap items-baseline gap-x-3">
        <h2 className="m-0 text-base">{titles[item.id] ?? item.id}</h2>
        <span
          className={
            item.status === 'missing' && item.required
              ? 'font-medium text-destructive'
              : 'text-sm text-muted-foreground'
          }
        >
          {statusText[item.status]}
          {!item.required && ' (optional)'}
        </span>
      </div>
      <p className="m-0 text-sm text-muted-foreground">{item.hint}</p>
    </li>
  )
}

// SetupCheckPage shows GET /api/setup-check: what this installation still
// needs, with a line on what to do. Read-only; it never calls Claude.
export default function SetupCheckPage() {
  const [check, setCheck] = useState<SetupCheck | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getSetupCheck()
      .then(setCheck)
      .catch((e) => setError(e.message))
  }, [])

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!check) return <p>Loading…</p>

  return (
    <>
      <h1>Installation</h1>
      <p className="mb-2 text-muted-foreground">
        {check.ready
          ? 'Everything this install needs is in place.'
          : 'Something this install needs is missing. Fix the items marked Missing, then reload.'}
      </p>
      <ul className="m-0 list-none p-0">
        {check.items.map((item) => (
          <Row key={item.id} item={item} />
        ))}
      </ul>
    </>
  )
}
