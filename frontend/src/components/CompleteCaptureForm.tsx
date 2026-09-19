import { useState } from 'react'
import { completePendingCapture, suggestPendingCaptureHints } from '@/api/client'
import type { PendingCapture, SaveJobListingResult } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

type Suggestion = { kind: 'idle' } | { kind: 'running' } | { kind: 'failed' }

interface CompleteCaptureFormProps {
  capture: PendingCapture
  onCompleted: (result: SaveJobListingResult) => void
  // The secondary action beside Save: Cancel in the inbox, Later on the
  // share screen.
  secondary: React.ReactNode
}

function canReadClipboard(): boolean {
  return typeof navigator !== 'undefined' && typeof navigator.clipboard?.readText === 'function'
}

// CompleteCaptureForm turns a Pending Capture into a Job Listing from a
// Company, a Job Title and a pasted Job Description. It is shared by the To
// complete inbox and the share screen, so a job shared from a phone can be
// finished on the phone (issue #200). Once a Job Description is pasted,
// Company and Job Title are suggested from it and fill only fields that are
// still empty: a suggestion never replaces what the user typed or kept.
export default function CompleteCaptureForm({ capture, onCompleted, secondary }: CompleteCaptureFormProps) {
  const [form, setForm] = useState({ company: capture.company ?? '', title: capture.title ?? '', jobDescription: '' })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [suggestion, setSuggestion] = useState<Suggestion>({ kind: 'idle' })

  const needsHints = !form.company.trim() || !form.title.trim()

  async function suggest(jobDescription: string) {
    if (!jobDescription.trim()) return
    setSuggestion({ kind: 'running' })
    try {
      const hints = await suggestPendingCaptureHints(capture.id, jobDescription)
      setForm((f) => ({
        ...f,
        company: f.company.trim() ? f.company : hints.company,
        title: f.title.trim() ? f.title : hints.title,
      }))
      setSuggestion({ kind: 'idle' })
    } catch {
      setSuggestion({ kind: 'failed' })
    }
  }

  // A paste lands in the textarea's value only after the event, so the
  // suggestion reads the element once the browser has applied it.
  function handlePaste(e: React.ClipboardEvent<HTMLTextAreaElement>) {
    if (!needsHints) return
    const textarea = e.currentTarget
    setTimeout(() => void suggest(textarea.value), 0)
  }

  async function pasteFromClipboard() {
    setError(null)
    try {
      const text = await navigator.clipboard.readText()
      if (!text.trim()) return
      setForm((f) => ({ ...f, jobDescription: text }))
      if (needsHints) await suggest(text)
    } catch {
      setError('Could not read the clipboard. Long-press the box below and paste instead.')
    }
  }

  async function handleSubmit(e: React.FormEvent) {
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

  return (
    <form onSubmit={handleSubmit} className="mt-3">
      <p className="mt-0 text-sm">
        <a href={capture.url} target="_blank" rel="noreferrer" className="font-medium">
          Open the posting ↗
        </a>{' '}
        <span className="text-muted-foreground">then copy its description and paste it below.</span>
      </p>
      {error && (
        <p role="alert" className="font-medium text-destructive">
          {error}
        </p>
      )}
      <FieldGroup>
        <Field>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <FieldLabel htmlFor={`jd-${capture.id}`}>Job Description</FieldLabel>
            {canReadClipboard() && (
              <Button type="button" variant="outline" size="sm" onClick={pasteFromClipboard}>
                Paste
              </Button>
            )}
          </div>
          <Textarea
            id={`jd-${capture.id}`}
            rows={8}
            value={form.jobDescription}
            onChange={(e) => setForm((f) => ({ ...f, jobDescription: e.target.value }))}
            onPaste={handlePaste}
            placeholder="Open the posting, copy its description and paste it here…"
            required
          />
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => suggest(form.jobDescription)}
              disabled={!form.jobDescription.trim() || suggestion.kind === 'running'}
            >
              {suggestion.kind === 'running' ? 'Reading the description…' : 'Suggest Company and Title'}
            </Button>
            {suggestion.kind === 'failed' && (
              <span className="text-muted-foreground">
                Couldn&apos;t suggest them this time. Type them in below.
              </span>
            )}
          </div>
        </Field>
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
      </FieldGroup>
      <div className="mt-4 flex flex-wrap gap-2">
        <Button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save Job Listing'}
        </Button>
        {secondary}
      </div>
    </form>
  )
}
