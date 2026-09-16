package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gio-del/sumisura/backend/internal/api"
	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// API contract fixtures (issue #99): every JSON-returning route's real
// handler response is captured here, normalized, and compared against a
// golden file under frontend/src/api/contract/fixtures/. The frontend's
// `tsc -b` then asserts each golden file against the type types.ts declares
// for it (frontend/src/api/contract/contract.ts). A normal `go test` run only
// verifies, and fails when a response no longer matches its fixture;
// regenerating is an explicit act:
//
//	cd backend && UPDATE_CONTRACT_FIXTURES=1 go test ./internal/api -run TestContract
const (
	contractFixturesDir   = "../../../frontend/src/api/contract/fixtures"
	contractAssertionFile = "../../../frontend/src/api/contract/contract.ts"
	contractUpdateEnv     = "UPDATE_CONTRACT_FIXTURES"
	contractRegenerateCmd = "cd backend && UPDATE_CONTRACT_FIXTURES=1 go test ./internal/api -run TestContract"
)

// contractFixture is one golden response: the route it was captured from
// (exactly as router.go registers it) and how to produce it.
type contractFixture struct {
	name    string
	route   string
	capture func(t *testing.T) []byte
}

// contractExemptRoutes are the routes with no JSON response body for the
// frontend to type, each with why. Every other route in router.go must have
// at least one fixture (TestContractRoutesAllCovered).
var contractExemptRoutes = map[string]string{
	"GET /api/healthz":                                      "fixed {\"status\":\"ok\"} body, not consumed by the frontend",
	"GET /api/export":                                       "zip download",
	"DELETE /api/master-data/entries/{id...}":               "204 No Content",
	"DELETE /api/master-data/cover-letter-snippets/{id...}": "204 No Content",
	"OPTIONS /api/job-listings/from-extension":              "CORS preflight, 204 No Content",
	"DELETE /api/job-listings/{id}":                         "204 No Content",
	"GET /api/job-listings/{id}/logo":                       "image file",
	"GET /api/generations/{slug}/{file}":                    "PDF or text file",
	"DELETE /api/ats/tracked-boards/{id}":                   "204 No Content",
}

