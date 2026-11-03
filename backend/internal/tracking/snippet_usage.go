package tracking

import "time"

// SnippetLastUsed scans every Application's recorded Generation history and
// returns, per Cover Letter Snippet id, the CreatedAt of the most recent
// Generation that cited it (RFC3339Nano string, matching GenerationRecord).
// A Generation record with no SourceSnippetIDs contributes nothing — on a
// current record that's "no Snippet was used", on a legacy one (see
// GenerationRecord.IsLegacy) "unknowable"; this map folds both into "never
// used" and the caller (the Snippets list) decides how to label an id
// absent from it. Telling the two apart in what the list shows is issue
// #48's follow-up.
func SnippetLastUsed(applications []Application) map[string]string {
	lastUsed := make(map[string]string)
	for _, app := range applications {
		for _, gen := range app.Generations {
			for _, id := range gen.SourceSnippetIDs {
				if IsMoreRecent(gen.CreatedAt, lastUsed[id]) {
					lastUsed[id] = gen.CreatedAt
				}
			}
		}
	}
	return lastUsed
}

// IsMoreRecent reports whether candidate is a later timestamp than current
// ("" counts as never). Malformed timestamps lose to any parseable one and
// tie amongst themselves, rather than panicking or silently misordering.
func IsMoreRecent(candidate, current string) bool {
	if current == "" {
		return true
	}
	c, cErr := time.Parse(time.RFC3339Nano, candidate)
	cur, curErr := time.Parse(time.RFC3339Nano, current)
	if cErr != nil {
		return false
	}
	if curErr != nil {
		return true
	}
	return c.After(cur)
}
