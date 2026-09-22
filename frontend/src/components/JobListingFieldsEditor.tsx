import { useState } from 'react'
import { correctJobListing, isConflict } from '@/api/client'
import type { JobListing } from '@/api/types'
import ConflictAlert from '@/components/ConflictAlert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

// Correcting a Job Listing (issue #206, stories 61-65, 69, 70). A capture
// that came out wrong — a mangled Job Title, a company name that splits one
// employer's roles across two groupings, a badly-parsed location — is
// something to fix here rather than something to live with.
//
// The URL and the Job Description are deliberately not here: the first is
// the record's identity, the second is what every recorded Generation was
// tailored to. The backend refuses both.

// ralDraft is what the user types. It carries no source: the backend
// stamps `manual`, so the page can never claim a figure was stated by the
// posting (story 65).
interface RALDraft {
  min: string
  max: string
  currency: string
}

const BLANK_RAL: RALDraft = { min: '', max: '', currency: 'EUR' }

export default function JobListingFieldsEditor({
  jobListing,
  onChange,
  onReload,
}: {
  jobListing: JobListing
  // onChange hands the corrected record back so every view of it updates
  // without a reload (story 70).
  onChange: (jobListing: JobListing) => void
  // onReload re-reads the record after a conflict (issue #89).
  onReload: () => Promise<void>
}) {
  const [editing, setEditing] = useState(false)
  const [enteringRAL, setEnteringRAL] = useState(false)
  const [draft, setDraft] = useState({ title: '', company: '', location: '' })
  const [ral, setRAL] = useState<RALDraft>(BLANK_RAL)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [conflict, setConflict] = useState(false)
  const [reloading, setReloading] = useState(false)

  function startEditing() {
    setDraft({
      title: jobListing.title ?? '',
      company: jobListing.company,
      location: jobListing.location ?? '',
    })
    setError(null)
    setConflict(false)
    setEditing(true)
  }

  function startEnteringRAL() {
    setRAL({
      min: jobListing.ral.min != null ? String(jobListing.ral.min) : '',
      max: jobListing.ral.max != null ? String(jobListing.ral.max) : '',
      currency: jobListing.ral.currency || 'EUR',
    })
    setError(null)
    setConflict(false)
    setEnteringRAL(true)
  }

  // send is the one write path, so every correction presents the version
  // token the page read and handles a 409 the same way.
  async function send(body: Record<string, unknown>, onDone: () => void) {
    setSaving(true)
    setError(null)
    setConflict(false)
    try {
      const updated = await correctJobListing(jobListing.id, body, jobListing.version)
      onChange(updated)
      onDone()
    } catch (err) {
      // A conflict keeps the draft on screen: the correction is the user's
      // work, and they decide whether to reload onto the current record or
      // copy what they typed.
      if (isConflict(err)) setConflict(true)
      else setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  async function handleSaveFields() {
    if (!draft.company.trim()) {
      setError('A Job Listing needs a Company.')
      return
    }
    await send(
      { title: draft.title.trim(), company: draft.company.trim(), location: draft.location.trim() },
      () => setEditing(false),
    )
  }

  async function handleSaveRAL() {
    const min = Number(ral.min)
    const max = Number(ral.max)
    if (!Number.isFinite(min) || !Number.isFinite(max) || min <= 0 || max <= 0) {
      setError('Enter both a minimum and a maximum.')
      return
    }
    if (max < min) {
      setError("A RAL Range's maximum cannot be below its minimum.")
      return
    }
    await send({ ral: { min, max, currency: ral.currency.trim() || 'EUR' } }, () => setEnteringRAL(false))
  }

  async function handleReload() {
    setReloading(true)
    try {
      await onReload()
      setConflict(false)
      setEditing(false)
      setEnteringRAL(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setReloading(false)
    }
  }

  const feedback = (
    <>
      {conflict && (
        <ConflictAlert record="Job Listing" className="mb-0 basis-full" onReload={handleReload} reloading={reloading} />
      )}
      {error && (
        <p role="alert" className="mb-0 basis-full text-sm font-medium text-destructive">
          {error}
        </p>
      )}
    </>
  )

  if (enteringRAL) {
    return (
      <div className="flex flex-wrap items-end gap-2">
        <label className="grid gap-1 text-xs text-muted-foreground">
          Minimum
          <Input
            className="h-7 w-28"
            inputMode="numeric"
            value={ral.min}
            onChange={(e) => setRAL((r) => ({ ...r, min: e.target.value }))}
          />
        </label>
        <label className="grid gap-1 text-xs text-muted-foreground">
          Maximum
          <Input
            className="h-7 w-28"
            inputMode="numeric"
            value={ral.max}
            onChange={(e) => setRAL((r) => ({ ...r, max: e.target.value }))}
          />
        </label>
        <label className="grid gap-1 text-xs text-muted-foreground">
          Currency
          <Input
            className="h-7 w-20"
            value={ral.currency}
            onChange={(e) => setRAL((r) => ({ ...r, currency: e.target.value }))}
          />
        </label>
        <Button size="sm" onClick={handleSaveRAL} disabled={saving}>
          {saving ? 'Saving…' : 'Save RAL Range'}
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setEnteringRAL(false)} disabled={saving}>
          Cancel
        </Button>
        {feedback}
      </div>
    )
  }

  if (editing) {
    return (
      <div className="flex flex-wrap items-end gap-2">
        <label className="grid gap-1 text-xs text-muted-foreground">
          Job Title
          <Input
            className="h-7 w-56"
            value={draft.title}
            onChange={(e) => setDraft((d) => ({ ...d, title: e.target.value }))}
          />
        </label>
        <label className="grid gap-1 text-xs text-muted-foreground">
          Company
          <Input
            className="h-7 w-44"
            value={draft.company}
            onChange={(e) => setDraft((d) => ({ ...d, company: e.target.value }))}
          />
        </label>
        <label className="grid gap-1 text-xs text-muted-foreground">
          Location
          <Input
            className="h-7 w-44"
            value={draft.location}
            onChange={(e) => setDraft((d) => ({ ...d, location: e.target.value }))}
          />
        </label>
        <Button size="sm" onClick={handleSaveFields} disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
        <Button size="sm" variant="ghost" onClick={() => setEditing(false)} disabled={saving}>
          Cancel
        </Button>
        {feedback}
      </div>
    )
  }

  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
      <span className="break-words">
        <strong className="font-semibold">{jobListing.title || jobListing.company}</strong>
        {jobListing.title && <> — {jobListing.company}</>}
        {jobListing.location && <> · {jobListing.location}</>}
      </span>
      <button
        type="button"
        className="cursor-pointer text-primary underline-offset-4 hover:underline"
        onClick={startEditing}
      >
        Correct details
      </button>
      <button
        type="button"
        className="cursor-pointer text-primary underline-offset-4 hover:underline"
        onClick={startEnteringRAL}
      >
        Enter the RAL Range yourself
      </button>
      {feedback}
    </div>
  )
}
