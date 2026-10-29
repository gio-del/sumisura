package masterdata

import (
	"sort"
	"strings"
	"unicode"
)

// MinGapListings is how many Job Listings must cite a term before it appears
// in the Gaps report.
const MinGapListings = 2

// GapListing is the part of a Job Listing the Gaps report reads: its id and
// Job Description. Archived Job Listings are filtered out by the caller.
type GapListing struct {
	ID             string
	JobDescription string
}

// Gap is one recurring term of the tracked Job Descriptions: how many Job
// Listings cite it, which ones, and whether any Entry's Tags or text carry it.
type Gap struct {
	Term       string
	ListingIDs []string
	Covered    bool
}

// ComputeGaps compares the recurring words and word pairs of the Job
// Descriptions with the Tags and text of every Entry. It is deterministic,
// local, and only reports: it never proposes text for the CV. Uncovered
// terms come first, then by number of Job Listings, then alphabetically.
func ComputeGaps(listings []GapListing, entries []Entry) []Gap {
	covered := map[string]bool{}
	for _, e := range entries {
		parts := []string{e.Employer, e.Client, e.ClientLabel, e.Title, e.Group, e.Role, e.Name}
		parts = append(parts, e.Tags...)
		parts = append(parts, e.Bullets...)
		for _, term := range gapTerms(strings.Join(parts, "\n")) {
			covered[term] = true
		}
		for _, tag := range e.Tags {
			var kept []string
			for _, w := range gapWords(tag) {
				if w != "" {
					kept = append(kept, w)
				}
			}
			if len(kept) > 0 {
				covered[strings.Join(kept, " ")] = true
			}
		}
	}

	cited := map[string][]string{}
	for _, l := range listings {
		for _, term := range gapTerms(l.JobDescription) {
			cited[term] = append(cited[term], l.ID)
		}
	}

	gaps := []Gap{}
	for term, ids := range cited {
		if len(ids) < MinGapListings {
			continue
		}
		sort.Strings(ids)
		gaps = append(gaps, Gap{Term: term, ListingIDs: ids, Covered: covered[term]})
	}
	sort.Slice(gaps, func(i, j int) bool {
		a, b := gaps[i], gaps[j]
		if a.Covered != b.Covered {
			return !a.Covered
		}
		if len(a.ListingIDs) != len(b.ListingIDs) {
			return len(a.ListingIDs) > len(b.ListingIDs)
		}
		return a.Term < b.Term
	})
	return gaps
}

// gapTerms returns the distinct words and adjacent word pairs of text, with
// stopwords dropped and case and spacing normalised.
func gapTerms(text string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(t string) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '.' || r == ';' || r == ',' || r == ':' || r == '(' || r == ')' }) {
		words := gapWords(line)
		for i, w := range words {
			if w == "" {
				continue
			}
			add(w)
			if i > 0 && words[i-1] != "" {
				add(words[i-1] + " " + w)
			}
		}
	}
	return out
}

// gapWords splits text into lowercase words, keeping a trailing + or # so
// C++ and C# stay whole. A stopword or one-letter word is returned as ""
// so it breaks word pairs: "design of systems" yields no pair.
func gapWords(text string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) == 0 {
			return
		}
		w := string(cur)
		cur = cur[:0]
		if len([]rune(w)) < 2 && w != "r" || gapStopwords[w] {
			words = append(words, "")
			return
		}
		words = append(words, w)
	}
	rs := []rune(strings.ToLower(text))
	for i, r := range rs {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			cur = append(cur, r)
		case (r == '+' || r == '#') && len(cur) > 0 && (i+1 == len(rs) || !unicode.IsLetter(rs[i+1])):
			cur = append(cur, r)
		default:
			flush()
		}
	}
	flush()
	return words
}

var gapStopwords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`
a an the and or of to in on for with by at as is are be been being was were it its this that these those from into
you your we our us they their will can may should must have has had do does done not no yes if than then so such
about over under per via within across including include etc able also more most other any all each both new
work working works team teams role roles job company experience experienced years year strong skills skill
looking join knowledge ability responsibilities requirements required plus good great
il lo la i gli le un uno una di del dello della dei degli delle da dal dallo dalla dai dagli dalle in nel nello nella nei negli nelle
su sul sullo sulla sui sugli sulle con per tra fra e ed o ma che chi cui non si sono è essere sarà come anche più
nostro nostra nostri nostre tuo tua tuoi tue suo sua suoi sue questo questa questi queste quello quella
lavoro lavorare team azienda ruolo esperienza anni anno competenze conoscenza capacità richiesta richiesto
`) {
		m[w] = true
	}
	return m
}()
