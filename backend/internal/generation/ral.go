package generation

import (
	"regexp"
	"strconv"
	"strings"
)

// RALSource labels where a RAL Range came from, per CONTEXT.md's RAL Range
// entry: never let the FE mistake an estimate for a fact.
type RALSource string

const (
	RALSourceStated    RALSource = "stated"
	RALSourceEstimated RALSource = "estimated"
	RALSourceNA        RALSource = "n/a"
	// RALSourceUnresolved means resolution couldn't even be attempted
	// (client.EstimateRAL returned an error) — distinct from RALSourceNA,
	// which means research ran and genuinely found nothing.
	RALSourceUnresolved RALSource = "unresolved"
	// RALSourceConflict means the Job Description text and the listing's
	// own salary field both state a figure and the two ranges don't
	// overlap at all — see RALRange's DescriptionStated/ListingStated.
	RALSourceConflict RALSource = "conflict"
	// RALSourceManual means the user typed the figure in themselves —
	// learned in a conversation with a recruiter, say (issue #206, stories
	// 64-66). It is the most reliable source on the record and the only
	// one no inference produces: ParseStatedRAL never yields it, and
	// tracking.Resolve leaves it alone exactly as it leaves a stated one,
	// so a retry can never overwrite it. It exists so the user is never
	// shown their own figure as though Claude had estimated it.
	RALSourceManual RALSource = "manual"
)

// RALFigure is one source's own stated salary figure, surfaced verbatim
// (never auto-picked) as part of a RALSourceConflict RALRange.
type RALFigure struct {
	Min      int    `json:"min" yaml:"min"`
	Max      int    `json:"max" yaml:"max"`
	Currency string `json:"currency" yaml:"currency"`
}

// RALRange is the gross annual salary range for a Job Listing. Min/Max are
// nil when Source is RALSourceNA, RALSourceUnresolved, or RALSourceConflict
// — a Conflict has no auto-picked winner, only DescriptionStated/
// ListingStated, each source's own figure.
type RALRange struct {
	Min      *int      `json:"min,omitempty"`
	Max      *int      `json:"max,omitempty"`
	Currency string    `json:"currency,omitempty"`
	Source   RALSource `json:"source"`

	// DescriptionStated and ListingStated are populated only when Source
	// is RALSourceConflict, one figure per source (CONTEXT.md's RAL Range
	// entry).
	DescriptionStated *RALFigure `json:"descriptionStated,omitempty" yaml:"descriptionStated,omitempty"`
	ListingStated     *RALFigure `json:"listingStated,omitempty" yaml:"listingStated,omitempty"`
}

var (
	// ralKeywordRe matches a line naming pay explicitly, so a plain number
	// range (years of experience, a date range) isn't mistaken for salary.
	ralKeywordRe = regexp.MustCompile(`(?i)\b(RAL|salary|compensation|gross annual|annual gross|pay range)\b`)

	currencySymbolRe = regexp.MustCompile(`(€|\$|£)`)
	currencyCodeRe   = regexp.MustCompile(`(?i)\b(EUR|USD|GBP)\b`)

	// numberToken: digits with optional thousand separators, an optional
	// single fractional digit (fractional-K shorthand, e.g. "63,2k" means
	// 63,200 — distinct from the full grouped-digit form's exactly-3-digit
	// groups), optional "k" (thousands) suffix. currencyPrefix optionally
	// absorbs a currency symbol/code directly before a number (e.g.
	// "€45,000"), since it can appear before either or both numbers in a
	// range.
	numberToken   = `[\d]{1,3}(?:[.,]\d{3})*(?:[.,]\d)?\s*(?:k)?`
	currencyPrefx = `(?:€|\$|£|EUR|USD|GBP)?\s*`
	// rangeFillerRe: a per-unit label (e.g. "€ /yr") can sit between the
	// first number and the separator — LinkedIn's own salary-insight badge
	// reads "63,2K € /yr - 70,8K € /yr", not "63,2K - 70,8K € /yr". No
	// digit or dash character is allowed through, so this can't jump over
	// an unrelated second range on the same line.
	rangeFillerRe = `[^\d\-–—]{0,20}`
	rangeRe       = regexp.MustCompile(`(?i)` + currencyPrefx + `(` + numberToken + `)` + rangeFillerRe + `(?:-|–|—|to)\s*` + currencyPrefx + `(` + numberToken + `)`)
	singleRe      = regexp.MustCompile(`(?i)` + currencyPrefx + `(` + numberToken + `)`)

	// fractionalKRe matches a numberToken that used the fractional-K
	// shorthand ("63,2k" / "63.2k"), as opposed to a whole-thousand
	// shorthand ("40k") or a full grouped-digit figure ("63.000").
	fractionalKRe = regexp.MustCompile(`(?i)^(\d{1,3})[.,](\d)k$`)
)

