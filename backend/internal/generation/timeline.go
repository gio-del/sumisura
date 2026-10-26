package generation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// The timeline check (ADR-0045) answers the question groundedness never
// asked: does what this CV says about *time* trace back to Master Data?
//
// Groundedness scores a rewritten bullet against its source bullet, so a
// sentence cannot invent a fact. Nothing scored the dates — and dates are
// claims too, among the most consequential a CV makes. The bug that
// prompted this check rendered five months of tenure at a junior title for
// what was two years ending in a senior one, and every existing check
// passed: the bullets traced, the text layer extracted, the page fit.
//
// It runs at two levels, because one level would not have caught it:
//
//   - against Master Data, where a wrongly assembled span is visible;
//   - against the rendered text layer, where a *correctly* assembled span
//     that the template then dropped is visible. That was this bug exactly:
//     data.json held every engagement's true dates and cv.typ printed none
//     of them.
//
// Advisory, like every check (ADR-0028): it reports, it never blocks.

// checkTimeline reports every temporal claim the rendered CV makes that
// Master Data does not support, or that the reader cannot actually see.
// extracted is the PDF's text layer, empty when extraction was unavailable
// — the Master Data comparisons still run, the rendered ones are skipped
// rather than guessed at.
func checkTimeline(cv cvData, entries []masterdata.Entry, extracted string) []string {
	var warnings []string

	seenBlock := make(map[string]bool)
	seenTenure := make(map[string]bool)
	for _, exp := range cv.Experience {
		who := exp.Employer
		if exp.Client != "" {
			who = fmt.Sprintf("%s / %s", exp.Employer, exp.Client)
		}

		// With no Master Data in hand the comparisons below have nothing to
		// compare against, and reporting every span as unbacked would be a
		// statement about the caller, not about the CV. The rendered-text
		// checks still run: they need only the PDF.
		source, found := matchEntry(entries, exp)
		if len(entries) == 0 {
			found, source = false, masterdata.Entry{}
		} else if !found {
			warnings = append(warnings, fmt.Sprintf(
				"experience %q renders a span (%s) that no Master Data Entry backs", who, formatSpan(exp.Start, exp.End)))
		}
		if found {
			if exp.Start != source.Start || !sameEnd(exp.End, source.End) {
				warnings = append(warnings, fmt.Sprintf(
					"experience %q renders %s but its Entry says %s",
					who, formatSpan(exp.Start, exp.End), formatSpan(source.Start, source.End)))
			}
			if source.End == nil && exp.End != nil {
				warnings = append(warnings, fmt.Sprintf(
					"experience %q is ongoing in Master Data but renders as ended (%s)", who, fmtDate(exp.End)))
			}

			// The Employer Tenure the CV claims, against the one derived
			// from every Entry of that employer — not just the Selected
			// ones, which is the whole point. Reported once per block: it is
			// a claim the block makes, not one each engagement repeats.
			if tenure, ok := masterdata.TenureOf(entries, source); ok && !seenTenure[tenureKey(exp)] {
				seenTenure[tenureKey(exp)] = true
				switch {
				case exp.EmployerStart == "":
					warnings = append(warnings, fmt.Sprintf(
						"employer %q carries no tenure in the assembled data, so the block states whichever engagement renders first instead of the real %s",
						exp.Employer, formatSpan(tenure.Start, tenure.End)))
				case exp.EmployerStart != tenure.Start || !sameEnd(exp.EmployerEnd, tenure.End):
					msg := fmt.Sprintf("employer %q renders as %s but Master Data shows %s",
						exp.Employer, formatSpan(exp.EmployerStart, exp.EmployerEnd), formatSpan(tenure.Start, tenure.End))
					if short := monthsShort(exp.EmployerStart, exp.EmployerEnd, tenure); short > 0 {
						msg += fmt.Sprintf(" — understates the tenure by %d months", short)
					}
					warnings = append(warnings, msg)
				}
			}
		}

		if extracted == "" {
			continue
		}

		// An In-house Role must show its own span. A Client Engagement prints
		// under its employer block without one (ADR-0048, amending ADR-0045):
		// the block states the Employer Tenure, checked below, and the
		// engagement's own dates stay checked against Master Data above.
		if exp.Kind == masterdata.KindInHouse && !textHasSpan(extracted, exp.Start, exp.End) {
			warnings = append(warnings, fmt.Sprintf(
				"experience %q renders no span of its own in the PDF text (expected %q)", who, formatSpan(exp.Start, exp.End)))
		}
		if exp.Kind != masterdata.KindInHouse && exp.EmployerStart != "" && !seenBlock[tenureKey(exp)] {
			seenBlock[tenureKey(exp)] = true
			if !textHasSpan(extracted, exp.EmployerStart, exp.EmployerEnd) {
				warnings = append(warnings, fmt.Sprintf(
					"employer %q states no tenure in the PDF text (expected %q) — a reader sees only the engagements' own dates",
					exp.Employer, formatSpan(exp.EmployerStart, exp.EmployerEnd)))
			}
		}
	}

	if extracted != "" {
		for _, edu := range cv.Education {
			end := &edu.End
			if !textHasSpan(extracted, edu.Start, end) {
				warnings = append(warnings, fmt.Sprintf(
					"education %q renders no span in the PDF text (expected %q)", edu.Institution, formatSpan(edu.Start, end)))
			}
		}
		for _, p := range cv.Projects {
			if !textHasSpan(extracted, p.Start, p.End) {
				warnings = append(warnings, fmt.Sprintf(
					"project %q renders no span in the PDF text (expected %q)", p.Name, formatSpan(p.Start, p.End)))
			}
		}
	}

	return warnings
}

