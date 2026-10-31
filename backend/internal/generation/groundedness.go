package generation

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// groundednessThreshold is the minimum fraction of a sentence's weighted
// tokens that must appear in its source(s) before it's left unflagged. Not
// user-configurable (PRD explicitly defers that) — a single hardcoded
// default, tuned against the fixtures in groundedness_test.go to accept
// heavy paraphrasing while still catching sentences with no plausible
// source at all.
const groundednessThreshold = 0.4

var (
	sentenceEndRe = regexp.MustCompile(`[.!?]+(?:\s+|$)`)
	wordRe        = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9'-]*`)
	numericRe     = regexp.MustCompile(`^\d`)
)

// stopwords are excluded from groundedness scoring entirely (weight 0):
// common function words that would otherwise inflate the overlap ratio
// regardless of whether the sentence's actual claims are grounded.
var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"by": true, "for": true, "from": true, "in": true, "into": true,
	"is": true, "it": true, "of": true, "on": true, "or": true, "that": true,
	"the": true, "this": true, "to": true, "was": true, "were": true,
	"which": true, "with": true,
}

// checkGroundedness splits generated into sentences and flags each one
// whose weighted token overlap with sources falls below
// groundednessThreshold, or which introduces a number/date/proper noun
// absent from every source (see groundSentence). Returns nil rather than
// erroring when it can't reach a verdict — an empty generated string or an
// empty/no-op sources list — since this check must never block a
// Generation (PRD story 8).
func checkGroundedness(sources []string, generated string) []GroundednessFlag {
	if strings.TrimSpace(generated) == "" {
		return nil
	}

	sourceTokens := sourceTokenSet(sources)
	if len(sourceTokens) == 0 {
		return nil
	}

	var flags []GroundednessFlag
	for _, sentence := range splitSentences(generated) {
		if flag, flagged := groundSentence(sourceTokens, sentence); flagged {
			flags = append(flags, flag)
		}
	}
	return flags
}

// computeGroundedness runs checkGroundedness over every rewritten bullet in
// selection (against its own source bullet) and over coverLetter's body
// (against the union of the Cover Letter Snippets it cites and every
// rewritten bullet in selection — the "Job Description-relevant Entries" it
// was drafted alongside, per PRD story 2).
func computeGroundedness(selection SelectionResult, coverLetter CoverLetterResult, snippets []masterdata.Snippet) GroundednessResult {
	var result GroundednessResult

	coverLetterSources := make([]string, 0, len(coverLetter.SourceSnippetIDs)+len(selection.Entries))
	snippetBodyByID := make(map[string]string, len(snippets))
	for _, s := range snippets {
		snippetBodyByID[s.ID] = s.Body
	}
	for _, id := range coverLetter.SourceSnippetIDs {
		if body, ok := snippetBodyByID[id]; ok {
			coverLetterSources = append(coverLetterSources, body)
		}
	}

	for _, se := range selection.Entries {
		for _, b := range se.Bullets {
			coverLetterSources = append(coverLetterSources, b.Rewritten)

			if flags := checkGroundedness([]string{b.Source}, b.Rewritten); len(flags) > 0 {
				result.Bullets = append(result.Bullets, BulletGroundedness{
					EntryID:     se.EntryID,
					SourceIndex: b.SourceIndex,
					Flags:       flags,
				})
			}
		}
	}

	result.CoverLetter = checkGroundedness(coverLetterSources, coverLetter.Body)
	return result
}

// weightedToken is one significant (non-stopword) token from a sentence
// being scored, weighted by how concerning it would be to see fabricated:
// numbers/dates highest, capitalized (proper-noun-shaped) words next,
// everything else at the baseline weight.
type weightedToken struct {
	lower   string
	weight  int
	numeric bool
}

func tokenize(text string) []weightedToken {
	words := wordRe.FindAllString(text, -1)
	tokens := make([]weightedToken, 0, len(words))
	for _, w := range words {
		lower := strings.ToLower(w)
		if stopwords[lower] {
			continue
		}
		numeric := numericRe.MatchString(w)
		weight := 1
		switch {
		case numeric:
			weight = 3
		case isProperNounShaped(w):
			weight = 2
		}
		tokens = append(tokens, weightedToken{lower: lower, weight: weight, numeric: numeric})
	}
	return tokens
}

func isProperNounShaped(w string) bool {
	r := []rune(w)
	return len(r) > 0 && unicode.IsUpper(r[0])
}

func sourceTokenSet(sources []string) map[string]bool {
	set := map[string]bool{}
	for _, src := range sources {
		for _, t := range tokenize(src) {
			set[t.lower] = true
		}
	}
	return set
}

// groundSentence scores sentence's weighted tokens against sourceTokens. A
// number/date token absent from every source always flags the sentence as
// ReasonNumericMismatch, regardless of overall overlap — a fabricated
// specific matters even inside an otherwise well-grounded sentence (PRD
// story 6). Otherwise, a sentence whose matched weight falls below
// groundednessThreshold flags as ReasonNoSourceMatch. A sentence with no
// significant (non-stopword) tokens at all can't be scored and is left
// unflagged, per the same degrade-gracefully rule as an empty source list.
func groundSentence(sourceTokens map[string]bool, sentence string) (GroundednessFlag, bool) {
	tokens := tokenize(sentence)
	if len(tokens) == 0 {
		return GroundednessFlag{}, false
	}

	var totalWeight, matchedWeight int
	numericUnmatched := false
	for _, t := range tokens {
		totalWeight += t.weight
		if sourceTokens[t.lower] {
			matchedWeight += t.weight
		} else if t.numeric {
			numericUnmatched = true
		}
	}
	if totalWeight == 0 {
		return GroundednessFlag{}, false
	}

	if numericUnmatched {
		return GroundednessFlag{Sentence: sentence, Reason: ReasonNumericMismatch}, true
	}
	if ratio := float64(matchedWeight) / float64(totalWeight); ratio < groundednessThreshold {
		return GroundednessFlag{Sentence: sentence, Reason: ReasonNoSourceMatch}, true
	}
	return GroundednessFlag{}, false
}

// splitSentences is a crude sentence splitter — good enough for the short,
// single-idea bullets and cover-letter sentences this check runs against,
// not a general NLP sentence boundary detector.
func splitSentences(text string) []string {
	var sentences []string
	start := 0
	for _, m := range sentenceEndRe.FindAllStringIndex(text, -1) {
		if s := strings.TrimSpace(text[start:m[1]]); s != "" {
			sentences = append(sentences, s)
		}
		start = m[1]
	}
	if rest := strings.TrimSpace(text[start:]); rest != "" {
		sentences = append(sentences, rest)
	}
	return sentences
}

// CheckGroundedness exposes checkGroundedness to callers outside the
// Generation pipeline, such as the Master Data import verifying a proposed
// bullet against the pasted CV text.
func CheckGroundedness(sources []string, generated string) []GroundednessFlag {
	return checkGroundedness(sources, generated)
}