// ParseStatedRAL looks for a salary figure or range stated directly in a
// Job Description's text, per the PRD's RAL Range lookup: "parse the Job
// Description text first". It reports ok=false if it can't find one
// confidently — a bare number range (years of experience, dates) is not
// treated as a salary without a currency symbol/code or a pay keyword
// (RAL, salary, compensation, ...) on the same line.
func ParseStatedRAL(jobDescription string) (RALRange, bool) {
	for _, line := range strings.Split(jobDescription, "\n") {
		currency := currencyFrom(line)
		if currency == "" && !ralKeywordRe.MatchString(line) {
			continue
		}
		if currency == "" {
			currency = "EUR" // RAL is an Italian-market term; default accordingly.
		}

		if min, max, ok := parseRange(line); ok {
			return RALRange{Min: &min, Max: &max, Currency: currency, Source: RALSourceStated}, true
		}
		if v, ok := parseSingle(line); ok {
			return RALRange{Min: &v, Max: &v, Currency: currency, Source: RALSourceStated}, true
		}
	}
	return RALRange{}, false
}

// parseRange looks for "<num> - <num>" in line. A "k" suffix on the second
// number implies the same unit for the first if the first looks like
// shorthand (e.g. "40-50k" means 40k-50k, not 40 and 50000).
func parseRange(line string) (min, max int, ok bool) {
	m := rangeRe.FindStringSubmatch(line)
	if m == nil {
		return 0, 0, false
	}
	v1, k1, ok1 := parseToken(m[1])
	v2, k2, ok2 := parseToken(m[2])
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	if k2 && !k1 && v1 < 1000 {
		v1 *= 1000
	}
	if v1 > v2 {
		v1, v2 = v2, v1
	}
	return v1, v2, true
}

func parseSingle(line string) (int, bool) {
	m := singleRe.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	v, _, ok := parseToken(m[1])
	return v, ok
}

// parseToken resolves one numberToken match into a whole-currency-unit
// integer, already scaled for whichever "k" shorthand it used (fractional-K
// like "63,2k" -> 63200, or whole-thousand like "40k" -> 40000) — or left
// unscaled for a full grouped-digit figure like "63.000" -> 63000. hasK
// reports whether a "k" suffix was present, so parseRange can still infer it
// on a first number lacking its own suffix (e.g. "40-50k").
func parseToken(token string) (value int, hasK bool, ok bool) {
	token = strings.TrimSpace(token)
	if m := fractionalKRe.FindStringSubmatch(token); m != nil {
		intPart, err1 := strconv.Atoi(m[1])
		fracDigit, err2 := strconv.Atoi(m[2])
		if err1 != nil || err2 != nil {
			return 0, false, false
		}
		return intPart*1000 + fracDigit*100, true, true
	}
	raw, k := splitK(token)
	v, ok := parseAmount(raw)
	if !ok {
		return 0, false, false
	}
	if k {
		v *= 1000
	}
	return v, k, true
}

func splitK(token string) (raw string, hasK bool) {
	token = strings.TrimSpace(token)
	if strings.HasSuffix(strings.ToLower(token), "k") {
		return strings.TrimSpace(token[:len(token)-1]), true
	}
	return token, false
}

func currencyFrom(line string) string {
	if currencySymbolRe.MatchString(line) {
		switch currencySymbolRe.FindString(line) {
		case "€":
			return "EUR"
		case "$":
			return "USD"
		case "£":
			return "GBP"
		}
	}
	if m := currencyCodeRe.FindString(line); m != "" {
		return strings.ToUpper(m)
	}
	return ""
}

// parseAmount turns a matched number token ("45,000", "90.000") into a
// whole-currency-unit integer. Any "k" suffix must already be stripped by
// the caller (see splitK) since it scales the parsed value, not the token.
func parseAmount(token string) (int, bool) {
	cleaned := strings.NewReplacer(",", "", ".", "").Replace(strings.TrimSpace(token))
	if cleaned == "" {
		return 0, false
	}
	v, err := strconv.Atoi(cleaned)
	if err != nil {
		return 0, false
	}
	return v, true
}