// contractFixtures is the checked-in endpoint-to-fixture list. A
// ".populated" fixture sends every optional field the scenario can reach, a
// ".sparse" one leaves optional fields out; contract.ts says which is which.
func contractFixtures() []contractFixture {
	return []contractFixture{
		// Master Data
		{"list-entries.populated", "GET /api/master-data/entries", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodGet, s.server.URL+"/api/master-data/entries", nil, http.StatusOK)
		}},
		{"get-entry.sparse", "GET /api/master-data/entries/{id...}", func(t *testing.T) []byte {
			dataDir := seedDataDir(t)
			writeFile(t, filepath.Join(dataDir, "experience", "minimal.md"), "---\nemployer: Minimal Co\nrole: Engineer\nstart: \"2020-01\"\nend: null\n---\n")
			server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: t.TempDir(), GenerationClient: &fakeGenerationClient{}}))
			t.Cleanup(server.Close)
			return call(t, http.MethodGet, server.URL+"/api/master-data/entries/experience/minimal", nil, http.StatusOK)
		}},
		{"create-entry", "POST /api/master-data/entries", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedDataDir(t))
			return call(t, http.MethodPost, server.URL+"/api/master-data/entries", map[string]any{
				"type": "experience", "employer": "Globex", "role": "Backend Engineer", "start": "2021-01", "end": nil,
				"tags": []string{"Go"}, "bullets": []string{"Built a Go HTTP API."},
			}, http.StatusCreated)
		}},
		{"update-entry", "PUT /api/master-data/entries/{id...}", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedDataDir(t))
			return call(t, http.MethodPut, server.URL+"/api/master-data/entries/projects/emall", map[string]any{
				"type": "project", "name": "eMall", "start": "2022", "end": "2022", "tags": []string{"React"},
				"repo": "GitHub", "bullets": []string{"Managed charging stations."},
			}, http.StatusOK)
		}},
		{"get-profile.populated", "GET /api/master-data/profile", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedDataDir(t))
			return call(t, http.MethodGet, server.URL+"/api/master-data/profile", nil, http.StatusOK)
		}},
		{"get-profile.sparse", "GET /api/master-data/profile", func(t *testing.T) []byte {
			dataDir := t.TempDir()
			writeFile(t, filepath.Join(dataDir, "profile.yaml"), "name: Test User\nlocation: Milan\nemail: test@example.com\nphone: \"1\"\nlinkedin: t\ngithub: t\n")
			server := newSimpleServer(t, dataDir)
			return call(t, http.MethodGet, server.URL+"/api/master-data/profile", nil, http.StatusOK)
		}},
		{"update-profile.sparse", "PUT /api/master-data/profile", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedDataDir(t))
			return call(t, http.MethodPut, server.URL+"/api/master-data/profile", map[string]any{
				"name": "Test User", "location": "Milan", "email": "test@example.com", "phone": "+39 333 000 0000",
				"linkedin": "testuser", "github": "testuser",
				"education":      []map[string]any{{"degree": "MSc", "institution": "Test University", "program": "CS", "start": "2022", "end": "2024", "grade": "110/110"}},
				"publications":   []map[string]any{{"title": "A Paper", "authors": "Test User", "venue": "Test Venue"}},
				"certifications": []map[string]any{{"title": "A Certification"}},
				"awards":         []map[string]any{{"title": "An Award"}},
				"activities":     []map[string]any{{"title": "An Activity"}},
				"languages":      []map[string]any{{"name": "Italian", "level": "Native"}},
			}, http.StatusOK)
		}},
		{"tag-lint", "GET /api/master-data/tag-lint", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedTagLintDataDir(t))
			return call(t, http.MethodGet, server.URL+"/api/master-data/tag-lint", nil, http.StatusOK)
		}},
		{"list-snippets.populated", "GET /api/master-data/cover-letter-snippets", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodGet, s.server.URL+"/api/master-data/cover-letter-snippets", nil, http.StatusOK)
		}},
		{"get-snippet.sparse", "GET /api/master-data/cover-letter-snippets/{id...}", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedSnippetsDataDir(t))
			return call(t, http.MethodGet, server.URL+"/api/master-data/cover-letter-snippets/closing-standard", nil, http.StatusOK)
		}},
		{"create-snippet", "POST /api/master-data/cover-letter-snippets", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedSnippetsDataDir(t))
			return call(t, http.MethodPost, server.URL+"/api/master-data/cover-letter-snippets", map[string]any{
				"kind": "opening", "tags": []string{"Go"}, "body": "I build backend systems.",
			}, http.StatusCreated)
		}},
		{"update-snippet", "PUT /api/master-data/cover-letter-snippets/{id...}", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedSnippetsDataDir(t))
			return call(t, http.MethodPut, server.URL+"/api/master-data/cover-letter-snippets/closing-standard", map[string]any{
				"kind": "closing", "tags": []string{"Go"}, "body": "Thank you.",
			}, http.StatusOK)
		}},

		// Job Listings
		{"list-job-listings.populated", "GET /api/job-listings", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodGet, s.server.URL+"/api/job-listings", nil, http.StatusOK)
		}},
		{"list-job-listings.sparse", "GET /api/job-listings", func(t *testing.T) []byte {
			server := newSparseServer(t)
			return call(t, http.MethodGet, server.URL+"/api/job-listings", nil, http.StatusOK)
		}},
		{"get-job-listing.populated", "GET /api/job-listings/{id}", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodGet, s.server.URL+"/api/job-listings/"+s.globexID, nil, http.StatusOK)
		}},
		{"get-job-listing.sparse", "GET /api/job-listings/{id}", func(t *testing.T) []byte {
			server := newSparseServer(t)
			return call(t, http.MethodGet, server.URL+"/api/job-listings/"+sparseListingID, nil, http.StatusOK)
		}},
		{"save-job-listing.sparse", "POST /api/job-listings", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedDataDir(t))
			return call(t, http.MethodPost, server.URL+"/api/job-listings", map[string]any{
				"company": "Hooli", "jobDescription": "A backend role.",
			}, http.StatusCreated)
		}},
		{"save-job-listing.duplicate", "POST /api/job-listings", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedDataDir(t))
			listing := map[string]any{
				"title": "Backend Engineer", "company": "Hooli", "url": "https://jobs.example/hooli/1",
				"jobDescription": "A backend role.",
			}
			call(t, http.MethodPost, server.URL+"/api/job-listings", listing, http.StatusCreated)
			return call(t, http.MethodPost, server.URL+"/api/job-listings", listing, http.StatusCreated)
		}},
		{"capture-job-listing-from-extension", "POST /api/job-listings/from-extension", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedDataDir(t))
			return call(t, http.MethodPost, server.URL+"/api/job-listings/from-extension", map[string]any{
				"title": "Backend Engineer", "company": "Hooli", "location": "Remote", "url": "https://jobs.example/hooli/1",
				"description": "A backend role.", "listingSalaryText": "€50,000 - €60,000",
			}, http.StatusCreated)
		}},
		{"suggest-contact", "POST /api/job-listings/{id}/suggest-contact", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodPost, s.server.URL+"/api/job-listings/"+s.globexID+"/suggest-contact", nil, http.StatusOK)
		}},
		{"resolve-job-listing", "POST /api/job-listings/{id}/resolve", func(t *testing.T) []byte {
			attempts := 0
			client := &fakeGenerationClient{inferApplicationMethod: func(ctx context.Context, jobDescription string) (tracking.ApplicationMethod, error) {
				attempts++
				if attempts == 1 {
					return tracking.ApplicationMethod{}, errors.New("rate limited")
				}
				return tracking.ApplicationMethod{Kind: tracking.MethodPortal, Value: "https://jobs.example/hooli/apply"}, nil
			}}
			server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: seedDataDir(t), GenerationClient: client}))
			t.Cleanup(server.Close)
			saved := call(t, http.MethodPost, server.URL+"/api/job-listings", map[string]any{"company": "Hooli", "jobDescription": "A backend role."}, http.StatusCreated)
			return call(t, http.MethodPost, server.URL+"/api/job-listings/"+jobListingIDOf(t, saved)+"/resolve", nil, http.StatusOK)
		}},
		{"check-freshness", "POST /api/job-listings/{id}/check-freshness", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodPost, s.server.URL+"/api/job-listings/"+s.globexID+"/check-freshness", nil, http.StatusOK)
		}},
		{"archive-job-listing", "POST /api/job-listings/{id}/archive", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodPost, s.server.URL+"/api/job-listings/"+s.globexID+"/archive", nil, http.StatusOK)
		}},
		{"unarchive-job-listing", "POST /api/job-listings/{id}/unarchive", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			call(t, http.MethodPost, s.server.URL+"/api/job-listings/"+s.globexID+"/archive", nil, http.StatusOK)
			return call(t, http.MethodPost, s.server.URL+"/api/job-listings/"+s.globexID+"/unarchive", nil, http.StatusOK)
		}},

		// Applications
		{"list-applications.populated", "GET /api/applications", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodGet, s.server.URL+"/api/applications", nil, http.StatusOK)
		}},
		{"list-applications.sparse", "GET /api/applications", func(t *testing.T) []byte {
			server := newSparseServer(t)
			return call(t, http.MethodGet, server.URL+"/api/applications", nil, http.StatusOK)
		}},
		{"application-stats.populated", "GET /api/applications/stats", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodGet, s.server.URL+"/api/applications/stats", nil, http.StatusOK)
		}},
		{"application-stats.empty", "GET /api/applications/stats", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedDataDir(t))
			return call(t, http.MethodGet, server.URL+"/api/applications/stats", nil, http.StatusOK)
		}},
		{"update-application-status", "PATCH /api/applications/{id}/status", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodPatch, s.server.URL+"/api/applications/"+s.globexID+"/status", map[string]any{"status": "interviewing"}, http.StatusOK)
		}},
		{"update-application-method", "PATCH /api/applications/{id}/method", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodPatch, s.server.URL+"/api/applications/"+s.globexID+"/method", map[string]any{"kind": "portal", "value": "https://jobs.example/globex/apply"}, http.StatusOK)
		}},
		{"update-application-contact", "PATCH /api/applications/{id}/contact", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodPatch, s.server.URL+"/api/applications/"+s.globexID+"/contact", map[string]any{"name": "Grace Hopper", "email": "grace@globex.example"}, http.StatusOK)
		}},
		{"application-mailto", "GET /api/applications/{id}/mailto", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodGet, s.server.URL+"/api/applications/"+s.globexID+"/mailto", nil, http.StatusOK)
		}},
		{"record-generation.populated", "POST /api/applications/{id}/generations", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodPost, s.server.URL+"/api/applications/"+s.initechID+"/generations", populatedGenerationRecord(), http.StatusCreated)
		}},
		{"record-generation.sparse", "POST /api/applications/{id}/generations", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedDataDir(t))
			id := saveJobListing(t, server.URL, "Hooli")
			return call(t, http.MethodPost, server.URL+"/api/applications/"+id+"/generations", map[string]any{
				"slug": "hooli", "cvPath": "output/hooli/cv.pdf",
			}, http.StatusCreated)
		}},
		{"add-application-note", "POST /api/applications/{id}/notes", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodPost, s.server.URL+"/api/applications/"+s.initechID+"/notes", map[string]any{"body": "Recruiter replied."}, http.StatusCreated)
		}},
		{"edit-application-note", "PATCH /api/applications/{id}/notes/{noteId}", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodPatch, s.server.URL+"/api/applications/"+s.globexID+"/notes/"+s.globexNoteID, map[string]any{"body": "Corrected again."}, http.StatusOK)
		}},
		{"delete-application-note", "DELETE /api/applications/{id}/notes/{noteId}", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			return call(t, http.MethodDelete, s.server.URL+"/api/applications/"+s.globexID+"/notes/"+s.globexNoteID, nil, http.StatusOK)
		}},

		// Generation pipeline
		{"create-generation.populated", "POST /api/generations", func(t *testing.T) []byte {
			server := newGenerationServer(t)
			return call(t, http.MethodPost, server.URL+"/api/generations", map[string]any{
				"jobDescription": "Looking for a Go engineer.\nSalary: €40,000 - €50,000",
			}, http.StatusOK)
		}},
		{"create-generation.sparse", "POST /api/generations", func(t *testing.T) []byte {
			server := newGenerationServer(t)
			return call(t, http.MethodPost, server.URL+"/api/generations", nil, http.StatusOK)
		}},
		{"preview-generation", "POST /api/generations/preview", func(t *testing.T) []byte {
			server := newGenerationServer(t)
			return call(t, http.MethodPost, server.URL+"/api/generations/preview", map[string]any{
				"jobDescription": "Looking for a Go engineer.",
			}, http.StatusOK)
		}},
		{"render-generation.populated", "POST /api/generations/render", func(t *testing.T) []byte {
			projectRoot, dataDir := seedProjectRoot(t)
			server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
			t.Cleanup(server.Close)
			return call(t, http.MethodPost, server.URL+"/api/generations/render", map[string]any{
				"slug": "globex",
				"selection": map[string]any{"entries": []map[string]any{{
					"entryId": "experience/example-client-a", "reason": "Relevant.",
					"bullets": []map[string]any{{"sourceIndex": 0, "source": "Designed and built an AI Platform.", "rewritten": "Designed and built an AI Platform."}},
				}}},
				"coverLetter": map[string]any{"body": "Dear Hiring Manager,\n\nI'm excited to apply.\n\nBest,\nCandidate"},
				"language":    "en",
			}, http.StatusOK)
		}},

		// ATS job boards
		{"list-ats-listings.populated", "GET /api/ats/{provider}/{slug}/listings", func(t *testing.T) []byte {
			board := &fakeBoard{jobs: []string{"1"}}
			server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: seedDataDir(t), GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: board}))
			t.Cleanup(server.Close)
			call(t, http.MethodPost, server.URL+"/api/ats/tracked-boards", map[string]any{"provider": "greenhouse", "slug": "acme", "label": "Acme"}, http.StatusCreated)
			call(t, http.MethodGet, server.URL+"/api/ats/greenhouse/acme/listings", nil, http.StatusOK)
			board.jobs = []string{"1", "2"}
			return call(t, http.MethodGet, server.URL+"/api/ats/greenhouse/acme/listings", nil, http.StatusOK)
		}},
		{"list-tracked-boards.populated", "GET /api/ats/tracked-boards", func(t *testing.T) []byte {
			board := &fakeBoard{jobs: []string{"1"}}
			server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: seedDataDir(t), GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: board}))
			t.Cleanup(server.Close)
			call(t, http.MethodPost, server.URL+"/api/ats/tracked-boards", map[string]any{"provider": "greenhouse", "slug": "acme", "label": "Acme"}, http.StatusCreated)
			call(t, http.MethodGet, server.URL+"/api/ats/greenhouse/acme/listings", nil, http.StatusOK)
			board.jobs = []string{"1", "2"}
			return call(t, http.MethodGet, server.URL+"/api/ats/tracked-boards", nil, http.StatusOK)
		}},
		{"add-tracked-board", "POST /api/ats/tracked-boards", func(t *testing.T) []byte {
			server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: seedDataDir(t), GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: &fakeBoard{}}))
			t.Cleanup(server.Close)
			return call(t, http.MethodPost, server.URL+"/api/ats/tracked-boards", map[string]any{"provider": "greenhouse", "slug": "acme", "label": "Acme"}, http.StatusCreated)
		}},

		// Usage
		{"usage.populated", "GET /api/usage", func(t *testing.T) []byte {
			s := newPopulatedScenario(t)
			// An unreadable standalone usage log leaves the Generations'
			// calls in the total and flags it incomplete (issue #102).
			writeFile(t, filepath.Join(s.dataDir, "usage-log.json"), "not json")
			return call(t, http.MethodGet, s.server.URL+"/api/usage", nil, http.StatusOK)
		}},
		{"usage.sparse", "GET /api/usage", func(t *testing.T) []byte {
			server := newSimpleServer(t, seedDataDir(t))
			return call(t, http.MethodGet, server.URL+"/api/usage", nil, http.StatusOK)
		}},
	}
}

