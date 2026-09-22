package tracking

import (
	"math"
	"strings"
	"time"
	"unicode"
)

// duplicateScoreThreshold is the minimum combined score (see scoreDuplicate)
// for FindLikelyDuplicate to surface a match. Chosen so that a same
// company+title pair still counts as a match with light formatting noise or
// a few minutes' saved-date drift, but a stale same-company/same-title
// re-hire a season or more later does not (see duplicate_test.go).
const duplicateScoreThreshold = 0.7

// duplicateDateDecayDays controls how fast saved-date proximity decays: two
// Job Listings saved this many days apart score roughly e^-1 (~0.37) on the
// date factor alone.
const duplicateDateDecayDays = 14.0

// DuplicateMatch is a likely cross-path duplicate of a Job Listing about to
// be saved, surfaced as a non-blocking warning (never a block, never an
// auto-merge — see the PRD's Implementation Decisions).
type DuplicateMatch struct {
	JobListingID string  `json:"jobListingId"`
	Company      string  `json:"company"`
	Title        string  `json:"title,omitempty"`
	SavedAt      string  `json:"savedAt"`
	Score        float64 `json:"score"`
}

// FindLikelyDuplicate compares candidate against existing (which may include
// candidate itself, e.g. when called with the freshly saved listing plus a
// full listing dump — candidate's own id is always excluded) and returns the
// highest-scoring listing that clears duplicateScoreThreshold, if any. It
// operates only on fields present on every JobListing regardless of Source
// (Company, Title, SavedAt) — JobDescription is intentionally not a
// matching signal, since its shape isn't comparable across sourcing paths.
func FindLikelyDuplicate(candidate JobListing, existing []JobListing) (DuplicateMatch, bool) {
	var best DuplicateMatch
	found := false
	for _, other := range existing {
		if other.ID == candidate.ID {
			continue
		}
		score := scoreDuplicate(candidate, other)
		if score >= duplicateScoreThreshold && (!found || score > best.Score) {
			best = DuplicateMatch{
				JobListingID: other.ID,
				Company:      other.Company,
				Title:        other.Title,
				SavedAt:      other.SavedAt,
				Score:        score,
			}
			found = true
		}
	}
	return best, found
}

// scoreDuplicate combines title similarity and company similarity
// (averaged), then discounts that average by how far apart the two Job
// Listings were saved — a decaying multiplier rather than an additive term,
// so two listings saved far apart never score as a match no matter how
// similar their title/company text is (a company re-hiring for the same
// title long after the first listing shouldn't be flagged).
func scoreDuplicate(a, b JobListing) float64 {
	titleScore := stringSimilarity(normalizeForMatch(a.Title), normalizeForMatch(b.Title))
	companyScore := stringSimilarity(normalizeCompanyForMatch(a.Company), normalizeCompanyForMatch(b.Company))
	textScore := 0.5*titleScore + 0.5*companyScore
	return textScore * savedAtProximity(a.SavedAt, b.SavedAt)
}

// savedAtProximity returns a weight in (0, 1] from the gap between two
// RFC3339 SavedAt timestamps: 1 for identical instants, decaying toward 0 as
// the gap grows. Unparseable timestamps (shouldn't happen for a persisted
// JobListing, but SavedAt is a plain string) contribute no signal.
func savedAtProximity(a, b string) float64 {
	ta, errA := time.Parse(time.RFC3339Nano, a)
	tb, errB := time.Parse(time.RFC3339Nano, b)
	if errA != nil || errB != nil {
		return 0
	}
	elapsed := ta.Sub(tb)
	if elapsed < 0 {
		elapsed = -elapsed
	}
	days := elapsed.Hours() / 24
	return math.Exp(-days / duplicateDateDecayDays)
}

// seniorityAliases folds common abbreviations to their full form so "Sr."
// and "Senior" (or "Jr."/"Junior") normalize identically (PRD story 5).
var seniorityAliases = map[string]string{
	"sr": "senior",
	"jr": "junior",
}

