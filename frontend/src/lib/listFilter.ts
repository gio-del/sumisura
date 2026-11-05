// Browser-side search and tag filtering for the Master Data lists (issue #294).
// Job Listings search on the server; Entries and Snippets are already all
// loaded, so they filter here and keep the choice in the URL.

/** matchesQuery reports whether every word of query occurs, ignoring case, in
 *  at least one of the fields. An empty query matches everything. */
export function matchesQuery(query: string, fields: Array<string | undefined>): boolean {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (words.length === 0) return true
  const haystack = fields
    .filter((f): f is string => Boolean(f))
    .join('\n')
    .toLowerCase()
  return words.every((w) => haystack.includes(w))
}

/** hasAllTags is the AND filter: every chosen tag must be among the item's. */
export function hasAllTags(tags: string[], chosen: string[]): boolean {
  return chosen.every((t) => tags.includes(t))
}

/** allTags lists the distinct tags of the items, sorted. */
export function allTags(items: Array<{ tags: string[] }>): string[] {
  return [...new Set(items.flatMap((i) => i.tags))].sort((a, b) => a.localeCompare(b))
}
