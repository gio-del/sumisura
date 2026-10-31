package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gio-del/sumisura/backend/internal/api"
)

// gitCommitFile writes content to path (relative to root) and commits it,
// with the commit's author/committer date pinned to at, so tests can create
// unambiguous "modified before/after" fixtures without depending on git-log
// second-level timestamp resolution or real wall-clock timing.
func gitCommitFile(t *testing.T, root, relPath, content, message string, at time.Time) {
	t.Helper()
	writeFile(t, filepath.Join(root, relPath), content)

	cmd := exec.Command("git", "-C", root, "add", "-A")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	dateStr := at.Format(time.RFC3339)
	cmd = exec.Command("git", "-C", root, "commit", "-q", "-m", message)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+dateStr,
		"GIT_COMMITTER_DATE="+dateStr,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
}

func TestListJobListings_GenerationWithEditedSourceEntry_ReportsStaleEntries(t *testing.T) {
	projectRoot, dataDir := seedGitProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":     "acme-corp",
		"cvPath":   "output/acme-corp/cv.pdf",
		"entryIds": []string{"experience/example-client-a", "projects/emall"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	// Edit only one of the two source Entries, committed well after the
	// Generation was recorded.
	future := time.Now().UTC().Add(48 * time.Hour)
	gitCommitFile(t, projectRoot, filepath.Join("data", "experience", "example-client-a.md"), `---
employer: Example Consulting S.p.A.
role: Data Engineer
client: Example Client A
location: Example City
start: "2024-10"
end: null
flagship: true
tags:
  - AI Platform
  - React
---

- Designed and built an AI Platform, now with an added bullet.
- Built the platform's front end in React.
`, "edit exampleClientA entry", future)

	listResp, err := http.Get(server.URL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()

	var results []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 job listing, got %d", len(results))
	}

	application := results[0]["application"].(map[string]any)
	generations := application["generations"].([]any)
	if len(generations) != 1 {
		t.Fatalf("expected 1 generation record, got %v", generations)
	}
	record := generations[0].(map[string]any)

	stale, ok := record["staleEntries"].([]any)
	if !ok || len(stale) != 1 {
		t.Fatalf("expected exactly 1 stale entry, got %v", record["staleEntries"])
	}
	if stale[0] != "Example Consulting S.p.A. – Data Engineer" {
		t.Errorf("expected stale entry named by employer + role, got %v", stale[0])
	}
}

func TestListJobListings_NoEntriesChangedSinceGeneration_NoStaleEntries(t *testing.T) {
	projectRoot, dataDir := seedGitProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":     "acme-corp",
		"cvPath":   "output/acme-corp/cv.pdf",
		"entryIds": []string{"experience/example-client-a"},
	})
	resp.Body.Close()

	listResp, err := http.Get(server.URL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()

	var results []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	application := results[0]["application"].(map[string]any)
	record := application["generations"].([]any)[0].(map[string]any)
	if _, present := record["staleEntries"]; present {
		t.Errorf("expected no staleEntries field when nothing changed, got %v", record["staleEntries"])
	}
}

func TestListJobListings_GenerationWithNoStoredEntryIDs_NoStaleEntries(t *testing.T) {
	projectRoot, dataDir := seedGitProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	// Pre-existing GenerationRecord shape: no entryIds at all.
	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":   "acme-corp",
		"cvPath": "output/acme-corp/cv.pdf",
	})
	resp.Body.Close()

	future := time.Now().UTC().Add(48 * time.Hour)
	gitCommitFile(t, projectRoot, filepath.Join("data", "experience", "example-client-a.md"), `---
employer: Example Consulting S.p.A.
role: Data Engineer
client: Example Client A
location: Example City
start: "2024-10"
end: null
flagship: true
tags:
  - AI Platform
  - React
---

- Edited bullet.
`, "edit exampleClientA entry", future)

	listResp, err := http.Get(server.URL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()

	var results []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	application := results[0]["application"].(map[string]any)
	record := application["generations"].([]any)[0].(map[string]any)
	if _, present := record["staleEntries"]; present {
		t.Errorf("expected no staleEntries field for a record with no stored entryIds, got %v", record["staleEntries"])
	}
}

