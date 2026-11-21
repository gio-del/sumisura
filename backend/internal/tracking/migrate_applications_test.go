package tracking_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func unknowableFields(m tracking.RecordMigration) map[string]string {
	fields := map[string]string{}
	for _, f := range m.Unknowable {
		fields[f.Field] = f.Reason
	}
	return fields
}

// The oldest vintage known to exist in the wild, byte for byte: a Saved
// Application with no statusUpdatedAt, no statusHistory, no generations and
// an unresolved method, paired with a Job Listing with no freshnessStatus.
func TestMigrateRecords_RealLegacyVintagePair(t *testing.T) {
	dataDir := seedLegacyPair(t)
	savedAt, _ := time.Parse(time.RFC3339Nano, legacyFixtureSavedAt)

	report := migrate(t, dataDir, false)

	if len(report.Migrated) != 2 || report.Scanned != 2 || len(report.Inconsistencies) != 0 {
		t.Fatalf("expected both records migrated with no inconsistency, got %+v", report)
	}
	m, ok := findMigration(report, "applications/example-co.md")
	if !ok {
		t.Fatalf("expected the report to name applications/example-co.md, got %+v", report.Migrated)
	}
	if m.Kind != tracking.RecordApplication || m.FromVersion != tracking.LegacySchemaVersion || m.ToVersion != tracking.CurrentSchemaVersion {
		t.Errorf("unexpected migration header: %+v", m)
	}
	if v, ok := backfilled(m, "statusUpdatedAt"); !ok || v != legacyFixtureSavedAt {
		t.Errorf("expected statusUpdatedAt backfilled from the Job Listing's savedAt, got %+v", m.Backfilled)
	}
	if _, ok := backfilled(m, "statusHistory"); !ok {
		t.Errorf("expected statusHistory backfilled, got %+v", m.Backfilled)
	}
	if len(m.Unknowable) != 0 {
		t.Errorf("expected nothing unknowable on a Saved Application with no Generations, got %+v", m.Unknowable)
	}

	got, err := tracking.Get(dataDir, legacyFixtureID)
	if err != nil {
		t.Fatal(err)
	}
	app := got.Application
	if app.SchemaVersion != tracking.CurrentSchemaVersion || got.JobListing.SchemaVersion != tracking.CurrentSchemaVersion {
		t.Errorf("expected both records at v%d, got listing v%d, application v%d", tracking.CurrentSchemaVersion, got.JobListing.SchemaVersion, app.SchemaVersion)
	}
	if app.Status != tracking.StatusSaved || app.StatusUpdatedAt != legacyFixtureSavedAt {
		t.Errorf("expected Saved since %s, got %s since %q", legacyFixtureSavedAt, app.Status, app.StatusUpdatedAt)
	}
	wantHistory := []tracking.StatusChange{{Status: tracking.StatusSaved, ChangedAt: savedAt}}
	if len(app.StatusHistory) != 1 || app.StatusHistory[0].Status != tracking.StatusSaved || !app.StatusHistory[0].ChangedAt.Equal(savedAt) {
		t.Errorf("expected history %+v, got %+v", wantHistory, app.StatusHistory)
	}
	if app.Method.Kind != tracking.MethodUnresolved || len(app.Contacts) != 0 || len(app.Generations) != 0 || len(app.Notes) != 0 {
		t.Errorf("expected the rest of the Application untouched, got %+v", app)
	}
	if got.JobListing.FreshnessStatus != tracking.FreshnessNotYetChecked {
		t.Errorf("expected freshness not-yet-checked, got %q", got.JobListing.FreshnessStatus)
	}
}

