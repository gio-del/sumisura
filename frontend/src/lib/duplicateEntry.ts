import type { Entry } from '@/api/types'

// DuplicateDraft is what "Duplicate" hands to the create form through the
// router's location.state: the Entry's fields as an unsaved draft. Nothing is
// written until the owner saves (like the import drafts, ADR-0049); the
// backend assigns the new id on create, never colliding with an existing one.
export type DuplicateDraft = Omit<Entry, 'id' | 'version' | 'lastModified'>

export const COPY_SUFFIX = ' (copia)'

export function duplicateDraft(entry: Entry): DuplicateDraft {
  const { id: _id, version: _version, lastModified: _lastModified, ...draft } = entry
  if (entry.type === 'project') {
    return { ...draft, name: `${entry.name ?? ''}${COPY_SUFFIX}` }
  }
  return { ...draft, title: `${entry.title || entry.role || ''}${COPY_SUFFIX}` }
}

// readDuplicateDraft returns the draft carried by a navigation, or null when
// the state is missing (a reload drops it) or not shaped like one.
export function readDuplicateDraft(state: unknown): DuplicateDraft | null {
  if (typeof state !== 'object' || state === null) return null
  const draft = (state as { duplicate?: unknown }).duplicate
  if (typeof draft !== 'object' || draft === null) return null
  const d = draft as Partial<DuplicateDraft>
  if ((d.type !== 'experience' && d.type !== 'project') || !Array.isArray(d.tags)) return null
  return d as DuplicateDraft
}
