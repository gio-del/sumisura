package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
	"github.com/gio-del/sumisura/backend/internal/generation"
)

func TestCreateGeneration_BasedOn(t *testing.T) {
	s := newPopulatedScenario(t)
	var got *generation.SelectionResult
	calls := 0
	client := &fakeGenerationClient{
		selectAndRewrite: func(ctx context.Context, req generation.SelectionRequest) (generation.SelectionResult, error) {
			calls++
			got = req.Previous
			return generation.SelectionResult{}, nil
		},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: s.dataDir, ProjectRoot: s.projectRoot, GenerationClient: client}))
	defer server.Close()
	url := server.URL + "/api/generations"

	// Without basedOn nothing changes.
	call(t, http.MethodPost, url, map[string]any{"jobDescription": "Go engineer."}, http.StatusOK)
	if calls != 1 || got != nil {
		t.Fatalf("no basedOn: calls=%d previous=%v", calls, got)
	}

	// Unknown, and unrecorded (an output directory no Application claims).
	body := call(t, http.MethodPost, url, map[string]any{"jobDescription": "Go engineer.", "basedOn": "nope"}, http.StatusBadRequest)
	if !strings.Contains(string(body), "not an approved Generation") {
		t.Errorf("message = %s", body)
	}
	dir := filepath.Join(s.projectRoot, "output", "orphan")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	call(t, http.MethodPost, url, map[string]any{"jobDescription": "Go engineer.", "basedOn": "orphan"}, http.StatusBadRequest)

	// Recorded but its selection.json is missing.
	body = call(t, http.MethodPost, url, map[string]any{"jobDescription": "Go engineer.", "basedOn": "globex"}, http.StatusBadRequest)
	if !strings.Contains(string(body), "no readable selection") {
		t.Errorf("message = %s", body)
	}
	if calls != 1 {
		t.Fatalf("the client must not be called for a bad basedOn, calls=%d", calls)
	}

	// Recorded with a selection: reaches the Client and is reported back.
	writeFile(t, filepath.Join(s.projectRoot, "output", "globex", "selection.json"),
		`{"entries":[{"entryId":"projects/emall","reason":"r","bullets":[],"tags":["Go"]}]}`)
	var result generation.GenerateResult
	decodeJSON(t, call(t, http.MethodPost, url, map[string]any{"jobDescription": "Go engineer.", "basedOn": "globex"}, http.StatusOK), &result)
	if got == nil || len(got.Entries) != 1 || got.Entries[0].EntryID != "projects/emall" {
		t.Errorf("previous = %+v", got)
	}
	if result.BasedOn != "globex" {
		t.Errorf("basedOn = %q", result.BasedOn)
	}
}
