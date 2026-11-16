import { useState } from 'react'
import { isConflict, suggestContact } from '@/api/client'
import type { Contact, ContactInput, ContactRole } from '@/api/types'
import ConflictAlert from '@/components/ConflictAlert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

const ROLES: { value: ContactRole; label: string }[] = [
  { value: 'recruiter', label: 'Recruiter' },
  { value: 'manager', label: 'Hiring manager' },
  { value: 'referral', label: 'Referral' },
  { value: 'other', label: 'Other' },
]

function roleLabel(role: ContactRole): string {
  return ROLES.find((r) => r.value === role)?.label ?? role
}

// editing is null (nothing open), 'new' (adding) or the id of the Contact
// being edited.
type Editing = null | 'new' | string

// Manages the Contacts of an Application (issue #321): the recruiter who
// writes, the person who interviews, whoever referred the user. Entered
// manually, or Claude-suggested via web search — either way the user
// reviews the form and must explicitly Save before anything is persisted
// (the on* callbacks are the only things that write).
export default function ContactSection({
  jobListingId,
  contacts,
  inferredEmail,
  onAdd,
  onUpdate,
  onDelete,
  onReload,
}: {
  jobListingId: string
  contacts: Contact[]
  inferredEmail: string | undefined
  onAdd: (contact: ContactInput) => Promise<void>
  onUpdate: (id: string, contact: ContactInput) => Promise<void>
  onDelete: (id: string) => Promise<void>
  // onReload re-reads the Application after a conflict (issue #89).
  onReload: () => Promise<void>
}) {
  const [editing, setEditing] = useState<Editing>(null)
  const [draft, setDraft] = useState<ContactInput>({ name: '', email: '', role: 'recruiter', detail: '' })
  const [confirmingDelete, setConfirmingDelete] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [suggesting, setSuggesting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [conflict, setConflict] = useState(false)
  const [reloading, setReloading] = useState(false)

  function startAdd() {
    setDraft({ name: '', email: contacts.length === 0 ? (inferredEmail ?? '') : '', role: 'recruiter', detail: '' })
    setError(null)
    setConflict(false)
    setEditing('new')
  }

  function startEdit(contact: Contact) {
    setDraft({ name: contact.name, email: contact.email, role: contact.role, detail: contact.detail ?? '' })
    setError(null)
    setConflict(false)
    setEditing(contact.id)
  }

  async function handleSuggest() {
    setError(null)
    setSuggesting(true)
    try {
      const suggestion = await suggestContact(jobListingId)
      setDraft({ name: suggestion.name, email: suggestion.email, role: 'recruiter', detail: '' })
      setConflict(false)
      setEditing('new')
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSuggesting(false)
    }
  }

  // run keeps the form on screen on a conflict: the user decides whether to
  // reload (which closes it onto the current value) or copy what they typed.
  async function run(action: () => Promise<void>, done: () => void) {
    setBusy(true)
    setError(null)
    setConflict(false)
    try {
      await action()
      done()
    } catch (err) {
      if (isConflict(err)) {
        setConflict(true)
      } else {
        setError(err instanceof Error ? err.message : String(err))
      }
    } finally {
      setBusy(false)
    }
  }

  function handleSave() {
    const body: ContactInput = { ...draft, detail: draft.detail?.trim() || undefined }
    return run(
      () => (editing === 'new' ? onAdd(body) : onUpdate(editing as string, body)),
      () => setEditing(null),
    )
  }

  function handleDelete(id: string) {
    return run(
      () => onDelete(id),
      () => setConfirmingDelete(null),
    )
  }

  async function handleReload() {
    setReloading(true)
    try {
      await onReload()
      setConflict(false)
      setEditing(null)
      setConfirmingDelete(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setReloading(false)
    }
  }

  const form = (
    <div className="flex flex-wrap items-center gap-2">
      <Input
        className="h-7 w-44"
        value={draft.name}
        onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))}
        placeholder="Contact name"
      />
      <Input
        className="h-7 w-56"
        type="email"
        value={draft.email}
        onChange={(e) => setDraft((d) => ({ ...d, email: e.target.value }))}
        placeholder="Contact email"
      />
      <Select value={draft.role} onValueChange={(value) => setDraft((d) => ({ ...d, role: value as ContactRole }))}>
        <SelectTrigger size="sm" aria-label="Contact role">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {ROLES.map((r) => (
            <SelectItem key={r.value} value={r.value}>
              {r.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Input
        className="h-7 w-48"
        value={draft.detail ?? ''}
        onChange={(e) => setDraft((d) => ({ ...d, detail: e.target.value }))}
        placeholder="Phone or link (optional)"
      />
      <Button size="sm" onClick={handleSave} disabled={busy}>
        {busy ? 'Saving…' : 'Confirm & Save'}
      </Button>
      <Button size="sm" variant="ghost" onClick={() => setEditing(null)} disabled={busy}>
        Cancel
      </Button>
    </div>
  )

  return (
    <div className="flex flex-col gap-2 text-sm">
      {contacts.length === 0 && editing === null && (
        <p className="mb-0 text-muted-foreground">No Contact confirmed yet for this application.</p>
      )}
      {contacts.length > 0 && (
        <ul aria-label="Contacts" className="m-0 flex list-none flex-col gap-1 p-0">
          {contacts.map((contact) => (
            <li key={contact.id} className="flex flex-wrap items-center gap-2">
              {editing === contact.id ? (
                form
              ) : (
                <>
                  <span>
                    <strong>{contact.name || contact.email}</strong>
                    {contact.name && <> ({contact.email})</>} · {roleLabel(contact.role)}
                    {contact.detail && <> · {contact.detail}</>}
                  </span>
                  <Button size="sm" variant="outline" aria-label={`Edit ${contact.name || contact.email}`} onClick={() => startEdit(contact)}>
                    Edit
                  </Button>
                  {confirmingDelete === contact.id ? (
                    <>
                      <Button
                        size="sm"
                        variant="destructive"
                        aria-label={`Confirm delete ${contact.name || contact.email}`}
                        onClick={() => handleDelete(contact.id)}
                        disabled={busy}
                      >
                        {busy ? 'Deleting…' : 'Yes, delete'}
                      </Button>
                      <Button size="sm" variant="ghost" onClick={() => setConfirmingDelete(null)} disabled={busy}>
                        Keep
                      </Button>
                    </>
                  ) : (
                    <Button
                      size="sm"
                      variant="outline"
                      aria-label={`Delete ${contact.name || contact.email}`}
                      onClick={() => setConfirmingDelete(contact.id)}
                    >
                      Delete
                    </Button>
                  )}
                </>
              )}
            </li>
          ))}
        </ul>
      )}
      {editing === 'new' && form}
      {editing === null && (
        <div className="flex flex-wrap items-center gap-2">
          <Button size="sm" variant="outline" onClick={startAdd}>
            {contacts.length === 0 ? 'Enter manually' : 'Add a Contact'}
          </Button>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button size="sm" variant="outline" onClick={handleSuggest} disabled={suggesting}>
                {suggesting ? 'Asking Claude…' : 'Suggest via Claude'}
              </Button>
            </TooltipTrigger>
            <TooltipContent>Searches the web via Claude for a likely hiring contact — review before saving</TooltipContent>
          </Tooltip>
        </div>
      )}
      {conflict && <ConflictAlert record="Application" className="mb-0" onReload={handleReload} reloading={reloading} />}
      {error && (
        <p role="alert" className="mb-0 text-sm font-medium text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