func TestContractFixtures(t *testing.T) {
	for _, fixture := range contractFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			assertContractFixture(t, fixture.name, fixture.capture(t))
		})
	}
}

// TestContractRoutesAllCovered keeps coverage growing with the API: a route
// added to router.go with neither a fixture nor an exemption fails here.
func TestContractRoutesAllCovered(t *testing.T) {
	source, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatal(err)
	}
	routes := map[string]bool{}
	for _, m := range regexp.MustCompile(`mux\.HandleFunc\("([^"]+)"`).FindAllStringSubmatch(string(source), -1) {
		routes[m[1]] = true
	}
	if len(routes) == 0 {
		t.Fatal("found no routes in router.go")
	}

	covered := map[string]bool{}
	for _, f := range contractFixtures() {
		if !routes[f.route] {
			t.Errorf("fixture %s names route %q, which router.go does not register", f.name, f.route)
		}
		covered[f.route] = true
	}
	for route := range contractExemptRoutes {
		if !routes[route] {
			t.Errorf("exempt route %q is not registered in router.go", route)
		}
		if covered[route] {
			t.Errorf("route %q has a fixture and is also exempt", route)
		}
	}
	for route := range routes {
		if !covered[route] && contractExemptRoutes[route] == "" {
			t.Errorf("route %q has no contract fixture: add one to contractFixtures (and an assertion to contract.ts), or exempt it with a reason", route)
		}
	}
}

