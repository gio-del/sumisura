import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getGaps } from '@/api/client'
import type { Gap, GapsReport } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function GapsPage() {
  const [report, setReport] = useState<GapsReport | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    getGaps()
      .then(setReport)
      .catch((e) => setError(e.message))
  }, [])

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!report) return <p>Loading…</p>

  const uncovered = report.gaps.filter((g) => !g.covered)
  const covered = report.gaps.filter((g) => g.covered)

  return (
    <>
      <div className="mb-6">
        <h1 className="mb-1">Gaps</h1>
        <p className="mb-0 text-muted-foreground">
          Words and word pairs that recur in at least two of your non-archived Job Listings, compared with the Tags and
          text of your Entries. This is a read-only report: it never suggests text for your CV — use it to decide
          whether to write or extend an Entry.
        </p>
      </div>

      {report.gaps.length === 0 && <p>No recurring terms yet.</p>}

      {uncovered.length > 0 && (
        <section className="mb-8" aria-label="Not covered">
          <h2>Not covered</h2>
          <div className="flex flex-col gap-3">
            {uncovered.map((gap) => (
              <GapCard key={gap.term} gap={gap} />
            ))}
          </div>
        </section>
      )}

      {covered.length > 0 && (
        <section aria-label="Covered">
          <h2>Covered</h2>
          <div className="flex flex-col gap-3">
            {covered.map((gap) => (
              <GapCard key={gap.term} gap={gap} />
            ))}
          </div>
        </section>
      )}
    </>
  )
}

function GapCard({ gap }: { gap: Gap }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          {gap.term} <span className="text-sm font-normal text-muted-foreground">in {gap.count} listings</span>
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-wrap gap-1">
        {gap.listingIds.map((id) => (
          <Link key={id} to={`/jobs/${id}`} className="no-underline">
            <Badge variant="secondary">{id}</Badge>
          </Link>
        ))}
      </CardContent>
    </Card>
  )
}
