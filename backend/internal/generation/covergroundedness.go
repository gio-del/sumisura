package generation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// claimWordRe splits text into the words a verifiable claim is made of. It
// breaks on every separator (hyphen, dot, slash), so "2019-03" in a source
// and "2019" in a letter, or "Spring-Boot" and "Spring Boot", compare equal.
var claimWordRe = regexp.MustCompile(`[\p{L}\p{N}]+`)

// CheckCoverLetterGroundedness is the Cover Letter's equivalent of the
// bullet check, for the claims a reader could verify: employers, clients,
// tools, figures and years. The letter is free prose, so it is not scored by
// overlap; instead every number, tool-shaped identifier (k8s, PostgreSQL)
// and capitalized name in it must appear somewhere in what the user wrote
// themselves: the Entries the Selection kept, the Cover Letter Snippets the
// letter drew from, and the profile. Each such term that does not yields one
// flag, naming it.
//
// Greeting and sign-off lines are skipped. A name taken from the Job
// Description (the company applied to) is not in Master Data and is flagged
// like any other; the flag is a hint for Text Review, never a block.
func CheckCoverLetterGroundedness(letter string, entries []masterdata.Entry, snippets []masterdata.Snippet, profile masterdata.Profile) []GroundednessFlag {
	if strings.TrimSpace(letter) == "" {
		return nil
	}
	known := map[string]bool{}
	addClaimWords(known, entryText(entries))
	for _, s := range snippets {
		addClaimWords(known, s.Body)
	}
	if b, err := json.Marshal(profile); err == nil {
		addClaimWords(known, string(b))
	}

	var flags []GroundednessFlag
	seen := map[string]bool{}
	for _, para := range strings.Split(letter, "\n") {
		para = strings.TrimSpace(para)
		if isLetterFormula(para) {
			continue
		}
		for _, sentence := range splitSentences(para) {
			for i, w := range claimWordRe.FindAllString(sentence, -1) {
				key := strings.ToLower(w)
				if seen[key] || known[key] || !isClaimTerm(w, i == 0) {
					continue
				}
				seen[key] = true
				flags = append(flags, GroundednessFlag{Sentence: sentence, Reason: ReasonUnsupportedTerm, Term: w})
			}
		}
	}
	return flags
}

// isLetterFormula says whether a line is a greeting or sign-off ("Dear
// Hiring Manager,", "Kind regards"): short and not a sentence.
func isLetterFormula(line string) bool {
	return len(claimWordRe.FindAllString(line, -1)) <= 6 && !sentenceEndRe.MatchString(line)
}

// isClaimTerm says whether w states something checkable: a number, a word
// mixing letters with digits or inner capitals (a tool name), or a
// capitalized word that does not open its sentence (a proper noun).
func isClaimTerm(w string, sentenceStart bool) bool {
	runes := []rune(w)
	if len(runes) < 2 {
		return false
	}
	if unicode.IsDigit(runes[0]) {
		return true
	}
	hasDigit, innerUpper := false, false
	for i, r := range runes {
		if unicode.IsDigit(r) {
			hasDigit = true
		}
		if i > 0 && unicode.IsUpper(r) {
			innerUpper = true
		}
	}
	if hasDigit || innerUpper {
		// An all-caps word of letters alone is shouting or an acronym the
		// reader knows (ATS, CV), not a tool name.
		return hasDigit || strings.ToUpper(w) != w
	}
	return !sentenceStart && unicode.IsUpper(runes[0])
}

func addClaimWords(set map[string]bool, text string) {
	for _, w := range claimWordRe.FindAllString(text, -1) {
		set[strings.ToLower(w)] = true
	}
}

// entryText joins every field of entries a letter may legitimately draw on.
func entryText(entries []masterdata.Entry) string {
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(strings.Join([]string{e.Employer, e.Client, e.ClientLabel, e.Title, e.Group, e.Role, e.Name, e.Location, e.Start, strings.Join(e.Tags, " "), strings.Join(e.Bullets, " ")}, " "))
		if e.End != nil {
			b.WriteString(" " + *e.End)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// selectedEntries are the Entries selection kept, each carrying its
// rewritten bullets besides its own: the letter is drafted alongside them
// and may repeat their wording.
func selectedEntries(selection SelectionResult, entries []masterdata.Entry) []masterdata.Entry {
	byID := make(map[string]masterdata.Entry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}
	var out []masterdata.Entry
	for _, se := range selection.Entries {
		e, ok := byID[se.EntryID]
		if !ok {
			continue
		}
		e.Bullets = append([]string(nil), e.Bullets...)
		for _, b := range se.Bullets {
			e.Bullets = append(e.Bullets, b.Rewritten)
		}
		out = append(out, e)
	}
	return out
}

// CheckSelectionCoverLetter is the Text Review check point's facade for the
// Cover Letter: it checks letter against the Entries selection kept, the
// Snippets named by snippetIDs (an unknown id is an error, as in Generate)
// and the profile under dataDir. selection must be traceable to Master Data,
// as for CheckSelectionGroundedness.
func CheckSelectionCoverLetter(dataDir string, selection SelectionResult, letter string, snippetIDs []string) ([]GroundednessFlag, error) {
	entries, err := masterdata.ListEntries(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading master data: %w", err)
	}
	if err := validateSelection(selection, entries); err != nil {
		return nil, err
	}
	snippets, err := masterdata.ListSnippets(dataDir)
	if err != nil {
		return nil, fmt.Errorf("loading cover letter snippets: %w", err)
	}
	if err := validateCoverLetter(CoverLetterResult{Body: letter, SourceSnippetIDs: snippetIDs}, snippets); err != nil {
		return nil, err
	}
	profile, err := loadProfile(dataDir)
	if err != nil {
		return nil, err
	}
	used := citedSnippets(CoverLetterResult{SourceSnippetIDs: snippetIDs}, snippets)
	return CheckCoverLetterGroundedness(letter, selectedEntries(selection, entries), used, profile), nil
}
