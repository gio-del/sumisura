import { useState } from 'react'
import { getSnippet, isConflict, updateSnippet } from '@/api/client'
import type { Snippet, SnippetInput } from '@/api/types'
import ConflictAlert from '@/components/ConflictAlert'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

function toFormState(snippet: Snippet) {
  return { kind: snippet.kind, lang: snippet.lang ?? '', tags: snippet.tags.join(', '), body: snippet.body }
}

type FormState = ReturnType<typeof toFormState>

function toSnippetInput(form: FormState): SnippetInput {
  return {
    kind: form.kind,
    // An empty field means unmarked, which is a real state: the Snippet is
    // usable in any language rather than claiming one.
    ...(form.lang.trim() ? { lang: form.lang.trim() } : {}),
    tags: form.tags
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean),
    body: form.body,
  }
}

export default function SnippetEditForm({
  snippet,
  onSaved,
  onReloaded,
  onCancel,
}: {
  snippet: Snippet
  onSaved: (snippet: Snippet) => void
  onReloaded?: (snippet: Snippet) => void
  onCancel: () => void
}) {
  // base is the Snippet as last read: its version token is what the save
  // presents as If-Match (issue #89).
  const [base, setBase] = useState<Snippet>(snippet)
  const [form, setForm] = useState<FormState>(toFormState(snippet))
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
      const saved = await updateSnippet(base.id, toSnippetInput(form), base.version)
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
      const fresh = await getSnippet(base.id)
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
      {conflict && <ConflictAlert record="Cover Letter Snippet" onReload={handleReload} reloading={reloading} />}
      {error && (
        <p role="alert" className="mb-4 font-medium text-destructive">
          {error}
        </p>
      )}

      <FieldGroup>
        <Field>
          <FieldLabel htmlFor="kind">Kind</FieldLabel>
          <Input id="kind" value={form.kind} onChange={(e) => set('kind', e.target.value)} />
        </Field>
        <Field>
          <FieldLabel htmlFor="lang">Language (ISO 639-1, e.g. en or it — optional)</FieldLabel>
          <Input id="lang" value={form.lang} onChange={(e) => set('lang', e.target.value)} />
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
          {saving ? 'Saving…' : 'Save'}
        </Button>
        <Button type="button" variant="outline" onClick={onCancel} disabled={saving}>
          Cancel
        </Button>
      </div>
    </form>
  )
}
