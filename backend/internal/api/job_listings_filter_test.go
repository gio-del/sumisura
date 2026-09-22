package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

func saveListing(t *testing.T, serverURL, company, jobDescription string) string {
	t.Helper()
	resp := postJSON(t, serverURL+"/api/job-listings", map[string]any{
		"company":        company,
		"url":            "https://example.com/jobs/" + company,
		"jobDescription": jobDescription,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed save failed for %s: %d", company, resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	listing := result["jobListing"].(map[string]any)
	return listing["id"].(string)
}

func TestListJobListings_FilterByCompany_ReturnsOnlyMatching(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir}))
	defer server.Close()

	saveListing(t, server.URL, "Acme Corp", "Go backend role.")
	saveListing(t, server.URL, "Beta Inc", "React frontend role.")

	resp, err := http.Get(server.URL + "/api/job-listings?company=acme")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 filtered result, got %d: %+v", len(got), got)
	}
}

func TestListJobListings_NoFilters_ReturnsEverything(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir}))
	defer server.Close()

	saveListing(t, server.URL, "Acme Corp", "Go backend role.")
	saveListing(t, server.URL, "Beta Inc", "React frontend role.")

	resp, err := http.Get(server.URL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 unfiltered results, got %d", len(got))
	}
}

func TestListJobListings_InvalidStatus_Returns400(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/job-listings?status=not-a-real-status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid status, got %d", resp.StatusCode)
	}
}

// TestListJobListings_FilterByWithdrawnStatus_ReturnsOnlyWithdrawn covers the
// Withdrawn Status (#50) the status filter had been rejecting with a 400.
func TestListJobListings_FilterByWithdrawnStatus_ReturnsOnlyWithdrawn(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	saveListing(t, server.URL, "Acme Corp", "Go backend role.")
	withdrawn := saveListing(t, server.URL, "Beta Inc", "React frontend role.")
	resp := patchJSON(t, server.URL+"/api/applications/"+withdrawn+"/status", map[string]any{"status": "withdrawn"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("seed withdraw failed: %d", resp.StatusCode)
	}

	assertCompanies(t, listCompanies(t, server.URL, "?status=withdrawn"), []string{"Beta Inc"})
}

func TestListJobListings_InvalidDate_Returns400(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/job-listings?savedFrom=not-a-date")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid savedFrom date, got %d", resp.StatusCode)
	}
}

func TestListJobListings_FilterByStatus_ReturnsOnlyMatching(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir}))
	defer server.Close()

	saveListing(t, server.URL, "Acme Corp", "Go backend role.")

	resp, err := http.Get(server.URL + "/api/job-listings?status=saved")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result for status=saved, got %d", len(got))
	}
}

// The location filter on the wire (issue #206, story 58).
func TestListJobListings_FilterByLocation(t *testing.T) {
	s := newLookupServer(t)
	postJSON(t, s.url+"/api/job-listings", map[string]any{
		"company": "Acme", "location": "Milan, Lombardy, Italy", "jobDescription": "A backend role.",
	}).Body.Close()
	postJSON(t, s.url+"/api/job-listings", map[string]any{
		"company": "Globex", "location": "Remote (EU)", "jobDescription": "A frontend role.",
	}).Body.Close()
	postJSON(t, s.url+"/api/job-listings", map[string]any{
		"company": "Initech", "jobDescription": "A role with no location.",
	}).Body.Close()

	tests := []struct {
		query string
		want  int
	}{
		{"", 3},
		{"?location=Milan", 1},
		{"?location=milan", 1},
		{"?location=remote", 1},
		{"?location=Berlin", 0},
	}
	for _, tt := range tests {
		resp, err := http.Get(s.url + "/api/job-listings" + tt.query)
		if err != nil {
			t.Fatal(err)
		}
		var rows []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if len(rows) != tt.want {
			t.Errorf("%q: expected %d listings, got %d", tt.query, tt.want, len(rows))
		}
	}
}
