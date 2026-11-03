package tracking_test

import (
	"errors"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func writeClosedV3(t *testing.T, dataDir string) {
	t.Helper()
	writeRecord(t, dataDir, "jobs", "acme.md", "---\n"+
		"schemaVersion: 3\n"+
		"title: Backend Engineer\n"+
		"company: Acme\n"+
		"source: manual\n"+
		"savedAt: \"2026-09-08T09:14:07Z\"\n"+
		"ral:\n    source: n/a\n"+
		"freshnessStatus: not-yet-checked\n"+
		"---\n\nBuild Go services.\n")
	writeRecord(t, dataDir, "applications", "acme.md", "schemaVersion: 3\n"+
		"jobListingId: acme\n"+
		"status: rejected\n"+
		"statusUpdatedAt: \"2026-09-09T09:14:07Z\"\n"+
		"method:\n    kind: portal\n    value: \"\"\n")
}

// ADR-0034: an Application closed before closeReason existed keeps no
// reason; the migration stamps the version and says so instead of guessing.
func TestMigrateRecords_ClosedApplicationKeepsNoCloseReason(t *testing.T) {
	dataDir := t.TempDir()
	writeClosedV3(t, dataDir)

	report := migrate(t, dataDir, false)

	m, ok := findMigration(report, "applications/acme.md")
	if !ok {
		t.Fatalf("expected the v3 Application migrated, got %+v", report.Migrated)
	}
	reported := false
	for _, f := range m.Unknowable {
		if f.Field == "closeReason" {
			reported = true
		}
	}
	if !reported {
		t.Errorf("expected closeReason reported as unknowable, got %+v", m.Unknowable)
	}
	if _, filled := backfilled(m, "closeReason"); filled {
		t.Error("expected closeReason never backfilled")
	}

	pair, err := tracking.Get(dataDir, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if pair.Application.SchemaVersion != tracking.CurrentSchemaVersion || pair.Application.CloseReason != "" {
		t.Errorf("expected stamped at v%d with no reason, got v%d %q", tracking.CurrentSchemaVersion, pair.Application.SchemaVersion, pair.Application.CloseReason)
	}
}

func TestUpdateApplicationCloseReason_RejectsOpenApplicationAndUnknownReason(t *testing.T) {
	dataDir := t.TempDir()
	writeClosedV3(t, dataDir)

	if _, err := tracking.UpdateApplicationCloseReasonIfMatch(dataDir, "acme", "bored", ""); !errors.Is(err, tracking.ErrValidation) {
		t.Errorf("expected ErrValidation for an unknown reason, got %v", err)
	}
	app, err := tracking.UpdateApplicationCloseReasonIfMatch(dataDir, "acme", tracking.CloseReasonPositionClosed, "")
	if err != nil {
		t.Fatal(err)
	}
	if app.CloseReason != tracking.CloseReasonPositionClosed {
		t.Errorf("expected position-closed, got %q", app.CloseReason)
	}
}
