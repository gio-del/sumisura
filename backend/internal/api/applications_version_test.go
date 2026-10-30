package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

// listingVersions reads GET /api/job-listings and returns the version
// tokens of the Job Listing and of its Application. An Application and its
// Job Listing are two separate files sharing one id, so they carry two
// separate tokens.
func listingVersions(t *testing.T, serverURL, id string) (jobListing, application string) {
	t.Helper()
	resp, err := http.Get(serverURL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var listings []map[string]map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&listings); err != nil {
		t.Fatal(err)
	}
	for _, l := range listings {
		if l["jobListing"]["id"] != id {
			continue
		}
		jobListing, _ = l["jobListing"]["version"].(string)
		application, _ = l["application"]["version"].(string)
		return jobListing, application
	}
	t.Fatalf("job listing %q not found in the list response", id)
	return "", ""
}

const applicationOutOfBand = `jobListingId: acme-corp
status: interviewing
method:
    kind: portal
`

func TestListJobListings_ReturnsATokenForBothTheListingAndItsApplication(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	jobListing, application := listingVersions(t, server.URL, id)
	if jobListing == "" {
		t.Error("expected a version token on the Job Listing")
	}
	if application == "" {
		t.Error("expected a version token on the Application")
	}
	if jobListing == application {
		t.Error("expected the two files to carry two distinct tokens")
	}
}

func TestUpdateApplicationStatus_WithCurrentVersion_Succeeds(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	_, version := listingVersions(t, server.URL, id)

	resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "tailoring"}, version)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if next := decodeVersion(t, resp); next == "" || next == version {
		t.Errorf("expected a fresh version token in the write response, got %q", next)
	}
}

func TestUpdateApplicationStatus_WithStaleVersion_Returns409AndLeavesFileUntouched(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	_, version := listingVersions(t, server.URL, id)

	// A second tab (or the skill) moved the Application on; the stale tab
	// must not be able to drag it backwards through the funnel.
	path := filepath.Join(dataDir, "applications", id+".md")
	writeOutOfBand(t, path, applicationOutOfBand)

	resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "tailoring"}, version)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	if got := readFile(t, path); got != applicationOutOfBand {
		t.Errorf("expected the file to be byte-identical to its pre-request state, got:\n%s", got)
	}
}

func TestUpdateApplicationStatus_WithNoVersion_IsUnconditional(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "tailoring"}, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUpdateApplicationStatus_ApplicationDeletedOutOfBand_Returns404NotConflict(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	_, version := listingVersions(t, server.URL, id)

	removeFile(t, filepath.Join(dataDir, "applications", id+".md"))

	resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "tailoring"}, version)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateApplicationMethod_WithStaleVersion_Returns409AndLeavesFileUntouched(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	_, version := listingVersions(t, server.URL, id)

	path := filepath.Join(dataDir, "applications", id+".md")
	writeOutOfBand(t, path, applicationOutOfBand)

	payload := map[string]any{"kind": "email", "value": "jobs@acme.example"}
	resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/method", payload, version)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	if got := readFile(t, path); got != applicationOutOfBand {
		t.Errorf("expected the file to be byte-identical to its pre-request state, got:\n%s", got)
	}
}

func TestUpdateApplicationContact_WithStaleVersion_Returns409AndLeavesFileUntouched(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	_, version := listingVersions(t, server.URL, id)

	path := filepath.Join(dataDir, "applications", id+".md")
	writeOutOfBand(t, path, applicationOutOfBand)

	payload := map[string]any{"name": "A Recruiter", "email": "recruiter@acme.example"}
	resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/contact", payload, version)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	if got := readFile(t, path); got != applicationOutOfBand {
		t.Errorf("expected the file to be byte-identical to its pre-request state, got:\n%s", got)
	}
}

func TestUpdateApplicationFollowUp_WithStaleVersion_Returns409AndLeavesFileUntouched(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	_, version := listingVersions(t, server.URL, id)

	path := filepath.Join(dataDir, "applications", id+".md")
	writeOutOfBand(t, path, applicationOutOfBand)

	payload := map[string]any{"followUpAt": "2030-01-15"}
	resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/follow-up", payload, version)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	if got := readFile(t, path); got != applicationOutOfBand {
		t.Errorf("expected the file to be byte-identical to its pre-request state, got:\n%s", got)
	}
}

func TestUpdateApplicationFollowUp_SetsThenClearsTheDate(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	for _, want := range []string{"2030-01-15", ""} {
		resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/follow-up", map[string]any{"followUpAt": want}, "")
		var got struct {
			FollowUpAt string `json:"followUpAt"`
		}
		err := json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || err != nil || got.FollowUpAt != want {
			t.Fatalf("followUpAt %q: status %d, err %v, got %q", want, resp.StatusCode, err, got.FollowUpAt)
		}
	}

	resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/follow-up", map[string]any{"followUpAt": "soon"}, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for a malformed date, got %d", resp.StatusCode)
	}
}

// Recording a Generation is deliberately out of the check: it appends to
// the Generation history rather than rewriting a field the user has been
// holding open, and a bookkeeping conflict must never block the end of a
// Tailoring run (issue #89's Out of Scope).
func TestRecordGeneration_StaysUnconditionalAfterAnOutOfBandChange(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	writeOutOfBand(t, filepath.Join(dataDir, "applications", id+".md"), applicationOutOfBand)

	payload := map[string]any{"slug": "acme-corp-2026-09", "cvPath": "output/acme-corp-2026-09/cv.pdf"}
	resp := doJSON(t, http.MethodPost, server.URL+"/api/applications/"+id+"/generations", payload, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

// GET /api/job-listings/{id} answers with the {jobListing, application}
// pair since the Job Listing detail page landed (issue #94), and that page
// runs every Application action and the delete from it — so the pair
// carries both tokens, the same ones the list rows do.
func TestGetJobListing_ReturnsTheJobListingAndApplicationTokens(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	listJobListing, listApplication := listingVersions(t, server.URL, id)

	resp, err := http.Get(server.URL + "/api/job-listings/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	jobListing, application := pairVersions(t, resp)
	if jobListing == "" || application == "" {
		t.Fatalf("expected both tokens on the detail pair, got (%q, %q)", jobListing, application)
	}
	if jobListing != listJobListing || application != listApplication {
		t.Errorf("expected the detail read to carry the same tokens as the list, got (%q, %q) want (%q, %q)", jobListing, application, listJobListing, listApplication)
	}
}

// The detail pair's tokens are usable as-is: the Application's for a Status
// patch, then both for the delete, the detail page's exact sequence.
func TestGetJobListing_PairTokensGuardTheDetailPageWrites(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp, err := http.Get(server.URL + "/api/job-listings/" + id)
	if err != nil {
		t.Fatal(err)
	}
	jobListing, application := pairVersions(t, resp)
	resp.Body.Close()

	patch := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "tailoring"}, application)
	defer patch.Body.Close()
	if patch.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", patch.StatusCode)
	}
	freshApplication := decodeVersion(t, patch)

	// The page swaps in the patch response, so the stale Application token
	// it read first must no longer delete, and the fresh one must.
	stale := deleteJobListing(t, server.URL, id, jobListing, application)
	stale.Body.Close()
	if stale.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for the pre-patch Application token, got %d", stale.StatusCode)
	}
	fresh := deleteJobListing(t, server.URL, id, jobListing, freshApplication)
	fresh.Body.Close()
	if fresh.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 with the fresh tokens, got %d", fresh.StatusCode)
	}
}
