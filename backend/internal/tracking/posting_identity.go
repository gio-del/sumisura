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
