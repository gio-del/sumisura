package main

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeZip(t *testing.T, name string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("{}")) //nolint:errcheck // bytes.Buffer
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "e.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunExitCodes(t *testing.T) {
	zipPath := writeZip(t, "jobs/a.json")
	dst := filepath.Join(t.TempDir(), "data")
	var out, errOut bytes.Buffer

	if got := run([]string{"-data-dir", dst, zipPath}, &out, &errOut); got != 3 {
		t.Fatalf("dry run exit = %d, want 3", got)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("dry run wrote")
	}
	if got := run([]string{"-data-dir", dst, "-write", zipPath}, &out, &errOut); got != 0 {
		t.Fatalf("write exit = %d, want 0", got)
	}
	if got := run([]string{"-data-dir", dst, zipPath}, &out, &errOut); got != 0 {
		t.Fatalf("second dry run exit = %d, want 0", got)
	}
	if got := run([]string{"-data-dir", dst, writeZip(t, "../x.json")}, &out, &errOut); got != 1 {
		t.Fatalf("unsafe exit = %d, want 1", got)
	}
	if got := run(nil, &out, &errOut); got != 2 {
		t.Fatalf("no args exit = %d, want 2", got)
	}
}
