import { useState } from 'react'
import { Link } from 'react-router-dom'
import RALBadge from '@/components/RALBadge'
import DuplicatePostingAlert from '@/components/DuplicatePostingAlert'
import { duplicatePosting, saveJobListing } from '@/api/client'
import type { SaveConflict, SaveJobListingResult } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { jobListingHeading } from '@/lib/utils'

const blankForm = { title: '', company: '', url: '', jobDescription: '', jobDescriptionUrl: '' }
type FormState = typeof blankForm

export default function JobListingCreatePage() {
  const [form, setForm] = useState<FormState>(blankForm)
  const [error, setError] = useState<string | null>(null)
  // A posting already tracked is refused rather than saved twice (issue
  // #206) — its own state, since the remedy is a record to open, not a
  // message to read.
  const [duplicate, setDuplicate] = useState<SaveConflict | null>(null)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState<SaveJobListingResult | null>(null)
  const [warningDismissed, setWarningDismissed] = useState(false)

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setDuplicate(null)
    setSaving(true)
    try {
      const result = await saveJobListing({
        title: form.title.trim() || undefined,
        company: form.company,
        url: form.url.trim() || undefined,
        jobDescription: form.jobDescription.trim() || undefined,
        jobDescriptionUrl: form.jobDescriptionUrl.trim() || undefined,
      })
      setSaved(result)
      setWarningDismissed(false)
      setForm(blankForm)
    } catch (err) {
      const refused = duplicatePosting(err)
      if (refused) setDuplicate(refused)
      else setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <h1>Save a Job Listing</h1>
      <form onSubmit={handleSubmit}>
        {error && (
          <p role="alert" className="mb-4 font-medium text-destructive">
            {error}
          </p>
        )}

        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="job-title">Job Title (optional)</FieldLabel>
            <Input id="job-title" value={form.title} onChange={(e) => set('title', e.target.value)} placeholder="e.g. Senior Backend Engineer" />
          </Field>
          <Field>
            <FieldLabel htmlFor="company">Company</FieldLabel>
            <Input id="company" value={form.company} onChange={(e) => set('company', e.target.value)} required />
          </Field>
          <Field>
            <FieldLabel htmlFor="listing-url">Posting URL (optional)</FieldLabel>
            <Input id="listing-url" type="url" value={form.url} onChange={(e) => set('url', e.target.value)} placeholder="https://…" />
          </Field>
          <Field>
            <FieldLabel htmlFor="job-description">Job Description (paste text)</FieldLabel>
            <Textarea
              id="job-description"
              rows={8}
              value={form.jobDescription}
              onChange={(e) => set('jobDescription', e.target.value)}
              placeholder="Paste the job description here…"
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="job-description-url">…or a URL to fetch it from</FieldLabel>
            <Input
              id="job-description-url"
              type="url"
              value={form.jobDescriptionUrl}
              onChange={(e) => set('jobDescriptionUrl', e.target.value)}
              placeholder="https://…"
            />
            <FieldDescription>One of the two Job Description fields is required.</FieldDescription>
          </Field>
        </FieldGroup>

        <div className="mt-6 flex gap-3">
          <Button type="submit" disabled={saving}>
            {saving ? 'Saving…' : 'Save Job Listing'}
          </Button>
        </div>
      </form>

      {duplicate && <DuplicatePostingAlert conflict={duplicate} />}

      {saved?.duplicateWarning && !warningDismissed && (
        <section role="alert" className="mt-6 flex items-start justify-between gap-4 rounded-xl border border-amber-500/50 bg-amber-500/10 p-4">
          <p className="m-0">
            This looks like one you already have:{' '}
            <strong>{jobListingHeading(saved.duplicateWarning)}</strong>, saved{' '}
            {new Date(saved.duplicateWarning.savedAt).toLocaleDateString()}. Check the{' '}
            <Link to="/jobs" className="font-medium underline">
              Job Listings list
            </Link>{' '}
            before deciding — you can still keep both, this is just a heads-up.
          </p>
          <Button type="button" variant="ghost" onClick={() => setWarningDismissed(true)}>
            Dismiss
          </Button>
        </section>
      )}

      {saved && (
        <section className="mt-6 rounded-xl border border-border bg-card p-5">
          <h2 className="mt-0">Saved: {jobListingHeading(saved.jobListing)}</h2>
          <p>
            Application status: <strong>{saved.application.status}</strong>
          </p>
          <p>
            Inferred Application Method: <strong>{saved.application.method.kind}</strong>
            {saved.application.method.value && <> ({saved.application.method.value})</>} — you can correct this on{' '}
            <Link to={`/jobs/${encodeURIComponent(saved.jobListing.id)}`}>its Job Listing page</Link>.
          </p>
          <RALBadge ral={saved.jobListing.ral} />
          <p>
            <Link to="/jobs" className="no-underline hover:underline">
              View all Job Listings →
            </Link>
          </p>
        </section>
      )}
    </>
  )
}
