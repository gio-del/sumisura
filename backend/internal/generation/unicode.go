package generation

import (
	"fmt"
	"strings"
)

// Invisible and lookalike characters survive a Rewrite, survive the render,
// and then quietly break the thing a CV exists to pass. A zero-width space
// inside "PostgreSQL" costs nothing visually and costs the keyword match: a
// screener searching for the term does not find it, while the ATS-parsability
// check reports the text layer as fine, because the text layer *is* fine —
// it contains a string that merely is not the one anybody searches for.
//
// Non-breaking spaces do the same to two-word terms ("Apache Spark"), and a
// soft hyphen inside a word splits it for any literal matcher.
//
// Reported, never rewritten: assembled content is the user's approved text,
// and silently editing it after Text Review would defeat the checkpoint.

type suspiciousRune struct {
	r    rune
	name string
	why  string
}

var suspiciousRunes = []suspiciousRune{
	{rune(0x200B), "zero-width space", "splits the word for any literal keyword match"},
	{rune(0x200C), "zero-width non-joiner", "splits the word for any literal keyword match"},
	{rune(0x200D), "zero-width joiner", "splits the word for any literal keyword match"},
	{rune(0xFEFF), "zero-width no-break space", "splits the word for any literal keyword match"},
	{rune(0x00AD), "soft hyphen", "splits the word for any literal keyword match"},
	{rune(0x00A0), "non-breaking space", "a screener matching a two-word term on a normal space will miss it"},
	{rune(0x202F), "narrow non-breaking space", "a screener matching a two-word term on a normal space will miss it"},
	{rune(0x2009), "thin space", "a screener matching a two-word term on a normal space will miss it"},
	{rune(0xFFFD), "replacement character", "marks text that already lost its original character somewhere upstream"},
	{rune(0x202A), "bidi embedding", "reorders the text layer relative to what is displayed"},
	{rune(0x202B), "bidi embedding", "reorders the text layer relative to what is displayed"},
	{rune(0x202D), "bidi override", "reorders the text layer relative to what is displayed"},
	{rune(0x202E), "bidi override", "reorders the text layer relative to what is displayed"},
}

// findSuspiciousUnicode reports invisible or ambiguous characters in the
// assembled strings that reach the PDF.
func findSuspiciousUnicode(cv cvData) []string {
	var warnings []string
	report := func(where, text string) {
		for _, s := range suspiciousRunes {
			if !strings.ContainsRune(text, s.r) {
				continue
			}
			warnings = append(warnings, fmt.Sprintf("%s contains a %s (U+%04X) — %s: %q",
				where, s.name, s.r, s.why, excerptAround(text, s.r)))
		}
	}

	for _, exp := range cv.Experience {
		who := exp.Employer
		if exp.Client != "" {
			who = exp.Client + " (" + exp.Employer + ")"
		}
		report(fmt.Sprintf("experience %q employer", who), exp.Employer)
		report(fmt.Sprintf("experience %q role", who), exp.Role)
		report(fmt.Sprintf("experience %q client", who), exp.Client)
		report(fmt.Sprintf("experience %q client label", who), exp.ClientLabel)
		report(fmt.Sprintf("experience %q title", who), exp.Title)
		for i, t := range exp.Tags {
			report(fmt.Sprintf("experience %q tag %d", who, i), t)
		}
		for i, b := range exp.Bullets {
			report(fmt.Sprintf("experience %q bullet %d", who, i), b)
		}
	}
	for _, p := range cv.Projects {
		report(fmt.Sprintf("project %q name", p.Name), p.Name)
		for i, t := range p.Tags {
			report(fmt.Sprintf("project %q tag %d", p.Name, i), t)
		}
		for i, b := range p.Bullets {
			report(fmt.Sprintf("project %q bullet %d", p.Name, i), b)
		}
	}
	report("headline", cv.Headline)
	report("summary", cv.Summary)
	return warnings
}

// excerptAround returns a short window around the offending rune, so the
// warning points at the word that will fail to match rather than at the
// whole bullet.
func excerptAround(text string, r rune) string {
	runes := []rune(text)
	at := -1
	for i, c := range runes {
		if c == r {
			at = i
			break
		}
	}
	if at < 0 {
		return text
	}
	const window = 18
	from, to := at-window, at+window
	if from < 0 {
		from = 0
	}
	if to > len(runes) {
		to = len(runes)
	}
	excerpt := string(runes[from:to])
	if from > 0 {
		excerpt = "…" + excerpt
	}
	if to < len(runes) {
		excerpt += "…"
	}
	return excerpt
}