// TestContractFixtureFilesMatchRegistry fails on a fixture file nothing
// produces any more, and on a fixture contract.ts never type-checks.
func TestContractFixtureFilesMatchRegistry(t *testing.T) {
	want := map[string]bool{}
	for _, f := range contractFixtures() {
		if want[f.name] {
			t.Errorf("fixture name %s is registered twice", f.name)
		}
		want[f.name] = true
	}

	files, err := filepath.Glob(filepath.Join(contractFixturesDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".json")
		if want[name] {
			continue
		}
		if os.Getenv(contractUpdateEnv) != "" {
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
			continue
		}
		t.Errorf("fixture %s is not produced by contractFixtures any more; delete it or run: %s", filepath.Base(file), contractRegenerateCmd)
	}

	assertions, err := os.ReadFile(contractAssertionFile)
	if err != nil {
		t.Fatalf("reading %s: %v", contractAssertionFile, err)
	}
	for name := range want {
		if !strings.Contains(string(assertions), "./fixtures/"+name+".json'") {
			t.Errorf("fixture %s.json is never type-checked: import and assert it in frontend/src/api/contract/contract.ts", name)
		}
	}
}

func assertContractFixture(t *testing.T, name string, body []byte) {
	t.Helper()
	got := canonicalFixture(t, body)
	path := filepath.Join(contractFixturesDir, name+".json")

	if os.Getenv(contractUpdateEnv) != "" {
		if err := os.MkdirAll(contractFixturesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("contract fixture %s is missing (%v); run: %s", name, err, contractRegenerateCmd)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("contract fixture %s.json is stale: the handler's response changed.\n"+
			"If the change is intended, run: %s\n"+
			"then fix frontend/src/api/types.ts until `npm run build` passes.\n\ngot:\n%s", name, contractRegenerateCmd, got)
	}
}

var fixtureTimestampRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)

