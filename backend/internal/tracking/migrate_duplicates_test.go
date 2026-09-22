package tracking_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// A corpus written before one-Job-Listing-per-Posting-Key (issue #206,
// ADR-0042) may hold duplicates. migrate-records names them so they can be
// cleaned up by hand, and never resolves one itself (stories 11, 12).

// writeListingPair writes a current-schema Job Listing and its Application.
func writeListingPair(t *testing.T, dataDir, id, title, url, savedAt string, status tracking.Status) {
	t.Helper()
	writeRecord(t, dataDir, "jobs", id+".md", "---\n"+
		"schemaVersion: 1\n"+
		"title: "+title+"\n"+
		"company: Acme\n"+
		"url: "+url+"\n"+
		"source: manual\n"+
		"savedAt: \""+savedAt+"\"\n"+
		"ral:\n    source: n/a\n"+
		"freshnessStatus: not-yet-checked\n"+
		"---\n\nBuild Go services.\n")
	writeRecord(t, dataDir, "applications", id+".md", "schemaVersion: 1\n"+
		"jobListingId: "+id+"\n"+
		"status: "+string(status)+"\n"+
		"statusUpdatedAt: \""+savedAt+"\"\n"+
		"method:\n    kind: portal\n    value: \"\"\n")
}

// seedDuplicatePosting writes two Job Listings for one LinkedIn posting,
// reached by the two URLs a real corpus would hold: the search pane and
// the posting's own page.
func seedDuplicatePosting(t *testing.T) string {
	t.Helper()
	dataDir := t.TempDir()
	writeListingPair(t, dataDir, "acme", "Backend Engineer",
		"https://www.linkedin.com/jobs/view/4012345678/", "2026-09-08T09:14:07Z", tracking.StatusSaved)
	writeListingPair(t, dataDir, "acme-2", "Backend Engineer",
		"https://www.linkedin.com/jobs/search/?currentJobId=4012345678", "2026-09-11T10:00:00Z", tracking.StatusSent)
	return dataDir
}

func TestMigrateRecords_PostingKeyHeldByTwoListings_IsReported(t *testing.T) {
	dataDir := seedDuplicatePosting(t)

	report := migrate(t, dataDir, true)

	if len(report.DuplicatePostings) != 1 {
		t.Fatalf("expected one duplicated posting, got %+v", report.DuplicatePostings)
	}
	dup := report.DuplicatePostings[0]
	if dup.PostingKey != "linkedin:4012345678" {
		t.Errorf("expected the posting named by its key, got %q", dup.PostingKey)
	}
	var ids []string
	for _, l := range dup.Listings {
		ids = append(ids, l.ID)
	}
	if !reflect.DeepEqual(ids, []string{"acme", "acme-2"}) {
		t.Errorf("expected both listings named, got %v", ids)
	}
	// Story 11: each one's Job Title, saved date and Status, so the user
	// can tell which to keep without opening the files.
	first := dup.Listings[0]
	if first.Title != "Backend Engineer" || first.SavedAt != "2026-09-08T09:14:07Z" || first.Status != tracking.StatusSaved {
		t.Errorf("expected title, saved date and status on the report, got %+v", first)
	}
	if dup.Listings[1].Status != tracking.StatusSent {
		t.Errorf("expected the second listing's own Status, got %q", dup.Listings[1].Status)
	}
}

func TestMigrateRecords_DuplicatePosting_CountsAsPendingWork(t *testing.T) {
	dataDir := seedDuplicatePosting(t)

	// Every record is already at the current schema version, so the only
	// pending work is the duplicate.
	report := migrate(t, dataDir, true)
	if len(report.Migrated) != 0 {
		t.Fatalf("expected nothing to migrate, got %+v", report.Migrated)
	}
	if !report.Pending() {
		t.Error("expected a duplicated posting to count as pending work")
	}
}

