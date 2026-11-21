package tracking_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

const sampleListingFile = "---\ntitle: Backend Engineer\ncompany: Acme Corp\nurl: https://boards.example.com/acme/jobs/1\nsource: manual\nsavedAt: \"2026-01-02T03:04:05Z\"\nral:\n    source: unresolved\n---\n\nGo backend engineer.\n"

const sampleApplicationFile = "jobListingId: acme-corp\nstatus: sent\nstatusUpdatedAt: \"2026-01-02T03:04:05Z\"\nmethod:\n    kind: portal\ngenerations:\n    - slug: acme-corp\n      createdAt: \"2026-01-02T03:04:05Z\"\n      cvPath: output/acme-corp/cv.pdf\n"

// makeReadOnly makes dir unwritable, which is what a write that cannot
// complete looks like from the store's side: the previous record must
// survive it.
func makeReadOnly(t *testing.T, dir string) {
	t.Helper()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func plantListingAndApplication(t *testing.T, dataDir string) {
	t.Helper()
	writeFile(t, filepath.Join(dataDir, "jobs", "acme-corp.md"), sampleListingFile)
	writeFile(t, filepath.Join(dataDir, "applications", "acme-corp.md"), sampleApplicationFile)
}

// Stories 1 and 6: a Status transition that cannot complete leaves the
// Application at its previous Status, with its Status history and
// Generation records readable — not a truncated file.
func TestUpdateApplicationStatus_FailedWriteLeavesApplicationIntact(t *testing.T) {
	dataDir := t.TempDir()
	plantListingAndApplication(t, dataDir)
	makeReadOnly(t, filepath.Join(dataDir, "applications"))

	if _, err := tracking.UpdateApplicationStatus(dataDir, "acme-corp", tracking.StatusInterviewing); err == nil {
		t.Fatal("expected an error when the write cannot complete, got nil")
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "applications", "acme-corp.md"))
	if err != nil {
		t.Fatalf("reading application after failed write: %v", err)
	}
	if string(got) != sampleApplicationFile {
		t.Errorf("application was damaged by a failed write:\nwant %q\ngot  %q", sampleApplicationFile, string(got))
	}

	listings, err := tracking.List(dataDir)
	if err != nil {
		t.Fatalf("the Job Listings view is unreadable after a failed write: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 job listing, got %d", len(listings))
	}
	application := listings[0].Application
	if application.Status != tracking.StatusSent {
		t.Errorf("expected the previous Status %q, got %q", tracking.StatusSent, application.Status)
	}
	if len(application.Generations) != 1 {
		t.Errorf("expected the Generation history preserved, got %d records", len(application.Generations))
	}
}

// Story 1: the same guarantee for a Generation history append.
func TestRecordGeneration_FailedWriteLeavesApplicationIntact(t *testing.T) {
	dataDir := t.TempDir()
	plantListingAndApplication(t, dataDir)
	makeReadOnly(t, filepath.Join(dataDir, "applications"))

	if _, err := tracking.RecordGeneration(dataDir, "acme-corp", tracking.GenerationRecord{Slug: "acme-corp-2"}); err == nil {
		t.Fatal("expected an error when the write cannot complete, got nil")
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "applications", "acme-corp.md"))
	if err != nil {
		t.Fatalf("reading application after failed write: %v", err)
	}
	if string(got) != sampleApplicationFile {
		t.Errorf("application was damaged by a failed write:\nwant %q\ngot  %q", sampleApplicationFile, string(got))
	}
}

// Story 1: the same guarantee for an Application Method correction.
func TestUpdateApplicationMethod_FailedWriteLeavesApplicationIntact(t *testing.T) {
	dataDir := t.TempDir()
	plantListingAndApplication(t, dataDir)
	makeReadOnly(t, filepath.Join(dataDir, "applications"))

	method := tracking.ApplicationMethod{Kind: tracking.MethodEmail}
	if _, err := tracking.UpdateApplicationMethod(dataDir, "acme-corp", method); err == nil {
		t.Fatal("expected an error when the write cannot complete, got nil")
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "applications", "acme-corp.md"))
	if err != nil {
		t.Fatalf("reading application after failed write: %v", err)
	}
	if string(got) != sampleApplicationFile {
		t.Errorf("application was damaged by a failed write:\nwant %q\ngot  %q", sampleApplicationFile, string(got))
	}
}

// Story 1: the same guarantee for a Contact update.
func TestUpdateApplicationContact_FailedWriteLeavesApplicationIntact(t *testing.T) {
	dataDir := t.TempDir()
	plantListingAndApplication(t, dataDir)
	makeReadOnly(t, filepath.Join(dataDir, "applications"))

	contact := tracking.Contact{Name: "Dana", Email: "dana@example.com"}
	if _, err := tracking.AddContactIfMatch(dataDir, "acme-corp", contact, ""); err == nil {
		t.Fatal("expected an error when the write cannot complete, got nil")
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "applications", "acme-corp.md"))
	if err != nil {
		t.Fatalf("reading application after failed write: %v", err)
	}
	if string(got) != sampleApplicationFile {
		t.Errorf("application was damaged by a failed write:\nwant %q\ngot  %q", sampleApplicationFile, string(got))
	}
}

