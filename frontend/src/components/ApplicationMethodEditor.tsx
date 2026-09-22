import { useEffect, useState } from 'react'
import { isConflict } from '@/api/client'
import type { ApplicationMethod, ApplicationMethodKind } from '@/api/types'
import ConflictAlert from '@/components/ConflictAlert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { methodKindLabel } from '@/lib/applicationMethod'
import { cn } from '@/lib/utils'

// The kinds a user can correct to by hand — excludes `unresolved`, a
// system-set sentinel meaning inference couldn't even be attempted, not a
// real method to pick (story 15: the user can still correct straight to
// any of these at any time, unresolved or not).
const correctableMethodKinds: ApplicationMethodKind[] = ['portal', 'email', 'easy_apply', 'other']

// Displays the inferred Application Method and lets the user correct it if
// it's wrong (story 6) — Claude's inference at save time (story 5) is a
// guess, never trusted as final.
export default function ApplicationMethodEditor({
  method,
  onSave,
  onReload,
  openEditor,
}: {
  method: ApplicationMethod
  onSave: (method: ApplicationMethod) => Promise<void>
  // onReload re-reads the Application after a conflict (issue #89).
  onReload: () => Promise<void>
  // openEditor opens the editor from outside — the "Needs attention"
  // popover's "Set it myself" (issue #206, story 76). It is a changing
  // timestamp rather than a boolean, so asking twice opens it twice.
  openEditor?: number
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState<ApplicationMethod>(method)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [conflict, setConflict] = useState(false)
  const [reloading, setReloading] = useState(false)

  useEffect(() => {
    if (openEditor === undefined) return
    setDraft(method)
    setError(null)
    setEditing(true)
    // method is deliberately not a dependency: this runs when something
    // asks the editor to open, not whenever the record changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openEditor])

  function startEditing() {
    setDraft(method)
    setError(null)
    setEditing(true)
  }

  async function handleSave() {
    setSaving(true)
    setError(null)
    setConflict(false)
    try {
      await onSave({ kind: draft.kind, value: draft.value?.trim() || undefined })
      setEditing(false)
    } catch (err) {
      // A conflict keeps the draft on screen: the user decides whether to
      // reload (which closes the draft onto the current value) or copy it.
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
      await onReload()
      setConflict(false)
      setEditing(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setReloading(false)
    }
  }

  if (!editing) {
    return (
      <p className="mb-0 text-sm break-words">
        Apply via{' '}
        <strong className={cn('font-semibold', method.kind === 'unresolved' && 'text-unresolved')}>
          {methodKindLabel[method.kind]}
        </strong>
        {method.value && <>: {method.value}</>}
        {' · '}
        <button
          type="button"
          className="cursor-pointer text-primary underline-offset-4 hover:underline"
          onClick={startEditing}
        >
          Correct
        </button>
      </p>
    )
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Select value={draft.kind} onValueChange={(value) => setDraft((d) => ({ ...d, kind: value as ApplicationMethodKind }))}>
        <SelectTrigger size="sm" aria-label="Application method">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {correctableMethodKinds.map((kind) => (
            <SelectItem key={kind} value={kind}>
              {methodKindLabel[kind]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        className="h-7 w-56"
        value={draft.value ?? ''}
        onChange={(e) => setDraft((d) => ({ ...d, value: e.target.value }))}
        placeholder="URL or email address"
      />
      <Button size="sm" onClick={handleSave} disabled={saving}>
        {saving ? 'Saving…' : 'Save'}
      </Button>
      <Button size="sm" variant="ghost" onClick={() => setEditing(false)} disabled={saving}>
        Cancel
      </Button>
      {conflict && (
        <ConflictAlert
          record="Application"
          className="mb-0 basis-full"
          onReload={handleReload}
          reloading={reloading}
        />
      )}
      {error && (
        <p role="alert" className="mb-0 basis-full text-sm font-medium text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
