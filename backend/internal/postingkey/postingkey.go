// Package postingkey turns the many URLs one job posting can be reached by
// into one stable key, so a link shared from a phone, a Pending Capture, an
// extension capture and a saved Job Listing can be recognised as the same
// posting (issue #182). It is pure: no network, no disk.
package postingkey

import (
	"net/url"
	"regexp"
	"strings"
)

// Provider is where a posting lives, as far as the URL tells.
type Provider string

const (
	LinkedIn   Provider = "linkedin"
	Indeed     Provider = "indeed"
	Greenhouse Provider = "greenhouse"
	Lever      Provider = "lever"
	Ashby      Provider = "ashby"
	Other      Provider = "other"
)

// Key identifies one posting. Board is the ATS board slug (empty for
// LinkedIn, Indeed and Other); ID is the provider's own posting id, or for
// Other the URL reduced to host and path.
type Key struct {
	Provider Provider
	Board    string
	ID       string
}

// String is the key's stable text form, e.g. "linkedin:4012345678" or
// "greenhouse:acme/4567890".
func (k Key) String() string {
	if k.Board != "" {
		return string(k.Provider) + ":" + k.Board + "/" + k.ID
	}
	return string(k.Provider) + ":" + k.ID
}

var (
	urlInText       = regexp.MustCompile(`https?://[^\s<>"']+`)
	trailingDigits  = regexp.MustCompile(`(\d{6,})$`)
	uuidLike        = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	greenhouseJobID = regexp.MustCompile(`^\d+$`)
)

// ExtractURL returns the first http(s) URL in s — a bare URL, or share-sheet
// text with one inside ("Check out this job at Acme: https://…") — with
// trailing punctuation a sentence would add removed. Empty when there is
// none.
func ExtractURL(s string) string {
	match := urlInText.FindString(s)
	return strings.TrimRight(match, ".,;:!?)]}")
}

// Of computes the Key for rawURL. ok is false when rawURL is not an
// absolute http(s) URL.
func Of(rawURL string) (key Key, ok bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Key{}, false
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	segments := pathSegments(u.Path)

	switch {
	case host == "linkedin.com" || strings.HasSuffix(host, ".linkedin.com"):
		if id := u.Query().Get("currentJobId"); id != "" {
			return Key{Provider: LinkedIn, ID: id}, true
		}
		for i, s := range segments {
			if s == "view" && i > 0 && segments[i-1] == "jobs" && i+1 < len(segments) {
				// /jobs/view/4012345678/ or the slugged form
				// /jobs/view/backend-engineer-at-acme-4012345678/
				if m := trailingDigits.FindString(segments[i+1]); m != "" {
					return Key{Provider: LinkedIn, ID: m}, true
				}
			}
		}
	case isIndeedHost(host):
		if jk := u.Query().Get("jk"); jk != "" {
			return Key{Provider: Indeed, ID: strings.ToLower(jk)}, true
		}
		if vjk := u.Query().Get("vjk"); vjk != "" {
			return Key{Provider: Indeed, ID: strings.ToLower(vjk)}, true
		}
	case host == "greenhouse.io" || strings.HasSuffix(host, ".greenhouse.io"):
		// boards.greenhouse.io/<board>/jobs/<id>, job-boards(.eu).greenhouse.io/<board>/jobs/<id>
		if len(segments) >= 3 && segments[1] == "jobs" && greenhouseJobID.MatchString(segments[2]) {
			return Key{Provider: Greenhouse, Board: strings.ToLower(segments[0]), ID: segments[2]}, true
		}
	case host == "jobs.lever.co" || host == "jobs.eu.lever.co":
		// jobs.lever.co/<company>/<uuid>[/apply]
		if len(segments) >= 2 && uuidLike.MatchString(segments[1]) {
			return Key{Provider: Lever, Board: strings.ToLower(segments[0]), ID: strings.ToLower(segments[1])}, true
		}
	case host == "jobs.ashbyhq.com":
		// jobs.ashbyhq.com/<board>/<uuid>[/application]
		if len(segments) >= 2 && uuidLike.MatchString(segments[1]) {
			return Key{Provider: Ashby, Board: strings.ToLower(segments[0]), ID: strings.ToLower(segments[1])}, true
		}
	}

	return Key{Provider: Other, ID: host + "/" + strings.Join(segments, "/")}, true
}

// Same reports whether two URLs point at the same posting. Two URLs that
// are not both valid are never the same.
func Same(a, b string) bool {
	ka, okA := Of(a)
	kb, okB := Of(b)
	return okA && okB && ka == kb
}

func isIndeedHost(host string) bool {
	// indeed.com, it.indeed.com, uk.indeed.com, indeed.co.uk, indeed.de, …
	labels := strings.Split(host, ".")
	for _, l := range labels {
		if l == "indeed" {
			return true
		}
	}
	return false
}

func pathSegments(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CanonicalURL is the clean link to keep for rawURL: for LinkedIn and
// Indeed the same tracking-free form the browser extension saves
// (…/jobs/view/<id>/, <origin>/viewjob?jk=<key>), so a Job Listing
// completed from a shared link reads like one captured on the desktop.
// Any other link is returned unchanged.
func CanonicalURL(rawURL string) string {
	key, ok := Of(rawURL)
	if !ok {
		return rawURL
	}
	switch key.Provider {
	case LinkedIn:
		return "https://www.linkedin.com/jobs/view/" + key.ID + "/"
	case Indeed:
		u, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil {
			return rawURL // unreachable: Of already parsed it
		}
		host := strings.ToLower(u.Hostname())
		if strings.HasPrefix(host, "m.") {
			host = "www." + strings.TrimPrefix(host, "m.")
		}
		return "https://" + host + "/viewjob?jk=" + key.ID
	}
	return rawURL
}
