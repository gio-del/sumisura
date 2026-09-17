package tracking_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

const linkedInShare = "Check out this job at Acme Corp: https://www.linkedin.com/jobs/view/4012345678/?trackingId=abc"

func TestAddPendingCapture_SharedText_SavesWithHints(t *testing.T) {
	dataDir := t.TempDir()

	result, err := tracking.AddPendingCapture(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.PendingCaptureInput{Text: linkedInShare, Title: "Backend Engineer"})
	if err != nil {
		t.Fatal(err)
	}

	if result.Outcome != tracking.OutcomePending || result.PendingCapture == nil {
		t.Fatalf("expected a new pending capture, got %+v", result)
	}
	c := *result.PendingCapture
	if c.URL != "https://www.linkedin.com/jobs/view/4012345678/" || c.PostingKey != "linkedin:4012345678" || c.Provider != "linkedin" {
		t.Fatalf("unexpected link fields: %+v", c)
	}
	if c.Title != "Backend Engineer" || c.Company != "Acme Corp" || c.SharedText != "Check out this job at Acme Corp:" {
		t.Fatalf("unexpected hints: %+v", c)
	}
	if c.SchemaVersion != tracking.PendingCaptureSchemaVersion || c.SavedAt == "" {
		t.Fatalf("missing schema version or savedAt: %+v", c)
	}
	listed, err := tracking.ListPendingCaptures(dataDir)
	if err != nil || len(listed) != 1 || listed[0].ID != c.ID {
		t.Fatalf("expected the capture to be listed, got %v, %v", listed, err)
	}
}

func TestAddPendingCapture_SamePostingTwice_KeepsOne(t *testing.T) {
	dataDir := t.TempDir()
	first, err := tracking.AddPendingCapture(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.PendingCaptureInput{Text: linkedInShare})
	if err != nil {
		t.Fatal(err)
	}

	second, err := tracking.AddPendingCapture(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.PendingCaptureInput{URL: "https://www.linkedin.com/jobs/search-results/?currentJobId=4012345678"})
	if err != nil {
		t.Fatal(err)
	}

	if second.Outcome != tracking.OutcomeAlreadyPending || second.PendingCapture.ID != first.PendingCapture.ID {
		t.Fatalf("expected already-pending with the first record, got %+v", second)
	}
	if listed, _ := tracking.ListPendingCaptures(dataDir); len(listed) != 1 {
		t.Fatalf("expected one pending capture, got %d", len(listed))
	}
}

func TestAddPendingCapture_PostingAlreadyTracked_ReportsJobListing(t *testing.T) {
	dataDir := t.TempDir()
	listing, _, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company: "Acme Corp", URL: "https://www.linkedin.com/jobs/view/4012345678/", JobDescription: "A role.",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := tracking.AddPendingCapture(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.PendingCaptureInput{Text: linkedInShare})
	if err != nil {
		t.Fatal(err)
	}

	if result.Outcome != tracking.OutcomeAlreadyTracked || result.JobListingID != listing.ID || result.PendingCapture != nil {
		t.Fatalf("expected already-tracked pointing at %s, got %+v", listing.ID, result)
	}
	if listed, _ := tracking.ListPendingCaptures(dataDir); len(listed) != 0 {
		t.Fatalf("expected nothing written, got %v", listed)
	}
}

func TestAddPendingCapture_NoLink_ValidationError(t *testing.T) {
	_, err := tracking.AddPendingCapture(context.Background(), t.TempDir(), &fakeFreshnessClient{}, nil, tracking.PendingCaptureInput{Text: "Backend Engineer at Acme"})
	if !errors.Is(err, tracking.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestListPendingCaptures_NoDirectory_Empty(t *testing.T) {
	listed, err := tracking.ListPendingCaptures(t.TempDir())
	if err != nil || len(listed) != 0 {
		t.Fatalf("expected empty inbox, got %v, %v", listed, err)
	}
}

func TestCompletePendingCapture_SavesJobListingAndRemovesCapture(t *testing.T) {
	dataDir := t.TempDir()
	added, err := tracking.AddPendingCapture(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.PendingCaptureInput{Text: linkedInShare, Title: "Backend Engineer"})
	if err != nil {
		t.Fatal(err)
	}

	listing, application, err := tracking.CompletePendingCapture(context.Background(), dataDir, &fakeFreshnessClient{}, nil, added.PendingCapture.ID,
		tracking.CompletePendingCaptureRequest{Company: "Acme Corp", JobDescription: "Build backends."})
	if err != nil {
		t.Fatal(err)
	}

	if listing.URL != added.PendingCapture.URL || listing.Title != "Backend Engineer" || listing.Company != "Acme Corp" || listing.JobDescription != "Build backends." {
		t.Fatalf("unexpected job listing: %+v", listing)
	}
	if application.Status != tracking.StatusSaved {
		t.Fatalf("expected a Saved application, got %+v", application)
	}
	if _, err := tracking.GetPendingCapture(dataDir, added.PendingCapture.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected the pending capture gone, got %v", err)
	}
}

func TestCompletePendingCapture_MissingJobDescription_KeepsCapture(t *testing.T) {
	dataDir := t.TempDir()
	added, err := tracking.AddPendingCapture(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.PendingCaptureInput{Text: linkedInShare})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = tracking.CompletePendingCapture(context.Background(), dataDir, &fakeFreshnessClient{}, nil, added.PendingCapture.ID,
		tracking.CompletePendingCaptureRequest{Company: "Acme Corp"})

	if !errors.Is(err, tracking.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if _, err := tracking.GetPendingCapture(dataDir, added.PendingCapture.ID); err != nil {
		t.Fatalf("expected the pending capture kept, got %v", err)
	}
}

func TestPendingCapture_UnknownOrMalformedID_NotExist(t *testing.T) {
	dataDir := t.TempDir()
	for _, id := range []string{"linkedin-0123456789ab", "../jobs/acme", ""} {
		if _, err := tracking.GetPendingCapture(dataDir, id); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Get(%q): expected ErrNotExist, got %v", id, err)
		}
		if err := tracking.DeletePendingCapture(dataDir, id); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Delete(%q): expected ErrNotExist, got %v", id, err)
		}
	}
}

func TestListPendingCaptures_IgnoresLeftoverTempFiles(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := tracking.AddPendingCapture(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.PendingCaptureInput{Text: linkedInShare}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "pending-captures", ".linkedin-0123456789ab.json.tmp123"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	listed, err := tracking.ListPendingCaptures(dataDir)
	if err != nil || len(listed) != 1 {
		t.Fatalf("expected one capture and no error, got %v, %v", listed, err)
	}
}
