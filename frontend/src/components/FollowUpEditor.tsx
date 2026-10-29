import { useState } from 'react'
import { isConflict } from '@/api/client'
import ConflictAlert from '@/components/ConflictAlert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { formatFollowUp } from '@/lib/followUp'

// Shows the Application's follow-up day and lets the user set, change or
// clear it (issue #240). An empty value clears it, and the Application goes
// back to the fixed 14-day stale threshold.
export default function FollowUpEditor({
  followUpAt,
  onSave,
  onReload,
}: {
  followUpAt?: string
  onSave: (followUpAt: string) => Promise<void>
  // onReload re-reads the Application after a conflict (issue #89).
  onReload: () => Promise<void>
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(followUpAt ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [conflict, setConflict] = useState(false)

  function startEditing() {
    setDraft(followUpAt ?? '')
    setError(null)
    setConflict(false)
    setEditing(true)
  }

  async function save(value: string) {
    setSaving(true)
    setError(null)
    setConflict(false)
    try {
      await onSave(value)
      setEditing(false)
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
    try {
      await onReload()
      setConflict(false)
      setEditing(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  if (!editing) {
    return (
      <p className="mb-0 text-sm">
        Follow up: <strong className="font-semibold">{followUpAt ? formatFollowUp(followUpAt) : 'not set'}</strong>
        {' · '}
        <button
          type="button"
          className="cursor-pointer text-primary underline-offset-4 hover:underline"
          onClick={startEditing}
        >
          {followUpAt ? 'Change' : 'Set date'}
        </button>
      </p>
    )
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Input
        type="date"
        className="h-7 w-40"
        aria-label="Follow-up date"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
      />
      <Button size="sm" onClick={() => save(draft)} disabled={saving}>
        {saving ? 'Saving…' : 'Save'}
      </Button>
      {followUpAt && (
        <Button size="sm" variant="outline" onClick={() => save('')} disabled={saving}>
          Clear
        </Button>
      )}
      <Button size="sm" variant="ghost" onClick={() => setEditing(false)} disabled={saving}>
        Cancel
      </Button>
      {conflict && <ConflictAlert record="Application" className="mb-0 basis-full" onReload={handleReload} />}
      {error && (
        <p role="alert" className="mb-0 basis-full text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
