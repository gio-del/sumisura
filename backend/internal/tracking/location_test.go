package tracking_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// A Job Listing keeps where the role is (issue #206, stories 55-60). It is
// free text as the board wrote it — no normalization is attempted.

func TestSave_KeepsTheLocationItWasGiven(t *testing.T) {
	dataDir := t.TempDir()

	listing, _, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company:        "Acme",
		Title:          "Backend Engineer",
		Location:       "Milan, Lombardy, Italy",
		JobDescription: "Build Go services.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if listing.Location != "Milan, Lombardy, Italy" {
		t.Errorf("expected the location kept, got %q", listing.Location)
	}

	reloaded, err := tracking.GetJobListing(dataDir, listing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Location != "Milan, Lombardy, Italy" {
		t.Errorf("expected the location persisted, got %q", reloaded.Location)
	}
}

// Story 60: existing Job Listings keep working with no location.
func TestSave_NoLocation_IsAnOrdinaryEmptyValue(t *testing.T) {
	dataDir := t.TempDir()

	listing, _, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company: "Acme", JobDescription: "Build Go services.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if listing.Location != "" {
		t.Errorf("expected no location, got %q", listing.Location)
	}

	// A record without one carries no stray key, so it reads exactly like
	// a file written before the field existed.
	content, err := os.ReadFile(filepath.Join(dataDir, "jobs", listing.ID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "location:") {
		t.Errorf("expected no location key on a listing without one:\n%s", content)
	}
}

// ADR-0034's rule: a new field means a new schema version, never another
// "absent in older records" tolerance in a reader.
func TestMigrateRecords_LocationIsReportedAsUnknowable(t *testing.T) {
	dataDir := seedLegacyPair(t)

	report := migrate(t, dataDir, true)

	listing, ok := findMigration(report, "jobs/example-co.md")
	if !ok {
		t.Fatalf("expected the legacy Job Listing migrated, got %+v", report.Migrated)
	}
	if listing.ToVersion != tracking.CurrentSchemaVersion {
		t.Errorf("expected the listing brought to v%d, got v%d", tracking.CurrentSchemaVersion, listing.ToVersion)
	}
	found := false
	for _, f := range listing.Unknowable {
		if f.Field == "location" {
			found = true
			if f.Reason == "" {
				t.Error("expected a reason beside the unknowable location")
			}
		}
	}
	if !found {
		t.Errorf("expected location reported as unknowable, got %+v", listing.Unknowable)
	}
	if _, backfilled := backfilled(listing, "location"); backfilled {
		t.Error("expected location never backfilled: nothing on disk records it")
	}
}

// A corpus already at the previous version still needs the stamp, or the
// version stops telling the truth about what an absent field means.
func TestMigrateRecords_PreviousVersionRecords_AreStampedForward(t *testing.T) {
	dataDir := t.TempDir()
	writeRecord(t, dataDir, "jobs", "acme.md", "---\n"+
		"schemaVersion: 1\n"+
		"title: Backend Engineer\n"+
		"company: Acme\n"+
		"source: manual\n"+
		"savedAt: \"2026-09-08T09:14:07Z\"\n"+
		"ral:\n    source: n/a\n"+
		"freshnessStatus: not-yet-checked\n"+
		"---\n\nBuild Go services.\n")
	writeRecord(t, dataDir, "applications", "acme.md", "schemaVersion: 1\n"+
		"jobListingId: acme\n"+
		"status: saved\n"+
		"statusUpdatedAt: \"2026-09-08T09:14:07Z\"\n"+
		"method:\n    kind: portal\n    value: \"\"\n")

	migrate(t, dataDir, false)

	listing, err := tracking.GetJobListing(dataDir, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if listing.SchemaVersion != tracking.CurrentSchemaVersion {
		t.Errorf("expected the v1 listing stamped to v%d, got v%d", tracking.CurrentSchemaVersion, listing.SchemaVersion)
	}
	pair, err := tracking.Get(dataDir, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if pair.Application.SchemaVersion != tracking.CurrentSchemaVersion {
		t.Errorf("expected the v1 Application stamped to v%d, got v%d", tracking.CurrentSchemaVersion, pair.Application.SchemaVersion)
	}
	// Nothing else about the records changed.
	if listing.Location != "" || listing.Title != "Backend Engineer" {
		t.Errorf("expected only the version stamped, got %+v", listing)
	}
}
