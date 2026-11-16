package tracking_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func TestRead_NewerSchemaVersionFailsLoudly(t *testing.T) {
	newer := tracking.CurrentSchemaVersion + 1

	t.Run("job listing", func(t *testing.T) {
		dataDir := seedLegacyPair(t)
		writeRecord(t, dataDir, "jobs", legacyFixtureID+".md", fmt.Sprintf("---\nschemaVersion: %d\ncompany: Example Co\nsource: manual\n---\n\nBody.\n", newer))

		if _, err := tracking.GetJobListing(dataDir, legacyFixtureID); !errors.Is(err, tracking.ErrUnsupportedSchemaVersion) {
			t.Errorf("GetJobListing: expected ErrUnsupportedSchemaVersion, got %v", err)
		}
		if _, err := tracking.List(dataDir); !errors.Is(err, tracking.ErrUnsupportedSchemaVersion) {
			t.Errorf("List: expected ErrUnsupportedSchemaVersion, got %v", err)
		}
	})

	t.Run("application", func(t *testing.T) {
		dataDir := seedLegacyPair(t)
		writeRecord(t, dataDir, "applications", legacyFixtureID+".md", fmt.Sprintf("schemaVersion: %d\n%s", newer, legacyApplicationFixture))

		_, err := tracking.Get(dataDir, legacyFixtureID)
		if !errors.Is(err, tracking.ErrUnsupportedSchemaVersion) {
			t.Fatalf("Get: expected ErrUnsupportedSchemaVersion, got %v", err)
		}
		if !strings.Contains(err.Error(), fmt.Sprint(newer)) {
			t.Errorf("expected the error to name the version it found, got %q", err)
		}
		if _, err := tracking.UpdateApplicationStatus(dataDir, legacyFixtureID, tracking.StatusTailoring); !errors.Is(err, tracking.ErrUnsupportedSchemaVersion) {
			t.Errorf("a write path must refuse to rewrite a newer record, got %v", err)
		}
	})

	t.Run("generation", func(t *testing.T) {
		dataDir := seedLegacyPair(t)
		writeRecord(t, dataDir, "applications", legacyFixtureID+".md", legacyApplicationFixture+fmt.Sprintf("generations:\n    - schemaVersion: %d\n      slug: s\n", newer))

		if _, err := tracking.Get(dataDir, legacyFixtureID); !errors.Is(err, tracking.ErrUnsupportedSchemaVersion) {
			t.Errorf("expected ErrUnsupportedSchemaVersion, got %v", err)
		}
	})
}

func TestSchemaVersion_LegacyIsTheZeroValue(t *testing.T) {
	if tracking.LegacySchemaVersion != 0 {
		t.Fatalf("an absent schemaVersion key must read as the legacy version, got LegacySchemaVersion=%d", tracking.LegacySchemaVersion)
	}
	if tracking.CurrentSchemaVersion <= tracking.LegacySchemaVersion {
		t.Fatalf("CurrentSchemaVersion (%d) must be newer than legacy", tracking.CurrentSchemaVersion)
	}
}

func TestSave_StampsBothRecordsAtCurrentSchemaVersion(t *testing.T) {
	dataDir := t.TempDir()
	listing, application, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company:        "Acme Corp",
		JobDescription: "Go backend engineer.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if listing.SchemaVersion != tracking.CurrentSchemaVersion || application.SchemaVersion != tracking.CurrentSchemaVersion {
		t.Fatalf("expected both returned records at v%d, got listing v%d, application v%d", tracking.CurrentSchemaVersion, listing.SchemaVersion, application.SchemaVersion)
	}

	reloaded, err := tracking.Get(dataDir, listing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.JobListing.SchemaVersion != tracking.CurrentSchemaVersion || reloaded.Application.SchemaVersion != tracking.CurrentSchemaVersion {
		t.Fatalf("expected both persisted records at v%d, got listing v%d, application v%d", tracking.CurrentSchemaVersion, reloaded.JobListing.SchemaVersion, reloaded.Application.SchemaVersion)
	}
}

func TestRead_UnversionedRecordsLoadAsLegacy(t *testing.T) {
	dataDir := seedLegacyPair(t)

	got, err := tracking.Get(dataDir, legacyFixtureID)
	if err != nil {
		t.Fatalf("unversioned records must keep loading before migration runs: %v", err)
	}
	if got.JobListing.SchemaVersion != tracking.LegacySchemaVersion || got.Application.SchemaVersion != tracking.LegacySchemaVersion {
		t.Fatalf("expected both records legacy, got listing v%d, application v%d", got.JobListing.SchemaVersion, got.Application.SchemaVersion)
	}
	if got.JobListing.FreshnessStatus != tracking.FreshnessNotYetChecked {
		t.Errorf("a legacy Job Listing without freshnessStatus must still read as not-yet-checked, got %q", got.JobListing.FreshnessStatus)
	}
}

// The not-yet-checked default is a legacy-record rule only: a current Job
// Listing always has freshnessStatus written down (by Save, or by
// migration), so the reader no longer invents one for it.
func TestRead_FreshnessDefaultAppliesToLegacyJobListingsOnly(t *testing.T) {
	dataDir := seedLegacyPair(t)
	legacy, err := tracking.GetJobListing(dataDir, legacyFixtureID)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.FreshnessStatus != tracking.FreshnessNotYetChecked {
		t.Errorf("legacy Job Listing: expected not-yet-checked, got %q", legacy.FreshnessStatus)
	}

	writeRecord(t, dataDir, "jobs", legacyFixtureID+".md", "---\nschemaVersion: 1\ncompany: Example Co\nsource: manual\n---\n\nBody.\n")
	current, err := tracking.GetJobListing(dataDir, legacyFixtureID)
	if err != nil {
		t.Fatal(err)
	}
	if current.FreshnessStatus != "" {
		t.Errorf("current Job Listing: expected freshnessStatus read exactly as stored (absent), got %q", current.FreshnessStatus)
	}
}

