import { useState } from 'react'
import { Link } from 'react-router-dom'
import { setJobListingArchived } from '@/api/client'
import type { SaveConflict } from '@/api/types'
import { Button } from '@/components/ui/button'
import { jobListingHeading } from '@/lib/utils'

// DuplicatePostingAlert reports a save the backend refused because the
// posting is already tracked (issue #206). One Job Listing exists per
// posting, so the useful answer is not "try again" but "here is the one
// you already have" — named, dated and one click away.
//
// An archived match reads exactly like any other refusal (story 7); the
// only difference is the extra offer to bring it back (story 8), which
// takes the case where the user did mean to pick that listing up again
// from "go and find it" to one click.

interface DuplicatePostingAlertProps {
  conflict: SaveConflict
}

export default function DuplicatePostingAlert({ conflict }: DuplicatePostingAlertProps) {
  const existing = conflict.existing
  const [unarchiving, setUnarchiving] = useState(false)
  const [unarchived, setUnarchived] = useState(false)
  const [unarchiveError, setUnarchiveError] = useState<string | null>(null)

  async function handleUnarchive() {
    if (!existing) return
    setUnarchiveError(null)
    setUnarchiving(true)
    try {
      await setJobListingArchived(existing.id, false, undefined)
      setUnarchived(true)
    } catch (err) {
      setUnarchiveError(err instanceof Error ? err.message : String(err))
    } finally {
      setUnarchiving(false)
    }
  }

  return (
    <section role="alert" className="mt-6 rounded-xl border border-amber-500/50 bg-amber-500/10 p-4">
      <p className="m-0">
        {conflict.message}
        {existing && (
          <>
            {' '}
            <strong>{jobListingHeading(existing)}</strong>, saved {new Date(existing.savedAt).toLocaleDateString()}.
          </>
        )}
      </p>

      {existing && (
        <div className="mt-3 flex flex-wrap items-center gap-3">
          <Link to={`/jobs/${encodeURIComponent(existing.id)}`} className="font-medium underline">
            Open the existing Job Listing
          </Link>
          {existing.archived && !unarchived && (
            <Button type="button" variant="outline" size="sm" disabled={unarchiving} onClick={handleUnarchive}>
              {unarchiving ? 'Bringing it back…' : 'Bring it back from the archive'}
            </Button>
          )}
          {unarchived && <span className="text-sm text-muted-foreground">Brought back — it is in your Job Listings again.</span>}
        </div>
      )}

      {unarchiveError && <p className="mt-2 mb-0 text-sm font-medium text-destructive">{unarchiveError}</p>}
    </section>
  )
}
