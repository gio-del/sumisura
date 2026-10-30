package tracking

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func makeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "export.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRestoreDataRoundTrip(t *testing.T) {
	src := t.TempDir()
	files := map[string]string{
		"jobs/a.json":               `{"a":1}`,
		"applications/a.json":       "{\"b\":2}\n",
		"pending-captures/c/d.json": "{}",
	}
	writeTree(t, src, files)
	var buf bytes.Buffer
	if err := ExportData(src, &buf); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "e.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "data")
	report, err := RestoreData(zipPath, dst, RestoreOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Created) != len(files) {
		t.Errorf("created %v, want %d files", report.Created, len(files))
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(name)))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", name, got, err, want)
		}
	}
}

func TestRestoreDataDryRunWritesNothing(t *testing.T) {
	zipPath := makeZip(t, map[string]string{"jobs/a.json": "x"})
	dst := filepath.Join(t.TempDir(), "data")
	report, err := RestoreData(zipPath, dst, RestoreOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Pending() || len(report.Created) != 1 {
		t.Errorf("report = %+v", report)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Errorf("dry run created %s", dst)
	}
}

func TestRestoreDataRejectsUnsafePaths(t *testing.T) {
	for _, name := range []string{
		"../evil.json", "jobs/../../evil.json", "/etc/evil.json", "jobs//a.json",
		"other/a.json", "a.json", "jobs\\..\\a.json", "jobs/./a.json",
	} {
		t.Run(name, func(t *testing.T) {
			zipPath := makeZip(t, map[string]string{"jobs/ok.json": "ok", name: "bad"})
			root := t.TempDir()
			dst := filepath.Join(root, "data")
			if _, err := RestoreData(zipPath, dst, RestoreOptions{}); err == nil {
				t.Fatal("expected an error")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 0 {
				t.Errorf("wrote %v despite the error", entries)
			}
		})
	}
}

func TestRestoreDataOverwrite(t *testing.T) {
	zipPath := makeZip(t, map[string]string{"jobs/a.json": "new", "jobs/b.json": "b"})
	dst := t.TempDir()
	writeTree(t, dst, map[string]string{"jobs/a.json": "old"})

	report, err := RestoreData(zipPath, dst, RestoreOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Skipped) != 1 || len(report.Created) != 1 || len(report.Overwritten) != 0 {
		t.Errorf("report = %+v", report)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "jobs", "a.json")); string(got) != "old" {
		t.Errorf("existing file touched: %q", got)
	}

	report, err = RestoreData(zipPath, dst, RestoreOptions{Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Overwritten) != 1 || report.Overwritten[0] != "jobs/a.json" {
		t.Errorf("report = %+v", report)
	}
	if got, _ := os.ReadFile(filepath.Join(dst, "jobs", "a.json")); string(got) != "new" {
		t.Errorf("not overwritten: %q", got)
	}
}
