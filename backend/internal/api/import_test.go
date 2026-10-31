package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

const importCVText = "Acme Corp — Backend Engineer, 2019-03 to 2021\n- Built a Go HTTP API for invoicing\n- Cut deploy time in half\n"

func importFake() *fakeGenerationClient {
	return &fakeGenerationClient{proposeEntries: func(context.Context, string) ([]masterdata.Entry, error) {
		end := "2021"
		return []masterdata.Entry{
			{Type: "experience", Employer: "Acme Corp", Role: "Backend Engineer", Start: "2019-03", End: &end, Tags: []string{"Go"},
				Bullets: []string{"Built a Go HTTP API for invoicing", "Led a team of twelve engineers"}},
			{Type: "experience", Employer: "Broken", Start: "someday"},
		}, nil
	}}
}

func newImportServer(t *testing.T, dataDir string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: t.TempDir(), GenerationClient: importFake()}))
	t.Cleanup(server.Close)
	return server
}

func TestImportPreview_FlagsBulletsNotInTextAndWritesNothing(t *testing.T) {
	dataDir := t.TempDir()
	server := newImportServer(t, dataDir)
	body := call(t, http.MethodPost, server.URL+"/api/master-data/import/preview", map[string]any{"text": importCVText}, http.StatusOK)

	var got struct {
		Drafts []struct {
			Entry     masterdata.Entry `json:"entry"`
			NotInText []string         `json:"notInText"`
		} `json:"drafts"`
		Skipped int `json:"skipped"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Drafts) != 1 || got.Skipped != 1 {
		t.Fatalf("drafts=%d skipped=%d, want 1 and 1", len(got.Drafts), got.Skipped)
	}
	d := got.Drafts[0]
	if d.Entry.ID != "experience/acme-corp" {
		t.Errorf("id = %q", d.Entry.ID)
	}
	if len(d.NotInText) != 1 || d.NotInText[0] != "Led a team of twelve engineers" {
		t.Errorf("notInText = %v", d.NotInText)
	}
	if entries, _ := os.ReadDir(dataDir); len(entries) != 0 {
		t.Errorf("preview wrote %d files into data/", len(entries))
	}
}

func TestImportPreview_ProposedIDAvoidsExistingEntry(t *testing.T) {
	dataDir := t.TempDir()
	writeFile(t, filepath.Join(dataDir, "experience", "acme-corp.md"), "---\nemployer: Acme Corp\nrole: X\nstart: \"2000\"\n---\n\n- old\n")
	server := newImportServer(t, dataDir)
	body := call(t, http.MethodPost, server.URL+"/api/master-data/import/preview", map[string]any{"text": importCVText}, http.StatusOK)
	var got struct {
		Drafts []struct {
			Entry masterdata.Entry `json:"entry"`
		} `json:"drafts"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if id := got.Drafts[0].Entry.ID; id != "experience/acme-corp-2" {
		t.Errorf("id = %q, want experience/acme-corp-2", id)
	}
}

func TestImportPreview_RequiresText(t *testing.T) {
	server := newImportServer(t, t.TempDir())
	call(t, http.MethodPost, server.URL+"/api/master-data/import/preview", map[string]any{"text": "  "}, http.StatusBadRequest)
}

func TestImportSave_WritesOnceThenConflicts(t *testing.T) {
	dataDir := t.TempDir()
	server := newImportServer(t, dataDir)
	entry := map[string]any{"id": "experience/acme-corp", "type": "experience", "employer": "Acme Corp", "role": "Backend Engineer",
		"start": "2019-03", "end": nil, "tags": []string{"Go"}, "bullets": []string{"Built a Go HTTP API"}}
	call(t, http.MethodPost, server.URL+"/api/master-data/import/entries", entry, http.StatusCreated)
	if _, err := masterdata.GetEntry(dataDir, "experience/acme-corp"); err != nil {
		t.Fatalf("saved entry unreadable: %v", err)
	}
	entry["role"] = "Overwritten"
	call(t, http.MethodPost, server.URL+"/api/master-data/import/entries", entry, http.StatusConflict)
	got, err := masterdata.GetEntry(dataDir, "experience/acme-corp")
	if err != nil || got.Role != "Backend Engineer" {
		t.Errorf("existing entry changed: %+v, %v", got, err)
	}
}

func TestImportSave_RejectsMismatchedID(t *testing.T) {
	server := newImportServer(t, t.TempDir())
	entry := map[string]any{"id": "projects/../x", "type": "experience", "employer": "A", "role": "B", "start": "2020", "end": nil, "tags": []string{}}
	call(t, http.MethodPost, server.URL+"/api/master-data/import/entries", entry, http.StatusBadRequest)
}
