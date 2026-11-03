package masterdata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTrashEntry(t *testing.T, dir string, role string) Entry {
	t.Helper()
	created, err := CreateEntry(dir, Entry{Type: TypeExperience, Employer: "Globex", Role: role, Start: "2021-01", Tags: []string{"go"}, Bullets: []string{"Built things."}})
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	return created
}

func TestTrash_DeleteThenRestoreKeepsTheID(t *testing.T) {
	dir := t.TempDir()
	e := newTrashEntry(t, dir, "Engineer")
	if err := DeleteEntry(dir, e.ID); err != nil {
		t.Fatal(err)
	}
	items, err := ListTrash(dir)
	if err != nil || len(items) != 1 {
		t.Fatalf("ListTrash = %v, %v", items, err)
	}
	if items[0].OriginalID != e.ID || items[0].Kind != "entry" {
		t.Errorf("item = %+v", items[0])
	}
	id, err := RestoreTrash(dir, items[0].ID)
	if err != nil || id != e.ID {
		t.Fatalf("RestoreTrash = %q, %v; want %q", id, err, e.ID)
	}
	if _, err := GetEntry(dir, e.ID); err != nil {
		t.Errorf("restored entry unreadable: %v", err)
	}
	if left, _ := ListTrash(dir); len(left) != 0 {
		t.Errorf("trash still holds %d items", len(left))
	}
}

func TestTrash_RestoreOverATakenIDGetsASuffix(t *testing.T) {
	dir := t.TempDir()
	e := newTrashEntry(t, dir, "Engineer")
	if err := DeleteEntry(dir, e.ID); err != nil {
		t.Fatal(err)
	}
	newTrashEntry(t, dir, "Other") // takes the freed id
	items, _ := ListTrash(dir)
	id, err := RestoreTrash(dir, items[0].ID)
	if err != nil || id != e.ID+"-2" {
		t.Fatalf("RestoreTrash = %q, %v; want %q", id, err, e.ID+"-2")
	}
}

func TestTrash_UpdateKeepsThePreviousVersionOnlyWhenContentChanges(t *testing.T) {
	dir := t.TempDir()
	e := newTrashEntry(t, dir, "Engineer")

	same, _ := GetEntry(dir, e.ID)
	if _, err := UpdateEntry(dir, e.ID, same); err != nil {
		t.Fatal(err)
	}
	if items, _ := ListTrash(dir); len(items) != 0 {
		t.Fatalf("identical save created %d copies", len(items))
	}

	changed := same
	changed.Role = "Staff Engineer"
	if _, err := UpdateEntry(dir, e.ID, changed); err != nil {
		t.Fatal(err)
	}
	items, _ := ListTrash(dir)
	if len(items) != 1 || !strings.Contains(items[0].Content, "Engineer") || strings.Contains(items[0].Content, "Staff") {
		t.Fatalf("trash = %+v", items)
	}
}

func TestTrash_SnippetDeleteAndUpdate(t *testing.T) {
	dir := t.TempDir()
	s, err := CreateSnippet(dir, Snippet{Kind: "closing", Body: "Thanks."})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateSnippet(dir, s.ID, Snippet{Kind: "closing", Body: "Thanks!"}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSnippet(dir, s.ID); err != nil {
		t.Fatal(err)
	}
	items, _ := ListTrash(dir)
	if len(items) != 2 || items[0].Kind != "snippet" {
		t.Fatalf("trash = %+v", items)
	}
	id, err := RestoreTrash(dir, items[0].ID)
	if err != nil || id != s.ID {
		t.Fatalf("RestoreTrash = %q, %v", id, err)
	}
}

func TestTrash_KeepsTheLastTenVersionsPerItem(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	for i := 0; i < 13; i++ {
		if err := stash(dir, "experience", "acme", []byte("v"), base.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if err := stash(dir, "experience", "acme-2", []byte("other"), base); err != nil {
		t.Fatal(err)
	}
	items, _ := ListTrash(dir)
	n := map[string]int{}
	for _, it := range items {
		n[it.OriginalID]++
	}
	if n["experience/acme"] != MaxTrashVersions || n["experience/acme-2"] != 1 {
		t.Errorf("counts = %v", n)
	}
}

func TestTrash_DropsVersionsOlderThanNinetyDays(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	if err := stash(dir, "projects", "old", []byte("x"), now.Add(-TrashRetention-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := stash(dir, "projects", "new", []byte("y"), now); err != nil {
		t.Fatal(err)
	}
	items, _ := ListTrash(dir)
	if len(items) != 1 || items[0].OriginalID != "projects/new" {
		t.Fatalf("trash = %+v", items)
	}
}

func TestTrash_IsNotListedAsMasterData(t *testing.T) {
	dir := t.TempDir()
	e := newTrashEntry(t, dir, "Engineer")
	if err := DeleteEntry(dir, e.ID); err != nil {
		t.Fatal(err)
	}
	if entries, _ := ListEntries(dir); len(entries) != 0 {
		t.Errorf("ListEntries = %v", entries)
	}
	if _, err := os.Stat(filepath.Join(dir, ".trash", "experience")); err != nil {
		t.Errorf("trash dir: %v", err)
	}
}

func TestTrash_PurgeAndUnknownID(t *testing.T) {
	dir := t.TempDir()
	e := newTrashEntry(t, dir, "Engineer")
	_ = DeleteEntry(dir, e.ID)
	items, _ := ListTrash(dir)
	if err := PurgeTrash(dir, items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := PurgeTrash(dir, items[0].ID); err != ErrTrashNotFound {
		t.Errorf("second purge = %v", err)
	}
}
