import { useState } from 'react'
import { Link } from 'react-router-dom'
import { getJobListingCoverage } from '@/api/client'
import type { CoverageReport } from '@/api/types'
import { Button } from '@/components/ui/button'

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

/**
 * CoverageSection shows which terms of one Job Description the Master Data
 * covers, before a Generation is spent. It is deterministic (no Claude call)
 * and loads only when opened, so the page stays one request lighter.
 */
export default function CoverageSection({ jobListingId }: { jobListingId: string }) {
  const [open, setOpen] = useState(false)
  const [report, setReport] = useState<CoverageReport | null>(null)
  const [error, setError] = useState<string | null>(null)

  function toggle() {
    const next = !open
    setOpen(next)
    if (next && !report) {
      setError(null)
      getJobListingCoverage(jobListingId)
        .then(setReport)
        .catch((err) => setError(errorMessage(err)))
    }
  }

  return (
    <section className="mt-6" aria-labelledby="coverage-heading">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h2 id="coverage-heading" className="my-0">
          Coverage
        </h2>
        <Button type="button" variant="outline" size="sm" aria-expanded={open} onClick={toggle}>
          {open ? 'Hide coverage' : 'Show coverage'}
        </Button>
      </div>
      {open && error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      {open && !error && !report && <p className="text-sm text-muted-foreground">Checking…</p>}
      {open && report && <CoverageBody report={report} />}
    </section>
  )
}

function CoverageBody({ report }: { report: CoverageReport }) {
  if (report.covered.length === 0 && report.uncovered.length === 0) {
    return <p className="text-sm text-muted-foreground">Nothing to compare: this listing has no Job Description.</p>
  }
  return (
    <div className="flex flex-col gap-3 text-sm">
      <div>
        <h3 className="mt-0 mb-1 text-sm">Found in your Master Data</h3>
        {report.covered.length === 0 ? (
          <p className="m-0 text-muted-foreground">No term of this description appears in your Entries.</p>
        ) : (
          <ul className="m-0 flex flex-wrap gap-x-4 gap-y-1 pl-0">
            {report.covered.map((c) => (
              <li key={c.term} className="list-none">
                {c.term}{' '}
                <span className="text-muted-foreground">
                  (
                  {c.entryIds.map((id, i) => (
                    <span key={id}>
                      {i > 0 && ', '}
                      <Link to={`/entries/${id}`}>{id}</Link>
                    </span>
                  ))}
                  )
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div>
        <h3 className="mt-0 mb-1 text-sm">Not found</h3>
        {report.uncovered.length === 0 ? (
          <p className="m-0 text-muted-foreground">Every term is covered.</p>
        ) : (
          <>
            <ul className="m-0 flex flex-wrap gap-x-4 gap-y-1 pl-0">
              {report.uncovered.map((term) => (
                <li key={term} className="list-none">{term}</li>
              ))}
            </ul>
            <p className="mt-2 mb-0">
              <Link to="/entries/new">Add an Entry</Link> or a tag to cover them before generating.
            </p>
          </>
        )}
      </div>
    </div>
  )
}
