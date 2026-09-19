package generation

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
	"strings"
)

const checksFixtureEntry = `---
employer: Acme Corp
role: Senior Engineer
start: "2022"
end: null
tags:
  - Go
---

- Migrated the legacy billing service to a microservices architecture, reducing incident response time by 60 percent.
- Led a cross-functional team of 5 engineers.
`

// writeChecksDataDir writes a minimal Master Data tree (one experience
// Entry, id "experience/acme") under a temp dir and returns its path.
func writeChecksDataDir(t *testing.T) string {
	t.Helper()
	dataDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "experience"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "experience", "acme.md"), []byte(checksFixtureEntry), 0o644); err != nil {
		t.Fatal(err)
	}
	return dataDir
}

func TestCheckSelectionGroundedness_FabricatedNumber_FlagsOnlyThatBullet(t *testing.T) {
	dataDir := writeChecksDataDir(t)
	selection := SelectionResult{
		Entries: []SelectedEntry{{
			EntryID: "experience/acme",
			Bullets: []SelectedBullet{
				{
					SourceIndex: 0,
					Source:      "Migrated the legacy billing service to a microservices architecture, reducing incident response time by 60 percent.",
					Rewritten:   "Moved the aging billing service onto microservices, cutting incident response time by 90 percent.",
				},
				{
					SourceIndex: 1,
					Source:      "Led a cross-functional team of 5 engineers.",
					Rewritten:   "Led a cross-functional team of five engineers across 5 disciplines.",
				},
			},
		}},
	}

	result, err := CheckSelectionGroundedness(dataDir, selection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Bullets) != 1 {
		t.Fatalf("expected exactly one flagged bullet, got %+v", result.Bullets)
	}
	got := result.Bullets[0]
	if got.EntryID != "experience/acme" || got.SourceIndex != 0 {
		t.Errorf("expected experience/acme bullet 0 flagged, got %+v", got)
	}
	if len(got.Flags) != 1 || got.Flags[0].Reason != ReasonNumericMismatch {
		t.Errorf("expected one numeric-mismatch flag, got %+v", got.Flags)
	}
	if result.CoverLetter != nil {
		t.Errorf("expected no cover letter flags from a bullets-only check, got %+v", result.CoverLetter)
	}
}

func TestCheckSelectionGroundedness_HeavyParaphrase_NoFlags(t *testing.T) {
	dataDir := writeChecksDataDir(t)
	selection := SelectionResult{
		Entries: []SelectedEntry{{
			EntryID: "experience/acme",
			Bullets: []SelectedBullet{{
				SourceIndex: 0,
				Source:      "Migrated the legacy billing service to a microservices architecture, reducing incident response time by 60 percent.",
				Rewritten:   "Moved the aging billing service onto a microservices architecture, cutting incident response time 60 percent.",
			}},
		}},
	}

	result, err := CheckSelectionGroundedness(dataDir, selection)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Bullets) != 0 || len(result.CoverLetter) != 0 {
		t.Errorf("expected an empty result for a grounded paraphrase, got %+v", result)
	}
}

func TestCheckSelectionGroundedness_SourceNotVerbatimFromMasterData_ReturnsInvalidSelection(t *testing.T) {
	// A source bullet that was itself edited would make the overlap check
	// meaningless (the rewrite would be scored against invented "source"
	// text), so the facade applies the same traceability rule Generate does.
	dataDir := writeChecksDataDir(t)
	selection := SelectionResult{
		Entries: []SelectedEntry{{
			EntryID: "experience/acme",
			Bullets: []SelectedBullet{{
				SourceIndex: 1,
				Source:      "Led a cross-functional team of 12 engineers.",
				Rewritten:   "Led a cross-functional team of 12 engineers.",
			}},
		}},
	}

	_, err := CheckSelectionGroundedness(dataDir, selection)
	if !errors.Is(err, ErrInvalidSelection) {
		t.Fatalf("expected ErrInvalidSelection, got %v", err)
	}
}

func TestCheckSelectionGroundedness_MissingDataDir_ReturnsError(t *testing.T) {
	_, err := CheckSelectionGroundedness(filepath.Join(t.TempDir(), "nope"), SelectionResult{})
	if err == nil {
		t.Fatal("expected an error for a missing Master Data directory")
	}
}

