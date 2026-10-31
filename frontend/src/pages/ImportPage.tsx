import { useState } from 'react'
import { Link } from 'react-router-dom'
import { previewImport, saveImportedEntry } from '@/api/client'
import type { ImportDraft, ImportPreview } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'

export default function ImportPage() {
  const [text, setText] = useState('')
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function propose() {
    setBusy(true)
    setError(null)
    try {
      setPreview(await previewImport(text))
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Import failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <div className="mb-6">
        <h1 className="mb-1">Import</h1>
        <p className="mb-0 text-muted-foreground">
          Paste the text of your current CV (or a LinkedIn export). Claude proposes draft Entries with bullets copied
          from that text; nothing is written until you save a draft, one by one, and an existing Entry is never
          overwritten.
        </p>
      </div>

      <Textarea
        aria-label="CV text"
        rows={12}
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder="Paste your CV here"
        className="mb-3"
      />
      <Button onClick={propose} disabled={busy || text.trim() === ''}>
        {busy ? 'Reading…' : 'Propose Entries'}
      </Button>
      {error && (
        <p role="alert" className="mt-3 font-medium text-destructive">
          {error}
        </p>
      )}

      {preview && (
        <section className="mt-8" aria-label="Drafts">
          <h2>Drafts</h2>
          {preview.drafts.length === 0 && <p>No Entries found in this text.</p>}
          {preview.skipped > 0 && (
            <p className="text-muted-foreground">
              {preview.skipped} proposed {preview.skipped === 1 ? 'Entry was' : 'Entries were'} left out because
              required fields or dates were missing or malformed.
            </p>
          )}
          <div className="flex flex-col gap-3">
            {preview.drafts.map((draft) => (
              <DraftCard key={draft.entry.id} draft={draft} />
            ))}
          </div>
        </section>
      )}
    </>
  )
}

function DraftCard({ draft }: { draft: ImportDraft }) {
  const { entry } = draft
  const [saved, setSaved] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const title = entry.type === 'project' ? entry.name : `${entry.role} at ${entry.employer}`

  async function save() {
    setSaving(true)
    setError(null)
    try {
      await saveImportedEntry(entry)
      setSaved(true)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          {title}{' '}
          <span className="text-sm font-normal text-muted-foreground">
            {entry.start} – {entry.end ?? 'present'}
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {entry.tags.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {entry.tags.map((tag) => (
              <Badge key={tag} variant="secondary">
                {tag}
              </Badge>
            ))}
          </div>
        )}
        <ul className="mb-0">
          {(entry.bullets ?? []).map((bullet) => (
            <li key={bullet}>
              {bullet}{' '}
              {draft.notInText.includes(bullet) && <Badge variant="destructive">not in the text</Badge>}
            </li>
          ))}
        </ul>
        {error && (
          <p role="alert" className="mb-0 font-medium text-destructive">
            {error}
          </p>
        )}
        {saved ? (
          <p className="mb-0">
            Saved as <Link to={`/entries/${entry.id}`}>{entry.id}</Link>.
          </p>
        ) : (
          <div>
            <Button onClick={save} disabled={saving} aria-label={`Save ${title}`}>
              {saving ? 'Saving…' : 'Save'}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