const fixtureTimestamp = "2026-01-02T03:04:05Z"

// fixtureOutputStampRe matches the UTC stamp Render puts in each
// Generation's output directory name (output/<label>-<yyyymmdd-hhmmss>,
// issue #105), wherever it appears in a string: the slug and both paths.
var fixtureOutputStampRe = regexp.MustCompile(`\b\d{8}-\d{6}\b`)

const fixtureOutputStamp = "20260102-030405"

// clockDerivedNumbers are numeric fields computed from the wall-clock gaps
// between a scenario's requests, fixed by field name.
var clockDerivedNumbers = map[string]json.Number{
	"averageDays": "1.5", // tracking.StageTime: days between Status moves
	"score":       "0.9", // tracking.DuplicateMatch: decays with the savedAt gap
}

// canonicalFixture makes a response body deterministic: object keys sorted,
// two-space indented, every timestamp replaced with one fixed instant, the
// Render output directory stamp fixed, Note ids (derived from the clock) and
// record version tokens (issue #89: a hash of file bytes that embed those
// timestamps) replaced in order of appearance, and clockDerivedNumbers fixed.
// Distinct tokens in one response get distinct placeholders and a repeated
// token keeps its one, so the fixture still shows which records share a file. Only values are normalized, never keys or kinds, so the
// shape the frontend type-checks is the handler's own.
func canonicalFixture(t *testing.T, body []byte) []byte {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, body)
	}

	n := fixtureNormalizer{noteIDs: map[string]string{}, versions: map[string]string{}}
	value = n.walk(value, "")

	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