// editExampleClientAEntry commits an edit to one of the Master Data Entries the
// seeded project root ships, dated well after any Generation recorded during
// a test — the "source Entry changed since the Tailored CV was produced"
// fixture (issue #52).
func editExampleClientAEntry(t *testing.T, projectRoot string) {
	t.Helper()
	gitCommitFile(t, projectRoot, filepath.Join("data", "experience", "example-client-a.md"), `---
employer: Example Consulting S.p.A.
role: Data Engineer
client: Example Client A
location: Example City
start: "2024-10"
end: null
flagship: true
tags:
  - AI Platform
  - React
---

- Designed and built an AI Platform, now with an added bullet.
- Built the platform's front end in React.
`, "edit exampleClientA entry", time.Now().UTC().Add(48*time.Hour))
}

// The stale-Entry notice moves to the Job Listing detail page (issue #94), so
// the detail endpoint has to run the same read-time attachment the list
// endpoint does — otherwise the notice silently renders nothing.
func TestGetJobListing_GenerationWithEditedSourceEntry_ReportsStaleEntries(t *testing.T) {
	projectRoot, dataDir := seedGitProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":     "acme-corp",
		"cvPath":   "output/acme-corp/cv.pdf",
		"entryIds": []string{"experience/example-client-a", "projects/emall"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	editExampleClientAEntry(t, projectRoot)

	application := getJobListingDetail(t, server.URL, id)["application"].(map[string]any)
	record := application["generations"].([]any)[0].(map[string]any)

	stale, ok := record["staleEntries"].([]any)
	if !ok || len(stale) != 1 {
		t.Fatalf("expected exactly 1 stale entry, got %v", record["staleEntries"])
	}
	if stale[0] != "Example Consulting S.p.A. – Data Engineer" {
		t.Errorf("expected stale entry named by employer + role, got %v", stale[0])
	}
}

func TestGetJobListing_NoEntriesChangedSinceGeneration_NoStaleEntries(t *testing.T) {
	projectRoot, dataDir := seedGitProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":     "acme-corp",
		"cvPath":   "output/acme-corp/cv.pdf",
		"entryIds": []string{"experience/example-client-a"},
	})
	resp.Body.Close()

	application := getJobListingDetail(t, server.URL, id)["application"].(map[string]any)
	record := application["generations"].([]any)[0].(map[string]any)
	if _, present := record["staleEntries"]; present {
		t.Errorf("expected no staleEntries field when nothing changed, got %v", record["staleEntries"])
	}
}

func TestListJobListings_NoGitRepository_EditedEntryDetectedByMtime(t *testing.T) {
	dataDir := seedDataDirAt(t, filepath.Join(t.TempDir(), "data"))
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: t.TempDir(), GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":     "acme-corp",
		"cvPath":   "output/acme-corp/cv.pdf",
		"entryIds": []string{"experience/example-client-a", "projects/emall"},
	})
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	// Only one Entry is touched after the Generation.
	future := time.Now().Add(48 * time.Hour)
	if err := os.Chtimes(filepath.Join(dataDir, "experience", "example-client-a.md"), future, future); err != nil {
		t.Fatal(err)
	}

	listResp, err := http.Get(server.URL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer listResp.Body.Close()
	var results []map[string]any
	if err := json.NewDecoder(listResp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	application := results[0]["application"].(map[string]any)
	record := application["generations"].([]any)[0].(map[string]any)
	stale, ok := record["staleEntries"].([]any)
	if !ok || len(stale) != 1 || stale[0] != "Example Consulting S.p.A. – Data Engineer" {
		t.Fatalf("expected only the touched Entry to be stale, got %v", record["staleEntries"])
	}
}
