package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
	"github.com/gio-del/sumisura/backend/internal/generation"
)

// TestATSReport_RenderKeepsItBesideThePDFsAndServesIt is issue #198 end to
// end for a Default Mode Generation: Render writes the report with its
// extracted text and term coverage, and the ATS Report route serves it
// with nothing recorded against an Application.
func TestATSReport_RenderKeepsItBesideThePDFsAndServesIt(t *testing.T) {
	projectRoot, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	t.Cleanup(server.Close)

	var rendered generation.RenderResult
	decodeJSON(t, call(t, http.MethodPost, server.URL+"/api/generations/render", map[string]any{
		"slug": "default",
		"selection": map[string]any{"entries": []map[string]any{{
			"entryId": "experience/example-client-a", "reason": "Relevant.",
			"bullets": []map[string]any{{"sourceIndex": 0, "source": "Designed and built an AI Platform.", "rewritten": "Designed and built an AI Platform."}},
		}}},
		"jobDescription": "An AI Platform role; JavaScript welcome.",
	}, http.StatusOK), &rendered)

	cv := rendered.ATSReports.CV
	if !strings.Contains(cv.ExtractedText, "Designed and built an AI Platform.") {
		t.Errorf("expected the extracted text layer in the report, got %q", cv.ExtractedText)
	}
	if cv.TermCoverage == nil || strings.Join(cv.TermCoverage.Present, ",") != "AI Platform" || strings.Join(cv.TermCoverage.Missing, ",") != "JavaScript" {
		t.Errorf("unexpected term coverage: %+v", cv.TermCoverage)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "output", rendered.Slug, generation.ATSReportFile)); err != nil {
		t.Fatalf("expected %s beside the PDFs: %v", generation.ATSReportFile, err)
	}

	var served generation.ATSReports
	decodeJSON(t, call(t, http.MethodGet, server.URL+"/api/generations/"+rendered.Slug+"/ats-report", nil, http.StatusOK), &served)
	if served.CV.ExtractedText != cv.ExtractedText || served.CoverLetter != nil {
		t.Errorf("served report differs from the rendered one: %+v", served)
	}

	var index []struct {
		Slug         string `json:"slug"`
		HasATSReport bool   `json:"hasAtsReport"`
	}
	decodeJSON(t, call(t, http.MethodGet, server.URL+"/api/generations", nil, http.StatusOK), &index)
	if len(index) != 1 || !index[0].HasATSReport {
		t.Errorf("expected the Generation listed with an ATS Report, got %+v", index)
	}
}

// A recorded Generation's report comes from its record, so it outlives the
// output directory; one recorded without a report has none to show.
func TestATSReport_FromTheRecordAfterTheFilesAreGone(t *testing.T) {
	s := newPopulatedScenario(t)
	if err := os.RemoveAll(filepath.Join(s.projectRoot, "output")); err != nil {
		t.Fatal(err)
	}

	var served generation.ATSReports
	decodeJSON(t, call(t, http.MethodGet, s.server.URL+"/api/generations/globex/ats-report", nil, http.StatusOK), &served)
	if served.CV.Status != generation.ParsabilityWarning || served.CoverLetter == nil {
		t.Errorf("expected the recorded reports, got %+v", served)
	}

	body := populatedGenerationRecord()
	delete(body, "atsReports")
	body["slug"] = "no-report"
	call(t, http.MethodPost, s.server.URL+"/api/applications/"+s.initechID+"/generations", body, http.StatusCreated)
	call(t, http.MethodGet, s.server.URL+"/api/generations/no-report/ats-report", nil, http.StatusNotFound)
	call(t, http.MethodGet, s.server.URL+"/api/generations/..%2Fetc/ats-report", nil, http.StatusNotFound)
}

func decodeJSON(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
}
