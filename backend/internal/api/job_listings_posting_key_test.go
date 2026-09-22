package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

// One Job Listing per Posting Key (issue #206, stories 1-10). These assert
// the HTTP refusal, never how the key is computed — postingkey_test.go
// already owns URL equivalence.

func postingKeyTestServer(t *testing.T) (string, func()) {
	t.Helper()
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	return server.URL, server.Close
}

// saveViaExtension captures a posting the way the browser extension does.
func saveViaExtension(t *testing.T, baseURL, company, url string) *http.Response {
	t.Helper()
	return postJSON(t, baseURL+"/api/job-listings/from-extension", map[string]any{
		"company":     company,
		"title":       "Backend Engineer",
		"url":         url,
		"description": "Backend Engineer at " + company + ".",
	})
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	return body
}

// createdListingID reads the id off a 201 save response.
func createdListingID(t *testing.T, resp *http.Response) string {
	t.Helper()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected the first save to succeed with 201, got %d", resp.StatusCode)
	}
	return decodeBody(t, resp)["jobListing"].(map[string]any)["id"].(string)
}

// assertDuplicateRefusal checks the shape every duplicate-posting refusal
// shares, whichever door the save came in through (story 9), and returns
// the refusal body.
func assertDuplicateRefusal(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected a re-save of the same posting to be refused with 409, got %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["reason"] != "duplicate-posting" {
		t.Errorf("expected reason duplicate-posting, got %v", body["reason"])
	}
	if message, ok := body["message"].(string); !ok || message == "" {
		t.Errorf("expected a refusal message, got %v", body)
	}
	if _, ok := body["existing"].(map[string]any); !ok {
		t.Fatalf("expected the refusal to name the existing listing, got %v", body)
	}
	return body
}

// refusedListing is the existing Job Listing a refusal names.
func refusedListing(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	return assertDuplicateRefusal(t, resp)["existing"].(map[string]any)
}

func TestSave_SamePostingTwice_IsRefused(t *testing.T) {
	baseURL, closeServer := postingKeyTestServer(t)
	defer closeServer()

	first := saveViaExtension(t, baseURL, "Acme", "https://www.linkedin.com/jobs/view/4012345678/")
	defer first.Body.Close()
	firstID := createdListingID(t, first)

	second := saveViaExtension(t, baseURL, "Acme", "https://www.linkedin.com/jobs/view/4012345678/")
	defer second.Body.Close()
	existing := refusedListing(t, second)

	if existing["id"] != firstID {
		t.Errorf("expected the refusal to name %q, got %v", firstID, existing["id"])
	}
}

// Story 2: the search-pane URL and the posting's own /jobs/view/ URL are
// the same posting. Story 3: the same holds for Indeed and the ATS boards.
func TestSave_EquivalentPostingURLs_AreRefused(t *testing.T) {
	cases := []struct{ name, first, second string }{
		{
			"linkedin search pane then jobs/view",
			"https://www.linkedin.com/jobs/search/?currentJobId=4012345678&keywords=go",
			"https://www.linkedin.com/jobs/view/4012345678/",
		},
		{
			"indeed vjk then jk",
			"https://it.indeed.com/jobs?q=go&vjk=abc123def456",
			"https://it.indeed.com/viewjob?jk=abc123def456",
		},
		{
			"greenhouse board then apply link",
			"https://boards.greenhouse.io/acme/jobs/4567890",
			"https://job-boards.greenhouse.io/acme/jobs/4567890",
		},
		{
			"lever posting then its apply page",
			"https://jobs.lever.co/acme/2f1d4e6a-1111-2222-3333-444455556666",
			"https://jobs.lever.co/acme/2f1d4e6a-1111-2222-3333-444455556666/apply",
		},
		{
			"ashby posting then its application page",
			"https://jobs.ashbyhq.com/acme/2f1d4e6a-1111-2222-3333-444455556666",
			"https://jobs.ashbyhq.com/acme/2f1d4e6a-1111-2222-3333-444455556666/application",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, closeServer := postingKeyTestServer(t)
			defer closeServer()

			first := saveViaExtension(t, baseURL, "Acme", tc.first)
			defer first.Body.Close()
			firstID := createdListingID(t, first)

			second := saveViaExtension(t, baseURL, "Acme", tc.second)
			defer second.Body.Close()
			existing := refusedListing(t, second)

			if existing["id"] != firstID {
				t.Errorf("expected the refusal to name %q, got %v", firstID, existing["id"])
			}
		})
	}
}

