package tracking

import (
	"errors"

	"github.com/gio-del/sumisura/backend/internal/postingkey"
)

// ErrDuplicate marks a save refused because the posting it describes is
// already held by a Job Listing: one posting is exactly one Job Listing,
// with one Application, one Status and one history (issue #206). It lives
// in Save alongside ErrValidation, so every caller inherits it — the
// extension, the manual save form, the ATS browse save, Pending Capture
// completion, and anything added later. There is deliberately no
// force/override flag: no caller wants one.
//
// Distinct from FindLikelyDuplicate, which answers a different, fuzzier
// question ("a similar role, possibly spelled differently") and stays a
// non-blocking warning on the routes that return it today.
var ErrDuplicate = errors.New("duplicate posting")

// DuplicatePostingError carries the Job Listing that already holds the
// posting, so a caller can name it rather than just refusing. Matches
// errors.Is(err, ErrDuplicate) and is reached with errors.As.
type DuplicatePostingError struct {
	Existing JobListing
}

func (e *DuplicatePostingError) Error() string {
	return "duplicate posting: already saved as job listing " + e.Existing.ID
}

// Unwrap is what makes errors.Is(err, ErrDuplicate) true.
func (e *DuplicatePostingError) Unwrap() error { return ErrDuplicate }

// findByPostingKey returns the Job Listing whose URL points at the same
// posting as rawURL. ok is false when rawURL yields no Posting Key —
// absent, or not an absolute http(s) URL — in which case identity cannot
// be computed and is never a reason to refuse (story 10).
//
// Archived Job Listings participate: they hold their Posting Key like any
// other record, so archiving never silently becomes a way to duplicate one
// (story 6).
func findByPostingKey(dataDir, rawURL string) (listing JobListing, ok bool, err error) {
	key, keyed := postingkey.Of(rawURL)
	if !keyed {
		return JobListing{}, false, nil
	}
	listings, err := List(dataDir)
	if err != nil {
		return JobListing{}, false, err
	}
	for _, l := range listings {
		if other, keyed := postingkey.Of(l.JobListing.URL); keyed && other == key {
			return l.JobListing, true, nil
		}
	}
	return JobListing{}, false, nil
}

// FindByPostingKey returns the Job Listing holding the same posting as
// rawURL, paired with its Application. ok is false when rawURL yields no
// Posting Key, or when no Job Listing holds it. Archived listings are
// included: they hold their Posting Key like any other record.
//
// It is the read behind the extension's capture lookup — the same identity
// question Save answers by refusing (issue #206).
func FindByPostingKey(dataDir, rawURL string) (listing ListingWithApplication, ok bool, err error) {
	key, keyed := postingkey.Of(rawURL)
	if !keyed {
		return ListingWithApplication{}, false, nil
	}
	listings, err := List(dataDir)
	if err != nil {
		return ListingWithApplication{}, false, err
	}
	for _, l := range listings {
		if other, keyed := postingkey.Of(l.JobListing.URL); keyed && other == key {
			return l, true, nil
		}
	}
	return ListingWithApplication{}, false, nil
}

// AllowedTransitions is the list of Statuses an Application at `from` may
// move to, in the state machine's own order. Returned to the extension
// card so it offers only legal moves (issue #206, story 44) without a
// second definition of the machine — the backend still re-validates every
// move, so a stale card can never corrupt anything (story 45).
func AllowedTransitions(from Status) []Status {
	next := allowedTransitions[from]
	out := make([]Status, len(next))
	copy(out, next)
	return out
}