// Story 5: an interrupted write of a Job Listing leaves the previous
// version — and its captured Job Description — intact.
func TestCheckFreshness_FailedWriteLeavesJobListingIntact(t *testing.T) {
	dataDir := t.TempDir()
	plantListingAndApplication(t, dataDir)
	makeReadOnly(t, filepath.Join(dataDir, "jobs"))

	doer := fakeFreshnessDoer{do: func(req *http.Request) (*http.Response, error) {
		return emptyFreshnessResponse(http.StatusOK), nil
	}}
	if _, err := tracking.CheckFreshness(context.Background(), dataDir, doer, "acme-corp"); err == nil {
		t.Fatal("expected an error when the write cannot complete, got nil")
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "jobs", "acme-corp.md"))
	if err != nil {
		t.Fatalf("reading job listing after failed write: %v", err)
	}
	if string(got) != sampleListingFile {
		t.Errorf("job listing was damaged by a failed write:\nwant %q\ngot  %q", sampleListingFile, string(got))
	}
}

// Stories 8 and 10: debris left by a crashed write neither blacks out the
// whole Job Listings view nor shows up as a phantom row.
func TestList_IgnoresLeftoverTempFiles(t *testing.T) {
	dataDir := t.TempDir()
	plantListingAndApplication(t, dataDir)
	writeFile(t, filepath.Join(dataDir, "jobs", ".acme-corp.md.123456.tmp"), "---\ncompany: half a fi")
	writeFile(t, filepath.Join(dataDir, "applications", ".acme-corp.md.654321.tmp"), "jobListingId: acme-c")

	listings, err := tracking.List(dataDir)
	if err != nil {
		t.Fatalf("unexpected error listing alongside leftover temp files: %v", err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 job listing, got %d: %+v", len(listings), listings)
	}
	if listings[0].JobListing.ID != "acme-corp" {
		t.Errorf("expected job listing acme-corp, got %q", listings[0].JobListing.ID)
	}
}

// Story 10: a leftover temp file for the same slug must not push a newly
// saved Job Listing onto a suffixed slug.
func TestSave_SlugUnaffectedByLeftoverTempFile(t *testing.T) {
	dataDir := t.TempDir()
	writeFile(t, filepath.Join(dataDir, "jobs", ".acme-corp.md.123456.tmp"), "---\ncompany: half a fi")

	listing, _, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company:        "Acme Corp",
		JobDescription: "Go backend engineer.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listing.ID != "acme-corp" {
		t.Errorf("expected slug unaffected by debris (acme-corp), got %q", listing.ID)
	}
}

// Story 11: a saved Job Listing and its Application keep the permissions
// they have today.
func TestSave_WrittenRecordsKeepTheirPermissionMode(t *testing.T) {
	dataDir := t.TempDir()
	listing, _, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company:        "Acme Corp",
		JobDescription: "Go backend engineer.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, rel := range []string{
		filepath.Join("jobs", listing.ID+".md"),
		filepath.Join("applications", listing.ID+".md"),
	} {
		info, err := os.Stat(filepath.Join(dataDir, rel))
		if err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Errorf("%s: expected mode %v, got %v", rel, os.FileMode(0o644), info.Mode().Perm())
		}
	}
}

// The same guarantee for archiving a Job Listing (issue #98's write,
// added after this helper existed).
func TestSetArchived_FailedWriteLeavesJobListingIntact(t *testing.T) {
	dataDir := t.TempDir()
	plantListingAndApplication(t, dataDir)
	makeReadOnly(t, filepath.Join(dataDir, "jobs"))

	if _, err := tracking.SetArchived(dataDir, "acme-corp", true); err == nil {
		t.Fatal("expected an error when the write cannot complete, got nil")
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "jobs", "acme-corp.md"))
	if err != nil {
		t.Fatalf("reading job listing after failed write: %v", err)
	}
	if string(got) != sampleListingFile {
		t.Errorf("job listing was damaged by a failed write:\nwant %q\ngot  %q", sampleListingFile, string(got))
	}
}

// The same guarantee for a Note write (issue #96's write, added after this
// helper existed); AddNote, EditNote and DeleteNote share one write path.
func TestAddNote_FailedWriteLeavesApplicationIntact(t *testing.T) {
	dataDir := t.TempDir()
	plantListingAndApplication(t, dataDir)
	makeReadOnly(t, filepath.Join(dataDir, "applications"))

	if _, err := tracking.AddNote(dataDir, "acme-corp", "Recruiter call on Tuesday."); err == nil {
		t.Fatal("expected an error when the write cannot complete, got nil")
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "applications", "acme-corp.md"))
	if err != nil {
		t.Fatalf("reading application after failed write: %v", err)
	}
	if string(got) != sampleApplicationFile {
		t.Errorf("application was damaged by a failed write:\nwant %q\ngot  %q", sampleApplicationFile, string(got))
	}
}
