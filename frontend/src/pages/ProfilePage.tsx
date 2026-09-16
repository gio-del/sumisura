import { useEffect, useState } from 'react'
import { getProfile, isConflict, updateProfile } from '@/api/client'
import type {
  Activity,
  Award,
  Certification,
  Education,
  Language,
  Profile,
  Publication,
} from '@/api/types'
import ConflictAlert from '@/components/ConflictAlert'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel, FieldLegend, FieldSet } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

function emptyEducation(): Education {
  return { degree: '', institution: '', program: '', start: '', end: '', grade: '', courses: [] }
}
function emptyPublication(): Publication {
  return { title: '', authors: '', venue: '', link: '', note: '' }
}
function emptyCertification(): Certification {
  return { title: '', issuer: '', date: '', link: '' }
}
function emptyAward(): Award {
  return { title: '', description: '' }
}
function emptyActivity(): Activity {
  return { title: '', description: '' }
}
function emptyLanguage(): Language {
  return { name: '', level: '' }
}

function ListSection<T>({
  title,
  itemLabel,
  items,
  onChange,
  makeEmpty,
  renderItem,
}: {
  title: string
  itemLabel: string
  items: T[]
  onChange: (items: T[]) => void
  makeEmpty: () => T
  renderItem: (item: T, update: (item: T) => void, idPrefix: string) => React.ReactNode
}) {
  const slug = title.toLowerCase().replace(/[^a-z0-9]+/g, '-')
  return (
    <FieldSet>
      <FieldLegend>{title}</FieldLegend>
      <div className="flex flex-col gap-4">
        {items.map((item, i) => (
          <div key={i} className="rounded-xl border border-border bg-card p-4">
            <FieldGroup>
              {renderItem(
                item,
                (updated) => {
                  const next = [...items]
                  next[i] = updated
                  onChange(next)
                },
                `${slug}-${i}`,
              )}
            </FieldGroup>
            <Button
              type="button"
              variant="outline"
              className="mt-4"
              onClick={() => onChange(items.filter((_, j) => j !== i))}
            >
              Remove
            </Button>
          </div>
        ))}
      </div>
      <Button type="button" variant="outline" onClick={() => onChange([...items, makeEmpty()])}>
        + Add {itemLabel}
      </Button>
    </FieldSet>
  )
}

