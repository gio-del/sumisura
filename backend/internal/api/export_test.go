package api_test

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

func TestExportData_ReturnsZipOfJobsAndApplications(t *testing.T) {
	dataDir := seedDataDir(t)
	if err := os.MkdirAll(filepath.Join(dataDir, "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dataDir, "jobs", "acme.md"), "---\ncompany: Acme\n---\n")

	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/export")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("expected Content-Type application/zip, got %q", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd == "" {
		t.Fatalf("expected a Content-Disposition header naming the download")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 {
		t.Fatalf("expected a non-empty archive body")
	}

	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("response body isn't a valid zip: %v", err)
	}
	found := false
	for _, f := range zr.File {
		if f.Name == "jobs/acme.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected archive to contain jobs/acme.md")
	}
}

func TestExportData_NoJobsOrApplicationsYet_Returns200EmptyArchive(t *testing.T) {
	// A brand-new install (seedDataDir alone, no jobs/applications dirs)
	// must still succeed — see the PRD's "zero Job Listings" user story.
	dataDir := seedDataDir(t)

	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/export")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zip.NewReader(bytes.NewReader(body), int64(len(body))); err != nil {
		t.Fatalf("response body isn't a valid zip: %v", err)
	}
}

func TestExportData_JobsPathIsAFileNotDirectory_Returns500(t *testing.T) {
	dataDir := seedDataDir(t)
	writeFile(t, filepath.Join(dataDir, "jobs"), "not a directory")

	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/export")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 500 {
		t.Fatalf("expected a 5xx response when data/jobs isn't a directory, got %d", resp.StatusCode)
	}
}

func TestExportData_ScopeParameter(t *testing.T) {
	dataDir := t.TempDir()
	for name, content := range map[string]string{
		"jobs/a.json":     "{}",
		"profile.yaml":    "name: A\n",
		"experience/x.md": "x",
	} {
		p := filepath.Join(dataDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()
	count := func(query string) (int, int) {
		resp, err := http.Get(server.URL + "/api/export" + query)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return resp.StatusCode, 0
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, len(zr.File)
	}
	if code, n := count(""); code != 200 || n != 3 {
		t.Errorf("default: %d, %d entries; want 200, 3", code, n)
	}
	if code, n := count("?scope=tracking"); code != 200 || n != 1 {
		t.Errorf("scope=tracking: %d, %d entries; want 200, 1", code, n)
	}
	if code, _ := count("?scope=bogus"); code != http.StatusBadRequest {
		t.Errorf("scope=bogus: %d, want 400", code)
	}
}