// Story 12: no tool decides which of two Status histories survives.
func TestMigrateRecords_DuplicatePosting_WriteMergesNothing(t *testing.T) {
	dataDir := seedDuplicatePosting(t)
	before := snapshotDir(t, dataDir)

	report := migrate(t, dataDir, false)

	if len(report.DuplicatePostings) != 1 {
		t.Errorf("expected the write run to report the duplicate too, got %+v", report.DuplicatePostings)
	}
	if after := snapshotDir(t, dataDir); !reflect.DeepEqual(before, after) {
		t.Error("expected -write to leave both duplicated Job Listings exactly as they were")
	}
}

func TestMigrateRecords_DistinctPostings_AreNotReported(t *testing.T) {
	dataDir := t.TempDir()
	writeListingPair(t, dataDir, "acme", "Backend Engineer",
		"https://www.linkedin.com/jobs/view/4012345678/", "2026-09-08T09:14:07Z", tracking.StatusSaved)
	writeListingPair(t, dataDir, "acme-2", "Frontend Engineer",
		"https://www.linkedin.com/jobs/view/4099999999/", "2026-09-11T10:00:00Z", tracking.StatusSaved)

	if report := migrate(t, dataDir, true); len(report.DuplicatePostings) != 0 {
		t.Errorf("expected two distinct postings to be left alone, got %+v", report.DuplicatePostings)
	}
}

// Listings with no computable Posting Key have no identity to collide on,
// so they are never grouped together.
func TestMigrateRecords_ListingsWithoutAPostingKey_AreNotReported(t *testing.T) {
	dataDir := t.TempDir()
	writeListingPair(t, dataDir, "acme", "Backend Engineer", "", "2026-09-08T09:14:07Z", tracking.StatusSaved)
	writeListingPair(t, dataDir, "acme-2", "Backend Engineer", "", "2026-09-11T10:00:00Z", tracking.StatusSaved)

	if report := migrate(t, dataDir, true); len(report.DuplicatePostings) != 0 {
		t.Errorf("expected URL-less listings to be left alone, got %+v", report.DuplicatePostings)
	}
}

// An archived duplicate is still a duplicate: archiving is not a way to
// resolve one, so the report keeps naming it and says which it is.
func TestMigrateRecords_ArchivedDuplicate_IsStillReported(t *testing.T) {
	dataDir := seedDuplicatePosting(t)
	path := filepath.Join(dataDir, "jobs", "acme-2.md")
	content := readRecord(t, path)
	content = strings.Replace(content, "freshnessStatus: not-yet-checked\n", "freshnessStatus: not-yet-checked\narchived: true\n", 1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	report := migrate(t, dataDir, true)

	if len(report.DuplicatePostings) != 1 {
		t.Fatalf("expected the archived duplicate still reported, got %+v", report.DuplicatePostings)
	}
	if !report.DuplicatePostings[0].Listings[1].Archived {
		t.Error("expected the report to say which of the two is archived")
	}
}

// A legacy corpus is the common case: the duplicate report must work
// alongside the schema migration rather than instead of it.
func TestMigrateRecords_LegacyDuplicate_IsMigratedAndReported(t *testing.T) {
	dataDir := seedLegacyPair(t)
	writeRecord(t, dataDir, "jobs", "example-co-2.md",
		strings.Replace(legacyJobListingFixture, "title: Backend Engineer", "title: Backend Engineer (reposted)", 1))
	writeRecord(t, dataDir, "applications", "example-co-2.md",
		strings.Replace(legacyApplicationFixture, "jobListingId: example-co", "jobListingId: example-co-2", 1))

	report := migrate(t, dataDir, false)

	if len(report.Migrated) != 4 {
		t.Errorf("expected all four legacy records migrated, got %d", len(report.Migrated))
	}
	if len(report.DuplicatePostings) != 1 {
		t.Fatalf("expected the duplicate reported alongside the migration, got %+v", report.DuplicatePostings)
	}
	if got := report.DuplicatePostings[0].PostingKey; got != "other:jobs.example.com/example-co/1" {
		t.Errorf("unexpected posting key %q", got)
	}
}
