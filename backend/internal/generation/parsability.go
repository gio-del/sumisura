package generation

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// ParsabilityStatus is the outcome of an ATS-parsability check (PRD "PDF
// ATS-parsability check") against a rendered PDF's extracted text layer.
type ParsabilityStatus string

const (
	// ParsabilityOK means every expected field was found in the extracted
	// text, in the expected relative order.
	ParsabilityOK ParsabilityStatus = "ok"
	// ParsabilityWarning means at least one expected field was missing or
	// out of order. Never blocks Render or Visual Review (PRD story 7) —
	// it's a signal surfaced alongside Visual Review, not a gate.
	ParsabilityWarning ParsabilityStatus = "warning"
	// ParsabilityUnavailable means the check itself couldn't run (e.g.
	// pdftotext missing or erroring) — distinct from ParsabilityWarning,
	// since a tooling failure says nothing about the PDF's actual
	// parsability (PRD story 10).
	ParsabilityUnavailable ParsabilityStatus = "unavailable"
)

// ATSReport is the ATS-parsability check's structured outcome for one
// rendered PDF (issue #49), kept on the Generation it belongs to (issue
// #198). It carries the extracted text layer itself, so what a screener
// reads can be shown long after output/ has been cleaned up, and a
// field-by-field verdict: which expected fields showed up, and whether in
// the order the template renders them.
//
// MissingFields and OrderingViolations are Fields restated as sentences,
// for the Visual Review badge and cvcheck's text output.
type ATSReport struct {
	Status ParsabilityStatus `json:"status" yaml:"status"`
	// Reason explains an Unavailable Status (PRD story 10) — empty
	// otherwise.
	Reason             string     `json:"reason,omitempty" yaml:"reason,omitempty"`
	Fields             []ATSField `json:"fields,omitempty" yaml:"fields,omitempty"`
	MissingFields      []string   `json:"missingFields,omitempty" yaml:"missingFields,omitempty"`
	OrderingViolations []string   `json:"orderingViolations,omitempty" yaml:"orderingViolations,omitempty"`
	// TermCoverage is set only when a Job Description was given: which of
	// its terms that are also Master Data tags made it into the text
	// layer. Informational — it never changes Status.
	TermCoverage *TermCoverage `json:"termCoverage,omitempty" yaml:"termCoverage,omitempty"`
	// ExtractedText is the PDF's text layer as pdftotext -layout reads it.
	ExtractedText string `json:"extractedText,omitempty" yaml:"extractedText,omitempty"`
}

// ATSFieldGroup says what kind of thing an expected field is, so a report
// can be read by section rather than as one flat list.
type ATSFieldGroup string

const (
	ATSGroupIdentity   ATSFieldGroup = "identity"
	ATSGroupContact    ATSFieldGroup = "contact"
	ATSGroupSection    ATSFieldGroup = "section"
	ATSGroupExperience ATSFieldGroup = "experience"
	ATSGroupProject    ATSFieldGroup = "project"
	ATSGroupBody       ATSFieldGroup = "body"
)

// ATSField is one expected field's verdict. InOrder is meaningful only when
// Found: a missing field has no position to compare.
type ATSField struct {
	Label   string        `json:"label" yaml:"label"`
	Group   ATSFieldGroup `json:"group" yaml:"group"`
	Found   bool          `json:"found" yaml:"found"`
	InOrder bool          `json:"inOrder" yaml:"inOrder"`
}

// TermCoverage lists the Job Description's terms, restricted to tags the
// installation owner's own Master Data uses, by whether the CV's text layer
// contains them. Only tags: the report never suggests claiming a skill the
// Master Data doesn't have.
type TermCoverage struct {
	Present []string `json:"present" yaml:"present"`
	Missing []string `json:"missing" yaml:"missing"`
}

// ATSReports is every ATS Report of one Generation: the CV's always, the
// Cover Letter's when there is one. It is the shape Render returns, the
// Generation record keeps, and output/<slug>/ats-report.json holds.
type ATSReports struct {
	CV          ATSReport  `json:"cv" yaml:"cv"`
	CoverLetter *ATSReport `json:"coverLetter,omitempty" yaml:"coverLetter,omitempty"`
}

// ATSReportFile is the name of the report file written beside a
// Generation's PDFs.
const ATSReportFile = "ats-report.json"

// WriteATSReports writes reports to ats-report.json in a Generation's
// output directory (output/<slug>/). Derived, like the PDFs next to it, so
// a plain write (ADR-0018).
func WriteATSReports(outputDir string, reports ATSReports) error {
	content, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, ATSReportFile), append(content, '\n'), 0o644)
}

// expectedField is one piece of source data (cvData or coverLetterData)
// the rendered PDF's text layer is expected to contain, in the order the
// template should render it in.
type expectedField struct {
	// label identifies the field in MissingFields/OrderingViolations,
	// e.g. "Experience employer \"Acme Corp\"".
	label string
	group ATSFieldGroup
	// text is the literal substring looked up in the extracted text.
	text string
}

