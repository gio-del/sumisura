import { useState } from 'react'
import { createSnippet } from '@/api/client'
import type { Snippet, SnippetInput } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

interface Props {
  /** Pre-fills the body, e.g. with a paragraph of an approved Cover Letter. */
  initialBody?: string
  onCreated: (created: Snippet) => void
  /** When given, a Cancel button is shown; cancelling writes nothing. */
  onCancel?: () => void
}

/** SnippetForm creates a Cover Letter Snippet; nothing is written until it is submitted. */
export default function SnippetForm({ initialBody = '', onCreated, onCancel }: Props) {
  const [form, setForm] = useState({ kind: '', tags: '', body: initialBody })
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  function set(key: keyof typeof form, value: string) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setSaving(true)
    try {
      const input: SnippetInput = {
        kind: form.kind,
        tags: form.tags
          .split(',')
          .map((t) => t.trim())
          .filter(Boolean),
        body: form.body,
      }
      onCreated(await createSnippet(input))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      {error && (
        <p role="alert" className="mb-4 font-medium text-destructive">
          {error}
        </p>
      )}

      <FieldGroup>
        <Field>
          <FieldLabel htmlFor="kind">Kind (e.g. opening, why-this-company, closing)</FieldLabel>
          <Input id="kind" value={form.kind} onChange={(e) => set('kind', e.target.value)} />
        </Field>
        <Field>
          <FieldLabel htmlFor="tags">Tags (comma-separated)</FieldLabel>
          <Input id="tags" value={form.tags} onChange={(e) => set('tags', e.target.value)} />
        </Field>
        <Field>
          <FieldLabel htmlFor="body">Body</FieldLabel>
          <Textarea id="body" rows={8} value={form.body} onChange={(e) => set('body', e.target.value)} />
        </Field>
      </FieldGroup>

      <div className="mt-6 flex gap-3">
        <Button type="submit" disabled={saving}>
          {saving ? 'Creating…' : 'Create'}
        </Button>
        {onCancel && (
          <Button type="button" variant="outline" onClick={onCancel}>
            Cancel
          </Button>
        )}
      </div>
    </form>
  )
}
