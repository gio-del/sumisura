package masterdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListEntriesSkipsBrokenFileAndReportsIt(t *testing.T) {
	dir := t.TempDir()
	exp := filepath.Join(dir, "experience")
	if err := os.MkdirAll(exp, 0o755); err != nil {
		t.Fatal(err)
	}
	good := "---\nemployer: Acme\nrole: Dev\nstart: \"2020-01\"\n---\n\n- Did things.\n"
	bad := "---\nemployer: \"Acme\nstart: 2020\n---\n"
	for name, c := range map[string]string{"good.md": good, "bad.md": bad} {
		if err := os.WriteFile(filepath.Join(exp, name), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := ListEntries(dir)
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "experience/good" {
		t.Fatalf("entries = %+v", entries)
	}
	_, problems, err := ListEntriesWithProblems(dir)
	if err != nil || len(problems) != 1 || problems[0].File != "experience/bad.md" || problems[0].Reason == "" {
		t.Fatalf("problems = %+v, err = %v", problems, err)
	}
	if _, err := GetEntry(dir, "experience/bad"); err == nil {
		t.Fatal("GetEntry on the broken file should still fail")
	}
	if _, err := os.Stat(filepath.Join(exp, "bad.md")); err != nil {
		t.Fatalf("broken file must stay: %v", err)
	}
}
