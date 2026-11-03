package tracking_test

import (
	"errors"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func prioritized(id string, p tracking.Priority) tracking.ListingWithApplication {
	l := listing(id, tracking.StatusSaved, "Co "+id, "2026-01-01T00:00:00Z")
	l.JobListing.Priority = p
	return l
}

func ids(items []tracking.ListingWithApplication) []string {
	var out []string
	for _, i := range items {
		out = append(out, i.JobListing.ID)
	}
	return out
}

func TestFilterListings_ByPriority(t *testing.T) {
	items := []tracking.ListingWithApplication{
		prioritized("a", tracking.PriorityLow),
		prioritized("b", tracking.PriorityHigh),
		prioritized("c", tracking.PriorityNone),
	}
	got := ids(tracking.FilterListings(items, tracking.FilterParams{Priority: tracking.PriorityHigh}))
	if len(got) != 1 || got[0] != "b" {
		t.Errorf("expected only b, got %v", got)
	}
	if n := len(tracking.FilterListings(items, tracking.FilterParams{})); n != 3 {
		t.Errorf("zero value must keep everything, got %d", n)
	}
}

func TestSortListingsByPriority_HighFirstUndecidedLastStable(t *testing.T) {
	items := []tracking.ListingWithApplication{
		prioritized("none1", tracking.PriorityNone),
		prioritized("low", tracking.PriorityLow),
		prioritized("high1", tracking.PriorityHigh),
		prioritized("med", tracking.PriorityMedium),
		prioritized("high2", tracking.PriorityHigh),
		prioritized("none2", tracking.PriorityNone),
	}
	got := ids(tracking.SortListingsByPriority(items))
	want := []string{"high1", "high2", "med", "low", "none1", "none2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
	if items[0].JobListing.ID != "none1" {
		t.Error("input must not be reordered")
	}
}

func TestCorrectJobListing_SetsAndClearsPriority(t *testing.T) {
	dataDir := t.TempDir()
	writeRecord(t, dataDir, "jobs", "acme.md", "---\nschemaVersion: 4\ncompany: Acme\nsource: manual\nsavedAt: \"2026-09-08T09:14:07Z\"\nral:\n    source: n/a\nfreshnessStatus: not-yet-checked\n---\n\nBuild.\n")

	high := tracking.PriorityHigh
	l, err := tracking.CorrectJobListingIfMatch(dataDir, "acme", tracking.JobListingCorrection{Priority: &high}, "")
	if err != nil || l.Priority != tracking.PriorityHigh {
		t.Fatalf("expected high, got %v %v", l.Priority, err)
	}
	reloaded, err := tracking.GetJobListing(dataDir, "acme")
	if err != nil || reloaded.Priority != tracking.PriorityHigh {
		t.Fatalf("expected high persisted, got %v %v", reloaded.Priority, err)
	}
	none := tracking.PriorityNone
	l, err = tracking.CorrectJobListingIfMatch(dataDir, "acme", tracking.JobListingCorrection{Priority: &none}, "")
	if err != nil || l.Priority != tracking.PriorityNone {
		t.Fatalf("expected cleared, got %v %v", l.Priority, err)
	}
	bad := tracking.Priority("urgent")
	if _, err := tracking.CorrectJobListingIfMatch(dataDir, "acme", tracking.JobListingCorrection{Priority: &bad}, ""); !errors.Is(err, tracking.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

// ADR-0034: a record written before priority existed gets no priority from
// the migration, only the version stamp.
func TestMigrateRecords_InventsNoPriority(t *testing.T) {
	dataDir := t.TempDir()
	writeClosedV3(t, dataDir)
	if _, err := tracking.MigrateRecords(dataDir, true); err != nil {
		t.Fatal(err)
	}
	l, err := tracking.GetJobListing(dataDir, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if l.Priority != tracking.PriorityNone || l.SchemaVersion != tracking.CurrentSchemaVersion {
		t.Errorf("expected no priority at v%d, got %q v%d", tracking.CurrentSchemaVersion, l.Priority, l.SchemaVersion)
	}
}
