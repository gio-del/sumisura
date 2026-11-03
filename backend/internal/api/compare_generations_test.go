package api_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/generation"
)

// writeCompareFixtureDirs writes two Generation directories that differ in
// every way a comparison reports.
func writeCompareFixtureDirs(t *testing.T, projectRoot string) {
	t.Helper()
	files := map[string]map[string]string{
		"compare-a": {
			"selection.json":   `{"entries":[{"entryId":"projects/emall","reason":"r","bullets":[{"sourceIndex":0,"source":"s","rewritten":"Built it."}],"tags":["Go"]},{"entryId":"projects/gone","reason":"r","bullets":[]}],"summary":"About A"}`,
			"cover-letter.txt": "Dear A",
		},
		"compare-b": {
			"selection.json":   `{"entries":[{"entryId":"projects/emall","reason":"r","bullets":[{"sourceIndex":0,"source":"s","rewritten":"Built it well."}],"tags":["Go","SQL"]},{"entryId":"projects/fresh","reason":"r","bullets":[]}],"summary":"About B"}`,
			"cover-letter.txt": "Dear B",
		},
	}
	for slug, fs := range files {
		dir := filepath.Join(projectRoot, "output", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range fs {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCompareGenerationsRoute(t *testing.T) {
	s := newPopulatedScenario(t)
	writeCompareFixtureDirs(t, s.projectRoot)

	var got generation.Comparison
	decodeJSON(t, call(t, http.MethodGet, s.server.URL+"/api/generations/compare?a=compare-a&b=compare-b", nil, http.StatusOK), &got)
	if len(got.EntriesAdded) != 1 || got.EntriesAdded[0].EntryID != "projects/fresh" ||
		len(got.EntriesRemoved) != 1 || len(got.RewrittenChanges) != 1 {
		t.Errorf("comparison = %+v", got)
	}

	call(t, http.MethodGet, s.server.URL+"/api/generations/compare?a=compare-a&b=missing", nil, http.StatusNotFound)
	call(t, http.MethodGet, s.server.URL+"/api/generations/compare?a=compare-a", nil, http.StatusBadRequest)
}
