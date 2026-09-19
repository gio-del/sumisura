package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/generation"
)

// assembledData returns a data.json in the shape SKILL.md's Assemble step
// writes (and template/cv.typ reads), with lang and the experience bullet
// list parameterized. extraProjects are appended to projects.
func assembledData(lang string, bullets []string, extraProjects ...string) string {
	projects := []map[string]any{
		{"name": "Widget Tool", "start": "2023", "end": nil, "bullets": []string{"Built a widget."}},
	}
	for _, p := range extraProjects {
		projects = append(projects, map[string]any{"name": p, "start": "2023", "end": nil, "bullets": []string{"Did a thing."}})
	}
	data := map[string]any{
		"name":     "Jane Doe",
		"location": "Example City, Country",
		"email":    "jane.doe@example.com",
		"phone":    "+1 555 0100",
		"linkedin": "janedoe",
		"github":   "janedoe",
		"education": []map[string]any{
			{"degree": "MSc", "institution": "Example University", "program": "Computer Science", "start": "2022", "end": "2024-04"},
		},
		"experience": []map[string]any{
			{"employer": "Acme Corp", "role": "Senior Engineer", "client": "Example Client", "location": "Milan", "start": "2022", "end": nil, "bullets": bullets},
		},
		"projects":     projects,
		"tech_stack":   []string{"Go"},
		"publications": []any{},
		"awards":       []any{},
		"activities":   []any{},
		"languages":    []map[string]any{{"name": "English", "level": "C2"}},
	}
	if lang != "<absent>" {
		data["lang"] = lang
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// renderFixture writes data to output/acme-corp/data.json under root and
// compiles it with the real typst binary and the repo's real template,
// exactly as SKILL.md's Render step does (no skip guard: CI installs typst
// and poppler-utils so this runs for real).
func renderFixture(t *testing.T, root, data string) {
	t.Helper()
	tmpl, err := os.ReadFile(filepath.Join("..", "..", "..", "template", "cv.typ"))
	if err != nil {
		t.Fatalf("reading real template: %v", err)
	}
	mustWrite(t, filepath.Join(root, "template", "cv.typ"), string(tmpl))
	mustWrite(t, filepath.Join(root, "output", "acme-corp", "data.json"), data)

	cmd := exec.Command("typst", "compile", "--root", ".", "template/cv.typ", "output/acme-corp/cv.pdf", "--input", "data=output/acme-corp/data.json")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("typst compile: %v\n%s", err, out)
	}
}

var pdfArgs = []string{"pdf", "--pdf", "output/acme-corp/cv.pdf", "--data", "output/acme-corp/data.json"}

func TestPDF_OnePageParsableSupportedLanguage_ExitsZero(t *testing.T) {
	root := writeFixtureProject(t)
	renderFixture(t, root, assembledData("it", []string{"Shipped things."}))

	res := runCLI(t, root, nil, pdfArgs...)

	if res.exitCode != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	for _, want := range []string{"Page count: 1", "ATS-parsability: ok", "Language: it"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("stdout missing %q: %q", want, res.stdout)
		}
	}
}

func TestPDF_Overflow_ExitsOneAndReportsPageCount(t *testing.T) {
	root := writeFixtureProject(t)
	bullets := make([]string, 90)
	for i := range bullets {
		bullets[i] = fmt.Sprintf("Delivered improvement number %d to a long-running backend platform.", i)
	}
	renderFixture(t, root, assembledData("en", bullets))

	res := runCLI(t, root, nil, pdfArgs...)

	if res.exitCode != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "Page count: 2") && !strings.Contains(res.stdout, "Page count: 3") {
		t.Errorf("expected a >1 page count to be reported, got %q", res.stdout)
	}
}

func TestPDF_ParsabilityWarning_ExitsOneAndNamesMissingField(t *testing.T) {
	root := writeFixtureProject(t)
	renderFixture(t, root, assembledData("en", []string{"Shipped things."}))
	// Check the PDF against data it was not rendered from: a project the
	// text layer can't contain stands in for content an ATS can't see.
	mustWrite(t, filepath.Join(root, "output", "acme-corp", "data.json"), assembledData("en", []string{"Shipped things."}, "Ghost Project"))

	res := runCLI(t, root, nil, pdfArgs...)

	if res.exitCode != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "ATS-parsability: warning") || !strings.Contains(res.stdout, `Project "Ghost Project"`) {
		t.Errorf("expected a warning naming the missing field, got %q", res.stdout)
	}
}

func TestPDF_MissingLang_ExitsOneWithLanguageWarning(t *testing.T) {
	root := writeFixtureProject(t)
	renderFixture(t, root, assembledData("<absent>", []string{"Shipped things."}))

	res := runCLI(t, root, nil, pdfArgs...)

	if res.exitCode != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "Language: en") || !strings.Contains(res.stdout, "no lang") {
		t.Errorf("expected a language warning, got %q", res.stdout)
	}
}

