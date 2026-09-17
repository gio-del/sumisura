package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type addPendingCaptureBody struct {
	Outcome        string `json:"outcome"`
	Message        string `json:"message"`
	JobListingID   string `json:"jobListingId"`
	PendingCapture *struct {
		ID         string `json:"id"`
		URL        string `json:"url"`
		PostingKey string `json:"postingKey"`
	} `json:"pendingCapture"`
}

func decodeAdd(t *testing.T, body []byte) addPendingCaptureBody {
	t.Helper()
	var out addPendingCaptureBody
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	return out
}

func TestPendingCaptures_ShareTwice_201ThenAlreadyPending(t *testing.T) {
	server := newSimpleServer(t, seedDataDir(t))

	first := decodeAdd(t, call(t, http.MethodPost, server.URL+"/api/pending-captures",
		map[string]any{"text": "Check out this job at Hooli: https://it.indeed.com/viewjob?jk=abc123def456"}, http.StatusCreated))
	second := decodeAdd(t, call(t, http.MethodPost, server.URL+"/api/pending-captures",
		map[string]any{"url": "https://www.indeed.com/viewjob?jk=ABC123DEF456&from=share"}, http.StatusOK))

	if first.Outcome != "pending" || first.PendingCapture == nil || first.PendingCapture.PostingKey != "indeed:abc123def456" {
		t.Fatalf("unexpected first share: %+v", first)
	}
	if second.Outcome != "already-pending" || second.PendingCapture.ID != first.PendingCapture.ID || second.Message == "" {
		t.Fatalf("unexpected second share: %+v", second)
	}
}

func TestPendingCaptures_AlreadyTracked_ReportsJobListingWithoutWriting(t *testing.T) {
	server := newSimpleServer(t, seedDataDir(t))
	saved := call(t, http.MethodPost, server.URL+"/api/job-listings", map[string]any{
		"company": "Hooli", "url": "https://www.linkedin.com/jobs/view/4012345678/", "jobDescription": "A role.",
	}, http.StatusCreated)

	got := decodeAdd(t, call(t, http.MethodPost, server.URL+"/api/pending-captures",
		map[string]any{"text": "https://www.linkedin.com/jobs/view/4012345678"}, http.StatusOK))

	if got.Outcome != "already-tracked" || got.JobListingID != jobListingIDOf(t, saved) || got.PendingCapture != nil {
		t.Fatalf("unexpected result: %+v", got)
	}
	if list := call(t, http.MethodGet, server.URL+"/api/pending-captures", nil, http.StatusOK); strings.TrimSpace(string(list)) != "[]" {
		t.Fatalf("expected an empty inbox, got %s", list)
	}
}

func TestPendingCaptures_NoLink_400(t *testing.T) {
	server := newSimpleServer(t, seedDataDir(t))
	call(t, http.MethodPost, server.URL+"/api/pending-captures", map[string]any{"text": "no link here"}, http.StatusBadRequest)
}

func TestPendingCaptures_Complete_CreatesJobListingAndEmptiesInbox(t *testing.T) {
	server := newSimpleServer(t, seedDataDir(t))
	added := decodeAdd(t, call(t, http.MethodPost, server.URL+"/api/pending-captures",
		map[string]any{"url": "https://jobs.example/hooli/1", "title": "Backend Engineer"}, http.StatusCreated))

	completed := call(t, http.MethodPost, server.URL+"/api/pending-captures/"+added.PendingCapture.ID+"/complete",
		map[string]any{"company": "Hooli", "jobDescription": "A backend role."}, http.StatusCreated)

	var result struct {
		JobListing struct {
			ID    string `json:"id"`
			URL   string `json:"url"`
			Title string `json:"title"`
		} `json:"jobListing"`
	}
	if err := json.Unmarshal(completed, &result); err != nil {
		t.Fatal(err)
	}
	if result.JobListing.URL != "https://jobs.example/hooli/1" || result.JobListing.Title != "Backend Engineer" {
		t.Fatalf("unexpected job listing: %s", completed)
	}
	call(t, http.MethodGet, server.URL+"/api/job-listings/"+result.JobListing.ID, nil, http.StatusOK)
	if list := call(t, http.MethodGet, server.URL+"/api/pending-captures", nil, http.StatusOK); strings.TrimSpace(string(list)) != "[]" {
		t.Fatalf("expected an empty inbox, got %s", list)
	}
}