func TestRecordGeneration_StampsTheGenerationAtCurrentSchemaVersion(t *testing.T) {
	dataDir := seedLegacyPair(t)

	application, err := tracking.RecordGeneration(dataDir, legacyFixtureID, tracking.GenerationRecord{Slug: "example-co-1", CreatedAt: "2026-09-12T10:00:00Z", CVPath: "output/example-co-1/cv.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(application.Generations); n != 1 {
		t.Fatalf("expected one Generation, got %d", n)
	}
	if application.Generations[0].SchemaVersion != tracking.CurrentSchemaVersion {
		t.Errorf("expected the returned Generation at v%d, got v%d", tracking.CurrentSchemaVersion, application.Generations[0].SchemaVersion)
	}

	reloaded, err := tracking.Get(dataDir, legacyFixtureID)
	if err != nil {
		t.Fatal(err)
	}
	gen := reloaded.Application.Generations[0]
	if gen.SchemaVersion != tracking.CurrentSchemaVersion || gen.IsLegacy() {
		t.Errorf("expected the persisted Generation at v%d and not legacy, got v%d", tracking.CurrentSchemaVersion, gen.SchemaVersion)
	}
	if !(tracking.GenerationRecord{}).IsLegacy() {
		t.Error("a Generation with no schemaVersion must report itself legacy")
	}
}

// TestWritePaths_PreserveTheRecordVersion drives every write path over a
// legacy pair and a current pair: none may drop a version, and none may
// silently advance a legacy record either — only migration does that, so
// the backfill it performs is never skipped by an edit that happened first.
func TestWritePaths_PreserveTheRecordVersion(t *testing.T) {
	ctx := context.Background()
	writes := []struct {
		name  string
		write func(t *testing.T, dataDir, id string) error
	}{
		{"status transition", func(t *testing.T, dataDir, id string) error {
			_, err := tracking.UpdateApplicationStatus(dataDir, id, tracking.StatusTailoring)
			return err
		}},
		{"contact update", func(t *testing.T, dataDir, id string) error {
			_, err := tracking.AddContactIfMatch(dataDir, id, tracking.Contact{Name: "Ada", Email: "ada@example.com"}, "")
			return err
		}},
		{"method correction", func(t *testing.T, dataDir, id string) error {
			_, err := tracking.UpdateApplicationMethod(dataDir, id, tracking.ApplicationMethod{Kind: tracking.MethodPortal, Value: "https://jobs.example.com/apply"})
			return err
		}},
		{"generation recording", func(t *testing.T, dataDir, id string) error {
			_, err := tracking.RecordGeneration(dataDir, id, tracking.GenerationRecord{Slug: "s", CreatedAt: "2026-09-12T10:00:00Z"})
			return err
		}},
		{"note added", func(t *testing.T, dataDir, id string) error {
			_, err := tracking.AddNote(dataDir, id, "Called the recruiter.")
			return err
		}},
		{"resolve", func(t *testing.T, dataDir, id string) error {
			_, _, err := tracking.Resolve(ctx, dataDir, &fakeFreshnessClient{}, id)
			return err
		}},
		{"archive", func(t *testing.T, dataDir, id string) error {
			_, err := tracking.SetArchived(dataDir, id, true)
			return err
		}},
		{"freshness check", func(t *testing.T, dataDir, id string) error {
			_, err := tracking.CheckFreshness(ctx, dataDir, fakeFreshnessDoer{do: func(*http.Request) (*http.Response, error) {
				return emptyFreshnessResponse(http.StatusOK), nil
			}}, id)
			return err
		}},
	}

	seeds := []struct {
		name  string
		seed  func(t *testing.T) (dataDir, id string)
		wantV int
	}{
		{"legacy", func(t *testing.T) (string, string) { return seedLegacyPair(t), legacyFixtureID }, tracking.LegacySchemaVersion},
		{"current", func(t *testing.T) (string, string) {
			dataDir := t.TempDir()
			listing, _, err := tracking.Save(ctx, dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
				Company:        "Acme Corp",
				URL:            "https://jobs.example.com/acme/1",
				JobDescription: "Go backend engineer.",
			})
			if err != nil {
				t.Fatal(err)
			}
			return dataDir, listing.ID
		}, tracking.CurrentSchemaVersion},
	}

	for _, s := range seeds {
		for _, w := range writes {
			t.Run(s.name+"/"+w.name, func(t *testing.T) {
				dataDir, id := s.seed(t)
				if err := w.write(t, dataDir, id); err != nil {
					t.Fatal(err)
				}
				got, err := tracking.Get(dataDir, id)
				if err != nil {
					t.Fatal(err)
				}
				if got.JobListing.SchemaVersion != s.wantV || got.Application.SchemaVersion != s.wantV {
					t.Errorf("expected both records still at v%d, got listing v%d, application v%d", s.wantV, got.JobListing.SchemaVersion, got.Application.SchemaVersion)
				}
			})
		}
	}
}
