package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedDataDir(t *testing.T) string {
	t.Helper()
	return seedDataDirAt(t, t.TempDir())
}

// seedDataDirAt seeds the same Master Data fixtures as seedDataDir, but
// under a caller-chosen dir instead of a fresh t.TempDir() — for tests
// (e.g. Render's) that need that dir nested inside a larger project root
// fixture.
func seedDataDirAt(t *testing.T, dir string) string {
	t.Helper()

	writeFile(t, filepath.Join(dir, "experience", "example-client-a.md"), `---
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

- Designed and built an AI Platform.
- Built the platform's front end in React.
`)

	writeFile(t, filepath.Join(dir, "projects", "emall.md"), `---
name: eMall
start: "2022"
end: "2022"
tags:
  - JavaScript
  - React
repo: "GitHub"
---

- Designed and implemented a system for managing charging stations.
`)

	writeFile(t, filepath.Join(dir, "profile.yaml"), `name: Test User
location: Milan, Italy
email: test@example.com
phone: "+39 333 000 0000"
linkedin: testuser
github: testuser

education:
  - degree: MSc
    institution: Test University
    program: Computer Science
    start: "2022"
    end: "2024"
    grade: "110/110"
    courses:
      - Distributed Systems

publications:
  - title: "A Paper"
    authors: "Test User"
    venue: "Test Venue"
    link: "https://example.com/paper"
    note: "A note."

certifications:
  - title: "A Certification"
    issuer: "An Issuer"
    date: "2025-03"
    link: "https://example.com/verify"

awards:
  - title: "An Award"
    description: "A description."

activities:
  - title: "An Activity"
    description: "A description."

languages:
  - name: Italian
    level: Native
  - name: English
    level: Fluent
`)

	return dir
}

func TestListEntries_ReturnsEntriesSeededOnDisk(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/master-data/entries")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var entries []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	byID := map[string]map[string]any{}
	for _, e := range entries {
		byID[e["id"].(string)] = e
	}

	exampleClientA, ok := byID["experience/example-client-a"]
	if !ok {
		t.Fatalf("expected entry with id experience/example-client-a, got %v", byID)
	}
	if exampleClientA["type"] != "experience" {
		t.Errorf("expected type experience, got %v", exampleClientA["type"])
	}
	if exampleClientA["employer"] != "Example Consulting S.p.A." {
		t.Errorf("expected employer Example Consulting S.p.A., got %v", exampleClientA["employer"])
	}
	if exampleClientA["client"] != "Example Client A" {
		t.Errorf("expected client Example Client A, got %v", exampleClientA["client"])
	}
	tags, ok := exampleClientA["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "AI Platform" {
		t.Errorf("expected tags [AI Platform, React], got %v", exampleClientA["tags"])
	}

	emall, ok := byID["projects/emall"]
	if !ok {
		t.Fatalf("expected entry with id projects/emall, got %v", byID)
	}
	if emall["type"] != "project" {
		t.Errorf("expected type project, got %v", emall["type"])
	}
	if emall["name"] != "eMall" {
		t.Errorf("expected name eMall, got %v", emall["name"])
	}
}

func TestGetEntry_ReturnsFrontmatterAndBullets(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/master-data/entries/experience/example-client-a")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var entry map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		t.Fatal(err)
	}

	if entry["id"] != "experience/example-client-a" {
		t.Errorf("expected id experience/example-client-a, got %v", entry["id"])
	}
	if entry["employer"] != "Example Consulting S.p.A." {
		t.Errorf("expected employer Example Consulting S.p.A., got %v", entry["employer"])
	}
	if entry["role"] != "Data Engineer" {
		t.Errorf("expected role Data Engineer, got %v", entry["role"])
	}
	if entry["start"] != "2024-10" {
		t.Errorf("expected start 2024-10, got %v", entry["start"])
	}
	if entry["end"] != nil {
		t.Errorf("expected end nil, got %v", entry["end"])
	}
	bullets, ok := entry["bullets"].([]any)
	if !ok || len(bullets) != 2 {
		t.Fatalf("expected 2 bullets, got %v", entry["bullets"])
	}
	if bullets[0] != "Designed and built an AI Platform." {
		t.Errorf("expected first bullet to match source file, got %v", bullets[0])
	}
}

func TestGetEntry_UnknownID_Returns404(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/master-data/entries/experience/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// seedGitProjectRoot builds a temp project root containing Master Data (the
// same fixtures seedDataDir provides), committed to a real git repo, so the
// Entries endpoints' lastModified field has real history to look up.
func seedGitProjectRoot(t *testing.T) (projectRoot, dataDir string) {
	t.Helper()
	root := t.TempDir()
	dataDir = seedDataDirAt(t, filepath.Join(root, "data"))

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("add", "-A")
	run("commit", "-q", "-m", "seed master data")

	return root, dataDir
}

func TestListEntries_IncludesLastModifiedFromGitHistory(t *testing.T) {
	projectRoot, dataDir := seedGitProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/master-data/entries")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var entries []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}

	byID := map[string]map[string]any{}
	for _, e := range entries {
		byID[e["id"].(string)] = e
	}

	exampleClientA, ok := byID["experience/example-client-a"]
	if !ok {
		t.Fatalf("expected entry with id experience/example-client-a, got %v", byID)
	}
	lastModified, ok := exampleClientA["lastModified"].(map[string]any)
	if !ok {
		t.Fatalf("expected lastModified object, got %v", exampleClientA["lastModified"])
	}
	if lastModified["subject"] != "seed master data" {
		t.Errorf("expected subject 'seed master data', got %v", lastModified["subject"])
	}
	if lastModified["at"] == "" || lastModified["at"] == nil {
		t.Errorf("expected a non-empty timestamp, got %v", lastModified["at"])
	}
}

func TestGetEntry_IncludesLastModifiedFromGitHistory(t *testing.T) {
	projectRoot, dataDir := seedGitProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/master-data/entries/experience/example-client-a")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var entry map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		t.Fatal(err)
	}
	lastModified, ok := entry["lastModified"].(map[string]any)
	if !ok {
		t.Fatalf("expected lastModified object, got %v", entry["lastModified"])
	}
	if lastModified["subject"] != "seed master data" {
		t.Errorf("expected subject 'seed master data', got %v", lastModified["subject"])
	}
}

func TestListEntries_UncommittedEntry_OmitsLastModified(t *testing.T) {
	dataDir := seedDataDir(t) // no git repo at all behind this dataDir
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/master-data/entries")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var entries []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, present := e["lastModified"]; present {
			t.Errorf("expected no lastModified field with no git history, got %v on %v", e["lastModified"], e["id"])
		}
	}
}
