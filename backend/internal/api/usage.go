package api

import (
	"net/http"
	"sort"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// usageItem is one Entry or Cover Letter Snippet in the usage report.
// LastUsedAt is absent when it was never chosen.
type usageItem struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Generations int     `json:"generations"`
	LastUsedAt  string  `json:"lastUsedAt,omitempty"`
	Share       float64 `json:"share"`
}

// usageReport is the wire shape of GET /api/master-data/usage's body.
// CountedGenerations is the denominator of every Share; LegacySkipped counts
// the Generations left out of it because they predate the usage fields.
type usageReport struct {
	Entries            []usageItem `json:"entries"`
	Snippets           []usageItem `json:"snippets"`
	CountedGenerations int         `json:"countedGenerations"`
	LegacySkipped      int         `json:"legacySkipped"`
}

// computeUsage crosses the recorded Generations with the ids of the current
// Entries and Snippets. Legacy Generations carry no usable entryIds or
// sourceSnippetIds, so they are skipped rather than counted as "chose
// nothing". Items come back least used first, then by id.
func computeUsage(applications []tracking.Application, entries, snippets []usageItem) usageReport {
	report := usageReport{Entries: entries, Snippets: snippets}
	entryIdx := make(map[string]int, len(entries))
	for i, e := range entries {
		entryIdx[e.ID] = i
	}
	snippetIdx := make(map[string]int, len(snippets))
	for i, s := range snippets {
		snippetIdx[s.ID] = i
	}

	bump := func(items []usageItem, idx map[string]int, ids []string, at string) {
		seen := make(map[string]bool, len(ids))
		for _, id := range ids {
			i, ok := idx[id]
			if !ok || seen[id] {
				continue
			}
			seen[id] = true
			items[i].Generations++
			if items[i].LastUsedAt == "" || tracking.IsMoreRecent(at, items[i].LastUsedAt) {
				items[i].LastUsedAt = at
			}
		}
	}

	for _, app := range applications {
		for _, gen := range app.Generations {
			if gen.IsLegacy() {
				report.LegacySkipped++
				continue
			}
			report.CountedGenerations++
			bump(entries, entryIdx, gen.EntryIDs, gen.CreatedAt)
			bump(snippets, snippetIdx, gen.SourceSnippetIDs, gen.CreatedAt)
		}
	}

	finish := func(items []usageItem) {
		if report.CountedGenerations > 0 {
			for i := range items {
				items[i].Share = float64(items[i].Generations) / float64(report.CountedGenerations)
			}
		}
		sort.SliceStable(items, func(a, b int) bool {
			if items[a].Generations != items[b].Generations {
				return items[a].Generations < items[b].Generations
			}
			return items[a].ID < items[b].ID
		})
	}
	finish(report.Entries)
	finish(report.Snippets)
	return report
}

// usageHandler recomputes the usage report on every call. Read-only: it
// writes nothing and makes no Claude call.
func usageHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := masterdata.ListEntries(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		snippets, err := masterdata.ListSnippets(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tracked, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		apps := make([]tracking.Application, 0, len(tracked))
		for _, t := range tracked {
			apps = append(apps, t.Application)
		}

		entryItems := make([]usageItem, 0, len(entries))
		for _, e := range entries {
			entryItems = append(entryItems, usageItem{ID: e.ID, Label: entryLabel(dataDir, e.ID)})
		}
		snippetItems := make([]usageItem, 0, len(snippets))
		for _, s := range snippets {
			snippetItems = append(snippetItems, usageItem{ID: s.ID, Label: s.Kind})
		}
		writeJSON(w, http.StatusOK, computeUsage(apps, entryItems, snippetItems))
	}
}
