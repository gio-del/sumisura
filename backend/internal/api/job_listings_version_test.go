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

// deleteJobListing issues DELETE /api/job-listings/{id} carrying the Job
// Listing's token as If-Match and the Application's token as
// Application-If-Match, each only when non-empty — the FE's exact shape.
func deleteJobListing(t *testing.T, serverURL, id, jobListingVersion, applicationVersion string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, serverURL+"/api/job-listings/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if jobListingVersion != "" {
		req.Header.Set("If-Match", jobListingVersion)
	}
	if applicationVersion != "" {
		req.Header.Set("Application-If-Match", applicationVersion)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestDeleteJobListing_WithCurrentVersions_Succeeds(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	jobListing, application := listingVersions(t, server.URL, id)

	resp := deleteJobListing(t, server.URL, id, jobListing, application)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if fileExists(filepath.Join(dataDir, "jobs", id+".md")) {
		t.Error("expected the Job Listing file to be removed")
	}
}

func TestDeleteJobListing_WithStaleJobListingVersion_Returns409AndBothFilesSurvive(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	jobListing, application := listingVersions(t, server.URL, id)

	listingPath := filepath.Join(dataDir, "jobs", id+".md")
	changed := readFile(t, listingPath) + "\nEdited by hand.\n"
	writeOutOfBand(t, listingPath, changed)

	resp := deleteJobListing(t, server.URL, id, jobListing, application)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	if got := readFile(t, listingPath); got != changed {
		t.Errorf("expected the Job Listing file to survive unchanged, got:\n%s", got)
	}
	if !fileExists(filepath.Join(dataDir, "applications", id+".md")) {
		t.Error("expected the Application file to survive a refused delete")
	}
}

// Deleting a Job Listing destroys its Application too, so a Status that
// moved on since the list was loaded must also refuse the delete (story
// 19).
func TestDeleteJobListing_WithStaleApplicationVersion_Returns409AndBothFilesSurvive(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	jobListing, application := listingVersions(t, server.URL, id)

	applicationPath := filepath.Join(dataDir, "applications", id+".md")
	writeOutOfBand(t, applicationPath, applicationOutOfBand)

	resp := deleteJobListing(t, server.URL, id, jobListing, application)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
	if got := readFile(t, applicationPath); got != applicationOutOfBand {
		t.Errorf("expected the Application file to survive unchanged, got:\n%s", got)
	}
	if !fileExists(filepath.Join(dataDir, "jobs", id+".md")) {
		t.Error("expected the Job Listing file to survive a refused delete")
	}
}

func TestDeleteJobListing_WithNoVersion_IsUnconditional(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	writeOutOfBand(t, filepath.Join(dataDir, "applications", id+".md"), applicationOutOfBand)

	resp := deleteJobListing(t, server.URL, id, "", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestDeleteJobListing_DeletedOutOfBand_Returns404NotConflict(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	jobListing, application := listingVersions(t, server.URL, id)
	removeFile(t, filepath.Join(dataDir, "jobs", id+".md"))

	resp := deleteJobListing(t, server.URL, id, jobListing, application)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// pairVersions decodes a {jobListing, application} response and returns
// both tokens (the application's is empty when the shape has none).
func pairVersions(t *testing.T, resp *http.Response) (jobListing, application string) {
	t.Helper()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	jl, _ := result["jobListing"].(map[string]any)
	app, _ := result["application"].(map[string]any)
	jobListing, _ = jl["version"].(string)
	application, _ = app["version"].(string)
	return jobListing, application
}

// The server-computed routes stay unconditional, but they do rewrite the
// files — so the FE, which merges their response into what it holds, must
// get fresh tokens back or its next write would present a stale one.
func TestSaveJobListing_ResponseCarriesCurrentTokens(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/api/job-listings", map[string]any{
		"company":        "Acme Corp",
		"jobDescription": "Some role. Salary: €40,000.",
	})
	defer resp.Body.Close()
	jobListing, application := pairVersions(t, resp)

	wantJobListing, wantApplication := listingVersions(t, server.URL, "acme-corp")
	if jobListing != wantJobListing || application != wantApplication {
		t.Errorf("expected the save response tokens to match a fresh read, got (%q, %q) want (%q, %q)", jobListing, application, wantJobListing, wantApplication)
	}
}

func TestResolveJobListing_ResponseCarriesTokensUsableForTheNextWrite(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/job-listings/"+id+"/resolve", nil)
	defer resp.Body.Close()
	_, application := pairVersions(t, resp)
	if application == "" {
		t.Fatal("expected a version token on the resolved Application")
	}

	patch := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "tailoring"}, application)
	defer patch.Body.Close()
	if patch.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", patch.StatusCode)
	}
}

func TestCheckFreshness_ResponseCarriesTheCurrentJobListingToken(t *testing.T) {
	dataDir := seedDataDir(t)
	doer := fakeATSDoer{do: func(req *http.Request) (*http.Response, error) {
		return jsonATSResponse(http.StatusOK, ""), nil
	}}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: doer}))
	defer server.Close()

	id := saveJobListingWithURL(t, server.URL, "Acme Corp", "https://boards.example.com/acme/jobs/1")

	resp := postJSON(t, server.URL+"/api/job-listings/"+id+"/check-freshness", nil)
	defer resp.Body.Close()
	jobListing, _ := pairVersions(t, resp)

	want, _ := listingVersions(t, server.URL, id)
	if jobListing == "" || jobListing != want {
		t.Errorf("expected the check-freshness response to carry the current Job Listing token, got %q want %q", jobListing, want)
	}
}
