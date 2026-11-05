import { useState } from 'react'
import { isConflict } from '@/api/client'
import type { Interview } from '@/api/types'
import ConflictAlert from '@/components/ConflictAlert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { formatInterview } from '@/lib/followUp'

// Lists the Application's interviews and lets the user add, edit and delete
// them (issue #283). Every change sends the whole list, which the server
// stores soonest first.
export default function InterviewEditor({
  interviews,
  onSave,
  onReload,
}: {
  interviews: Interview[]
  onSave: (interviews: Interview[]) => Promise<void>
  // onReload re-reads the Application after a conflict (issue #89).
  onReload: () => Promise<void>
}) {
  // editing is the index being edited, 'new' for an addition, null for none.
  const [editing, setEditing] = useState<number | 'new' | null>(null)
  const [at, setAt] = useState('')
  const [note, setNote] = useState('')
  const [location, setLocation] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [conflict, setConflict] = useState(false)

  function startEditing(target: number | 'new') {
    const current = target === 'new' ? undefined : interviews[target]
    setAt(current?.at ?? '')
    setNote(current?.note ?? '')
    setLocation(current?.location ?? '')
    setError(null)
    setConflict(false)
    setEditing(target)
  }

  async function save(next: Interview[]) {
    setSaving(true)
    setError(null)
    setConflict(false)
    try {
      await onSave(next)
      setEditing(null)
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

  function submit() {
    const entry: Interview = { at, ...(note.trim() && { note: note.trim() }), ...(location.trim() && { location: location.trim() }) }
    const next = editing === 'new' ? [...interviews, entry] : interviews.map((iv, i) => (i === editing ? entry : iv))
    return save(next)
  }

  async function handleReload() {
    try {
      await onReload()
      setConflict(false)
      setEditing(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="text-sm">
      <p className="mb-1">
        Interviews: {interviews.length === 0 && <strong className="font-semibold">none</strong>}
        {editing === null && (
          <>
            {interviews.length === 0 ? ' · ' : ' '}
            <button type="button" className="cursor-pointer text-primary underline-offset-4 hover:underline" onClick={() => startEditing('new')}>
              Add interview
            </button>
          </>
        )}
      </p>
      {interviews.length > 0 && (
        <ul className="mb-1 list-none pl-0">
          {interviews.map((iv, i) => (
            <li key={`${iv.at}-${i}`}>
              <strong className="font-semibold">{formatInterview(iv.at)}</strong>
              {iv.note && ` · ${iv.note}`}
              {iv.location && ` · ${iv.location}`}
              {editing === null && (
                <>
                  {' · '}
                  <button type="button" className="cursor-pointer text-primary underline-offset-4 hover:underline" aria-label={`Edit interview ${formatInterview(iv.at)}`} onClick={() => startEditing(i)}>
                    Edit
                  </button>
                  {' · '}
                  <button
                    type="button"
                    className="cursor-pointer text-primary underline-offset-4 hover:underline"
                    disabled={saving}
                    aria-label={`Delete interview ${formatInterview(iv.at)}`}
                    onClick={() => save(interviews.filter((_, j) => j !== i))}
                  >
                    Delete
                  </button>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
      {editing !== null && (
        <div className="flex flex-wrap items-center gap-2">
          <Input type="datetime-local" className="h-7 w-52" aria-label="Interview date and time" value={at} onChange={(e) => setAt(e.target.value)} />
          <Input className="h-7 w-52" aria-label="Interview note" placeholder="Note (optional)" value={note} onChange={(e) => setNote(e.target.value)} />
          <Input className="h-7 w-52" aria-label="Interview address or link" placeholder="Address or link (optional)" value={location} onChange={(e) => setLocation(e.target.value)} />
          <Button size="sm" onClick={submit} disabled={saving || !at}>
            {saving ? 'Saving…' : 'Save interview'}
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setEditing(null)} disabled={saving}>
            Cancel
          </Button>
        </div>
      )}
      {conflict && <ConflictAlert record="Application" className="mb-0 basis-full" onReload={handleReload} />}
      {error && (
        <p role="alert" className="mb-0 basis-full text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