// Never synthesise a Status history past Saved: stats.go computes
// time-in-stage from consecutive history pairs, so an invented entry would
// be a confidently wrong number, and an invented statusUpdatedAt would make
// the stale nudges fire on a clock nobody set.
func TestMigrateRecords_ApplicationPastSaved_LeavesTimestampAndHistoryEmpty(t *testing.T) {
	for _, status := range []tracking.Status{tracking.StatusTailoring, tracking.StatusSent, tracking.StatusInterviewing, tracking.StatusRejected, tracking.StatusOffer, tracking.StatusWithdrawn} {
		t.Run(string(status), func(t *testing.T) {
			dataDir := seedLegacyPair(t)
			writeRecord(t, dataDir, "applications", legacyFixtureID+".md", strings.Replace(legacyApplicationFixture, "status: saved", "status: "+string(status), 1))

			m, ok := findMigration(migrate(t, dataDir, false), "applications/example-co.md")
			if !ok {
				t.Fatal("expected the Application migrated")
			}
			if len(m.Backfilled) != 0 {
				t.Errorf("expected nothing backfilled past Saved, got %+v", m.Backfilled)
			}
			unknowable := unknowableFields(m)
			for _, field := range []string{"statusUpdatedAt", "statusHistory"} {
				if unknowable[field] == "" {
					t.Errorf("expected %s reported unknowable with a reason, got %+v", field, m.Unknowable)
				}
			}

			app, err := tracking.Get(dataDir, legacyFixtureID)
			if err != nil {
				t.Fatal(err)
			}
			if app.Application.SchemaVersion != tracking.CurrentSchemaVersion {
				t.Errorf("expected the Application stamped, got v%d", app.Application.SchemaVersion)
			}
			if app.Application.StatusUpdatedAt != "" || len(app.Application.StatusHistory) != 0 || app.Application.IsStale {
				t.Errorf("expected no timestamp, no history and not stale, got %+v", app.Application)
			}
		})
	}
}

func TestMigrateRecords_BackfillsOnlyWhatIsAbsent(t *testing.T) {
	t.Run("saved with a timestamp but no history", func(t *testing.T) {
		dataDir := seedLegacyPair(t)
		writeRecord(t, dataDir, "applications", legacyFixtureID+".md", "jobListingId: example-co\nstatus: saved\nstatusUpdatedAt: \"2026-09-08T09:14:07.2Z\"\nmethod:\n    kind: other\n")

		m, _ := findMigration(migrate(t, dataDir, false), "applications/example-co.md")
		if _, ok := backfilled(m, "statusUpdatedAt"); ok {
			t.Errorf("expected an existing statusUpdatedAt left alone, got %+v", m.Backfilled)
		}
		if _, ok := backfilled(m, "statusHistory"); !ok {
			t.Errorf("expected statusHistory backfilled, got %+v", m.Backfilled)
		}
		app, _ := tracking.Get(dataDir, legacyFixtureID)
		if app.Application.StatusUpdatedAt != "2026-09-08T09:14:07.2Z" || len(app.Application.StatusHistory) != 1 {
			t.Errorf("unexpected Application after migration: %+v", app.Application)
		}
	})

	t.Run("past saved with a timestamp but no history", func(t *testing.T) {
		dataDir := seedLegacyPair(t)
		writeRecord(t, dataDir, "applications", legacyFixtureID+".md", "jobListingId: example-co\nstatus: sent\nstatusUpdatedAt: \"2026-09-10T12:00:00Z\"\nmethod:\n    kind: other\n")

		m, _ := findMigration(migrate(t, dataDir, false), "applications/example-co.md")
		unknowable := unknowableFields(m)
		if _, ok := unknowable["statusUpdatedAt"]; ok {
			t.Errorf("a present statusUpdatedAt is not unknowable, got %+v", m.Unknowable)
		}
		if _, ok := unknowable["statusHistory"]; !ok {
			t.Errorf("expected statusHistory unknowable, got %+v", m.Unknowable)
		}
		app, _ := tracking.Get(dataDir, legacyFixtureID)
		if app.Application.StatusUpdatedAt != "2026-09-10T12:00:00Z" || len(app.Application.StatusHistory) != 0 {
			t.Errorf("unexpected Application after migration: %+v", app.Application)
		}
	})

	t.Run("saved with an unparseable savedAt", func(t *testing.T) {
		dataDir := seedLegacyPair(t)
		writeRecord(t, dataDir, "jobs", legacyFixtureID+".md", strings.Replace(legacyJobListingFixture, legacyFixtureSavedAt, "last tuesday", 1))

		m, _ := findMigration(migrate(t, dataDir, false), "applications/example-co.md")
		if len(m.Backfilled) != 0 || unknowableFields(m)["statusHistory"] == "" || unknowableFields(m)["statusUpdatedAt"] == "" {
			t.Errorf("expected nothing backfilled from an unparseable savedAt, got %+v", m)
		}
	})
}