type fixtureNormalizer struct {
	noteIDs  map[string]string
	versions map[string]string
}

func (n fixtureNormalizer) walk(value any, key string) any {
	switch v := value.(type) {
	case map[string]any:
		// Visit keys in the order the encoder writes them, so "order of
		// appearance" for placeholders is the fixture's reading order rather
		// than Go's random map order.
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v[k] = n.walk(v[k], k)
		}
	case []any:
		for i, child := range v {
			if note, ok := child.(map[string]any); ok && key == "notes" {
				if id, ok := note["id"].(string); ok {
					note["id"] = n.noteID(id)
				}
			}
			v[i] = n.walk(child, "")
		}
	case string:
		if fixtureTimestampRe.MatchString(v) {
			return fixtureTimestamp
		}
		if key == "version" {
			return n.version(v)
		}
		return fixtureOutputStampRe.ReplaceAllString(v, fixtureOutputStamp)
	case json.Number:
		if fixed, ok := clockDerivedNumbers[key]; ok {
			return fixed
		}
	}
	return value
}

func (n fixtureNormalizer) version(token string) string {
	if fixed, ok := n.versions[token]; ok {
		return fixed
	}
	fixed := fmt.Sprintf("version-%d", len(n.versions)+1)
	n.versions[token] = fixed
	return fixed
}

func (n fixtureNormalizer) noteID(id string) string {
	if fixed, ok := n.noteIDs[id]; ok {
		return fixed
	}
	fixed := fmt.Sprintf("note-%d", len(n.noteIDs)+1)
	n.noteIDs[id] = fixed
	return fixed
}

// call issues one request and returns the body, failing unless the status
// is want.
func call(t *testing.T, method, url string, payload any, want int) []byte {
	t.Helper()
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("%s %s: expected %d, got %d: %s", method, url, want, resp.StatusCode, data)
	}
	return data
}

func newSimpleServer(t *testing.T, dataDir string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: t.TempDir(), GenerationClient: &fakeGenerationClient{}}))
	t.Cleanup(server.Close)
	return server
}

