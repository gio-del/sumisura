import { useEffect, useState } from 'react'
import { getApplicationsStats } from '@/api/client'
import type { ApplicationStats } from '@/api/types'
import { statusLabel } from '@/lib/applicationStatus'

export default function StatsPage() {
  const [stats, setStats] = useState<ApplicationStats | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getApplicationsStats()
      .then(setStats)
      .catch((e) => setError(e.message))
  }, [])

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!stats) return <p>Loading…</p>

  return (
    <>
      <h1>Application Funnel</h1>

      {stats.total === 0 && <p>No tracked Applications yet — save a Job Listing to see stats here.</p>}

      <section className="mb-8">
        <h2>Status counts</h2>
        <ul className="flex flex-col gap-2">
          {stats.counts.map((c) => (
            <li
              key={c.status}
              className="flex items-center justify-between rounded-xl border border-border bg-card px-4 py-3"
            >
              <span className="font-medium">{statusLabel[c.status]}</span>
              <span>{c.count}</span>
            </li>
          ))}
        </ul>
      </section>

      {stats.conversions.length > 0 && (
        <section className="mb-8">
          <h2>Conversion rates</h2>
          <ul className="flex flex-col gap-2">
            {stats.conversions.map((c) => (
              <li
                key={`${c.from}-${c.to}-${c.without ?? ''}`}
                className="flex items-center justify-between rounded-xl border border-border bg-card px-4 py-3"
              >
                <span className="font-medium">
                  {statusLabel[c.from]} → {statusLabel[c.to]}
                  {c.without && ` without ${statusLabel[c.without]}`}
                </span>
                <span>{Math.round(c.rate * 100)}%</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {stats.timeInStage.length > 0 && (
        <section>
          <h2>Average time in stage</h2>
          <ul className="flex flex-col gap-2">
            {stats.timeInStage.map((s) => (
              <li
                key={s.status}
                className="flex items-center justify-between rounded-xl border border-border bg-card px-4 py-3"
              >
                <span className="font-medium">{statusLabel[s.status]}</span>
                <span>
                  {s.averageDays.toFixed(1)} days (n={s.sampleSize})
                </span>
              </li>
            ))}
          </ul>
        </section>
      )}
    </>
  )
}