// tenureKey identifies one employer block: the employer plus the tenure it
// states, so two separate stints at one employer are two blocks.
func tenureKey(exp cvExperience) string { return exp.Employer + "\x00" + exp.EmployerStart }

// matchEntry finds the Master Data Entry a rendered experience item came
// from. Assembled data carries no Entry id, so it is matched on the
// identifying frontmatter: employer, client and start.
func matchEntry(entries []masterdata.Entry, exp cvExperience) (masterdata.Entry, bool) {
	for _, e := range entries {
		if e.Type == masterdata.TypeExperience &&
			e.Employer == exp.Employer &&
			e.Client == exp.Client &&
			e.Start == exp.Start {
			return e, true
		}
	}
	// Fall back to employer + client, so a wrong *date* is reported as the
	// mismatch it is rather than as a missing Entry.
	for _, e := range entries {
		if e.Type == masterdata.TypeExperience && e.Employer == exp.Employer && e.Client == exp.Client {
			return e, true
		}
	}
	return masterdata.Entry{}, false
}

// monthsShort reports how many months of a true Tenure the rendered one
// leaves out, 0 when it does not understate it.
func monthsShort(start string, end *string, tenure masterdata.Tenure) int {
	rendered, ok := masterdata.SpanMonths(start, end)
	if !ok {
		return 0
	}
	actual, ok := masterdata.SpanMonths(tenure.Start, tenure.End)
	if !ok {
		return 0
	}
	if actual > rendered {
		return actual - rendered
	}
	return 0
}

func sameEnd(a, b *string) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	}
	return *a == *b
}

var monthNames = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// fmtDate mirrors template/cv.typ's fmt-date: "YYYY-MM" renders as
// "Mon YYYY", a bare year as itself, and an absent end as "present". The
// rendered-text checks compare against what the template actually prints,
// so the two must agree.
func fmtDate(date *string) string {
	if date == nil || strings.TrimSpace(*date) == "" {
		return "Present"
	}
	parts := strings.Split(strings.TrimSpace(*date), "-")
	if len(parts) != 2 {
		return parts[0]
	}
	month := 0
	if _, err := fmt.Sscanf(parts[1], "%d", &month); err != nil || month < 1 || month > 12 {
		return *date
	}
	return monthNames[month-1] + " " + parts[0]
}

// formatSpan mirrors the template's dated-row: one date when start and end
// are the same, otherwise "start – end".
func formatSpan(start string, end *string) string {
	from := fmtDate(&start)
	to := fmtDate(end)
	if end != nil && start == *end {
		return from
	}
	return from + " – " + to
}

// textHasSpan reports whether the PDF's text layer shows this span.
// Extraction collapses and re-wraps whitespace, and typst renders the
// separator as an en dash, so the tokens are matched with flexible
// whitespace rather than compared literally.
func textHasSpan(extracted, start string, end *string) bool {
	span := formatSpan(start, end)
	parts := strings.Fields(span)
	if len(parts) == 0 {
		return true
	}
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = regexp.QuoteMeta(p)
	}
	// Accept any dash the renderer or extractor may have produced.
	pattern := strings.ReplaceAll(strings.Join(quoted, `\s+`), regexp.QuoteMeta("–"), `[–—-]`)
	re, err := regexp.Compile(pattern)
	if err != nil {
		return true
	}
	return re.MatchString(extracted)
}