func TestPDF_PdftotextAbsent_ReportsUnavailableNotUnparsable(t *testing.T) {
	root := writeFixtureProject(t)
	renderFixture(t, root, assembledData("en", []string{"Shipped things."}))

	res := runCLI(t, root, []string{"PATH=" + t.TempDir()}, pdfArgs...)

	if res.exitCode != 2 {
		t.Fatalf("exit = %d, want 2; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "ATS-parsability: unavailable") || strings.Contains(res.stdout, "ATS-parsability: warning") {
		t.Errorf("expected parsability reported unavailable (not a warning), got %q", res.stdout)
	}
	if !strings.Contains(res.stdout, "Page count: 1") {
		t.Errorf("expected page count still reported without pdftotext, got %q", res.stdout)
	}
}

func TestPDF_JSONMode_DecodesIntoFacadeResultType(t *testing.T) {
	root := writeFixtureProject(t)
	data := assembledData("it", []string{"Shipped things."})
	renderFixture(t, root, data)

	res := runCLI(t, root, nil, append(pdfArgs, "--json")...)

	if res.exitCode != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	dec := json.NewDecoder(strings.NewReader(res.stdout))
	dec.DisallowUnknownFields()
	var got generation.RenderedCVCheck
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("decoding --json output: %v\n%s", err, res.stdout)
	}
	// ATSReport is the same ATSReport the API's RenderResult
	// carries as atsReports.cv, so the FE's existing badge applies as-is.
	if got.PageCount != 1 || got.ATSReport.Status != generation.ParsabilityOK || got.Language != "it" {
		t.Errorf("got %+v, want 1 page, parsability ok, language it", got)
	}
}

func TestPDF_CouldNotRun_ExitsTwo(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string)
		args  []string
	}{
		{
			name:  "missing pdf",
			setup: func(t *testing.T, root string) {},
			args:  pdfArgs,
		},
		{
			name: "malformed data",
			setup: func(t *testing.T, root string) {
				renderFixture(t, root, assembledData("en", []string{"Shipped things."}))
				mustWrite(t, filepath.Join(root, "output", "acme-corp", "data.json"), `{"name": `)
			},
			args: pdfArgs,
		},
		{
			name:  "missing --data flag",
			setup: func(t *testing.T, root string) {},
			args:  []string{"pdf", "--pdf", "output/acme-corp/cv.pdf"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeFixtureProject(t)
			tt.setup(t, root)

			res := runCLI(t, root, nil, tt.args...)

			if res.exitCode != 2 {
				t.Fatalf("exit = %d, want 2; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
			}
			if !strings.Contains(res.stderr, "unavailable") {
				t.Errorf("expected stderr to say the check is unavailable, got %q", res.stderr)
			}
		})
	}
}

// TestPDF_ATSReport_TermsContactAndReportFile is issue #198 from the
// skill's side: with the Job Description and the Cover Letter, cvcheck
// prints contact and term coverage, and --report-out writes the same
// ats-report.json Render writes, for the skill to attach to the record.
func TestPDF_ATSReport_TermsContactAndReportFile(t *testing.T) {
	root := writeFixtureProject(t)
	mustWrite(t, filepath.Join(root, "data", "projects", "cluster.md"), "---\nname: Cluster\nstart: \"2021\"\nend: \"2021\"\ntags:\n  - Kubernetes\n---\n\n- Ran a cluster.\n")
	renderFixture(t, root, assembledData("en", []string{"Shipped things in Go."}))
	mustWrite(t, filepath.Join(root, "output", "acme-corp", "job-description.txt"), "We write Go and run Kubernetes.")
	mustWrite(t, filepath.Join(root, "output", "acme-corp", "cover-letter-data.json"),
		`{"name": "Jane Doe", "location": "Example City", "email": "jane.doe@example.com", "phone": "+1 555 0100", "linkedin": "janedoe", "github": "janedoe", "body": "Dear Acme,\n\nHello."}`)
	tmpl, err := os.ReadFile(filepath.Join("..", "..", "..", "template", "cover-letter.typ"))
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "template", "cover-letter.typ"), string(tmpl))
	cmd := exec.Command("typst", "compile", "--root", ".", "template/cover-letter.typ", "output/acme-corp/cover-letter.pdf", "--input", "data=output/acme-corp/cover-letter-data.json")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("typst compile: %v\n%s", err, out)
	}

	res := runCLI(t, root, nil, append(pdfArgs,
		"--job-description", "output/acme-corp/job-description.txt",
		"--cover-letter", "output/acme-corp/cover-letter.pdf", "--cover-letter-data", "output/acme-corp/cover-letter-data.json",
		"--report-out", "output/acme-corp/ats-report.json")...)

	if res.exitCode != 0 {
		t.Fatalf("exit = %d, want 0 (term coverage never flags); stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	for _, want := range []string{
		"Contact: Email found, Phone found, LinkedIn found, GitHub found",
		"Job Description terms in the text layer: 1 of 2",
		"not on this CV: Kubernetes",
		"Cover Letter ATS-parsability: ok",
	} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, res.stdout)
		}
	}

	content, err := os.ReadFile(filepath.Join(root, "output", "acme-corp", "ats-report.json"))
	if err != nil {
		t.Fatalf("expected --report-out to write the report: %v", err)
	}
	var reports generation.ATSReports
	if err := json.Unmarshal(content, &reports); err != nil {
		t.Fatal(err)
	}
	if reports.CoverLetter == nil || !strings.Contains(reports.CV.ExtractedText, "Jane Doe") || reports.CV.TermCoverage == nil {
		t.Errorf("unexpected report file: %s", content)
	}
}

func TestPDF_CoverLetterWithoutItsData_Unavailable(t *testing.T) {
	root := writeFixtureProject(t)
	renderFixture(t, root, assembledData("en", []string{"Shipped things."}))

	res := runCLI(t, root, nil, append(pdfArgs, "--cover-letter", "output/acme-corp/cover-letter.pdf")...)

	if res.exitCode != 2 || !strings.Contains(res.stderr, "--cover-letter and --cover-letter-data go together") {
		t.Errorf("exit = %d, stderr = %q; want 2 and the pairing error", res.exitCode, res.stderr)
	}
}
