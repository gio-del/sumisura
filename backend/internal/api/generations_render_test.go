package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
	"github.com/gio-del/sumisura/backend/internal/generation"
)

// seedProjectRoot builds a temp project root containing Master Data (the
// same fixtures seedDataDir provides) plus a copy of the real
// template/*.typ files, so Render can invoke the real typst CLI against it
// without touching the repo's own output/ directory (see the PRD's Testing
// Decisions: "Render step tested by asserting a PDF file is produced and
// is one page").
func seedProjectRoot(t *testing.T) (projectRoot, dataDir string) {
	t.Helper()
	root := t.TempDir()
	dataDir = seedDataDirAt(t, filepath.Join(root, "data"))
	copyTemplate(t, root, "cv.typ")
	copyTemplate(t, root, "cover-letter.typ")
	return root, dataDir
}

// derivedSlugRe matches the output slug Render derives from label:
// <label>-<UTC yyyymmdd-hhmmss>, optionally with a -N disambiguator.
func derivedSlugRe(label string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(label) + `-\d{8}-\d{6}(-\d+)?$`)
}

func copyTemplate(t *testing.T, root, name string) {
	t.Helper()
	src := filepath.Join("..", "..", "..", "template", name)
	content, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading real template %s: %v", src, err)
	}
	dst := filepath.Join(root, "template", name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRenderGeneration_ApprovedSelection_ProducesOnePagePDF(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	payload := map[string]any{
		"slug": "acme-corp",
		"selection": map[string]any{
			"entries": []map[string]any{
				{
					"entryId": "experience/example-client-a",
					"reason":  "Relevant",
					"bullets": []map[string]any{
						{"sourceIndex": 0, "source": "Designed and built an AI Platform.", "rewritten": "Designed and built an AI Platform."},
					},
				},
			},
		},
	}
	resp := postJSON(t, server.URL+"/api/generations/render", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result generation.RenderResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	// The request's slug is only a label: Render derives a unique
	// label-plus-UTC-timestamp directory from it and reports it back
	// (issue #105).
	if !derivedSlugRe("acme-corp").MatchString(result.Slug) {
		t.Errorf("expected a slug derived from label %q, got %q", "acme-corp", result.Slug)
	}
	if result.CVPath != filepath.Join("output", result.Slug, "cv.pdf") {
		t.Errorf("expected CVPath inside the returned slug's directory, got %q", result.CVPath)
	}
	if result.CVPageCount != 1 {
		t.Errorf("expected a one-page CV, got %d pages", result.CVPageCount)
	}
	if result.ATSReports.CV.Status != generation.ParsabilityOK {
		t.Errorf("expected ParsabilityOK for a clean render, got %+v", result.ATSReports.CV)
	}

	pdfPath := filepath.Join(projectRoot, result.CVPath)
	info, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("expected rendered PDF to exist at %s: %v", pdfPath, err)
	}
	if info.Size() == 0 {
		t.Error("expected rendered PDF to be non-empty")
	}
}

func TestRenderGeneration_WithCoverLetter_AlsoProducesCoverLetterPDF(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	payload := map[string]any{
		"slug":      "acme-corp",
		"selection": map[string]any{"entries": []map[string]any{}},
		"coverLetter": map[string]any{
			"body": "Dear Hiring Manager,\n\nI'm excited to apply.\n\nBest,\nCandidate",
		},
	}
	resp := postJSON(t, server.URL+"/api/generations/render", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result generation.RenderResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.CoverLetterPath == "" {
		t.Fatal("expected a coverLetterPath")
	}
	if _, err := os.Stat(filepath.Join(projectRoot, result.CoverLetterPath)); err != nil {
		t.Fatalf("expected rendered cover letter PDF to exist: %v", err)
	}
	if result.ATSReports.CoverLetter == nil {
		t.Fatal("expected a Cover Letter ATS Report when a Cover Letter was rendered")
	}
	if result.ATSReports.CoverLetter.Status != generation.ParsabilityOK {
		t.Errorf("expected ParsabilityOK for a clean cover letter render, got %+v", result.ATSReports.CoverLetter)
	}

	txt, err := os.ReadFile(filepath.Join(projectRoot, "output", result.Slug, "cover-letter.txt"))
	if err != nil {
		t.Fatalf("expected a cover-letter.txt for story 11's text download: %v", err)
	}
	if string(txt) != "Dear Hiring Manager,\n\nI'm excited to apply.\n\nBest,\nCandidate" {
		t.Errorf("expected cover-letter.txt to match the approved body, got %q", txt)
	}
}

// TestRenderGeneration_RegenerateWithSameSlug_KeepsBothGenerations drives
// issue #105's app-side trigger: GenerationPage defaults the slug to the
// Job Listing id, so every regenerate posts the same slug. Both renders
// must survive, each individually servable by the file endpoint.
func TestRenderGeneration_RegenerateWithSameSlug_KeepsBothGenerations(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	payload := map[string]any{"slug": "job-listing-1", "selection": map[string]any{"entries": []map[string]any{}}}
	var slugs []string
	for i := 0; i < 2; i++ {
		resp := postJSON(t, server.URL+"/api/generations/render", payload)
		var result generation.RenderResult
		err := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || err != nil {
			t.Fatalf("render %d: expected 200 with a result, got %d (%v)", i+1, resp.StatusCode, err)
		}
		if !derivedSlugRe("job-listing-1").MatchString(result.Slug) {
			t.Errorf("render %d: expected a slug derived from the label, got %q", i+1, result.Slug)
		}
		slugs = append(slugs, result.Slug)
	}

	if slugs[0] == slugs[1] {
		t.Fatalf("expected two distinct output slugs, both were %q", slugs[0])
	}
	for _, slug := range slugs {
		resp, err := http.Get(server.URL + "/api/generations/" + slug + "/cv.pdf")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected output/%s/cv.pdf to be served, got %d", slug, resp.StatusCode)
		}
	}
}

func TestRenderGeneration_LanguageThreadedIntoPDFLangMetadata(t *testing.T) {
	tests := []struct {
		name     string
		language any
		wantLang string
	}{
		{"explicit Italian", "it", "it"},
		{"explicit English", "en", "en"},
		{"omitted defaults to English", nil, "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectRoot, dataDir := seedProjectRoot(t)
			server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
			defer server.Close()

			payload := map[string]any{
				"slug":      "acme-corp",
				"selection": map[string]any{"entries": []map[string]any{}},
			}
			if tt.language != nil {
				payload["language"] = tt.language
			}
			resp := postJSON(t, server.URL+"/api/generations/render", payload)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}

			var result generation.RenderResult
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
			pdf, err := os.ReadFile(filepath.Join(projectRoot, result.CVPath))
			if err != nil {
				t.Fatalf("reading rendered PDF: %v", err)
			}
			want := "/Lang(" + tt.wantLang + ")"
			if !bytes.Contains(pdf, []byte(want)) {
				t.Errorf("expected rendered PDF catalog to contain %q (the CV's actual document language), got none — the template may not be reading data.lang", want)
			}
		})
	}
}

func TestRenderGeneration_InvalidSlug_Returns400(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	payload := map[string]any{"slug": "Not Kebab Case!", "selection": map[string]any{"entries": []map[string]any{}}}
	resp := postJSON(t, server.URL+"/api/generations/render", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRenderGeneration_UnknownEntryID_Returns400(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	payload := map[string]any{
		"slug": "acme-corp",
		"selection": map[string]any{
			"entries": []map[string]any{
				{"entryId": "experience/does-not-exist", "bullets": []map[string]any{{"sourceIndex": 0, "source": "x", "rewritten": "x"}}},
			},
		},
	}
	resp := postJSON(t, server.URL+"/api/generations/render", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
