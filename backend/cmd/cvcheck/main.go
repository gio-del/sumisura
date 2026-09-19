// Command cvcheck runs the web app's automated Generation quality checks
// from the command line, so the tailor-cv skill can apply the same scrutiny
// to a skill-run Generation that the app applies to its own (issue #104,
// ADR-0028). It calls the generation package's exported check facades —
// the same code the backend runs — rather than reimplementing any check.
//
// Every check is pure and offline (no Claude call, no network, no running
// backend), which is why this is a CLI the skill shells out to rather than
// an HTTP call to the container.
//
// Usage:
//
//	cvcheck groundedness --selection output/<slug>/selection.json [--data-dir data] [--json]
//	cvcheck pdf --pdf output/<slug>/cv.pdf --data output/<slug>/data.json [--json]
//
// groundedness is the Text Review check point: every rewritten bullet in
// the Selection+Rewrite artifact scored against its source bullet. pdf is
// the Visual Review check point: page count, ATS-parsability of the PDF's
// text layer (via pdftotext) and the assembled data's target language.
//
// Exit status: 0 = ran, nothing flagged; 1 = ran, something flagged;
// 2 = the check could not run (bad usage, missing/malformed artifact,
// artifact not traceable to Master Data, or — with nothing else flagged —
// pdftotext unavailable). None of these is meant to stop
// the skill's pipeline: flags are information for the human checkpoint,
// not failures.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/generation"
)

const (
	exitClean       = 0
	exitFlagged     = 1
	exitUnavailable = 2
)

const usage = `usage:
  cvcheck groundedness --selection <output/<slug>/selection.json> [--data-dir data] [--json]
  cvcheck pdf --pdf <output/<slug>/cv.pdf> --data <output/<slug>/data.json>
              [--job-description <file> [--data-dir data]]
              [--cover-letter <output/<slug>/cover-letter.pdf> --cover-letter-data <output/<slug>/cover-letter-data.json>]
              [--report-out <output/<slug>/ats-report.json>] [--json]`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return unavailable(stderr, errors.New("no subcommand given\n"+usage))
	}
	switch args[0] {
	case "groundedness":
		return runGroundedness(args[1:], stdout, stderr)
	case "pdf":
		return runPDF(args[1:], stdout, stderr)
	default:
		return unavailable(stderr, fmt.Errorf("unknown subcommand %q\n%s", args[0], usage))
	}
}

// unavailable reports that a check could not run at all — distinct from a
// check that ran and flagged something, so a tooling or input problem is
// never mistaken for a verdict about the CV.
func unavailable(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "cvcheck: check unavailable: %v\n", err) //nolint:errcheck // CLI output; if the terminal write fails, the exit status still reports the outcome
	return exitUnavailable
}

func runGroundedness(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("groundedness", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	selectionPath := fs.String("selection", "", "path to the Selection+Rewrite artifact (output/<slug>/selection.json)")
	dataDir := fs.String("data-dir", "data", "path to the Master Data directory the selection was drawn from")
	asJSON := fs.Bool("json", false, "print the GroundednessResult as JSON instead of human-readable text")
	if err := fs.Parse(args); err != nil {
		return unavailable(stderr, fmt.Errorf("%v\n%s", err, usage))
	}
	if *selectionPath == "" {
		return unavailable(stderr, errors.New("--selection is required\n"+usage))
	}

	selection, err := readSelection(*selectionPath)
	if err != nil {
		return unavailable(stderr, err)
	}

	result, err := generation.CheckSelectionGroundedness(*dataDir, selection)
	if errors.Is(err, generation.ErrInvalidSelection) {
		// The sentinel's wording is written for the app's Claude client;
		// here the artifact's author is the skill, so name the file instead.
		detail := strings.TrimPrefix(err.Error(), generation.ErrInvalidSelection.Error()+": ")
		return unavailable(stderr, fmt.Errorf("groundedness: selection artifact %s is not traceable to Master Data in %s: %s (source bullets must be copied verbatim)", *selectionPath, *dataDir, detail))
	}
	if err != nil {
		return unavailable(stderr, fmt.Errorf("groundedness: %w", err))
	}

	if *asJSON {
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			return unavailable(stderr, fmt.Errorf("encoding result: %w", err))
		}
	} else {
		printGroundedness(stdout, selection, result)
	}

	if len(result.Bullets) > 0 {
		return exitFlagged
	}
	return exitClean
}

