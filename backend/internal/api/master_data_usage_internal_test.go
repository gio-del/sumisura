package api

import (
	"testing"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func TestComputeUsage(t *testing.T) {
	apps := []tracking.Application{{Generations: []tracking.GenerationRecord{
		{SchemaVersion: 1, CreatedAt: "2025-01-01T00:00:00Z", EntryIDs: []string{"experience/a", "experience/a"}, SourceSnippetIDs: []string{"s1"}},
		{SchemaVersion: 1, CreatedAt: "2025-03-01T00:00:00Z", EntryIDs: []string{"experience/a", "projects/b"}},
		{CreatedAt: "2025-02-01T00:00:00Z"}, // legacy
	}}}
	r := computeUsage(apps,
		[]usageItem{{ID: "experience/a"}, {ID: "projects/b"}, {ID: "projects/never"}},
		[]usageItem{{ID: "s1"}, {ID: "s2"}})

	if r.CountedGenerations != 2 || r.LegacySkipped != 1 {
		t.Fatalf("counted=%d legacy=%d", r.CountedGenerations, r.LegacySkipped)
	}
	if got := r.Entries[0]; got.ID != "projects/never" || got.Generations != 0 || got.LastUsedAt != "" {
		t.Errorf("never-used entry should be first and empty, got %+v", got)
	}
	a := r.Entries[2]
	if a.ID != "experience/a" || a.Generations != 2 || a.Share != 1 || a.LastUsedAt != "2025-03-01T00:00:00Z" {
		t.Errorf("entry a = %+v", a)
	}
	if r.Snippets[0].ID != "s2" || r.Snippets[1].Share != 0.5 {
		t.Errorf("snippets = %+v", r.Snippets)
	}
}

func TestComputeUsage_NoGenerationsHasZeroShares(t *testing.T) {
	r := computeUsage(nil, []usageItem{{ID: "experience/a"}}, nil)
	if r.CountedGenerations != 0 || r.Entries[0].Share != 0 {
		t.Errorf("got %+v", r)
	}
}
