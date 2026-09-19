import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ApiError, getATSReports } from '@/api/client'
import type { ATSField, ATSFieldGroup, ATSReport } from '@/api/types'
import ParsabilityBadge from '@/components/ParsabilityBadge'
import { Badge } from '@/components/ui/badge'

type LoadState = { kind: 'loading' } | { kind: 'missing' } | { kind: 'failed'; error: string } | { kind: 'ready'; reports: { cv: ATSReport; coverLetter?: ATSReport } }

const groupLabels: Record<ATSFieldGroup, string> = {
  identity: 'Name',
  contact: 'Contact',
  section: 'Section headers',
  experience: 'Employers',
  project: 'Projects',
  body: 'Letter',
}

function fieldVerdict(field: ATSField): { text: string; tone: 'ok' | 'bad' } {
  if (!field.found) return { text: 'missing', tone: 'bad' }
  if (!field.inOrder) return { text: 'out of order', tone: 'bad' }
  return { text: 'found', tone: 'ok' }
}

// ATSReportPage is "what an ATS sees" for one Generation (issue #198): the
// report kept when it was rendered, including the PDF's extracted text
// layer, so it can be checked right before sending, long after Visual
// Review.
export default function ATSReportPage() {
  const { slug = '' } = useParams()
  const [state, setState] = useState<LoadState>({ kind: 'loading' })

  useEffect(() => {
    getATSReports(slug)
      .then((reports) => setState({ kind: 'ready', reports }))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 404) setState({ kind: 'missing' })
        else setState({ kind: 'failed', error: err instanceof Error ? err.message : String(err) })
      })
  }, [slug])

  return (
    <>
      <p className="text-sm">
        <Link to="/generations">← Generated CVs</Link>
      </p>
      <h1>ATS Report</h1>
      <p className="font-mono text-xs text-muted-foreground">{slug}</p>

      {state.kind === 'loading' && <p role="status">Loading…</p>}
      {state.kind === 'failed' && (
        <p role="alert" className="font-medium text-destructive">
          {state.error}
        </p>
      )}
      {state.kind === 'missing' && (
        <p>
          No ATS Report was recorded for this Generation. It was probably made before Sumisura kept them. Render it again
          to get one.
        </p>
      )}
      {state.kind === 'ready' && (
        <>
          <ReportSection title="CV" report={state.reports.cv} />
          {state.reports.coverLetter && <ReportSection title="Cover Letter" report={state.reports.coverLetter} />}
        </>
      )}
    </>
  )
}

function ReportSection({ title, report }: { title: string; report: ATSReport }) {
  return (
    <section aria-label={`${title} ATS Report`} className="mt-6">
      <h2>{title}</h2>
      <ParsabilityBadge result={report} label={title} />

      {report.status === 'unavailable' ? (
        <p className="text-muted-foreground">
          The check couldn&apos;t run, so this says nothing about the PDF itself
          {report.reason ? `: ${report.reason}` : '.'}
        </p>
      ) : (
        <>
          {report.fields && report.fields.length > 0 && (
            <table className="mt-2 w-full text-sm">
              <thead>
                <tr className="text-left text-muted-foreground">
                  <th className="py-1 pr-3 font-medium">Field</th>
                  <th className="py-1 pr-3 font-medium">Kind</th>
                  <th className="py-1 font-medium">In the text layer</th>
                </tr>
              </thead>
              <tbody>
                {report.fields.map((field) => {
                  const verdict = fieldVerdict(field)
                  return (
                    <tr key={field.label} className="border-t border-border">
                      <td className="py-1 pr-3">{field.label}</td>
                      <td className="py-1 pr-3 text-muted-foreground">{groupLabels[field.group] ?? field.group}</td>
                      <td className={verdict.tone === 'bad' ? 'py-1 font-medium text-destructive' : 'py-1'}>
                        {verdict.text}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}

          {report.termCoverage && <TermCoverageSummary coverage={report.termCoverage} />}

          {report.extractedText !== undefined && (
            <>
              <h3>What an ATS reads</h3>
              <p className="text-sm text-muted-foreground">
                The PDF&apos;s text layer, top to bottom, as a screener extracts it. If something reads out of order
                here, it reads out of order there.
              </p>
              <pre className="max-h-[32rem] overflow-auto rounded-xl border border-border bg-muted p-3 text-xs whitespace-pre-wrap">
                {report.extractedText}
              </pre>
            </>
          )}
        </>
      )}
    </section>
  )
}

function TermCoverageSummary({ coverage }: { coverage: { present: string[]; missing: string[] } }) {
  const total = coverage.present.length + coverage.missing.length
  return (
    <>
      <h3>Job Description terms</h3>
      {total === 0 ? (
        <p className="text-sm text-muted-foreground">
          The Job Description mentions none of the tags in your Master Data.
        </p>
      ) : (
        <>
          <p className="text-sm">
            {coverage.present.length} of {total} terms the Job Description shares with your Master Data are on this CV.
            Only your own tags are counted, so nothing here asks you to claim a skill you don&apos;t have.
          </p>
          <div className="flex flex-wrap gap-1">
            {coverage.present.map((term) => (
              <Badge key={term} variant="secondary">
                {term}
              </Badge>
            ))}
            {coverage.missing.map((term) => (
              <Badge key={term} variant="outline" aria-label={`${term} (not on this CV)`}>
                {term} — not on this CV
              </Badge>
            ))}
          </div>
        </>
      )}
    </>
  )
}
