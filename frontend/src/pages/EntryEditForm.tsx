import { useState } from 'react'
import { getEntry, isConflict, updateEntry } from '@/api/client'
import type { Entry, EntryInput } from '@/api/types'
import ConflictAlert from '@/components/ConflictAlert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Field, FieldDescription, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

function toFormState(entry: Entry) {
  return {
    employer: entry.employer ?? '',
    kind: entry.kind ?? '',
    client: entry.client ?? '',
    clientLabel: entry.clientLabel ?? '',
    title: entry.title ?? '',
    group: entry.group ?? '',
    role: entry.role ?? '',
    name: entry.name ?? '',
    location: entry.location ?? '',
    start: entry.start,
    end: entry.end ?? '',
    flagship: entry.flagship ?? false,
    tags: entry.tags.join(', '),
    repo: entry.repo ?? '',
    bullets: (entry.bullets ?? []).join('\n'),
  }
}

type FormState = ReturnType<typeof toFormState>

function toEntryInput(entry: Entry, form: FormState): EntryInput {
  return {
    type: entry.type,
    employer: form.employer,
    kind: (form.kind || undefined) as 'consultancy' | 'in-house' | undefined,
    client: form.client,
    clientLabel: form.clientLabel,
    title: form.title,
    group: form.group,
    role: form.role,
    name: form.name,
    location: form.location,
    start: form.start,
    end: form.end === '' ? null : form.end,
    flagship: form.flagship,
    tags: form.tags
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean),
    repo: form.repo,
    bullets: form.bullets
      .split('\n')
      .map((b) => b.trim())
      .filter(Boolean),
  }
}

export default function EntryEditForm({
  entry,
  onSaved,
  onReloaded,
  onCancel,
}: {
  entry: Entry
  onSaved: (entry: Entry) => void
  // onReloaded lets the parent show the on-disk version the user chose to
  // reload after a conflict, even if they then cancel.
  onReloaded?: (entry: Entry) => void
  onCancel: () => void
}) {
  // base is the Entry as last read from the server: its version token is
  // what the save presents as If-Match (issue #89).
  const [base, setBase] = useState<Entry>(entry)
  const [form, setForm] = useState<FormState>(toFormState(entry))
  const [error, setError] = useState<string | null>(null)
  const [conflict, setConflict] = useState(false)
  const [saving, setSaving] = useState(false)
  const [reloading, setReloading] = useState(false)

  function set<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setConflict(false)
    setSaving(true)
    try {
      const saved = await updateEntry(base.id, toEntryInput(base, form), base.version)
      onSaved(saved)
    } catch (err) {
      if (isConflict(err)) {
        setConflict(true)
      } else {
        setError(err instanceof Error ? err.message : String(err))
      }
    } finally {
      setSaving(false)
    }
  }

  async function handleReload() {
    setReloading(true)
    try {
      const fresh = await getEntry(base.id)
      setBase(fresh)
      setForm(toFormState(fresh))
      setConflict(false)
      setError(null)
      onReloaded?.(fresh)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setReloading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      {conflict && <ConflictAlert record="Entry" onReload={handleReload} reloading={reloading} />}
      {error && (
        <p role="alert" className="mb-4 font-medium text-destructive">
          {error}
        </p>
      )}

      <FieldGroup>
        {base.type === 'experience' ? (
          <>
            <Field>
              <FieldLabel htmlFor="employer">Employer</FieldLabel>
              <Input id="employer" value={form.employer} onChange={(e) => set('employer', e.target.value)} />
            </Field>
            <Field>
              <FieldLabel htmlFor="kind">Shape</FieldLabel>
              <Select value={form.kind || 'infer'} onValueChange={(v) => set('kind', v === 'infer' ? '' : v)}>
                <SelectTrigger id="kind" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="infer">Infer from client</SelectItem>
                  <SelectItem value="consultancy">Consultancy — one Client Engagement</SelectItem>
                  <SelectItem value="in-house">In-house — the employer is the job</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Field>
              <FieldLabel htmlFor="client">Client</FieldLabel>
              <Input id="client" value={form.client} onChange={(e) => set('client', e.target.value)} />
            </Field>
            <Field>
              <FieldLabel htmlFor="client-label">Client label on the CV</FieldLabel>
              <Input
                id="client-label"
                value={form.clientLabel}
                placeholder="e.g. Global hearing-care retailer"
                onChange={(e) => set('clientLabel', e.target.value)}
              />
              <FieldDescription>Optional. Printed instead of the client, which stays in Master Data.</FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="title">Project title</FieldLabel>
              <Input id="title" value={form.title} onChange={(e) => set('title', e.target.value)} />
              <FieldDescription>Optional. The project name the CV prints; without it, the role.</FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="group">Group</FieldLabel>
              <Input id="group" value={form.group} onChange={(e) => set('group', e.target.value)} />
              <FieldDescription>
                Optional. Engagements of this employer with the same group print as one project, titled by the group.
              </FieldDescription>
            </Field>
            <Field>
              <FieldLabel htmlFor="role">Role</FieldLabel>
              <Input id="role" value={form.role} onChange={(e) => set('role', e.target.value)} />
            </Field>
            <Field>
              <FieldLabel htmlFor="location">Location</FieldLabel>
              <Input id="location" value={form.location} onChange={(e) => set('location', e.target.value)} />
            </Field>
            <Field orientation="horizontal">
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="flex items-center gap-2">
                    <Checkbox
                      id="flagship"
                      checked={form.flagship}
                      onCheckedChange={(checked) => set('flagship', checked === true)}
                    />
                    <FieldLabel htmlFor="flagship" className="font-normal">
                      Flagship
                    </FieldLabel>
                  </span>
                </TooltipTrigger>
                <TooltipContent>
                  Flagship Entries are shown first when generating a Default Mode CV (no job description)
                </TooltipContent>
              </Tooltip>
            </Field>
          </>
        ) : (
          <>
            <Field>
              <FieldLabel htmlFor="name">Name</FieldLabel>
              <Input id="name" value={form.name} onChange={(e) => set('name', e.target.value)} />
            </Field>
            <Field>
              <FieldLabel htmlFor="repo">Repo</FieldLabel>
              <Input id="repo" value={form.repo} onChange={(e) => set('repo', e.target.value)} />
            </Field>
          </>
        )}

        <Field>
          <FieldLabel htmlFor="start">Start (YYYY or YYYY-MM)</FieldLabel>
          <Input id="start" value={form.start} onChange={(e) => set('start', e.target.value)} />
        </Field>
        <Field>
          <FieldLabel htmlFor="end">End (YYYY or YYYY-MM, blank for present)</FieldLabel>
          <Input id="end" value={form.end} onChange={(e) => set('end', e.target.value)} />
        </Field>
        <Field>
          <FieldLabel htmlFor="tags">Tags (comma-separated)</FieldLabel>
          <Input id="tags" value={form.tags} onChange={(e) => set('tags', e.target.value)} />
        </Field>
        <Field>
          <FieldLabel htmlFor="bullets">Bullets (one per line)</FieldLabel>
          <Textarea id="bullets" rows={6} value={form.bullets} onChange={(e) => set('bullets', e.target.value)} />
        </Field>
      </FieldGroup>

      <div className="mt-6 flex gap-3">
        <Button type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
        <Button type="button" variant="outline" onClick={onCancel} disabled={saving}>
          Cancel
        </Button>
      </div>
    </form>
  )
}