// readSelection decodes the Selection+Rewrite artifact strictly into
// generation.SelectionResult: an unknown field (e.g. "rewrittenText" for
// "rewritten") is an error rather than a silently empty rewrite that would
// pass the check without having been checked.
func readSelection(path string) (generation.SelectionResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return generation.SelectionResult{}, fmt.Errorf("reading selection artifact: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.DisallowUnknownFields()
	var selection generation.SelectionResult
	if err := dec.Decode(&selection); err != nil {
		return generation.SelectionResult{}, fmt.Errorf("parsing selection artifact %s: %w", path, err)
	}
	return selection, nil
}

//nolint:errcheck // prints a CLI report; if the terminal write fails, the exit status still reports the outcome
func printGroundedness(w io.Writer, selection generation.SelectionResult, result generation.GroundednessResult) {
	total := 0
	for _, e := range selection.Entries {
		total += len(e.Bullets)
	}

	if len(result.Bullets) == 0 {
		fmt.Fprintf(w, "Groundedness: checked %s against their source bullets — no flags.\n", pluralBullets(total))
	} else {
		fmt.Fprintf(w, "Groundedness: %d of %s flagged (non-blocking — look hardest at these at Text Review):\n", len(result.Bullets), pluralBullets(total))
		for _, b := range result.Bullets {
			for _, f := range b.Flags {
				fmt.Fprintf(w, "- %s bullet %d [%s: %s]\n  %q\n", b.EntryID, b.SourceIndex, f.Reason, explainReason(f.Reason), f.Sentence)
			}
		}
	}

	fmt.Fprintln(w, describeLanguage(selection.Language))
}

func pluralBullets(n int) string {
	if n == 1 {
		return "1 rewritten bullet"
	}
	return fmt.Sprintf("%d rewritten bullets", n)
}

func explainReason(r generation.GroundednessReason) string {
	switch r {
	case generation.ReasonNumericMismatch:
		return "states a number or date its source bullet doesn't"
	case generation.ReasonNoSourceMatch:
		return "no plausible match in its source bullet"
	default:
		return "flagged"
	}
}

// describeLanguage applies generation.NormalizeLanguage — the app's own
// supported-set-with-fallback rule — to the language recorded in the
// selection artifact, so the skill writes the same resolved code into the
// assembled data's lang field that the app would.
func describeLanguage(detected string) string {
	resolved := generation.NormalizeLanguage(detected)
	switch {
	case strings.TrimSpace(detected) == "":
		return fmt.Sprintf("Target language: %s (none recorded in the selection artifact; using the default)", resolved)
	case strings.ToLower(strings.TrimSpace(detected)) != resolved:
		return fmt.Sprintf("Target language: %s (detected %q is not supported; falling back to the default)", resolved, detected)
	default:
		return "Target language: " + resolved
	}
}

// pdfOutput is what `cvcheck pdf --json` prints: the CV's check, plus the
// Cover Letter's ATS Report when one was checked.
type pdfOutput struct {
	generation.RenderedCVCheck
	CoverLetterATSReport *generation.ATSReport `json:"coverLetterAtsReport,omitempty"`
}

func runPDF(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pdf", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pdfPath := fs.String("pdf", "", "path to the rendered CV (output/<slug>/cv.pdf)")
	dataPath := fs.String("data", "", "path to the assembled data it was rendered from (output/<slug>/data.json)")
	jobDescriptionPath := fs.String("job-description", "", "path to a file holding the Job Description, for the ATS Report's term coverage")
	dataDir := fs.String("data-dir", "data", "path to the Master Data directory whose tags term coverage draws from")
	coverLetterPath := fs.String("cover-letter", "", "path to the rendered Cover Letter (output/<slug>/cover-letter.pdf)")
	coverLetterDataPath := fs.String("cover-letter-data", "", "path to the data it was rendered from (output/<slug>/cover-letter-data.json)")
	reportOut := fs.String("report-out", "", "write the ATS Reports to this file (output/<slug>/ats-report.json)")
	asJSON := fs.Bool("json", false, "print the result as JSON instead of human-readable text")
	if err := fs.Parse(args); err != nil {
		return unavailable(stderr, fmt.Errorf("%v\n%s", err, usage))
	}
	if *pdfPath == "" || *dataPath == "" {
		return unavailable(stderr, errors.New("--pdf and --data are both required\n"+usage))
	}
	if (*coverLetterPath == "") != (*coverLetterDataPath == "") {
		return unavailable(stderr, errors.New("--cover-letter and --cover-letter-data go together\n"+usage))
	}

	data, err := os.ReadFile(*dataPath)
	if err != nil {
		return unavailable(stderr, fmt.Errorf("reading assembled data: %w", err))
	}
	var terms generation.TermSource
	if *jobDescriptionPath != "" {
		jd, err := os.ReadFile(*jobDescriptionPath)
		if err != nil {
			return unavailable(stderr, fmt.Errorf("reading job description: %w", err))
		}
		tags, err := generation.TagVocabulary(*dataDir)
		if err != nil {
			return unavailable(stderr, fmt.Errorf("reading master data tags: %w", err))
		}
		terms = generation.TermSource{JobDescription: string(jd), Tags: tags}
	}
	result, err := generation.CheckRenderedCV(*pdfPath, data, terms)
	if err != nil {
		return unavailable(stderr, fmt.Errorf("pdf: %w", err))
	}
	out := pdfOutput{RenderedCVCheck: result}
	if *coverLetterPath != "" {
		clData, err := os.ReadFile(*coverLetterDataPath)
		if err != nil {
			return unavailable(stderr, fmt.Errorf("reading cover letter data: %w", err))
		}
		report, err := generation.CheckRenderedCoverLetter(*coverLetterPath, clData)
		if err != nil {
			return unavailable(stderr, fmt.Errorf("cover letter: %w", err))
		}
		out.CoverLetterATSReport = &report
	}

	if *reportOut != "" {
		reports := generation.ATSReports{CV: result.ATSReport, CoverLetter: out.CoverLetterATSReport}
		content, err := json.MarshalIndent(reports, "", "  ")
		if err == nil {
			err = os.WriteFile(*reportOut, append(content, '\n'), 0o644)
		}
		if err != nil {
			return unavailable(stderr, fmt.Errorf("writing ATS report: %w", err))
		}
	}

	if *asJSON {
		if err := json.NewEncoder(stdout).Encode(out); err != nil {
			return unavailable(stderr, fmt.Errorf("encoding result: %w", err))
		}
	} else {
		printPDF(stdout, out)
	}

	statuses := []generation.ParsabilityStatus{result.ATSReport.Status}
	if out.CoverLetterATSReport != nil {
		statuses = append(statuses, out.CoverLetterATSReport.Status)
	}
	switch {
	case result.PageCount != 1 || slices.Contains(statuses, generation.ParsabilityWarning) || result.LanguageWarning != "":
		return exitFlagged
	case slices.Contains(statuses, generation.ParsabilityUnavailable):
		return exitUnavailable
	default:
		return exitClean
	}
}

//nolint:errcheck // prints a CLI report; if the terminal write fails, the exit status still reports the outcome
func printPDF(w io.Writer, out pdfOutput) {
	r := out.RenderedCVCheck
	if r.PageCount == 1 {
		fmt.Fprintln(w, "Page count: 1")
	} else {
		fmt.Fprintf(w, "Page count: %d (a Tailored CV must be one page — trim Selection and re-render)\n", r.PageCount)
	}

	printATSReport(w, "ATS-parsability", r.ATSReport)
	if out.CoverLetterATSReport != nil {
		printATSReport(w, "Cover Letter ATS-parsability", *out.CoverLetterATSReport)
	}

	if len(r.MarkupWarnings) > 0 {
		fmt.Fprintln(w, "Markdown markup: warning (non-blocking — the template prints bullets literally):")
		for _, warning := range r.MarkupWarnings {
			fmt.Fprintln(w, "  - "+warning)
		}
	}
	if r.LanguageWarning == "" {
		fmt.Fprintln(w, "Language: "+r.Language)
	} else {
		fmt.Fprintf(w, "Language: %s (warning: %s)\n", r.Language, r.LanguageWarning)
	}
}

//nolint:errcheck // prints a CLI report; if the terminal write fails, the exit status still reports the outcome
func printATSReport(w io.Writer, title string, report generation.ATSReport) {
	switch report.Status {
	case generation.ParsabilityOK:
		fmt.Fprintln(w, title+": ok")
	case generation.ParsabilityWarning:
		fmt.Fprintln(w, title+": warning (non-blocking — an ATS reading the text layer may not see these):")
		for _, f := range report.MissingFields {
			fmt.Fprintf(w, "- missing from the extracted text: %s\n", f)
		}
		for _, v := range report.OrderingViolations {
			fmt.Fprintf(w, "- out of order: %s\n", v)
		}
	default:
		fmt.Fprintf(w, "%s: unavailable (%s) — the check could not run; this says nothing about the PDF itself\n", title, report.Reason)
		return
	}

	var contact []string
	for _, f := range report.Fields {
		if f.Group != generation.ATSGroupContact {
			continue
		}
		verdict := "found"
		if !f.Found {
			verdict = "MISSING"
		}
		contact = append(contact, f.Label+" "+verdict)
	}
	if len(contact) > 0 {
		fmt.Fprintln(w, "  Contact: "+strings.Join(contact, ", "))
	}
	if tc := report.TermCoverage; tc != nil {
		fmt.Fprintf(w, "  Job Description terms in the text layer: %d of %d\n", len(tc.Present), len(tc.Present)+len(tc.Missing))
		if len(tc.Missing) > 0 {
			fmt.Fprintln(w, "  - mentioned by the Job Description, in your Master Data, but not on this CV: "+strings.Join(tc.Missing, ", "))
		}
	}
}
