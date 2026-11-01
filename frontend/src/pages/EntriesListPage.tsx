import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { listEntries } from '@/api/client'
import type { Entry } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { formatRelativeTime } from '@/lib/utils'

export default function EntriesListPage() {
  const [entries, setEntries] = useState<Entry[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    listEntries()
      .then((e) => setEntries(e ?? []))
      .catch((e) => setError(e.message))
  }, [])

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!entries) return <p>Loading…</p>

  const experience = entries.filter((e) => e.type === 'experience')
  const projects = entries.filter((e) => e.type === 'project')

  const byEmployer = new Map<string, Entry[]>()
  for (const entry of experience) {
    const key = entry.employer ?? 'Unknown employer'
    if (!byEmployer.has(key)) byEmployer.set(key, [])
    byEmployer.get(key)!.push(entry)
  }

  return (
    <>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="mb-0">Master Data</h1>
        <div className="flex gap-2">
          <Button asChild variant="outline">
            <Link to="/usage">Usage</Link>
          </Button>
          <Button asChild>
            <Link to="/entries/new">+ New Entry</Link>
          </Button>
        </div>
      </div>

      <section>
        <h2>Experience</h2>
        {[...byEmployer.entries()].map(([employer, employerEntries]) => (
          <div className="mb-6" key={employer}>
            <h3>{employer}</h3>
            <ul className="flex flex-col gap-2">
              {employerEntries.map((entry) => (
                <EntryListItem key={entry.id} entry={entry} label={entry.client ?? entry.role ?? entry.id} />
              ))}
            </ul>
          </div>
        ))}
      </section>

      <section>
        <h2>Projects</h2>
        <ul className="flex flex-col gap-2">
          {projects.map((entry) => (
            <EntryListItem key={entry.id} entry={entry} label={entry.name ?? entry.id} />
          ))}
        </ul>
      </section>
    </>
  )
}

function EntryListItem({ entry, label }: { entry: Entry; label: string }) {
  return (
    <li className="rounded-xl border border-border bg-card px-4 py-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <Link to={`/entries/${entry.id}`} className="font-medium no-underline hover:underline">
          {label}
        </Link>
        <span className="text-sm whitespace-nowrap text-muted-foreground">
          {entry.start} – {entry.end ?? 'present'}
        </span>
      </div>
      <div className="mt-1 flex flex-wrap gap-1">
        {entry.tags.map((tag) => (
          <Badge variant="secondary" key={tag}>
            {tag}
          </Badge>
        ))}
      </div>
      {entry.lastModified?.at && (
        <p
          className="mt-1 text-xs text-muted-foreground"
          title={entry.lastModified.subject}
        >
          Edited {formatRelativeTime(entry.lastModified.at)}
        </p>
      )}
    </li>
  )
}
