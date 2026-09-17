package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

const fixtureGreenhouseCustomDomainResponse = `{
  "jobs": [
    {"id": 7, "title": "Data Engineer", "location": {"name": "Milan"}, "absolute_url": "https://careers.acme.example/open-roles?gh_jid=4567890", "content": "<p>Pipelines.</p>"},
    {"id": 8, "title": "Backend Engineer", "location": {"name": "Remote"}, "absolute_url": "https://boards.greenhouse.io/acme-corp/jobs/4567999", "content": "<p>Join us.</p>"}
  ]
}`

func newATSShareServer(t *testing.T, respond func(*http.Request) *http.Response) (*httptest.Server, *[]string) {
	t.Helper()
	var requested []string
	doer := fakeATSDoer{do: func(req *http.Request) (*http.Response, error) {
		requested = append(requested, req.URL.String())
		return respond(req), nil
	}}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: seedDataDir(t), GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: doer}))
	t.Cleanup(server.Close)
	return server, &requested
}

// TestShareATSLink_ResolvesStraightIntoJobListing is issue #184: a public
// Greenhouse posting shared from a phone needs no desktop step.
func TestShareATSLink_ResolvesStraightIntoJobListing(t *testing.T) {
	server, requested := newATSShareServer(t, func(*http.Request) *http.Response {
		return jsonATSResponse(http.StatusOK, fixtureGreenhouseCustomDomainResponse)
	})

	body := call(t, http.MethodPost, server.URL+"/api/pending-captures",
		map[string]any{"url": "https://job-boards.greenhouse.io/acme-corp/jobs/4567999?gh_src=share"}, http.StatusCreated)

	var got struct {
		Outcome      string `json:"outcome"`
		Message      string `json:"message"`
		JobListingID string `json:"jobListingId"`
		JobListing   struct {
			Title          string `json:"title"`
			Company        string `json:"company"`
			URL            string `json:"url"`
			JobDescription string `json:"jobDescription"`
		} `json:"jobListing"`
		Application struct {
			Status string `json:"status"`
		} `json:"application"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Outcome != "job-listing" || got.Message != "Saved as a Job Listing." || got.JobListingID == "" {
		t.Fatalf("unexpected outcome: %s", body)
	}
	if got.JobListing.Title != "Backend Engineer" || got.JobListing.Company != "Acme Corp" || !strings.Contains(got.JobListing.JobDescription, "Join us.") || got.Application.Status != "saved" {
		t.Fatalf("unexpected job listing: %s", body)
	}
	if len(*requested) != 1 || !strings.Contains((*requested)[0], "/boards/acme-corp/") {
		t.Fatalf("expected one Greenhouse board request for acme-corp, got %v", *requested)
	}
	if list := call(t, http.MethodGet, server.URL+"/api/pending-captures", nil, http.StatusOK); strings.TrimSpace(string(list)) != "[]" {
		t.Fatalf("expected nothing pending, got %s", list)
	}

	again := call(t, http.MethodPost, server.URL+"/api/pending-captures",
		map[string]any{"url": "https://boards.greenhouse.io/acme-corp/jobs/4567999"}, http.StatusOK)
	if !strings.Contains(string(again), `"outcome":"already-tracked"`) {
		t.Fatalf("expected a second share to be already tracked, got %s", again)
	}
}

func TestShareATSLink_PostingOnCustomCareersDomain_FoundByJobID(t *testing.T) {
	server, _ := newATSShareServer(t, func(*http.Request) *http.Response {
		return jsonATSResponse(http.StatusOK, fixtureGreenhouseCustomDomainResponse)
	})

	body := call(t, http.MethodPost, server.URL+"/api/pending-captures",
		map[string]any{"url": "https://boards.greenhouse.io/acme-corp/jobs/4567890"}, http.StatusCreated)

	if !strings.Contains(string(body), `"outcome":"job-listing"`) || !strings.Contains(string(body), `"title":"Data Engineer"`) {
		t.Fatalf("expected the custom-domain posting resolved, got %s", body)
	}
}

func TestShareATSLink_LookupFails_FallsBackToPendingCapture(t *testing.T) {
	for _, tc := range []struct {
		name    string
		respond func(*http.Request) *http.Response
	}{
		{"board not found", func(*http.Request) *http.Response { return jsonATSResponse(http.StatusNotFound, `{}`) }},
		{"posting closed", func(*http.Request) *http.Response { return jsonATSResponse(http.StatusOK, `{"jobs": []}`) }},
		{"server error", func(*http.Request) *http.Response { return jsonATSResponse(http.StatusBadGateway, ``) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := newATSShareServer(t, tc.respond)

			body := call(t, http.MethodPost, server.URL+"/api/pending-captures",
				map[string]any{"url": "https://jobs.lever.co/acme/0f8b2c3a-1d2e-4f5a-9b8c-7d6e5f4a3b2c"}, http.StatusCreated)

			if !strings.Contains(string(body), `"outcome":"pending"`) || !strings.Contains(string(body), `"provider":"lever"`) {
				t.Fatalf("expected a pending capture, got %s", body)
			}
		})
	}
}

func TestShareNonATSLink_NeverCallsTheATSDoer(t *testing.T) {
	server, requested := newATSShareServer(t, func(*http.Request) *http.Response {
		t.Error("ATS doer called for a non-ATS link")
		return jsonATSResponse(http.StatusInternalServerError, ``)
	})

	call(t, http.MethodPost, server.URL+"/api/pending-captures",
		map[string]any{"text": "https://www.linkedin.com/jobs/view/4012345678/"}, http.StatusCreated)

	if len(*requested) != 0 {
		t.Fatalf("expected no ATS requests, got %v", *requested)
	}
}