// Migration must not drop data it does not understand, nor stamp a legacy
// Generation current: its absent fields stay unknowable for good.
func TestMigrateRecords_ContactGenerationsAndNotesSurviveUnchanged(t *testing.T) {
	dataDir := seedLegacyPair(t)
	full := legacyApplicationFixture +
		"contact:\n" +
		"    name: Ada Lovelace\n" +
		"    email: ada@example.com\n" +
		"generations:\n" +
		"    - slug: example-co-1\n" +
		"      createdat: \"2026-09-09T10:00:00Z\"\n" +
		"      cvpath: output/example-co-1/cv.pdf\n" +
		"    - slug: example-co-2\n" +
		"      createdat: \"2026-09-10T10:00:00Z\"\n" +
		"      cvpath: output/example-co-2/cv.pdf\n" +
		"      coverletterpath: output/example-co-2/cover-letter.pdf\n" +
		"      groundedness:\n" +
		"        coverLetter:\n" +
		"            - claim: led a team\n" +
		"      sourcesnippetids:\n" +
		"        - why-go\n" +
		"      usage:\n" +
		"        inputtokens: 1200\n" +
		"        outputtokens: 300\n" +
		"        estimatedcostusd: 0.02\n" +
		"      language: en\n" +
		"      entryids:\n" +
		"        - example-client-a\n" +
		"notes:\n" +
		"    - id: \"1757410000000000000\"\n" +
		"      createdAt: \"2026-09-09T11:00:00Z\"\n" +
		"      body: Recruiter replied.\n" +
		"futureKey:\n" +
		"    nested: value\n"
	path := writeRecord(t, dataDir, "applications", legacyFixtureID+".md", full)
	before, err := tracking.Get(dataDir, legacyFixtureID)
	if err != nil {
		t.Fatal(err)
	}

	m, _ := findMigration(migrate(t, dataDir, false), "applications/example-co.md")

	after, err := tracking.Get(dataDir, legacyFixtureID)
	if err != nil {
		t.Fatal(err)
	}
	a, b := after.Application, before.Application
	if !reflect.DeepEqual(a.Generations, b.Generations) || !reflect.DeepEqual(a.Notes, b.Notes) || a.Method != b.Method {
		t.Errorf("migration changed data it only had to carry:\nbefore: %+v\nafter:  %+v", b, a)
	}
	if len(a.Contacts) != 1 || a.Contacts[0].Name != "Ada Lovelace" || a.Contacts[0].Email != "ada@example.com" || a.Contacts[0].Role != tracking.ContactRecruiter || a.Contacts[0].ID == "" {
		t.Errorf("expected the legacy contact as a one-element recruiter list, got %+v", a.Contacts)
	}
	if strings.Contains(readRecord(t, path), "\ncontact:") {
		t.Errorf("expected the single contact key gone after migration, got:\n%s", readRecord(t, path))
	}
	for _, g := range a.Generations {
		if !g.IsLegacy() {
			t.Errorf("expected Generation %s left legacy, got v%d", g.Slug, g.SchemaVersion)
		}
	}
	if !strings.Contains(readRecord(t, path), "nested: value") {
		t.Error("expected an unknown key kept")
	}
	if len(a.Generations) != 2 || a.Generations[1].Usage.InputTokens != 1200 || a.Generations[1].Language != "en" {
		t.Fatalf("fixture did not parse as intended: %+v", a.Generations)
	}

	unknowable := unknowableFields(m)
	for _, field := range []string{"generations[0].sourceSnippetIds", "generations[0].entryIds", "generations[0].usage", "generations[0].language"} {
		if unknowable[field] == "" {
			t.Errorf("expected %s reported unknowable, got %+v", field, m.Unknowable)
		}
	}
	for field := range unknowable {
		if strings.HasPrefix(field, "generations[1]") {
			t.Errorf("a Generation carrying every field has nothing unknowable, got %s", field)
		}
	}
}