// normalizeForMatch lowercases s, replaces punctuation with spaces, and
// expands known seniority abbreviations, so trivial formatting differences
// don't hide a real match.
func normalizeForMatch(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	fields := strings.Fields(b.String())
	for i, f := range fields {
		if alias, ok := seniorityAliases[f]; ok {
			fields[i] = alias
		}
	}
	return strings.Join(fields, " ")
}

// companyLegalSuffixes are stripped from the end of a normalized company
// name so "Acme Inc." and "Acme" compare as equal (PRD story 7).
var companyLegalSuffixes = map[string]bool{
	"inc": true, "llc": true, "ltd": true, "corp": true, "co": true,
	"gmbh": true, "spa": true, "srl": true, "sa": true, "ag": true, "plc": true,
}

// maxDottedSuffixLetters bounds how many trailing single letters are
// joined looking for a dotted legal suffix. The longest in
// companyLegalSuffixes is four ("gmbh"), and a larger window would start
// eating real one-letter words off the end of a name.
const maxDottedSuffixLetters = 4

func normalizeCompanyForMatch(s string) string {
	fields := strings.Fields(normalizeForMatch(s))
	for len(fields) > 1 {
		if companyLegalSuffixes[fields[len(fields)-1]] {
			fields = fields[:len(fields)-1]
			continue
		}
		// A dotted legal form ("Acme S.p.A.", "Acme S.r.l.") arrives here
		// as trailing single letters, because normalizeForMatch turned
		// every period into a space. Join them back up and check whether
		// they spell a suffix — so the dotted and undotted spellings of
		// one company normalize the same way.
		if joined := trailingLetterRun(fields); joined > 0 {
			fields = fields[:len(fields)-joined]
			continue
		}
		break
	}
	return strings.Join(fields, " ")
}

// trailingLetterRun returns how many trailing single-letter fields spell a
// known legal suffix when joined, or 0 when none do. It prefers the
// longest run, so "s r l" is read as "srl" rather than leaving "s r".
// It never consumes every field: a company actually named "SPA" keeps its
// name.
func trailingLetterRun(fields []string) int {
	longest := maxDottedSuffixLetters
	if max := len(fields) - 1; max < longest {
		longest = max
	}
	for n := longest; n >= 2; n-- {
		run := fields[len(fields)-n:]
		var b strings.Builder
		singles := true
		for _, f := range run {
			if len(f) != 1 {
				singles = false
				break
			}
			b.WriteString(f)
		}
		if singles && companyLegalSuffixes[b.String()] {
			return n
		}
	}
	return 0
}

// stringSimilarity is the Sørensen-Dice coefficient over character bigrams:
// simple, dependency-free, and tolerant of the punctuation/word-order noise
// Job Titles and Company names pick up across sourcing paths.
func stringSimilarity(a, b string) float64 {
	if a == b {
		return 1
	}
	ag, bg := bigramCounts(a), bigramCounts(b)
	if len(ag) == 0 || len(bg) == 0 {
		return 0
	}
	var overlap, total int
	for bigram, countA := range ag {
		if countB, ok := bg[bigram]; ok {
			if countA < countB {
				overlap += countA
			} else {
				overlap += countB
			}
		}
		total += countA
	}
	for _, countB := range bg {
		total += countB
	}
	return 2 * float64(overlap) / float64(total)
}

func bigramCounts(s string) map[string]int {
	runes := []rune(s)
	counts := map[string]int{}
	if len(runes) < 2 {
		return counts
	}
	for i := 0; i < len(runes)-1; i++ {
		counts[string(runes[i:i+2])]++
	}
	return counts
}

// SameCompany reports whether two company names are the same company,
// ignoring the formatting and legal-suffix noise they pick up across
// sourcing paths: "Acme Inc.", "ACME, Inc" and "Acme S.p.A." are one
// company (issue #206, story 21). It is the exact normalization
// scoreDuplicate already uses for the company half of its score, exposed
// so the same-company warning and the fuzzy duplicate score can never
// disagree about what counts as one company.
//
// Two names that normalize to nothing (punctuation only, or empty) are
// never the same company — otherwise every unnamed company would collide.
func SameCompany(a, b string) bool {
	normalized := normalizeCompanyForMatch(a)
	return normalized != "" && normalized == normalizeCompanyForMatch(b)
}
