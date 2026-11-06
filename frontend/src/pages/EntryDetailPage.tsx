import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { deleteEntry, getEntry, isConflict } from '@/api/client'
import type { Entry } from '@/api/types'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import ConflictAlert from '@/components/ConflictAlert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { duplicateDraft } from '@/lib/duplicateEntry'
import { formatRelativeTime } from '@/lib/utils'
import EntryEditForm from './EntryEditForm'

export default function EntryDetailPage() {
  const params = useParams()
  const navigate = useNavigate()
  const id = params['*'] ?? ''
  const [entry, setEntry] = useState<Entry | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [deleteConflict, setDeleteConflict] = useState(false)
  const [reloading, setReloading] = useState(false)

  useEffect(() => {
    setEntry(null)
    setError(null)
    setEditing(false)
    setDeleteOpen(false)
    setDeleteConflict(false)
    getEntry(id)
      .then(setEntry)
      .catch((e) => setError(e.message))
  }, [id])

  async function handleDelete() {
    setDeleting(true)
    try {
      await deleteEntry(id, entry?.version)
      navigate('/entries')
    } catch (e) {
      // A refused delete leaves the record on disk and the page usable, so
      // it shows the conflict alert rather than replacing the page with an
      // error (issue #89, story 18).
      if (isConflict(e)) {
        setDeleteConflict(true)
      } else {
        setError(e instanceof Error ? e.message : String(e))
      }
      setDeleting(false)
      setDeleteOpen(false)
    }
  }

  async function handleReload() {
    setReloading(true)
    try {
      setEntry(await getEntry(id))
      setDeleteConflict(false)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setReloading(false)
    }
  }

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!entry) return <p>Loading…</p>

  const title = entry.type === 'experience' ? `${entry.client ?? entry.employer} — ${entry.role}` : entry.name

  if (editing) {
    return (
      <>
        <h1>Edit {title}</h1>
        <EntryEditForm
          entry={entry}
          onSaved={(saved) => {
            setEntry(saved)
            setEditing(false)
          }}
          onReloaded={setEntry}
          onCancel={() => setEditing(false)}
        />
      </>
    )
  }

  return (
    <>
      {deleteConflict && (
        <ConflictAlert record="Entry" action="delete" keepsEdits={false} onReload={handleReload} reloading={reloading} />
      )}
      <h1>{title}</h1>
      {entry.type === 'experience' && (
        <dl>
          <dt>Employer</dt>
          <dd>{entry.employer}</dd>
          {entry.client && (
            <>
              <dt>Client</dt>
              <dd>{entry.client}</dd>
            </>
          )}
          <dt>Location</dt>
          <dd>{entry.location}</dd>
        </dl>
      )}
      <p>
        {entry.start} – {entry.end ?? 'present'}
      </p>
      {entry.lastModified?.at && (
        <p className="text-sm text-muted-foreground" title={entry.lastModified.subject}>
          Last edited {formatRelativeTime(entry.lastModified.at)}
        </p>
      )}
      <div className="flex flex-wrap gap-1">
        {entry.tags.map((tag) => (
          <Badge variant="secondary" key={tag}>
            {tag}
          </Badge>
        ))}
      </div>
      <ul className="list-disc pl-5">
        {entry.bullets?.map((bullet, i) => (
          <li key={i}>{bullet}</li>
        ))}
      </ul>
      <div className="mt-6 flex items-center gap-3">
        <Button type="button" onClick={() => setEditing(true)}>
          Edit
        </Button>
        <Button
          type="button"
          variant="outline"
          onClick={() => navigate('/entries/new', { state: { duplicate: duplicateDraft(entry) } })}
        >
          Duplica
        </Button>
        <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
          <AlertDialogTrigger asChild>
            <Button type="button" variant="outline">
              Delete
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Delete this entry?</AlertDialogTitle>
              <AlertDialogDescription>This action cannot be undone.</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
              <AlertDialogAction
                variant="destructive"
                onClick={(e) => {
                  e.preventDefault()
                  handleDelete()
                }}
                disabled={deleting}
              >
                {deleting ? 'Deleting…' : 'Yes, delete'}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </>
  )
}
