import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { addPendingCapture } from '@/api/client'
import type { AddPendingCaptureResult } from '@/api/types'
import { Button } from '@/components/ui/button'

type ShareState = { kind: 'saving' } | { kind: 'done'; result: AddPendingCaptureResult } | { kind: 'failed'; error: string }

// SharePage is the Web Share Target (issue #185): the installed app opens
// /share?title=…&text=…&url=… when a job is shared to Sumisura from another
// app, and this page hands whatever arrived to POST /api/pending-captures
// as-is — the backend finds the link, including inside text, where the
// LinkedIn app puts it. The share parameters stay in the URL, so a device
// that first has to enter its access token comes back to the same share.
export default function SharePage() {
  const [params] = useSearchParams()
  const [state, setState] = useState<ShareState>({ kind: 'saving' })
  const [attempt, setAttempt] = useState(0)
  const started = useRef(-1)

  const title = params.get('title') ?? ''
  const text = params.get('text') ?? ''
  const url = params.get('url') ?? ''

  useEffect(() => {
    // StrictMode runs effects twice in development; a share must be sent once.
    if (started.current === attempt) return
    started.current = attempt
    addPendingCapture({ title: title || undefined, text: text || undefined, url: url || undefined })
      .then((result) => setState({ kind: 'done', result }))
      .catch((err) => setState({ kind: 'failed', error: err instanceof Error ? err.message : String(err) }))
  }, [attempt, title, text, url])

  function retry() {
    setState({ kind: 'saving' })
    setAttempt((n) => n + 1)
  }

  return (
    <>
      <h1>Share to Sumisura</h1>
      {state.kind === 'saving' && <p role="status">Saving…</p>}
      {state.kind === 'failed' && (
        <>
          <p role="alert" className="font-medium text-destructive">
            {state.error}
          </p>
          <Button type="button" onClick={retry}>
            Try again
          </Button>
        </>
      )}
      {state.kind === 'done' && <ShareOutcome result={state.result} />}
    </>
  )
}

function ShareOutcome({ result }: { result: AddPendingCaptureResult }) {
  const jobLink = result.jobListingId && (
    <Link to={`/jobs/${encodeURIComponent(result.jobListingId)}`} className="font-medium">
      Open the Job Listing →
    </Link>
  )
  const inboxLink = (
    <Link to="/inbox" className="font-medium">
      Open To complete →
    </Link>
  )
  return (
    <div role="status" className="rounded-xl border border-border bg-card p-5">
      <p className="mt-0 text-lg font-medium">{result.message}</p>
      {result.pendingCapture?.title || result.pendingCapture?.company ? (
        <p className="text-muted-foreground">
          {[result.pendingCapture.title, result.pendingCapture.company].filter(Boolean).join(' · ')}
        </p>
      ) : null}
      {result.outcome === 'pending' && (
        <p>Add its Job Description later — or, for a LinkedIn or Indeed job, capture it with the browser extension on your computer.</p>
      )}
      <p className="mb-0">{result.outcome === 'job-listing' || result.outcome === 'already-tracked' ? jobLink : inboxLink}</p>
    </div>
  )
}
