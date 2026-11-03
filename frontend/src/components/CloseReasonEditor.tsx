import { useState } from 'react'
import { isConflict } from '@/api/client'
import { CLOSE_REASONS, CLOSE_REASON_LABELS } from '@/api/types'
import type { CloseReason } from '@/api/types'
import { NO_REASON } from '@/components/ApplicationStatusControl'
import ConflictAlert from '@/components/ConflictAlert'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

// Shows why a Rejected or Withdrawn Application closed and lets the user set,
// change or clear it (issue #278). It only ever informs the Stats page.
export default function CloseReasonEditor({
  closeReason,
  onSave,
  onReload,
}: {
  closeReason?: CloseReason
  onSave: (closeReason: CloseReason | '') => Promise<void>
  // onReload re-reads the Application after a conflict (issue #89).
  onReload: () => Promise<void>
}) {
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [conflict, setConflict] = useState(false)

  async function change(value: string) {
    setSaving(true)
    setError(null)
    setConflict(false)
    try {
      await onSave(value === NO_REASON ? '' : (value as CloseReason))
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
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-2 text-sm">
      <span>Close reason:</span>
      <Select value={closeReason ?? NO_REASON} onValueChange={change} disabled={saving}>
        <SelectTrigger size="sm" aria-label="Close reason">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={NO_REASON}>Not set</SelectItem>
          {CLOSE_REASONS.map((reason) => (
            <SelectItem key={reason} value={reason}>
              {CLOSE_REASON_LABELS[reason]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {conflict && <ConflictAlert record="Application" className="mb-0 basis-full" onReload={handleReload} />}
      {error && (
        <p role="alert" className="mb-0 basis-full text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