func TestMigrateRecords_ApplicationWithoutJobListingIsReported(t *testing.T) {
	dataDir := seedLegacyPair(t)
	os.Remove(filepath.Join(dataDir, "jobs", legacyFixtureID+".md"))

	report := migrate(t, dataDir, false)

	found := false
	for _, inc := range report.Inconsistencies {
		if inc.Path == "applications/example-co.md" && strings.Contains(inc.Problem, "jobs/example-co.md") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the missing Job Listing reported, got %+v", report.Inconsistencies)
	}
	m, ok := findMigration(report, "applications/example-co.md")
	if !ok {
		t.Fatal("expected the orphan Application still stamped")
	}
	if len(m.Backfilled) != 0 || unknowableFields(m)["statusUpdatedAt"] == "" {
		t.Errorf("expected no saved date to backfill from, got %+v", m)
	}
}

func TestMigrateRecords_UnparseableApplicationHalts(t *testing.T) {
	for name, content := range map[string]string{
		"invalid yaml":      "jobListingId: [unclosed\n",
		"not a mapping":     "- just\n- a list\n",
		"newer generation":  legacyApplicationFixture + "generations:\n    - schemaVersion: 99\n      slug: s\n",
		"newer application": "schemaVersion: 99\n" + legacyApplicationFixture,
	} {
		t.Run(name, func(t *testing.T) {
			dataDir := seedLegacyPair(t)
			writeRecord(t, dataDir, "applications", legacyFixtureID+".md", content)
			before := snapshotDir(t, dataDir)

			_, err := tracking.MigrateRecords(dataDir, tracking.MigrationOptions{})
			if err == nil || !strings.Contains(err.Error(), "applications/example-co.md") {
				t.Fatalf("expected an error naming applications/example-co.md, got %v", err)
			}
			if strings.HasPrefix(name, "newer") && !errors.Is(err, tracking.ErrUnsupportedSchemaVersion) {
				t.Errorf("expected ErrUnsupportedSchemaVersion, got %v", err)
			}
			if after := snapshotDir(t, dataDir); !reflect.DeepEqual(before, after) {
				t.Error("expected nothing written, not even the valid Job Listing")
			}
		})
	}
}

func TestMigrateRecords_StatusTransitionAfterMigrationKeepsVersionAndHistory(t *testing.T) {
	dataDir := seedLegacyPair(t)
	migrate(t, dataDir, false)

	app, err := tracking.UpdateApplicationStatus(dataDir, legacyFixtureID, tracking.StatusTailoring)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := tracking.Get(dataDir, legacyFixtureID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Application.SchemaVersion != tracking.CurrentSchemaVersion {
		t.Errorf("expected the transitioned Application still at v%d, got v%d", tracking.CurrentSchemaVersion, reloaded.Application.SchemaVersion)
	}
	h := reloaded.Application.StatusHistory
	if len(h) != 2 || h[0].Status != tracking.StatusSaved || h[1].Status != tracking.StatusTailoring || app.Status != tracking.StatusTailoring {
		t.Errorf("expected Saved then Tailoring, got %+v", h)
	}
	stats := tracking.ComputeStats([]tracking.Application{reloaded.Application})
	if len(stats.TimeInStage) != 1 || stats.TimeInStage[0].Status != tracking.StatusSaved {
		t.Errorf("expected one real time-in-Saved sample from the backfilled start, got %+v", stats.TimeInStage)
	}
}
