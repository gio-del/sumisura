package tracking_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readZip(t *testing.T, data []byte) map[string]string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	files := make(map[string]string)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		rc.Close()
		files[f.Name] = buf.String()
	}
	return files
}

func TestExportData_EmptyDirs_ProducesValidEmptyArchive(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "jobs"), 0o755); err != nil {
		t.Fatalf("mkdir jobs: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "applications"), 0o755); err != nil {
		t.Fatalf("mkdir applications: %v", err)
	}

	var buf bytes.Buffer
	if err := tracking.ExportData(dataDir, &buf); err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	files := readZip(t, buf.Bytes())
	if len(files) != 0 {
		t.Fatalf("expected an empty archive, got %v", files)
	}
}

func TestExportData_PopulatedDir_RoundTripsFilesUnderJobsAndApplications(t *testing.T) {
	dataDir := t.TempDir()
	writeFile(t, filepath.Join(dataDir, "jobs", "acme.md"), "---\ncompany: Acme\n---\n")
	writeFile(t, filepath.Join(dataDir, "jobs", "acme-logo.png"), "fake-png-bytes")
	writeFile(t, filepath.Join(dataDir, "applications", "acme.md"), "---\nstatus: Saved\n---\n")
	writeFile(t, filepath.Join(dataDir, "pending-captures", "linkedin-0123456789ab.json"), "{}\n")
	// Master Data must never leak into the export.
	writeFile(t, filepath.Join(dataDir, "experience", "example.md"), "---\nemployer: Example\n---\n")

	var buf bytes.Buffer
	if err := tracking.ExportData(dataDir, &buf); err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	files := readZip(t, buf.Bytes())
	want := map[string]string{
		"jobs/acme.md":                                "---\ncompany: Acme\n---\n",
		"jobs/acme-logo.png":                          "fake-png-bytes",
		"applications/acme.md":                        "---\nstatus: Saved\n---\n",
		"pending-captures/linkedin-0123456789ab.json": "{}\n",
	}
	for path, content := range want {
		got, ok := files[path]
		if !ok {
			t.Fatalf("expected archive to contain %q, files: %v", path, files)
		}
		if got != content {
			t.Fatalf("content mismatch for %q: got %q, want %q", path, got, content)
		}
	}
	for path := range files {
		if !bytes.HasPrefix([]byte(path), []byte("jobs/")) && !bytes.HasPrefix([]byte(path), []byte("applications/")) && !bytes.HasPrefix([]byte(path), []byte("pending-captures/")) {
			t.Fatalf("archive leaked non job/application file: %s", path)
		}
	}
}

func TestExportData_NoJobsOrApplicationsYet_ProducesEmptyArchiveNotError(t *testing.T) {
	// A brand-new install has no data/jobs or data/applications directory
	// at all until the first Job Listing is saved (see .gitignore/ADR-0008)
	// — exporting early must succeed with an empty archive, not error.
	dataDir := t.TempDir()

	var buf bytes.Buffer
	if err := tracking.ExportData(dataDir, &buf); err != nil {
		t.Fatalf("ExportData: %v", err)
	}

	files := readZip(t, buf.Bytes())
	if len(files) != 0 {
		t.Fatalf("expected an empty archive, got %v", files)
	}
}

func TestExportData_JobsPathIsAFileNotDirectory_ReturnsError(t *testing.T) {
	dataDir := t.TempDir()
	// Simulate genuine on-disk corruption: something occupies data/jobs
	// that isn't a directory at all.
	writeFile(t, filepath.Join(dataDir, "jobs"), "not a directory")

	var buf bytes.Buffer
	err := tracking.ExportData(dataDir, &buf)
	if err == nil {
		t.Fatalf("expected an error when data/jobs isn't a directory, got nil")
	}
}
