package masterdata_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// initGitRepo initializes a git repo at root with a usable local identity,
// for tests that need real commit history to look up.
func initGitRepo(t *testing.T, root string) {
	t.Helper()
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
}

func commitFile(t *testing.T, root, relPath, content, message string) {
	t.Helper()
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", relPath)
	run("commit", "-q", "-m", message)
}

func TestEntryLastModified_ReturnsMostRecentCommit(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	dataDir := filepath.Join(root, "data")

	commitFile(t, root, "data/experience/foo.md", "---\nemployer: Foo\n---\n\n- one\n", "add foo entry")
	commitFile(t, root, "data/experience/foo.md", "---\nemployer: Foo\n---\n\n- one\n- two\n", "add second bullet to foo")

	lm, err := masterdata.EntryLastModified(root, dataDir, "experience/foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lm.Subject != "add second bullet to foo" {
		t.Errorf("expected subject from most recent commit, got %q", lm.Subject)
	}
	if lm.At == "" {
		t.Errorf("expected a non-empty timestamp")
	}
}

func TestEntryLastModified_UntrackedFile_FallsBackToMtimeWithoutSubject(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	dataDir := filepath.Join(root, "data")

	full := filepath.Join(dataDir, "experience", "untracked.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("---\nemployer: New\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(full, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	lm, err := masterdata.EntryLastModified(root, dataDir, "experience/untracked")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lm.Subject != "" {
		t.Errorf("mtime fallback must not carry a subject, got %q", lm.Subject)
	}
	got, err := time.Parse(time.RFC3339Nano, lm.At)
	if err != nil || !got.Equal(mtime) {
		t.Errorf("expected At = mtime %v, got %q (%v)", mtime, lm.At, err)
	}
}

func TestEntryLastModified_NoGitRepository_FallsBackToMtime(t *testing.T) {
	// Release image: no repository at all, and dataDir unrelated to root.
	root := t.TempDir()
	dataDir := t.TempDir()

	full := filepath.Join(dataDir, "experience", "foo.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("---\nemployer: Foo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	lm, err := masterdata.EntryLastModified(root, dataDir, "experience/foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lm.At == "" || lm.Subject != "" {
		t.Errorf("expected mtime timestamp and empty subject, got %+v", lm)
	}
}

func TestEntryLastModified_MissingFile_ReturnsZeroValueNoError(t *testing.T) {
	root := t.TempDir()
	lm, err := masterdata.EntryLastModified(root, filepath.Join(root, "data"), "experience/gone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lm != (masterdata.LastModified{}) {
		t.Errorf("expected zero value for missing file, got %+v", lm)
	}
}

func TestEntryLastModified_InvalidID_ReturnsError(t *testing.T) {
	root := t.TempDir()
	initGitRepo(t, root)
	dataDir := filepath.Join(root, "data")

	if _, err := masterdata.EntryLastModified(root, dataDir, "not-a-valid-id"); err == nil {
		t.Fatal("expected an error for an invalid entry id")
	}
}