// checkParsability is the PRD's pure comparison function: given a PDF's
// extracted text (from pdftotext) and the ordered list of fields the
// template should have rendered from the source data, it reports which
// fields are missing and which appear out of their expected relative
// order. It does no I/O and no PDF parsing of its own — extractPDFText
// handles that — which is what makes it directly unit-testable against
// fixture strings (PRD's Testing Decisions).
//
// Ordering is checked by tracking the furthest-right index seen so far
// among present fields: a field whose index falls before that point has
// been read out of order relative to something that should have preceded
// it. A field skipped as missing doesn't participate in ordering at all —
// there's no index to compare.
func checkParsability(extractedText string, fields []expectedField) ATSReport {
	var missing []string
	var violations []string
	var verdicts []ATSField

	runningMax := -1
	lastInOrderLabel := ""
	for _, f := range fields {
		idx := strings.Index(extractedText, f.text)
		if idx == -1 {
			missing = append(missing, f.label)
			verdicts = append(verdicts, ATSField{Label: f.label, Group: f.group})
			continue
		}
		if idx < runningMax {
			violations = append(violations, fmt.Sprintf("%s appears before %s in the extracted text", f.label, lastInOrderLabel))
			verdicts = append(verdicts, ATSField{Label: f.label, Group: f.group, Found: true})
			continue
		}
		runningMax = idx
		lastInOrderLabel = f.label
		verdicts = append(verdicts, ATSField{Label: f.label, Group: f.group, Found: true, InOrder: true})
	}

	status := ParsabilityOK
	if len(missing) > 0 || len(violations) > 0 {
		status = ParsabilityWarning
	}
	return ATSReport{
		Status:             status,
		Fields:             verdicts,
		MissingFields:      missing,
		OrderingViolations: violations,
		ExtractedText:      extractedText,
	}
}

// termCoverage finds which of tags the Job Description mentions, then
// which of those the extracted text also contains. Tags are matched as
// whole terms, case-insensitively — except very short ones ("Go", "R",
// "C"), which only match as written, since lowercase "go" is an ordinary
// English word. Each term is reported in its Master Data spelling, sorted.
func termCoverage(extractedText, jobDescription string, tags []string) *TermCoverage {
	coverage := &TermCoverage{Present: []string{}, Missing: []string{}}
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		key := strings.ToLower(tag)
		if tag == "" || seen[key] {
			continue
		}
		seen[key] = true
		if !containsTerm(jobDescription, tag) {
			continue
		}
		if containsTerm(extractedText, tag) {
			coverage.Present = append(coverage.Present, tag)
		} else {
			coverage.Missing = append(coverage.Missing, tag)
		}
	}
	sort.Strings(coverage.Present)
	sort.Strings(coverage.Missing)
	return coverage
}

// shortTermLength is the longest tag matched case-sensitively.
const shortTermLength = 2

func containsTerm(text, term string) bool {
	flags := "(?i)"
	if len([]rune(term)) <= shortTermLength {
		flags = ""
	}
	re := regexp.MustCompile(flags + `(^|[^\p{L}\p{N}])` + regexp.QuoteMeta(term) + `($|[^\p{L}\p{N}])`)
	return re.MatchString(text)
}

// cvExpectedFields builds the ordered list of fields cv.typ is expected to
// render from cv, per the PRD's Implementation Decisions: the name, each
// section's expected header text (only for sections cv.typ actually
// renders — Education and Experience unconditionally, the rest only when
// non-empty), and each Experience employer / Project name.
func cvExpectedFields(cv cvData) []expectedField {
	fields := []expectedField{{label: "Name", group: ATSGroupIdentity, text: cv.Name}}
	fields = append(fields, contactFields(cv.Email, cv.Phone, cv.LinkedIn, cv.GitHub)...)
	fields = append(fields,
		expectedField{label: "Education section header", group: ATSGroupSection, text: "Education"},
		expectedField{label: "Experience section header", group: ATSGroupSection, text: "Experience"},
	)

	seenEmployer := map[string]bool{}
	for _, exp := range cv.Experience {
		if exp.Employer == "" || seenEmployer[exp.Employer] {
			continue
		}
		seenEmployer[exp.Employer] = true
		fields = append(fields, expectedField{
			label: fmt.Sprintf("Experience employer %q", exp.Employer),
			group: ATSGroupExperience,
			text:  exp.Employer,
		})
	}

	if len(cv.Projects) > 0 {
		fields = append(fields, expectedField{label: "Projects section header", group: ATSGroupSection, text: "Projects"})
		for _, p := range cv.Projects {
			fields = append(fields, expectedField{label: fmt.Sprintf("Project %q", p.Name), group: ATSGroupProject, text: p.Name})
		}
	}

	if len(cv.TechStack) > 0 {
		fields = append(fields, expectedField{label: "Tech Stack section header", group: ATSGroupSection, text: "Tech Stack"})
	}
	if len(cv.Publications) > 0 {
		fields = append(fields, expectedField{label: "Publications section header", group: ATSGroupSection, text: "Publications"})
	}
	if len(cv.Awards) > 0 {
		fields = append(fields, expectedField{label: "Awards section header", group: ATSGroupSection, text: "Awards"})
	}
	if len(cv.Activities) > 0 {
		fields = append(fields, expectedField{label: "Activities section header", group: ATSGroupSection, text: "Activities"})
	}
	if len(cv.Languages) > 0 {
		fields = append(fields, expectedField{label: "Languages section header", group: ATSGroupSection, text: "Languages"})
	}

	return fields
}

