import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { getUsage } from '@/api/client'
import type { UsageItem, UsageReport } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function UsagePage() {
  const [report, setReport] = useState<UsageReport | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [ascending, setAscending] = useState(true)

  useEffect(() => {
    getUsage()
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

  const order = (items: UsageItem[]) => (ascending ? items : [...items].reverse())

  return (
    <>
      <div className="mb-6">
        <h1 className="mb-1">Master Data usage</h1>
        <p className="mb-2 text-muted-foreground">
          How often each Entry and Cover Letter Snippet was chosen in a Generation. Read-only: it changes nothing —
          use it to decide what to rewrite, merge or remove.
        </p>
        <Button variant="outline" size="sm" onClick={() => setAscending(!ascending)}>
          {ascending ? 'Least used first' : 'Most used first'}
        </Button>
      </div>

      {report.countedGenerations === 0 ? (
        <p>No Generations to measure yet.</p>
      ) : (
        <p>
          Counted over {report.countedGenerations} Generation{report.countedGenerations === 1 ? '' : 's'}.
          {report.legacySkipped > 0 &&
            ` ${report.legacySkipped} older Generation${report.legacySkipped === 1 ? '' : 's'} recorded without usage data ${report.legacySkipped === 1 ? 'is' : 'are'} left out.`}
        </p>
      )}

      <UsageSection title="Entries" items={order(report.entries)} href={(id) => `/entries/${id}`} counted={report.countedGenerations} />
      <UsageSection title="Cover Letter Snippets" items={order(report.snippets)} href={(id) => `/snippets/${id}`} counted={report.countedGenerations} />
    </>
  )
}

function UsageSection({
  title,
  items,
  href,
  counted,
}: {
  title: string
  items: UsageItem[]
  href: (id: string) => string
  counted: number
}) {
  return (
    <section className="mb-8" aria-label={title}>
      <h2>{title}</h2>
      {items.length === 0 && <p>None yet.</p>}
      <div className="flex flex-col gap-3">
        {items.map((item) => (
          <Card key={item.id}>
            <CardHeader>
              <CardTitle>
                <Link to={href(item.id)} className="no-underline hover:underline">
                  {item.label}
                </Link>
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground">
              {item.generations === 0
                ? 'Never chosen'
                : `Chosen in ${item.generations} Generation${item.generations === 1 ? '' : 's'}` +
                  (counted > 0 ? ` (${Math.round(item.share * 100)}%)` : '') +
                  (item.lastUsedAt ? `, last on ${new Date(item.lastUsedAt).toLocaleDateString()}` : '')}
            </CardContent>
          </Card>
        ))}
      </div>
    </section>
  )
}
