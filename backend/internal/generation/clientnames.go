package generation

import (
	"fmt"
	"regexp"
	"strings"
)

// findClientNames reports every assembled string that names a client whose
// Entry sets a Client Label (issue #232). The label exists to keep the real
// name off the CV, and Master Data bullets often name the client ("Amplifon's
// AI Platform"), so the label alone does not do it. The client's full name
// is searched, and its first word when that is distinctive (four letters or
// more), as whole words, case-insensitively.
func findClientNames(cv cvData) []string {
	type hidden struct{ client, pattern string }
	var names []hidden
	for _, exp := range cv.Experience {
		if exp.ClientLabel == "" || strings.TrimSpace(exp.Client) == "" {
			continue
		}
		terms := []string{regexp.QuoteMeta(exp.Client)}
		if first := strings.Fields(exp.Client)[0]; len([]rune(first)) >= 4 && !strings.EqualFold(first, exp.Client) {
			terms = append(terms, regexp.QuoteMeta(first))
		}
		names = append(names, hidden{exp.Client, `(?i)\b(` + strings.Join(terms, "|") + `)\b`})
	}
	if len(names) == 0 {
		return nil
	}

	var warnings []string
	check := func(where, text string) {
		for _, n := range names {
			if m := regexp.MustCompile(n.pattern).FindString(text); m != "" {
				warnings = append(warnings, fmt.Sprintf("%s names %q, whose client label is meant to hide it: %q", where, n.client, excerptAroundWord(text, m)))
			}
		}
	}
	check("headline", cv.Headline)
	check("summary", cv.Summary)
	for _, exp := range cv.Experience {
		who := exp.Employer
		if exp.ClientLabel != "" {
			who = exp.ClientLabel
		}
		check(fmt.Sprintf("experience %q title", who), exp.Title)
		check(fmt.Sprintf("experience %q group", who), exp.Group)
		for i, b := range exp.Bullets {
			check(fmt.Sprintf("experience %q bullet %d", who, i), b)
		}
	}
	for _, p := range cv.Projects {
		for i, b := range p.Bullets {
			check(fmt.Sprintf("project %q bullet %d", p.Name, i), b)
		}
	}
	return warnings
}

// excerptAroundWord is excerptAround for a matched word instead of a rune.
func excerptAroundWord(text, word string) string {
	at := strings.Index(text, word)
	if at < 0 {
		return text
	}
	from, to := at-25, at+len(word)+25
	if from < 0 {
		from = 0
	}
	if to > len(text) {
		to = len(text)
	}
	return strings.TrimSpace(text[from:to])
}
