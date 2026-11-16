import { useEffect, useMemo, useState } from 'react'
import ContactSection from '@/components/ContactSection'
import GenerationFilesReminder from '@/components/GenerationFilesReminder'
import { getApplicationMailto } from '@/api/client'
import type { Application, ContactInput, JobListing } from '@/api/types'

// Branches the FE's apply guidance by Application Method (story 8): every
// Application gets Contact management (story 7, issue #321: 0..N Contacts);
// email additionally gets a prefilled mailto: draft (story 9) to the chosen
// Contact — by default the first recruiter, else the first of the list —
// plus the CV/Cover Letter reminder (story 10); portal/Easy Apply/other go
// straight to the application link with the same reminder.
export default function ApplyGuidance({
  jobListing,
  application,
  onAddContact,
  onUpdateContact,
  onDeleteContact,
  onReload,
}: {
  jobListing: JobListing
  application: Application
  onAddContact: (contact: ContactInput) => Promise<void>
  onUpdateContact: (id: string, contact: ContactInput) => Promise<void>
  onDeleteContact: (id: string) => Promise<void>
  onReload: () => Promise<void>
}) {
  const { method } = application
  const contacts = useMemo(() => application.contacts ?? [], [application.contacts])
  // chosenId is the Contact the user picked for the draft; empty means the
  // server's default. A pick that no longer exists falls back to the default.
  const [chosenId, setChosenId] = useState('')
  const contactId = contacts.some((c) => c.id === chosenId) ? chosenId : ''
  const [mailtoUri, setMailtoUri] = useState<string | null>(null)
  const [mailtoError, setMailtoError] = useState<string | null>(null)

  useEffect(() => {
    if (method.kind !== 'email' || contacts.length === 0) {
      setMailtoUri(null)
      return
    }
    setMailtoError(null)
    getApplicationMailto(jobListing.id, contactId || undefined)
      .then((r) => setMailtoUri(r.uri))
      .catch((err) => setMailtoError(err instanceof Error ? err.message : String(err)))
  }, [method.kind, contacts, contactId, jobListing.id])

  const contactSection = (
    <ContactSection
      jobListingId={jobListing.id}
      contacts={contacts}
      inferredEmail={method.kind === 'email' ? method.value : undefined}
      onAdd={onAddContact}
      onUpdate={onUpdateContact}
      onDelete={onDeleteContact}
      onReload={onReload}
    />
  )

  if (method.kind === 'email') {
    return (
      <div className="flex flex-col gap-2">
        {contactSection}
        {contacts.length > 0 && (
          <>
            {contacts.length > 1 && (
              <label className="flex items-center gap-2 text-sm">
                Draft to
                <select
                  className="h-7 rounded-md border bg-background px-2 text-sm"
                  value={contactId}
                  onChange={(e) => setChosenId(e.target.value)}
                >
                  <option value="">Default (first recruiter, else first)</option>
                  {contacts.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name || c.email}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {mailtoUri && (
              <p className="mb-0 text-sm">
                <a href={mailtoUri}>Open a prefilled email draft →</a>
              </p>
            )}
            {mailtoError && (
              <p role="alert" className="mb-0 text-sm font-medium text-destructive">
                {mailtoError}
              </p>
            )}
            <GenerationFilesReminder generations={application.generations} />
          </>
        )}
      </div>
    )
  }

  const link = method.value || jobListing.url

  return (
    <div className="flex flex-col gap-2">
      {contactSection}
      {link ? (
        <p className="mb-0 text-sm break-words">
          Apply here:{' '}
          <a href={link} target="_blank" rel="noreferrer">
            {link}
          </a>
        </p>
      ) : (
        <p className="mb-0 text-sm text-muted-foreground">No application link detected — check the posting.</p>
      )}
      <GenerationFilesReminder generations={application.generations} />
    </div>
  )
}