// renderChecksFixtureCV renders cv through the real typst binary (no skip
// guard, matching api's generations_render_test.go — CI installs typst and
// poppler-utils so these paths run for real) and returns the PDF path plus
// the assembled data.json bytes it was rendered from.
func renderChecksFixtureCV(t *testing.T, cv cvData) (string, []byte) {
	t.Helper()
	projectRoot := t.TempDir()
	copyTemplateFixture(t, projectRoot, "cv.typ")
	if err := os.MkdirAll(filepath.Join(projectRoot, "output", "checks"), 0o755); err != nil {
		t.Fatal(err)
	}
	pdfRelPath, err := renderTypst(projectRoot, "template/cv.typ", "checks", "data.json", "cv.pdf", cv)
	if err != nil {
		t.Fatalf("renderTypst: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(projectRoot, "output", "checks", "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(projectRoot, pdfRelPath), data
}

func checksFixtureCVData(lang string) cvData {
	return cvData{
		Name:      "Jane Doe",
		Lang:      lang,
		Location:  "Milan, Italy",
		Email:     "jane@example.com",
		Phone:     "+39 000 000 000",
		LinkedIn:  "janedoe",
		GitHub:    "janedoe",
		Education: []masterdata.Education{},
		Experience: []cvExperience{{
			Employer: "Acme Corp",
			Role:     "Senior Engineer",
			Start:    "2022",
			Bullets:  []string{"Shipped things."},
		}},
		Projects:     []cvProject{{Name: "Widget Tool", Start: "2023", Bullets: []string{"Built a widget."}}},
		TechStack:    []string{"Go"},
		Publications: []masterdata.Publication{},
		Awards:       []masterdata.Award{},
		Activities:   []masterdata.Activity{},
		Languages:    []masterdata.Language{},
	}
}

func TestCheckRenderedCV_RealRender_OnePageParsableInSupportedLanguage(t *testing.T) {
	pdfPath, data := renderChecksFixtureCV(t, checksFixtureCVData("it"))

	result, err := CheckRenderedCV(pdfPath, data, TermSource{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", result.PageCount)
	}
	if result.ATSReport.Status != ParsabilityOK {
		t.Errorf("Parsability = %+v, want ok", result.ATSReport)
	}
	if result.Language != "it" || result.LanguageWarning != "" {
		t.Errorf("Language = %q (warning %q), want \"it\" with no warning", result.Language, result.LanguageWarning)
	}
}

func TestCheckRenderedCV_MissingOrUnsupportedLang_WarnsAndReportsDefault(t *testing.T) {
	for _, lang := range []string{"", "fr"} {
		t.Run("lang="+lang, func(t *testing.T) {
			pdfPath, data := renderChecksFixtureCV(t, checksFixtureCVData(lang))

			result, err := CheckRenderedCV(pdfPath, data, TermSource{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Language != DefaultLanguage {
				t.Errorf("Language = %q, want %q", result.Language, DefaultLanguage)
			}
			if result.LanguageWarning == "" {
				t.Error("expected a LanguageWarning for a missing/unsupported lang")
			}
		})
	}
}

func TestCheckRenderedCV_PdftotextMissing_ReportsUnavailableNotWarning(t *testing.T) {
	pdfPath, data := renderChecksFixtureCV(t, checksFixtureCVData("en"))
	t.Setenv("PATH", t.TempDir())

	result, err := CheckRenderedCV(pdfPath, data, TermSource{})
	if err != nil {
		t.Fatalf("a missing pdftotext must degrade, not error: %v", err)
	}
	if result.ATSReport.Status != ParsabilityUnavailable || result.ATSReport.Reason == "" {
		t.Errorf("Parsability = %+v, want unavailable with a reason", result.ATSReport)
	}
	if result.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1 (page counting needs no external tool)", result.PageCount)
	}
}

func TestCheckRenderedCV_MalformedData_ReturnsError(t *testing.T) {
	pdfPath, _ := renderChecksFixtureCV(t, checksFixtureCVData("en"))

	if _, err := CheckRenderedCV(pdfPath, []byte(`{"name": `), TermSource{}); err == nil {
		t.Fatal("expected an error for malformed assembled data")
	}
}

func TestCheckRenderedCV_MissingPDF_ReturnsError(t *testing.T) {
	data := []byte(`{"name": "Jane Doe", "lang": "en"}`)

	if _, err := CheckRenderedCV(filepath.Join(t.TempDir(), "missing.pdf"), data, TermSource{}); err == nil {
		t.Fatal("expected an error for a missing PDF")
	}
}

// Master Data is Markdown, but template/cv.typ prints a bullet literally —
// so `server.json` reaches the PDF with its backticks showing (issue #170).
// The check is advisory: it names what will be visible, and blocks nothing.
func TestCheckRenderedCV_FlagsMarkdownMarkupInBullets(t *testing.T) {
	cv := cvData{
		Lang: "en",
		Experience: []cvExperience{{
			Employer: "Example Consulting", Client: "Example Client A",
			Bullets: []string{"Built the wizards that generate MCP `server.json` manifests."},
		}},
		Projects: []cvProject{{
			Name:    "Example",
			Bullets: []string{"Installable with the **npx skills** convention."},
		}},
	}

	warnings := findMarkdownMarkup(cv)

	if len(warnings) != 2 {
		t.Fatalf("expected both bullets flagged, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "`server.json`") {
		t.Errorf("expected the code span quoted back, got %q", warnings[0])
	}
	if !strings.Contains(warnings[1], "**npx skills**") {
		t.Errorf("expected the bold span quoted back, got %q", warnings[1])
	}
}

// Plain bullets are the normal case and must not be flagged — including a
// snake_case identifier, where the underscores are the name, not markup.
func TestCheckRenderedCV_DoesNotFlagPlainBullets(t *testing.T) {
	cv := cvData{
		Lang: "en",
		Experience: []cvExperience{{
			Bullets: []string{
				"Migrated tables of several billion rows into AWS.",
				"Tuned the max_workers setting on the Glue job.",
				"Cut a 6 h load to 40 min — a 9x improvement.",
			},
		}},
	}

	if warnings := findMarkdownMarkup(cv); len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

// A code span is unwrapped when the data file is assembled, so the common
// case never reaches the PDF at all.
func TestStripInlineMarkup_UnwrapsCodeSpans(t *testing.T) {
	got := stripInlineMarkup("generate A2A agent cards and MCP `server.json` manifests")
	want := "generate A2A agent cards and MCP server.json manifests"
	if got != want {
		t.Errorf("stripInlineMarkup() = %q, want %q", got, want)
	}
}

// Asterisks and underscores are left alone: they can be the author's own
// punctuation, and deleting one would change what the bullet claims.
func TestStripInlineMarkup_LeavesOtherPunctuationAlone(t *testing.T) {
	for _, s := range []string{"a *real* asterisk", "snake_case_name", "2 * 3 = 6"} {
		if got := stripInlineMarkup(s); got != s {
			t.Errorf("stripInlineMarkup(%q) = %q, want it unchanged", s, got)
		}
	}
}
