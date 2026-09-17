import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { completePendingCapture, deletePendingCapture, listPendingCaptures } from '@/api/client'
import type { PendingCapture, PostingProvider, SaveJobListingResult } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { jobListingHeading } from '@/lib/utils'

const providerLabels: Record<PostingProvider, string> = {
  linkedin: 'LinkedIn',
  indeed: 'Indeed',
  greenhouse: 'Greenhouse',
  lever: 'Lever',
  ashby: 'Ashby',
  other: 'Link',
}

// captureHeading is what a Pending Capture is called in the inbox: its
// hints when the share carried any, otherwise the link's host.
function captureHeading(capture: PendingCapture): string {
  const hints = [capture.title, capture.company].filter(Boolean).join(' · ')
  if (hints) return hints
  try {
    return new URL(capture.url).host.replace(/^www\./, '')
  } catch {
    return capture.url
  }
}

// PendingCapturesPage is the To complete inbox (issue #182): links shared
// from a phone, each waiting for its Job Description before it can become a
// Job Listing.
export default function PendingCapturesPage() {
  const [captures, setCaptures] = useState<PendingCapture[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [completed, setCompleted] = useState<SaveJobListingResult | null>(null)

  useEffect(() => {
    listPendingCaptures()
      .then(setCaptures)
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
  }, [])

  function remove(id: string) {
    setCaptures((current) => (current ?? []).filter((c) => c.id !== id))
  }

  return (
    <>
      <h1>To complete</h1>
      <p className="text-muted-foreground">
        Job links you shared from your phone. Add each one&apos;s Job Description to turn it into a Job Listing.
      </p>
      <p className="text-sm text-muted-foreground">
        On your computer, a LinkedIn or Indeed link is quicker: open it and capture the posting with the browser
        extension, and it leaves this list by itself.
      </p>

      {error && (
        <p role="alert" className="font-medium text-destructive">
          {error}
        </p>
      )}

      {completed && (
        <p role="status" className="rounded-xl border border-border bg-card p-4">
          Saved as a Job Listing:{' '}
          <Link to={`/jobs/${encodeURIComponent(completed.jobListing.id)}`} className="font-medium">
            {jobListingHeading(completed.jobListing)}
          </Link>
          {completed.duplicateWarning && (
            <> — it looks like one you already have ({jobListingHeading(completed.duplicateWarning)}).</>
          )}
        </p>
      )}

      {captures !== null && captures.length === 0 && (
        <p>Nothing waiting. Share a job from your phone to Sumisura and it lands here.</p>
      )}

      {captures !== null && captures.length > 0 && (
        <ul className="m-0 flex list-none flex-col gap-3 p-0">
          {captures.map((capture) => (
            <PendingCaptureItem
              key={capture.id}
              capture={capture}
              onCompleted={(result) => {
                remove(capture.id)
                setCompleted(result)
              }}
              onDismissed={() => remove(capture.id)}
            />
          ))}
        </ul>
      )}
    </>
  )
}

interface PendingCaptureItemProps {
  capture: PendingCapture
  onCompleted: (result: SaveJobListingResult) => void
  onDismissed: () => void
}

function PendingCaptureItem({ capture, onCompleted, onDismissed }: PendingCaptureItemProps) {
  const [completing, setCompleting] = useState(false)
  const [form, setForm] = useState({ company: capture.company ?? '', title: capture.title ?? '', jobDescription: '' })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const heading = captureHeading(capture)

  async function handleComplete(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSaving(true)
    try {
      onCompleted(
        await completePendingCapture(capture.id, {
          company: form.company.trim(),
          title: form.title.trim() || undefined,
          jobDescription: form.jobDescription.trim(),
        }),
      )
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setSaving(false)
    }
  }

  async function handleDismiss() {
    setError(null)
    try {
      await deletePendingCapture(capture.id)
      onDismissed()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <li className="rounded-xl border border-border bg-card p-4" aria-label={heading}>
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="secondary">{providerLabels[capture.provider] ?? 'Link'}</Badge>
        <strong className="min-w-0 break-words">{heading}</strong>
        <span className="text-xs text-muted-foreground">shared {new Date(capture.savedAt).toLocaleDateString()}</span>
      </div>
      <p className="my-2 truncate text-sm">
        <a href={capture.url} target="_blank" rel="noreferrer">
          {capture.url}
        </a>
      </p>

      {error && (
        <p role="alert" className="font-medium text-destructive">
          {error}
        </p>
      )}

      {!completing && (
        <div className="flex flex-wrap gap-2">
          <Button type="button" onClick={() => setCompleting(true)}>
            Complete
          </Button>
          <Button type="button" variant="ghost" onClick={handleDismiss}>
            Dismiss
          </Button>
        </div>
      )}

      {completing && (
        <form onSubmit={handleComplete} className="mt-3">
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor={`company-${capture.id}`}>Company</FieldLabel>
              <Input
                id={`company-${capture.id}`}
                value={form.company}
                onChange={(e) => setForm((f) => ({ ...f, company: e.target.value }))}
                required
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`title-${capture.id}`}>Job Title (optional)</FieldLabel>
              <Input
                id={`title-${capture.id}`}
                value={form.title}
                onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`jd-${capture.id}`}>Job Description</FieldLabel>
              <Textarea
                id={`jd-${capture.id}`}
                rows={8}
                value={form.jobDescription}
                onChange={(e) => setForm((f) => ({ ...f, jobDescription: e.target.value }))}
                placeholder="Open the posting, copy its description and paste it here…"
                required
              />
            </Field>
          </FieldGroup>
          <div className="mt-4 flex flex-wrap gap-2">
            <Button type="submit" disabled={saving}>
              {saving ? 'Saving…' : 'Save Job Listing'}
            </Button>
            <Button type="button" variant="ghost" onClick={() => setCompleting(false)} disabled={saving}>
              Cancel
            </Button>
          </div>
        </form>
      )}
    </li>
  )
}