// Story 4: the refusal names the Job Title and when it was saved, so the
// user can go and look at it.
func TestSave_DuplicateRefusal_NamesTitleAndSavedAt(t *testing.T) {
	baseURL, closeServer := postingKeyTestServer(t)
	defer closeServer()

	first := saveViaExtension(t, baseURL, "Acme", "https://www.linkedin.com/jobs/view/4012345678/")
	defer first.Body.Close()
	firstBody := decodeBody(t, first)
	saved := firstBody["jobListing"].(map[string]any)

	second := saveViaExtension(t, baseURL, "Acme", "https://www.linkedin.com/jobs/view/4012345678/")
	defer second.Body.Close()
	existing := refusedListing(t, second)

	if existing["title"] != saved["title"] {
		t.Errorf("expected title %v, got %v", saved["title"], existing["title"])
	}
	if existing["company"] != saved["company"] {
		t.Errorf("expected company %v, got %v", saved["company"], existing["company"])
	}
	if existing["savedAt"] != saved["savedAt"] {
		t.Errorf("expected savedAt %v, got %v", saved["savedAt"], existing["savedAt"])
	}
	if existing["archived"] != false {
		t.Errorf("expected archived false on a live listing, got %v", existing["archived"])
	}
}

// Stories 6-7: an archived match is refused too, and says so, but the
// message itself reads exactly like any other refusal.
func TestSave_ArchivedMatch_IsRefusedAndFlaggedWithoutChangingTheMessage(t *testing.T) {
	baseURL, closeServer := postingKeyTestServer(t)
	defer closeServer()

	live := saveViaExtension(t, baseURL, "Globex", "https://www.linkedin.com/jobs/view/4099999999/")
	defer live.Body.Close()
	liveID := createdListingID(t, live)
	liveRefusal := saveViaExtension(t, baseURL, "Globex", "https://www.linkedin.com/jobs/view/4099999999/")
	defer liveRefusal.Body.Close()
	liveMessage := assertDuplicateRefusal(t, liveRefusal)["message"]

	first := saveViaExtension(t, baseURL, "Acme", "https://www.linkedin.com/jobs/view/4012345678/")
	defer first.Body.Close()
	firstID := createdListingID(t, first)

	archive := postJSON(t, baseURL+"/api/job-listings/"+firstID+"/archive", map[string]any{})
	defer archive.Body.Close()
	if archive.StatusCode != http.StatusOK {
		t.Fatalf("expected archiving to succeed, got %d", archive.StatusCode)
	}

	second := saveViaExtension(t, baseURL, "Acme", "https://www.linkedin.com/jobs/view/4012345678/")
	defer second.Body.Close()
	refusal := assertDuplicateRefusal(t, second)
	existing := refusal["existing"].(map[string]any)

	if existing["id"] != firstID {
		t.Errorf("expected the refusal to name the archived listing %q, got %v", firstID, existing["id"])
	}
	if existing["archived"] != true {
		t.Errorf("expected archived true on an archived match, got %v", existing["archived"])
	}
	if refusal["message"] != liveMessage {
		t.Errorf("expected an archived match to read exactly like any other refusal\n live:     %q\n archived: %q", liveMessage, refusal["message"])
	}
	_ = liveID
}

// Story 9: the invariant is a property of the record, not the door.
func TestSave_DuplicateRefusal_AppliesToTheManualSavePath(t *testing.T) {
	baseURL, closeServer := postingKeyTestServer(t)
	defer closeServer()

	first := postJSON(t, baseURL+"/api/job-listings", map[string]any{
		"company":        "Acme",
		"title":          "Backend Engineer",
		"url":            "https://www.linkedin.com/jobs/view/4012345678/",
		"jobDescription": "Backend Engineer at Acme.",
	})
	defer first.Body.Close()
	firstID := createdListingID(t, first)

	// Captured later from the extension, on the search-pane URL.
	second := saveViaExtension(t, baseURL, "Acme", "https://www.linkedin.com/jobs/search/?currentJobId=4012345678")
	defer second.Body.Close()
	if existing := refusedListing(t, second); existing["id"] != firstID {
		t.Errorf("expected the refusal to name %q, got %v", firstID, existing["id"])
	}

	// And the other way round: the manual form refuses an extension capture.
	third := postJSON(t, baseURL+"/api/job-listings", map[string]any{
		"company":        "Acme",
		"title":          "Backend Engineer",
		"url":            "https://www.linkedin.com/jobs/view/4012345678/",
		"jobDescription": "Backend Engineer at Acme, pasted by hand.",
	})
	defer third.Body.Close()
	if existing := refusedListing(t, third); existing["id"] != firstID {
		t.Errorf("expected the refusal to name %q, got %v", firstID, existing["id"])
	}
}

