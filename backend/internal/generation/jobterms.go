package generation

import (
	"sort"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// MinJobTermCount is how many times a term must occur in the Job Description
// to count as recurring: a term named once is more likely incidental.
const MinJobTermCount = 2

// maxJobTerms caps the list so a long Job Description does not bury the
// terms that matter.
const maxJobTerms = 15

// MissingJobTerm is a recurring term of the Job Description that the CV and
// Cover Letter text do not contain. EntryIDs names the Entries that Selection
// left out and that do contain it, so Text Review can point at "add an
// excluded Entry"; empty when no unselected Entry carries the term.
type MissingJobTerm struct {
	Term     string   `json:"term"`
	Count    int      `json:"count"`
	EntryIDs []string `json:"entryIds,omitempty"`
}

// JobTermCheck is CheckJobTermCoverage's verdict. An empty Missing means the
// check ran and every recurring term is present. Advisory, like every other
// check (ADR-0028): it never produces CV text and never blocks a checkpoint.
type JobTermCheck struct {
	Missing []MissingJobTerm `json:"missing,omitempty"`
}

// CheckJobTermCoverage lists the recurring terms of jobDescription (the
// Gaps report's term rules, masterdata.TermCounts) absent from text, the
// concatenated CV and Cover Letter text. For each one it names the entries
// outside selectedIDs whose Master Data contains it. Deterministic, local, no
// Claude call. Single words that are part of a listed pair are dropped, so
// "machine learning" is not repeated as "machine" and "learning".
func CheckJobTermCoverage(jobDescription, text string, entries []masterdata.Entry, selectedIDs []string) JobTermCheck {
	if strings.TrimSpace(jobDescription) == "" {
		return JobTermCheck{}
	}
	present := masterdata.TextTerms(text)
	var missing []MissingJobTerm
	for term, n := range masterdata.TermCounts(jobDescription) {
		if n >= MinJobTermCount && !present[term] {
			missing = append(missing, MissingJobTerm{Term: term, Count: n})
		}
	}

	inPair := map[string]bool{}
	for _, m := range missing {
		if words := strings.Split(m.Term, " "); len(words) > 1 {
			for _, w := range words {
				inPair[w] = true
			}
		}
	}
	kept := missing[:0]
	for _, m := range missing {
		if !strings.Contains(m.Term, " ") && inPair[m.Term] {
			continue
		}
		kept = append(kept, m)
	}
	missing = kept

	sort.Slice(missing, func(i, j int) bool {
		if missing[i].Count != missing[j].Count {
			return missing[i].Count > missing[j].Count
		}
		return missing[i].Term < missing[j].Term
	})
	if len(missing) > maxJobTerms {
		missing = missing[:maxJobTerms]
	}

	selected := map[string]bool{}
	for _, id := range selectedIDs {
		selected[id] = true
	}
	entryTerms := map[string]map[string]bool{}
	for _, e := range entries {
		if !selected[e.ID] {
			entryTerms[e.ID] = masterdata.EntryTerms(e)
		}
	}
	for i := range missing {
		for _, e := range entries {
			if entryTerms[e.ID][missing[i].Term] {
				missing[i].EntryIDs = append(missing[i].EntryIDs, e.ID)
			}
		}
	}
	return JobTermCheck{Missing: missing}
}

// generatedText is the text a Generation's CV and Cover Letter print from
// its Selection and Cover Letter: About me, the Entries' identifying fields,
// printed Tags and rewritten bullets, and the letter body.
func generatedText(selection SelectionResult, entries []masterdata.Entry, letter string) string {
	byID := make(map[string]masterdata.Entry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}
	parts := []string{selection.Summary, selection.Headline, letter}
	for _, se := range selection.Entries {
		e, ok := byID[se.EntryID]
		if !ok {
			continue
		}
		parts = append(parts, e.Employer, e.Client, e.ClientLabel, e.Title, e.Group, e.Role, e.Name)
		tags := se.Tags
		if len(tags) == 0 {
			tags = e.Tags
			if len(tags) > MaxEntryTags {
				tags = tags[:MaxEntryTags]
			}
		}
		parts = append(parts, tags...)
		for _, b := range se.Bullets {
			parts = append(parts, b.Rewritten)
		}
	}
	return strings.Join(parts, "\n")
}

// selectedIDs lists the Entry ids of selection.
func selectedIDs(selection SelectionResult) []string {
	ids := make([]string, 0, len(selection.Entries))
	for _, se := range selection.Entries {
		ids = append(ids, se.EntryID)
	}
	return ids
}