// coverLetterExpectedFields builds the ordered list of fields
// cover-letter.typ is expected to render from cl: the name (in the
// contact-line header) followed by the body's opening line (cover-letter.typ
// renders the full body verbatim, but a whole-body substring match would be
// fragile against pdftotext's line-wrapping, so the first non-empty line is
// used as a representative anchor).
func coverLetterExpectedFields(cl coverLetterData) []expectedField {
	fields := []expectedField{{label: "Name", group: ATSGroupIdentity, text: cl.Name}}
	fields = append(fields, contactFields(cl.Email, cl.Phone, cl.LinkedIn, cl.GitHub)...)
	if firstLine := firstNonEmptyLine(cl.Body); firstLine != "" {
		fields = append(fields, expectedField{label: "Body opening", group: ATSGroupBody, text: firstLine})
	}
	return fields
}

// contactFields are the contact line's expected fields, in the order both
// templates print them after the name: email, phone, then the LinkedIn and
// GitHub links as their visible text (template/cv.typ prints
// "linkedin.com/in/<handle>" and "github.com/<handle>"). A contact detail
// the profile leaves empty isn't expected.
func contactFields(email, phone, linkedIn, gitHub string) []expectedField {
	var fields []expectedField
	add := func(label, text string) {
		if strings.TrimSpace(text) != "" {
			fields = append(fields, expectedField{label: label, group: ATSGroupContact, text: text})
		}
	}
	add("Email", email)
	add("Phone", phone)
	if strings.TrimSpace(linkedIn) != "" {
		add("LinkedIn", "linkedin.com/in/"+linkedIn)
	}
	if strings.TrimSpace(gitHub) != "" {
		add("GitHub", "github.com/"+gitHub)
	}
	return fields
}

// firstNonEmptyLine returns s's first line with non-whitespace content,
// trimmed, or "" if s has none.
func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// extractPDFText shells out to `pdftotext -layout <pdfPath> -`, printing
// the PDF's text layer to stdout, exactly the way renderTypst shells out
// to `typst compile` (see its godoc and ADR-0012): a pinned external tool
// invoked via os/exec rather than a Go-native PDF text-layer decoder (per
// the PRD's Implementation Decisions — reimplementing that decoding was
// judged a much larger, more fragile undertaking than shelling out to
// Poppler). -layout preserves the PDF's visual column/row layout as
// whitespace, which keeps multi-column or absolutely-positioned content —
// the exact failure mode this check exists to catch — from being silently
// re-flowed into a different reading order than what an ATS parser would
// actually see.
func extractPDFText(pdfPath string) (string, error) {
	cmd := exec.Command("pdftotext", "-layout", pdfPath, "-")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext failed: %w", err)
	}
	return string(out), nil
}

// buildATSReport runs the full ATS-parsability check against a rendered
// PDF: extract its text layer with pdftotext, compare it against fields
// with checkParsability, and, when jobDescription is non-empty, add its
// term coverage over tags. If pdftotext is missing or exits non-zero, it
// degrades to ParsabilityUnavailable with Reason set (PRD story 10) rather
// than returning an error — a tooling problem here must never fail Render,
// which already has its PDF.
func buildATSReport(pdfPath string, fields []expectedField, jobDescription string, tags []string) ATSReport {
	text, err := extractPDFText(pdfPath)
	if err != nil {
		return ATSReport{Status: ParsabilityUnavailable, Reason: err.Error()}
	}
	report := checkParsability(text, fields)
	if strings.TrimSpace(jobDescription) != "" {
		report.TermCoverage = termCoverage(text, jobDescription, tags)
	}
	return report
}

// TagVocabulary is every tag the Master Data under dataDir uses, the
// vocabulary term coverage draws from.
func TagVocabulary(dataDir string) ([]string, error) {
	entries, err := masterdata.ListEntries(dataDir)
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, e := range entries {
		tags = append(tags, e.Tags...)
	}
	return tags, nil
}
