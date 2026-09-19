// Package docsdrift compares what the code exposes to a self-hoster or an
// integrator — API routes, environment variables, CLI flags — with what the
// docs site says about them (issue #199). Its test collects the real names
// from the tree and fails on anything undocumented, anything documented
// that no longer exists, and any exemption without a reason: the same
// "golden list plus reasoned exemption" shape the API contract fixtures
// use for response types (ADR-0035).
package docsdrift

import (
	"fmt"
	"regexp"
	"sort"
)

// Surface is one kind of documented thing, and where its docs live.
type Surface struct {
	// Kind names the surface in problem messages, e.g. "route".
	Kind string
	// Page is the docs page every item must appear on, e.g.
	// "site/src/content/docs/api.md".
	Page string
	// Items are the names the code actually exposes.
	Items []string
	// Documented reports whether name appears in the page's text.
	Documented func(pageText, name string) bool
	// Mentioned lists the names the page documents, for the reverse
	// check; nil skips it for surfaces whose mentions can't be told apart
	// from ordinary prose.
	Mentioned func(pageText string) []string
	// Exempt maps an item that deliberately goes undocumented to why.
	Exempt map[string]string
}

// Check returns every drift problem between s's items and pageText, sorted
// so a failure reads the same on every run.
func Check(s Surface, pageText string) []string {
	var problems []string
	exists := map[string]bool{}
	for _, item := range s.Items {
		exists[item] = true
	}
	for item, reason := range s.Exempt {
		switch {
		case reason == "":
			problems = append(problems, fmt.Sprintf("%s %q is exempt without a reason: say why it stays out of %s", s.Kind, item, s.Page))
		case !exists[item]:
			problems = append(problems, fmt.Sprintf("%s %q is exempt but the code no longer has it: drop the exemption", s.Kind, item))
		}
	}
	for item := range exists {
		if _, exempt := s.Exempt[item]; exempt {
			continue
		}
		if !s.Documented(pageText, item) {
			problems = append(problems, fmt.Sprintf("%s %q is not documented: add it to %s, or exempt it with a reason", s.Kind, item, s.Page))
		}
	}
	if s.Mentioned != nil {
		for _, name := range s.Mentioned(pageText) {
			if !exists[name] {
				problems = append(problems, fmt.Sprintf("%s documents %s %q, which the code no longer has: remove it", s.Page, s.Kind, name))
			}
		}
	}
	sort.Strings(problems)
	return problems
}

// InlineCode reports whether name appears as an inline code span,
// `name`, which is how every page writes a route or variable it defines.
func InlineCode(pageText, name string) bool {
	return regexp.MustCompile("`" + regexp.QuoteMeta(name) + "`").MatchString(pageText)
}

// Flag reports whether a CLI flag appears as -name or --name, anywhere on
// the page (code blocks included), not followed by more of a longer flag.
func Flag(pageText, name string) bool {
	return regexp.MustCompile(`(^|[^\w-])--?` + regexp.QuoteMeta(name) + `($|[^\w-])`).MatchString(pageText)
}

var (
	routeMention  = regexp.MustCompile("`((?:GET|POST|PUT|PATCH|DELETE|OPTIONS) /api/[^`]*)`")
	envVarMention = regexp.MustCompile("`([A-Z][A-Z0-9]*_[A-Z0-9_]+)`")
)

// RouteMentions lists every `METHOD /api/...` inline code span on a page.
func RouteMentions(pageText string) []string {
	return submatches(routeMention, pageText)
}

// EnvVarMentions lists every `UPPER_SNAKE` inline code span on a page.
func EnvVarMentions(pageText string) []string {
	return submatches(envVarMention, pageText)
}

func submatches(re *regexp.Regexp, text string) []string {
	var names []string
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		names = append(names, m[1])
	}
	return names
}