// Story 10: identity that can't be computed is never a reason to refuse.
func TestSave_PostingWithNoRecognisableURL_StillSaves(t *testing.T) {
	baseURL, closeServer := postingKeyTestServer(t)
	defer closeServer()

	for _, url := range []string{"", "not a url", "careers@acme.example"} {
		first := postJSON(t, baseURL+"/api/job-listings", map[string]any{
			"company":        "Acme",
			"url":            url,
			"jobDescription": "Pasted by hand, no link.",
		})
		firstID := createdListingID(t, first)
		first.Body.Close()

		second := postJSON(t, baseURL+"/api/job-listings", map[string]any{
			"company":        "Acme",
			"url":            url,
			"jobDescription": "Pasted by hand again, still no link.",
		})
		secondID := createdListingID(t, second)
		second.Body.Close()

		if secondID == firstID {
			t.Fatalf("url %q: expected two distinct listings, got the same id %q", url, secondID)
		}
	}
}

// A refusal must leave the corpus untouched — no half-written record.
func TestSave_DuplicateRefusal_WritesNothing(t *testing.T) {
	baseURL, closeServer := postingKeyTestServer(t)
	defer closeServer()

	first := saveViaExtension(t, baseURL, "Acme", "https://www.linkedin.com/jobs/view/4012345678/")
	defer first.Body.Close()
	createdListingID(t, first)

	second := saveViaExtension(t, baseURL, "Acme", "https://www.linkedin.com/jobs/view/4012345678/")
	defer second.Body.Close()
	assertDuplicateRefusal(t, second)

	listed, err := http.Get(baseURL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer listed.Body.Close()
	var rows []map[string]any
	if err := json.NewDecoder(listed.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("expected the refused save to write nothing, found %d listings", len(rows))
	}
}

// Story 9 again: Pending Capture completion is a save path too. A capture
// whose posting was saved some other way is normally cleared from the
// inbox at save time, best-effort — when that clearing failed, the stale
// entry is still there to be completed, and must be refused rather than
// failing opaquely.
func TestCompletePendingCapture_StaleEntryForATrackedPosting_IsRefused(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	saved := saveViaExtension(t, server.URL, "Acme", "https://www.linkedin.com/jobs/view/4012345678/")
	defer saved.Body.Close()
	savedID := createdListingID(t, saved)

	// The inbox entry the best-effort clearing left behind.
	pendingID := writeStalePendingCapture(t, dataDir, "https://www.linkedin.com/jobs/view/4012345678/")

	completed := postJSON(t, server.URL+"/api/pending-captures/"+pendingID+"/complete", map[string]any{
		"company": "Acme", "title": "Backend Engineer", "jobDescription": "Pasted by hand.",
	})
	defer completed.Body.Close()
	if existing := refusedListing(t, completed); existing["id"] != savedID {
		t.Errorf("expected the refusal to name %q, got %v", savedID, existing["id"])
	}
}

// writeStalePendingCapture puts a Pending Capture on disk directly, which
// is the only way to reach the state the best-effort clearing normally
// prevents. Its id must match what the app derives from the Posting Key,
// so it is read back off the record the API writes for the same link.
func writeStalePendingCapture(t *testing.T, dataDir, url string) string {
	t.Helper()
	tmp := t.TempDir()
	scratch := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: tmp, GenerationClient: &fakeGenerationClient{}}))
	defer scratch.Close()

	shared := postJSON(t, scratch.URL+"/api/pending-captures", map[string]any{"url": url})
	defer shared.Body.Close()
	if shared.StatusCode != http.StatusCreated {
		t.Fatalf("expected the share to be accepted, got %d", shared.StatusCode)
	}
	id := decodeBody(t, shared)["pendingCapture"].(map[string]any)["id"].(string)

	record, err := os.ReadFile(filepath.Join(tmp, "pending-captures", id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(dataDir, "pending-captures")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), record, 0o644); err != nil {
		t.Fatal(err)
	}
	return id
}
