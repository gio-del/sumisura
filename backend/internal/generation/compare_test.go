package generation

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeCompareDir(t *testing.T, root, slug string, sel *SelectionResult, letter *string) {
	t.Helper()
	dir := filepath.Join(root, "output", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if sel != nil {
		b, _ := json.Marshal(sel)
		if err := os.WriteFile(filepath.Join(dir, "selection.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if letter != nil {
		if err := os.WriteFile(filepath.Join(dir, "cover-letter.txt"), []byte(*letter), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCompareGenerations(t *testing.T) {
	root := t.TempDir()
	la, lb := "Dear A", "Dear B"
	a := SelectionResult{Summary: "one", Entries: []SelectedEntry{
		{EntryID: "kept", Tags: []string{"go"}, Bullets: []SelectedBullet{{SourceIndex: 0, Rewritten: "same"}, {SourceIndex: 1, Rewritten: "old"}}},
		{EntryID: "gone"},
	}}
	b := SelectionResult{Summary: "two", Entries: []SelectedEntry{
		{EntryID: "kept", Tags: []string{"go", "sql"}, Bullets: []SelectedBullet{{SourceIndex: 0, Rewritten: "same"}, {SourceIndex: 1, Rewritten: "new"}, {SourceIndex: 2, Rewritten: "extra"}}},
		{EntryID: "fresh"},
	}}
	writeCompareDir(t, root, "a-1", &a, &la)
	writeCompareDir(t, root, "b-1", &b, &lb)

	got, err := CompareGenerations(filepath.Join(root, "data"), root, "a-1", "b-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.EntriesAdded) != 1 || got.EntriesAdded[0].EntryID != "fresh" {
		t.Errorf("added = %+v", got.EntriesAdded)
	}
	if len(got.EntriesRemoved) != 1 || got.EntriesRemoved[0].Title != "gone" {
		t.Errorf("removed = %+v", got.EntriesRemoved)
	}
	want := []CompareBulletChange{
		{EntryID: "kept", Title: "kept", SourceIndex: 1, A: "old", B: "new"},
		{EntryID: "kept", Title: "kept", SourceIndex: 2, A: "", B: "extra"},
	}
	if !reflect.DeepEqual(got.RewrittenChanges, want) {
		t.Errorf("changes = %+v", got.RewrittenChanges)
	}
	if len(got.TagChanges) != 1 || !reflect.DeepEqual(got.TagChanges[0].B, []string{"go", "sql"}) {
		t.Errorf("tags = %+v", got.TagChanges)
	}
	if got.Summary == nil || !got.Summary.Changed {
		t.Errorf("summary = %+v", got.Summary)
	}
	if got.CoverLetter == nil || !got.CoverLetter.Changed || got.CoverLetter.B != "Dear B" {
		t.Errorf("letter = %+v", got.CoverLetter)
	}
}

func TestCompareGenerationsLegacyAndMissing(t *testing.T) {
	root := t.TempDir()
	a := SelectionResult{Summary: "x"}
	writeCompareDir(t, root, "new-1", &a, nil)
	writeCompareDir(t, root, "legacy-1", nil, nil)

	got, err := CompareGenerations(root, root, "new-1", "legacy-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.B.SelectionAvailable || !got.A.SelectionAvailable || got.Summary != nil || got.CoverLetter != nil {
		t.Errorf("legacy comparison = %+v", got)
	}

	if _, err := CompareGenerations(root, root, "new-1", "nope"); !errors.Is(err, ErrGenerationNotFound) {
		t.Errorf("err = %v", err)
	}
}
