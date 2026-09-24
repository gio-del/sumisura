package masterdata

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// The two shapes an experience Entry can have (ADR-0045). A Client
// Engagement is one client's work at a consultancy employer, and carries
// its own role and span; an In-house Role is an employer that is itself the
// job. Which one an Entry is decides how a CV may state its dates.
const (
	KindConsultancy = "consultancy"
	KindInHouse     = "in-house"
)

// Shape resolves the Entry's shape, falling back to inference when the
// frontmatter does not record one: a named client means a Client
// Engagement, its absence an In-house Role.
//
// Inference is the compatibility path, not the intended one — Master Data
// written before `kind` existed keeps working — and it is exactly wrong for
// a Client Engagement whose client cannot be named, which is why the field
// exists (ADR-0045).
func (e Entry) Shape() string {
	switch e.Kind {
	case KindConsultancy, KindInHouse:
		return e.Kind
	}
	if e.Client != "" {
		return KindConsultancy
	}
	return KindInHouse
}

// Tenure is one unbroken span of employment at one employer — the union of
// the spans of the Entries making it up, derived and never stored. End is
// nil while any of that work is ongoing.
//
// Start and End are carried in the form Master Data wrote them ("2024-10",
// "2024"), so a caller renders the user's own dates rather than a
// normalized rewrite of them.
type Tenure struct {
	Employer string
	Start    string
	End      *string
}

// Ongoing reports whether the employment has no end yet.
func (t Tenure) Ongoing() bool { return t.End == nil }

// EmployerTenures returns every Tenure at employer, earliest first. An
// employer worked at twice with a real gap between yields two Tenures: a
// single span across the gap would claim continuous employment that did not
// happen, which is the failure this whole derivation exists to prevent.
//
// Contiguity is generous by one month, since Master Data routinely ends one
// engagement and starts the next in the same month or the one after.
func EmployerTenures(entries []Entry, employer string) []Tenure {
	var spans []Entry
	for _, e := range entries {
		if e.Type != TypeExperience || e.Employer != employer {
			continue
		}
		if _, ok := startMonth(e.Start); !ok {
			continue
		}
		spans = append(spans, e)
	}
	if len(spans) == 0 {
		return nil
	}

	sort.SliceStable(spans, func(i, j int) bool {
		a, _ := startMonth(spans[i].Start)
		b, _ := startMonth(spans[j].Start)
		return a < b
	})

	var tenures []Tenure
	var current Tenure
	var currentEnd int
	var currentOpen bool
	open := false

	flush := func() {
		if !open {
			return
		}
		tenures = append(tenures, current)
		open = false
	}

	for _, e := range spans {
		start, _ := startMonth(e.Start)
		end, endOpen := endMonth(e.End)

		if !open {
			current = Tenure{Employer: employer, Start: e.Start, End: cloneEnd(e.End)}
			currentEnd, currentOpen = end, endOpen
			open = true
			continue
		}

		// An ongoing stint swallows everything that starts after it; a
		// closed one only reaches one month past its end.
		if !currentOpen && start > currentEnd+1 {
			flush()
			current = Tenure{Employer: employer, Start: e.Start, End: cloneEnd(e.End)}
			currentEnd, currentOpen = end, endOpen
			open = true
			continue
		}

		switch {
		case currentOpen:
			// Already runs to the present; nothing can extend it further.
		case endOpen:
			current.End = nil
			currentOpen = true
		case end > currentEnd:
			current.End = cloneEnd(e.End)
			currentEnd = end
		}
	}
	flush()
	return tenures
}

// TenureOf returns the Tenure the given Entry falls inside, derived from
// every Entry of that employer in entries — not only the Selected ones, so
// leaving a Client Engagement off a CV narrows what the CV shows and never
// the employment it claims.
func TenureOf(entries []Entry, target Entry) (Tenure, bool) {
	if target.Type != TypeExperience || target.Employer == "" {
		return Tenure{}, false
	}
	start, ok := startMonth(target.Start)
	if !ok {
		return Tenure{}, false
	}
	for _, t := range EmployerTenures(entries, target.Employer) {
		tStart, _ := startMonth(t.Start)
		tEnd, tOpen := endMonth(t.End)
		if start >= tStart && (tOpen || start <= tEnd+1) {
			return t, true
		}
	}
	return Tenure{}, false
}

// SpanMonths returns how many months a span covers, counting ongoing work
// up to the current month. Reports false when the dates cannot be read.
func SpanMonths(start string, end *string) (int, bool) {
	from, ok := startMonth(start)
	if !ok {
		return 0, false
	}
	to, open := endMonth(end)
	if open {
		now := time.Now().UTC()
		to = int(now.Year())*12 + int(now.Month()) - 1
	}
	if to < from {
		return 0, false
	}
	return to - from + 1, true
}

// CompareStarts orders two Master Data start dates chronologically,
// returning -1 when a is the earlier. An unparseable date compares equal to
// everything, so a stable sort leaves malformed frontmatter where it was
// rather than moving it somewhere that looks deliberate.
func CompareStarts(a, b string) int {
	ai, aok := startMonth(a)
	bi, bok := startMonth(b)
	switch {
	case !aok || !bok:
		return 0
	case ai < bi:
		return -1
	case ai > bi:
		return 1
	}
	return 0
}

func cloneEnd(end *string) *string {
	if end == nil || strings.TrimSpace(*end) == "" {
		return nil
	}
	v := *end
	return &v
}

// startMonth maps "YYYY-MM" or "YYYY" to a comparable month index, a bare
// year counting from its January.
func startMonth(s string) (int, bool) { return monthIndex(s, 1) }

// endMonth maps an end date to a comparable month index, a bare year
// counting to its December. A nil or blank end is ongoing, reported by the
// second return value.
func endMonth(end *string) (int, bool) {
	if end == nil || strings.TrimSpace(*end) == "" {
		return 0, true
	}
	idx, ok := monthIndex(*end, 12)
	if !ok {
		return 0, true
	}
	return idx, false
}

func monthIndex(s string, defaultMonth int) (int, bool) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	month := defaultMonth
	if len(parts) > 1 {
		m, err := strconv.Atoi(parts[1])
		if err != nil || m < 1 || m > 12 {
			return 0, false
		}
		month = m
	}
	return year*12 + (month - 1), true
}