func jobListingIDOf(t *testing.T, body []byte) string {
	t.Helper()
	var result struct {
		JobListing struct {
			ID string `json:"id"`
		} `json:"jobListing"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	return result.JobListing.ID
}

// populatedScenario is two tracked Job Listings built through the API so
// that, between them, every optional field the wire can carry is set:
// Globex (captured from the extension with a conflicting listing salary, a
// logo, a freshness check, a Contact, two Status moves, a fully populated
// Generation whose source Entry was since edited, and an edited Note) and
// Initech (a stated RAL Range).
type populatedScenario struct {
	server       *httptest.Server
	dataDir      string
	globexID     string
	globexNoteID string
	initechID    string
}

func newPopulatedScenario(t *testing.T) populatedScenario {
	t.Helper()
	projectRoot, dataDir := seedGitProjectRoot(t)
	writeFile(t, filepath.Join(dataDir, "cover-letter-snippets", "opening-ai-platforms.md"), "---\nkind: opening\ntags:\n  - AI Platform\n---\n\nI build AI platforms.\n")
	writeFile(t, filepath.Join(dataDir, "cover-letter-snippets", "closing-standard.md"), "---\nkind: closing\n---\n\nThank you for your time.\n")

	client := &fakeGenerationClientWithUsage{
		fakeGenerationClient: fakeGenerationClient{
			inferApplicationMethod: func(ctx context.Context, jobDescription string) (tracking.ApplicationMethod, error) {
				return tracking.ApplicationMethod{Kind: tracking.MethodEmail, Value: "jobs@example.com"}, nil
			},
			suggestContact: func(ctx context.Context, company, jobDescription string) (tracking.Contact, error) {
				return tracking.Contact{Name: "Ada Lovelace", Email: "ada@example.com"}, nil
			},
		},
		usage: []generation.CallUsage{{
			CallType: "estimate-ral", Model: "claude-test", InputTokens: 100, OutputTokens: 20,
			CacheReadTokens: 5, CacheWriteTokens: 7, WebSearchUses: 1, EstimatedCostUSD: 0.25,
		}},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: client, ATSHTTPDoer: &fakeBoard{}}))
	t.Cleanup(server.Close)

	saved := call(t, http.MethodPost, server.URL+"/api/job-listings/from-extension", map[string]any{
		"title": "Senior Go Engineer", "company": "Globex", "location": "Milan",
		"url": "https://jobs.example/globex/1", "logoUrl": "https://jobs.example/globex/logo.png",
		"description":       "Senior Go Engineer.\nSalary: €40,000 - €50,000",
		"listingSalaryText": "€70,000 - €80,000",
	}, http.StatusCreated)
	s := populatedScenario{server: server, dataDir: dataDir, globexID: jobListingIDOf(t, saved)}
	base := server.URL + "/api/applications/" + s.globexID

	call(t, http.MethodPost, server.URL+"/api/job-listings/"+s.globexID+"/check-freshness", nil, http.StatusOK)
	call(t, http.MethodPatch, base+"/contact", map[string]any{"name": "Ada Lovelace", "email": "ada@example.com"}, http.StatusOK)
	call(t, http.MethodPatch, base+"/status", map[string]any{"status": "tailoring"}, http.StatusOK)
	call(t, http.MethodPatch, base+"/status", map[string]any{"status": "sent"}, http.StatusOK)
	call(t, http.MethodPost, base+"/generations", populatedGenerationRecord(), http.StatusCreated)
	call(t, http.MethodPost, base+"/notes", map[string]any{"body": "Applied through the careers page."}, http.StatusCreated)
	noted := call(t, http.MethodPost, base+"/notes", map[string]any{"body": "Recruiter called."}, http.StatusCreated)
	var application tracking.Application
	if err := json.Unmarshal(noted, &application); err != nil {
		t.Fatal(err)
	}
	s.globexNoteID = application.Notes[0].ID
	call(t, http.MethodPatch, base+"/notes/"+s.globexNoteID, map[string]any{"body": "Recruiter called back."}, http.StatusOK)

	// Edit a source Entry after the Generation was recorded, so it reads as
	// stale.
	gitCommitFile(t, projectRoot, filepath.Join("data", "experience", "example-client-a.md"),
		"---\nemployer: Example Consulting S.p.A.\nrole: Data Engineer\nclient: Example Client A\nlocation: Example City\nstart: \"2024-10\"\nend: null\nflagship: true\ntags:\n  - React\n---\n\n- Designed and built an AI Platform.\n",
		"edit exampleClientA", time.Now().UTC().Add(48*time.Hour))

	initech := call(t, http.MethodPost, server.URL+"/api/job-listings", map[string]any{
		"company": "Initech", "jobDescription": "Backend role.\nSalary: €45,000 - €55,000",
	}, http.StatusCreated)
	s.initechID = jobListingIDOf(t, initech)
	return s
}

// populatedGenerationRecord is a record-generation request with every field
// set, as the frontend posts it after Render.
func populatedGenerationRecord() map[string]any {
	return map[string]any{
		"slug": "globex", "cvPath": "output/globex/cv.pdf", "coverLetterPath": "output/globex/cover-letter.pdf",
		"sourceSnippetIds": []string{"opening-ai-platforms"},
		"entryIds":         []string{"experience/example-client-a", "projects/emall"},
		"language":         "it",
		"usage": map[string]any{
			"inputTokens": 1200, "outputTokens": 340, "cacheReadTokens": 50, "cacheWriteTokens": 60, "webSearchUses": 1,
			"estimatedCostUsd": 0.5,
			"calls": []map[string]any{{
				"callType": "select-and-rewrite", "model": "claude-test", "inputTokens": 1200, "outputTokens": 340,
				"cacheReadTokens": 50, "cacheWriteTokens": 60, "webSearchUses": 1, "estimatedCostUsd": 0.5,
			}},
		},
		"groundedness": map[string]any{
			"bullets":     []map[string]any{{"entryId": "experience/example-client-a", "sourceIndex": 0, "flags": []map[string]any{{"sentence": "Served 4 million users.", "reason": "numeric-mismatch"}}}},
			"coverLetter": []map[string]any{{"sentence": "I led a team of 40.", "reason": "no-source-match"}},
		},
	}
}

const sparseListingID = "legacy-co"

// newSparseServer serves one Job Listing and Application carrying only the
// fields every current record must have: no title, URL, logo, freshness
// check, Job Description, Contact, Status timestamp or history, Generations
// or Notes (an Application migrated while already past Saved, per ADR-0034).
func newSparseServer(t *testing.T) *httptest.Server {
	t.Helper()
	dataDir := seedDataDir(t)
	writeFile(t, filepath.Join(dataDir, "jobs", sparseListingID+".md"),
		"---\nschemaVersion: 1\ncompany: Legacy Co\nsource: manual\nsavedAt: \"2025-01-01T00:00:00Z\"\nral:\n  source: n/a\nfreshnessStatus: not-yet-checked\n---\n")
	writeFile(t, filepath.Join(dataDir, "applications", sparseListingID+".md"),
		"schemaVersion: 1\njobListingId: "+sparseListingID+"\nstatus: sent\nmethod:\n  kind: other\n")
	return newSimpleServer(t, dataDir)
}

// newGenerationServer serves Generate and Preview against Master Data with
// Snippets, a Client whose Rewrite and Cover Letter both state specifics the
// sources do not (so groundedness flags both), and recorded usage.
func newGenerationServer(t *testing.T) *httptest.Server {
	t.Helper()
	selection := generation.SelectionResult{
		Language: "it",
		Entries: []generation.SelectedEntry{{
			EntryID: "experience/example-client-a",
			Reason:  "Relevant AI platform work.",
			Bullets: []generation.SelectedBullet{{
				SourceIndex: 0,
				Source:      "Designed and built an AI Platform.",
				Rewritten:   "Designed and built an AI Platform serving 4 million users since 2019.",
			}},
		}},
	}
	client := &fakeGenerationClientWithUsage{
		fakeGenerationClient: fakeGenerationClient{
			selectAndRewrite: func(ctx context.Context, req generation.SelectionRequest) (generation.SelectionResult, error) {
				return selection, nil
			},
			selectOnly: func(ctx context.Context, req generation.SelectionRequest) (generation.SelectionResult, error) {
				return generation.SelectionResult{Entries: []generation.SelectedEntry{{
					EntryID: "experience/example-client-a", Reason: "Relevant.",
					Bullets: []generation.SelectedBullet{{SourceIndex: 0, Source: "Designed and built an AI Platform.", Rewritten: "Designed and built an AI Platform."}},
				}}}, nil
			},
			draftCoverLetter: func(ctx context.Context, req generation.CoverLetterRequest) (generation.CoverLetterResult, error) {
				return generation.CoverLetterResult{
					Body:             "I build AI platforms. I once led a team of 40 engineers at Initech in 2015.",
					SourceSnippetIDs: []string{"opening-ai-platforms"},
				}, nil
			},
		},
		usage: []generation.CallUsage{{
			CallType: "select-and-rewrite", Model: "claude-test", InputTokens: 1200, OutputTokens: 340,
			CacheReadTokens: 50, CacheWriteTokens: 60, WebSearchUses: 1, EstimatedCostUSD: 0.5,
		}},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: seedSnippetsDataDir(t), GenerationClient: client}))
	t.Cleanup(server.Close)
	return server
}

// fakeBoard is an ATS HTTP doer serving a Greenhouse board holding jobs (by
// id), a logo image, and a live posting page for any other URL.
type fakeBoard struct {
	jobs []string
}

func (b *fakeBoard) Do(req *http.Request) (*http.Response, error) {
	switch {
	case strings.Contains(req.URL.Host, "greenhouse"):
		ids := append([]string{}, b.jobs...)
		sort.Strings(ids)
		jobs := make([]string, len(ids))
		for i, id := range ids {
			jobs[i] = fmt.Sprintf(`{"id": %s, "title": "Engineer %s", "location": {"name": "Remote"}, "absolute_url": "https://boards.greenhouse.io/acme/jobs/%s", "content": "<p>Join us.</p>"}`, id, id, id)
		}
		return jsonATSResponse(http.StatusOK, `{"jobs": [`+strings.Join(jobs, ",")+`]}`), nil
	case strings.HasSuffix(req.URL.Path, ".png"):
		header := http.Header{"Content-Type": []string{"image/png"}}
		png := "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"
		return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(strings.NewReader(png)), Request: req}, nil
	default:
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("<html>open</html>")), Request: req}, nil
	}
}
