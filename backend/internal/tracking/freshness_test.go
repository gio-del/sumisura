package tracking_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

type fakeFreshnessDoer struct {
	do func(*http.Request) (*http.Response, error)
}

func (f fakeFreshnessDoer) Do(req *http.Request) (*http.Response, error) {
	return f.do(req)
}

func emptyFreshnessResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}
}

func TestCheckFreshness_LiveResponse_UpdatesStatusAndTimestamp(t *testing.T) {
	dataDir := t.TempDir()
	listing, _, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company:        "Acme Corp",
		URL:            "https://boards.example.com/acme/jobs/1",
		JobDescription: "Go backend engineer.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if listing.FreshnessStatus != tracking.FreshnessNotYetChecked {
		t.Fatalf("expected default not-yet-checked, got %q", listing.FreshnessStatus)
	}

	doer := fakeFreshnessDoer{do: func(req *http.Request) (*http.Response, error) {
		return emptyFreshnessResponse(http.StatusOK), nil
	}}
	updated, err := tracking.CheckFreshness(context.Background(), dataDir, doer, listing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.FreshnessStatus != tracking.FreshnessLive {
		t.Errorf("expected live, got %q", updated.FreshnessStatus)
	}
	if updated.FreshnessCheckedAt == "" {
		t.Error("expected a non-empty FreshnessCheckedAt timestamp")
	}

	reloaded, err := tracking.GetJobListing(dataDir, listing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.FreshnessStatus != tracking.FreshnessLive || reloaded.FreshnessCheckedAt == "" {
		t.Errorf("expected the check to persist to disk, got %+v", reloaded)
	}
}

func TestCheckFreshness_404_MarksUnreachable(t *testing.T) {
	dataDir := t.TempDir()
	listing, _, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company:        "Acme Corp",
		URL:            "https://boards.example.com/acme/jobs/1",
		JobDescription: "Go backend engineer.",
	})
	if err != nil {
		t.Fatal(err)
	}

	doer := fakeFreshnessDoer{do: func(req *http.Request) (*http.Response, error) {
		return emptyFreshnessResponse(http.StatusNotFound), nil
	}}
	updated, err := tracking.CheckFreshness(context.Background(), dataDir, doer, listing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.FreshnessStatus != tracking.FreshnessUnreachable {
		t.Errorf("expected unreachable, got %q", updated.FreshnessStatus)
	}
}

func TestCheckFreshness_403_MarksUnknownNotUnreachable(t *testing.T) {
	dataDir := t.TempDir()
	listing, _, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company:        "Acme Corp",
		URL:            "https://boards.example.com/acme/jobs/1",
		JobDescription: "Go backend engineer.",
	})
	if err != nil {
		t.Fatal(err)
	}

	doer := fakeFreshnessDoer{do: func(req *http.Request) (*http.Response, error) {
		return emptyFreshnessResponse(http.StatusForbidden), nil
	}}
	updated, err := tracking.CheckFreshness(context.Background(), dataDir, doer, listing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.FreshnessStatus != tracking.FreshnessUnknown {
		t.Errorf("expected unknown, got %q", updated.FreshnessStatus)
	}
}

// A prior Live result must not be corrupted/cleared by an attempt that
// cannot even be made (story 12) — here, a Job Listing with no source URL
// recorded at all.
func TestCheckFreshness_NoURLRecorded_LeavesStatusUntouched(t *testing.T) {
	dataDir := t.TempDir()
	listing, _, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company:        "Acme Corp",
		JobDescription: "Go backend engineer.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if listing.URL != "" {
		t.Fatalf("expected no URL recorded for this fixture, got %q", listing.URL)
	}

	doerCalled := false
	doer := fakeFreshnessDoer{do: func(req *http.Request) (*http.Response, error) {
		doerCalled = true
		return emptyFreshnessResponse(http.StatusOK), nil
	}}
	updated, err := tracking.CheckFreshness(context.Background(), dataDir, doer, listing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if doerCalled {
		t.Error("expected no outbound request when the Job Listing has no source URL")
	}
	if updated.FreshnessStatus != tracking.FreshnessNotYetChecked || updated.FreshnessCheckedAt != "" {
		t.Errorf("expected status/timestamp to stay untouched, got %+v", updated)
	}
}

func TestCheckFreshness_NetworkError_MarksUnknownAndAdvancesTimestamp(t *testing.T) {
	dataDir := t.TempDir()
	listing, _, err := tracking.Save(context.Background(), dataDir, &fakeFreshnessClient{}, nil, tracking.SaveRequest{
		Company:        "Acme Corp",
		URL:            "https://boards.example.com/acme/jobs/1",
		JobDescription: "Go backend engineer.",
	})
	if err != nil {
		t.Fatal(err)
	}

	doer := fakeFreshnessDoer{do: func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: i/o timeout")
	}}
	updated, err := tracking.CheckFreshness(context.Background(), dataDir, doer, listing.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.FreshnessStatus != tracking.FreshnessUnknown {
		t.Errorf("expected unknown on a timeout, got %q", updated.FreshnessStatus)
	}
	if updated.FreshnessCheckedAt == "" {
		t.Error("a completed (if ambiguous) classification should still advance the checked-at timestamp")
	}
}

func TestCheckFreshness_UnknownID_ReturnsNotExistError(t *testing.T) {
	dataDir := t.TempDir()
	_, err := tracking.CheckFreshness(context.Background(), dataDir, fakeFreshnessDoer{}, "does-not-exist")
	if err == nil {
		t.Fatal("expected an error for an unknown Job Listing id")
	}
}

type fakeFreshnessClient struct{}

func (fakeFreshnessClient) SelectAndRewrite(ctx context.Context, req generation.SelectionRequest) (generation.SelectionResult, error) {
	return generation.SelectionResult{}, nil
}

func (fakeFreshnessClient) SelectOnly(ctx context.Context, req generation.SelectionRequest) (generation.SelectionResult, error) {
	return generation.SelectionResult{}, nil
}

func (fakeFreshnessClient) DraftCoverLetter(ctx context.Context, req generation.CoverLetterRequest) (generation.CoverLetterResult, error) {
	return generation.CoverLetterResult{}, nil
}

func (fakeFreshnessClient) EstimateRAL(ctx context.Context, jobDescription string) (generation.RALRange, error) {
	return generation.RALRange{Source: generation.RALSourceNA}, nil
}

func (fakeFreshnessClient) InferApplicationMethod(ctx context.Context, jobDescription string) (tracking.ApplicationMethod, error) {
	return tracking.ApplicationMethod{Kind: tracking.MethodOther}, nil
}

func (fakeFreshnessClient) SuggestCaptureHints(ctx context.Context, jobDescription string) (tracking.CaptureHints, error) {
	return tracking.CaptureHints{}, nil
}

func (fakeFreshnessClient) SuggestContact(ctx context.Context, company, jobDescription string) (tracking.Contact, error) {
	return tracking.Contact{}, nil
}