func TestPendingCaptures_CompleteWithoutJobDescription_400AndKept(t *testing.T) {
	server := newSimpleServer(t, seedDataDir(t))
	added := decodeAdd(t, call(t, http.MethodPost, server.URL+"/api/pending-captures",
		map[string]any{"url": "https://jobs.example/hooli/1"}, http.StatusCreated))

	call(t, http.MethodPost, server.URL+"/api/pending-captures/"+added.PendingCapture.ID+"/complete",
		map[string]any{"company": "Hooli"}, http.StatusBadRequest)

	if list := call(t, http.MethodGet, server.URL+"/api/pending-captures", nil, http.StatusOK); !strings.Contains(string(list), added.PendingCapture.ID) {
		t.Fatalf("expected the capture kept, got %s", list)
	}
}

func TestPendingCaptures_DeleteAndUnknown(t *testing.T) {
	server := newSimpleServer(t, seedDataDir(t))
	added := decodeAdd(t, call(t, http.MethodPost, server.URL+"/api/pending-captures",
		map[string]any{"url": "https://jobs.example/hooli/1"}, http.StatusCreated))

	call(t, http.MethodDelete, server.URL+"/api/pending-captures/"+added.PendingCapture.ID, nil, http.StatusNoContent)
	call(t, http.MethodDelete, server.URL+"/api/pending-captures/"+added.PendingCapture.ID, nil, http.StatusNotFound)
	call(t, http.MethodPost, server.URL+"/api/pending-captures/other-000000000000/complete",
		map[string]any{"company": "Hooli", "jobDescription": "x"}, http.StatusNotFound)
}

// TestExtensionCapture_CompletesMatchingPendingCapture is issue #183: the
// posting shared from a phone, captured later on the desktop through a
// different URL form, leaves the To complete inbox on its own.
func TestExtensionCapture_CompletesMatchingPendingCapture(t *testing.T) {
	for _, tc := range []struct{ name, shared, captured string }{
		{"linkedin", "Check out this job at Hooli: https://www.linkedin.com/jobs/view/4012345678/?trk=x", "https://www.linkedin.com/jobs/view/4012345678/"},
		{"indeed across country hosts", "https://it.indeed.com/viewjob?jk=abc123def456", "https://www.indeed.com/viewjob?jk=abc123def456"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newSimpleServer(t, seedDataDir(t))
			added := decodeAdd(t, call(t, http.MethodPost, server.URL+"/api/pending-captures", map[string]any{"text": tc.shared}, http.StatusCreated))

			captured := call(t, http.MethodPost, server.URL+"/api/job-listings/from-extension", map[string]any{
				"title": "Backend Engineer", "company": "Hooli", "url": tc.captured, "description": "A backend role.",
			}, http.StatusCreated)

			var result struct {
				CompletedPendingCaptureID string `json:"completedPendingCaptureId"`
			}
			if err := json.Unmarshal(captured, &result); err != nil {
				t.Fatal(err)
			}
			if result.CompletedPendingCaptureID != added.PendingCapture.ID {
				t.Fatalf("expected completedPendingCaptureId %q, got %s", added.PendingCapture.ID, captured)
			}
			if list := call(t, http.MethodGet, server.URL+"/api/pending-captures", nil, http.StatusOK); strings.TrimSpace(string(list)) != "[]" {
				t.Fatalf("expected an empty inbox, got %s", list)
			}
		})
	}
}

func TestExtensionCapture_NothingPending_NoCompletedField(t *testing.T) {
	server := newSimpleServer(t, seedDataDir(t))
	call(t, http.MethodPost, server.URL+"/api/pending-captures", map[string]any{"url": "https://www.linkedin.com/jobs/view/4099999999/"}, http.StatusCreated)

	captured := call(t, http.MethodPost, server.URL+"/api/job-listings/from-extension", map[string]any{
		"company": "Hooli", "url": "https://www.linkedin.com/jobs/view/4012345678/", "description": "A backend role.",
	}, http.StatusCreated)

	if strings.Contains(string(captured), "completedPendingCaptureId") {
		t.Fatalf("expected no completedPendingCaptureId, got %s", captured)
	}
	if list := call(t, http.MethodGet, server.URL+"/api/pending-captures", nil, http.StatusOK); !strings.Contains(string(list), "4099999999") {
		t.Fatalf("expected the unrelated capture kept, got %s", list)
	}
}

func TestManualSave_CompletesMatchingPendingCapture(t *testing.T) {
	server := newSimpleServer(t, seedDataDir(t))
	added := decodeAdd(t, call(t, http.MethodPost, server.URL+"/api/pending-captures", map[string]any{"url": "https://jobs.example/hooli/1"}, http.StatusCreated))

	saved := call(t, http.MethodPost, server.URL+"/api/job-listings", map[string]any{
		"company": "Hooli", "url": "https://jobs.example/hooli/1?utm_source=x", "jobDescription": "A backend role.",
	}, http.StatusCreated)

	if !strings.Contains(string(saved), `"completedPendingCaptureId":"`+added.PendingCapture.ID+`"`) {
		t.Fatalf("expected the pending capture completed, got %s", saved)
	}
}
