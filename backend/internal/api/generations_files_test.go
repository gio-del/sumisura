package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

func TestGetGenerationFile_ServesRenderedPDF(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	outputDir := filepath.Join(projectRoot, "output", "acme-corp")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pdfBytes := []byte("%PDF-1.7 fake pdf content")
	if err := os.WriteFile(filepath.Join(outputDir, "cv.pdf"), pdfBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/generations/acme-corp/cv.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %q", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(pdfBytes) {
		t.Errorf("expected served bytes to match the file on disk")
	}
}

func TestGetGenerationFile_ServesCoverLetterText(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	outputDir := filepath.Join(projectRoot, "output", "acme-corp")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "cover-letter.txt"), []byte("Dear Hiring Manager,"), 0o644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/generations/acme-corp/cover-letter.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "Dear Hiring Manager," {
		t.Errorf("expected cover letter text to be served, got %q", body)
	}
}

func TestGetGenerationFile_UnknownFilename_Returns404(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/generations/acme-corp/secrets.env")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for a non-allowlisted filename, got %d", resp.StatusCode)
	}
}

func TestGetGenerationFile_PathTraversalAttempt_Returns404(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/generations/..%2f..%2fetc/cv.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected a path traversal attempt not to succeed, got %d", resp.StatusCode)
	}
}

func TestGetGenerationFile_MissingFile_Returns404(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/generations/never-rendered/cv.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestHeadGenerationFile_ReportsWhetherFilesAreStillOnDisk pins the
// contract the front end relies on to show "files no longer on disk" for
// a Generation whose output/ directory was cleared (issue #105): a HEAD
// request answers 200 while the file exists and 404 once it's gone.
func TestHeadGenerationFile_ReportsWhetherFilesAreStillOnDisk(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	outputDir := filepath.Join(projectRoot, "output", "acme-corp-20260911-143022")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "cv.pdf"), []byte("%PDF-1.7 fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	head := func() int {
		t.Helper()
		resp, err := http.Head(server.URL + "/api/generations/acme-corp-20260911-143022/cv.pdf")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if got := head(); got != http.StatusOK {
		t.Fatalf("expected HEAD 200 while the file exists, got %d", got)
	}
	if err := os.RemoveAll(outputDir); err != nil {
		t.Fatal(err)
	}
	if got := head(); got != http.StatusNotFound {
		t.Fatalf("expected HEAD 404 once output/ was cleared, got %d", got)
	}
}

// deleteGeneration issues the DELETE and returns its status code.
func deleteGeneration(t *testing.T, serverURL, slug string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, serverURL+"/api/generations/"+slug, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// Deleting a Generation clears its derived artifacts — the whole
// output/<slug>/ directory — and nothing else.
func TestDeleteGeneration_RemovesTheOutputDirectory(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	outputDir := filepath.Join(projectRoot, "output", "acme-corp-20260911-143022")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cv.pdf", "cover-letter.pdf", "data.json", "selection.json"} {
		if err := os.WriteFile(filepath.Join(outputDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A second Generation, to prove the delete is scoped to one directory.
	keep := filepath.Join(projectRoot, "output", "other-20260911-143022")
	if err := os.MkdirAll(keep, 0o755); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	if code := deleteGeneration(t, server.URL, "acme-corp-20260911-143022"); code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", code)
	}
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Errorf("expected the output directory to be gone, got err=%v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("expected the other Generation to be untouched: %v", err)
	}
}

// The caller asked for it not to be there, and it is not there. A record
// whose files were already deleted must not fail the request.
func TestDeleteGeneration_AlreadyGoneSucceeds(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	if code := deleteGeneration(t, server.URL, "never-existed-20260101-000000"); code != http.StatusNoContent {
		t.Errorf("expected 204 for an absent directory, got %d", code)
	}
}

// The slug names a path that is about to be removed recursively, so
// anything that is not a plain kebab-case slug is refused before the join.
func TestDeleteGeneration_RefusesSlugsThatCouldEscapeOutput(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	canary := filepath.Join(projectRoot, "data", "profile.yaml")
	if _, err := os.Stat(canary); err != nil {
		t.Fatalf("expected the seeded profile to exist: %v", err)
	}

	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	for _, slug := range []string{"..", "..%2f..%2fdata", "Acme-Corp", "acme_corp", "acme corp", ""} {
		if code := deleteGeneration(t, server.URL, slug); code == http.StatusNoContent {
			t.Errorf("slug %q: expected the request to be refused, got 204", slug)
		}
	}
	if _, err := os.Stat(canary); err != nil {
		t.Errorf("data/ must be untouched by any delete attempt: %v", err)
	}
	if _, err := os.Stat(projectRoot); err != nil {
		t.Errorf("projectRoot must still exist: %v", err)
	}
}
