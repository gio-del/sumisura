package masterdata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A bilingual library is the point of lang (issue #168): the same kind
// twice, once per language, round-tripping through disk unchanged.
func TestSnippet_LangRoundTrips(t *testing.T) {
	dir := t.TempDir()
	created, err := CreateSnippet(dir, Snippet{Kind: "opening", Lang: "it", Body: "Vi scrivo perché…"})
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}

	got, err := GetSnippet(dir, created.ID)
	if err != nil {
		t.Fatalf("GetSnippet: %v", err)
	}
	if got.Lang != "it" {
		t.Errorf("expected lang it, got %q", got.Lang)
	}
}

// An unmarked Snippet is the state every Snippet written before this field
// was in, and it stays legal: it means "usable in any language".
func TestSnippet_LangIsOptional(t *testing.T) {
	dir := t.TempDir()
	created, err := CreateSnippet(dir, Snippet{Kind: "closing", Body: "Thanks for reading."})
	if err != nil {
		t.Fatalf("CreateSnippet: %v", err)
	}
	got, err := GetSnippet(dir, created.ID)
	if err != nil {
		t.Fatalf("GetSnippet: %v", err)
	}
	if got.Lang != "" {
		t.Errorf("expected no lang, got %q", got.Lang)
	}
	content, err := os.ReadFile(filepath.Join(dir, snippetDir, created.ID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "lang:") {
		t.Errorf("an unmarked Snippet must not write an empty lang, got:\n%s", content)
	}
}

// lang is compared against a Generation's resolved target language, so a
// value that cannot be one is refused rather than silently never matching.
func TestValidateSnippet_RejectsALangThatIsNotAnISOCode(t *testing.T) {
	for _, lang := range []string{"english", "EN", "it-IT", "i"} {
		err := ValidateSnippet(Snippet{Kind: "opening", Lang: lang, Body: "..."})
		if err == nil || !strings.Contains(err.Error(), "ISO 639-1") {
			t.Errorf("lang %q: expected an ISO 639-1 validation error, got %v", lang, err)
		}
	}
}