function ProfileEditForm({
  profile,
  onSaved,
  onReloaded,
  onCancel,
}: {
  profile: Profile
  onSaved: (p: Profile) => void
  onReloaded: (p: Profile) => void
  onCancel: () => void
}) {
  // form carries the Profile's version token from its read; updateProfile
  // sends it as If-Match rather than in the body (issue #89).
  const [form, setForm] = useState<Profile>(profile)
  const [error, setError] = useState<string | null>(null)
  const [conflict, setConflict] = useState(false)
  const [saving, setSaving] = useState(false)
  const [reloading, setReloading] = useState(false)

  function set<K extends keyof Profile>(key: K, value: Profile[K]) {
    setForm((f) => ({ ...f, [key]: value }))
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setConflict(false)
    setSaving(true)
    try {
      const saved = await updateProfile(form)
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
      const fresh = await getProfile()
      setForm(fresh)
      setConflict(false)
      setError(null)
      onReloaded(fresh)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setReloading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      {conflict && <ConflictAlert record="Profile" onReload={handleReload} reloading={reloading} />}
      {error && (
        <p role="alert" className="mb-4 font-medium text-destructive">
          {error}
        </p>
      )}

      <FieldSet>
        <FieldLegend>Contact Info</FieldLegend>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="name">Name</FieldLabel>
            <Input id="name" value={form.name} onChange={(e) => set('name', e.target.value)} />
          </Field>
          <Field>
            <FieldLabel htmlFor="location">Location</FieldLabel>
            <Input id="location" value={form.location} onChange={(e) => set('location', e.target.value)} />
          </Field>
          <Field>
            <FieldLabel htmlFor="email">Email</FieldLabel>
            <Input id="email" value={form.email} onChange={(e) => set('email', e.target.value)} />
          </Field>
          <Field>
            <FieldLabel htmlFor="phone">Phone</FieldLabel>
            <Input id="phone" value={form.phone} onChange={(e) => set('phone', e.target.value)} />
          </Field>
          <Field>
            <FieldLabel htmlFor="linkedin">LinkedIn</FieldLabel>
            <Input id="linkedin" value={form.linkedin} onChange={(e) => set('linkedin', e.target.value)} />
          </Field>
          <Field>
            <FieldLabel htmlFor="github">GitHub</FieldLabel>
            <Input id="github" value={form.github} onChange={(e) => set('github', e.target.value)} />
          </Field>
        </FieldGroup>
      </FieldSet>

      <ListSection
        title="Education"
        itemLabel="Education"
        items={form.education}
        onChange={(items) => set('education', items)}
        makeEmpty={emptyEducation}
        renderItem={(item, update, idPrefix) => (
          <>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-degree`}>Degree</FieldLabel>
              <Input
                id={`${idPrefix}-degree`}
                value={item.degree}
                onChange={(e) => update({ ...item, degree: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-institution`}>Institution</FieldLabel>
              <Input
                id={`${idPrefix}-institution`}
                value={item.institution}
                onChange={(e) => update({ ...item, institution: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-program`}>Program</FieldLabel>
              <Input
                id={`${idPrefix}-program`}
                value={item.program}
                onChange={(e) => update({ ...item, program: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-start`}>Start</FieldLabel>
              <Input
                id={`${idPrefix}-start`}
                value={item.start}
                onChange={(e) => update({ ...item, start: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-end`}>End</FieldLabel>
              <Input id={`${idPrefix}-end`} value={item.end} onChange={(e) => update({ ...item, end: e.target.value })} />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-grade`}>Grade</FieldLabel>
              <Input
                id={`${idPrefix}-grade`}
                value={item.grade}
                onChange={(e) => update({ ...item, grade: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-courses`}>Courses (comma-separated)</FieldLabel>
              <Input
                id={`${idPrefix}-courses`}
                value={(item.courses ?? []).join(', ')}
                onChange={(e) =>
                  update({
                    ...item,
                    courses: e.target.value
                      .split(',')
                      .map((c) => c.trim())
                      .filter(Boolean),
                  })
                }
              />
            </Field>
          </>
        )}
      />

      <ListSection
        title="Publications"
        itemLabel="Publication"
        items={form.publications}
        onChange={(items) => set('publications', items)}
        makeEmpty={emptyPublication}
        renderItem={(item, update, idPrefix) => (
          <>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-title`}>Title</FieldLabel>
              <Input
                id={`${idPrefix}-title`}
                value={item.title}
                onChange={(e) => update({ ...item, title: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-authors`}>Authors</FieldLabel>
              <Input
                id={`${idPrefix}-authors`}
                value={item.authors}
                onChange={(e) => update({ ...item, authors: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-venue`}>Venue</FieldLabel>
              <Input
                id={`${idPrefix}-venue`}
                value={item.venue}
                onChange={(e) => update({ ...item, venue: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-link`}>Link</FieldLabel>
              <Input
                id={`${idPrefix}-link`}
                value={item.link ?? ''}
                onChange={(e) => update({ ...item, link: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-note`}>Note</FieldLabel>
              <Textarea
                id={`${idPrefix}-note`}
                value={item.note ?? ''}
                onChange={(e) => update({ ...item, note: e.target.value })}
              />
            </Field>
          </>
        )}
      />

      <ListSection
        title="Certifications"
        itemLabel="Certification"
        items={form.certifications}
        onChange={(items) => set('certifications', items)}
        makeEmpty={emptyCertification}
        renderItem={(item, update, idPrefix) => (
          <>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-title`}>Title</FieldLabel>
              <Input
                id={`${idPrefix}-title`}
                value={item.title}
                onChange={(e) => update({ ...item, title: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-issuer`}>Issuer</FieldLabel>
              <Input
                id={`${idPrefix}-issuer`}
                value={item.issuer ?? ''}
                onChange={(e) => update({ ...item, issuer: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-date`}>Date (YYYY-MM)</FieldLabel>
              <Input
                id={`${idPrefix}-date`}
                value={item.date ?? ''}
                onChange={(e) => update({ ...item, date: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-link`}>Verification link (optional)</FieldLabel>
              <Input
                id={`${idPrefix}-link`}
                value={item.link ?? ''}
                onChange={(e) => update({ ...item, link: e.target.value })}
              />
            </Field>
          </>
        )}
      />

      <ListSection
        title="Awards"
        itemLabel="Award"
        items={form.awards}
        onChange={(items) => set('awards', items)}
        makeEmpty={emptyAward}
        renderItem={(item, update, idPrefix) => (
          <>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-title`}>Title</FieldLabel>
              <Input
                id={`${idPrefix}-title`}
                value={item.title}
                onChange={(e) => update({ ...item, title: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-description`}>Description</FieldLabel>
              <Input
                id={`${idPrefix}-description`}
                value={item.description ?? ''}
                onChange={(e) => update({ ...item, description: e.target.value })}
              />
            </Field>
          </>
        )}
      />

      <ListSection
        title="Activities"
        itemLabel="Activity"
        items={form.activities}
        onChange={(items) => set('activities', items)}
        makeEmpty={emptyActivity}
        renderItem={(item, update, idPrefix) => (
          <>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-title`}>Title</FieldLabel>
              <Input
                id={`${idPrefix}-title`}
                value={item.title}
                onChange={(e) => update({ ...item, title: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-description`}>Description</FieldLabel>
              <Input
                id={`${idPrefix}-description`}
                value={item.description ?? ''}
                onChange={(e) => update({ ...item, description: e.target.value })}
              />
            </Field>
          </>
        )}
      />

      <ListSection
        title="Languages"
        itemLabel="Language"
        items={form.languages}
        onChange={(items) => set('languages', items)}
        makeEmpty={emptyLanguage}
        renderItem={(item, update, idPrefix) => (
          <>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-name`}>Name</FieldLabel>
              <Input
                id={`${idPrefix}-name`}
                value={item.name}
                onChange={(e) => update({ ...item, name: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor={`${idPrefix}-level`}>Level</FieldLabel>
              <Input
                id={`${idPrefix}-level`}
                value={item.level}
                onChange={(e) => update({ ...item, level: e.target.value })}
              />
            </Field>
          </>
        )}
      />

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

export default function ProfilePage() {
  const [profile, setProfile] = useState<Profile | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState(false)

  useEffect(() => {
    getProfile()
      .then(setProfile)
      .catch((e) => setError(e.message))
  }, [])

  if (error)
    return (
      <p role="alert" className="font-medium text-destructive">
        {error}
      </p>
    )
  if (!profile) return <p>Loading…</p>

  if (editing) {
    return (
      <>
        <h1>Edit Profile</h1>
        <ProfileEditForm
          profile={profile}
          onSaved={(saved) => {
            setProfile(saved)
            setEditing(false)
          }}
          onReloaded={setProfile}
          onCancel={() => setEditing(false)}
        />
      </>
    )
  }

  return (
    <>
      <h1>Profile</h1>

      <section>
        <h2>Contact Info</h2>
        <dl>
          <dt>Name</dt>
          <dd>{profile.name}</dd>
          <dt>Location</dt>
          <dd>{profile.location}</dd>
          <dt>Email</dt>
          <dd>{profile.email}</dd>
          <dt>Phone</dt>
          <dd>{profile.phone}</dd>
          <dt>LinkedIn</dt>
          <dd>{profile.linkedin}</dd>
          <dt>GitHub</dt>
          <dd>{profile.github}</dd>
        </dl>
      </section>

      <section>
        <h2>Education</h2>
        <ul className="list-disc pl-5">
          {profile.education.map((e, i) => (
            <li key={i}>
              {e.degree}, {e.institution} — {e.program} ({e.start}–{e.end})
            </li>
          ))}
        </ul>
      </section>

      <section>
        <h2>Publications</h2>
        <ul className="list-disc pl-5">
          {profile.publications.map((p, i) => (
            <li key={i}>{p.title}</li>
          ))}
        </ul>
      </section>

      <section>
        <h2>Certifications</h2>
        <ul className="list-disc pl-5">
          {profile.certifications.map((c, i) => (
            <li key={i}>
              {c.link ? (
                <a href={c.link} target="_blank" rel="noreferrer" className="underline">
                  {c.title}
                </a>
              ) : (
                c.title
              )}
              {c.issuer ? ` — ${c.issuer}` : ''}
              {c.date ? ` (${c.date})` : ''}
            </li>
          ))}
        </ul>
      </section>

      <section>
        <h2>Awards</h2>
        <ul className="list-disc pl-5">
          {profile.awards.map((a, i) => (
            <li key={i}>{a.title}</li>
          ))}
        </ul>
      </section>

      <section>
        <h2>Activities</h2>
        <ul className="list-disc pl-5">
          {profile.activities.map((a, i) => (
            <li key={i}>{a.title}</li>
          ))}
        </ul>
      </section>

      <section>
        <h2>Languages</h2>
        <ul className="list-disc pl-5">
          {profile.languages.map((l, i) => (
            <li key={i}>
              {l.name} — {l.level}
            </li>
          ))}
        </ul>
      </section>

      <div className="mt-6 flex gap-3">
        <Button type="button" onClick={() => setEditing(true)}>
          Edit
        </Button>
      </div>
    </>
  )
}
