package atsboard

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
	"gopkg.in/yaml.v3"
)

const seenStateFile = "tracked-boards-seen.yaml"

// ListingWithNew pairs a fetched Listing with whether it's new since the
// board's last check (stories 1-3), independent of whether it's already
// been saved — composes with (does not replace) ListingWithSaved.
type ListingWithNew struct {
	Listing
	New bool `json:"new"`
}

// MarkNewSinceLastCheck flags each listing not present in seenURLs as new.
// A nil/empty seenURLs marks everything new, matching the "no seen-state
// yet" case for a board that's never been fetched — callers seed a
// board's first-ever fetch with its own listing URLs (story 5) rather
// than an empty seenURLs, so the backlog isn't reported as new.
func MarkNewSinceLastCheck(listings []Listing, seenURLs []string) []ListingWithNew {
	seen := make(map[string]bool, len(seenURLs))
	for _, u := range seenURLs {
		seen[u] = true
	}

	result := make([]ListingWithNew, len(listings))
	for i, l := range listings {
		result[i] = ListingWithNew{Listing: l, New: !seen[l.URL]}
	}
	return result
}

// ListingDigest combines the AlreadySaved and New annotations onto one
// Listing (story 7: the two signals are independent, so a Listing can be
// new-and-unsaved, old-and-unsaved, or already-saved).
type ListingDigest struct {
	Listing
	AlreadySaved bool `json:"alreadySaved"`
	New          bool `json:"new"`
}

// CombineDigest zips MarkAlreadySaved's and MarkNewSinceLastCheck's output
// into ListingDigest, assuming both were computed from the same listings
// slice in the same order (so they're the same length, index-aligned).
func CombineDigest(withSaved []ListingWithSaved, withNew []ListingWithNew) []ListingDigest {
	result := make([]ListingDigest, len(withSaved))
	for i := range withSaved {
		result[i] = ListingDigest{
			Listing:      withSaved[i].Listing,
			AlreadySaved: withSaved[i].AlreadySaved,
			New:          withNew[i].New,
		}
	}
	return result
}

type seenRecord struct {
	BoardID string   `yaml:"boardId"`
	URLs    []string `yaml:"urls"`
	// CheckedAt and Present describe the last check of the board (the last
	// time its listings were fetched for the user). Present holds the URLs of
	// the listings on the board then, and NewURLs those not seen before it.
	// Absent in older state: such a board has no stored check.
	CheckedAt time.Time `yaml:"checkedAt,omitempty"`
	Present   []string  `yaml:"present,omitempty"`
	NewURLs   []string  `yaml:"newUrls,omitempty"`
}

// BoardCheck is the stored result of a board's last check.
type BoardCheck struct {
	CheckedAt time.Time
	Present   []string
	NewURLs   []string
}

// RecordCheck stores, for boardID, when it was checked, which listings were
// present and which of them were new. It leaves the seen-set untouched.
func RecordCheck(dataDir, boardID string, at time.Time, present, newURLs []string) error {
	records, err := readSeenState(dataDir)
	if err != nil {
		return err
	}
	for i, rec := range records {
		if rec.BoardID == boardID {
			records[i].CheckedAt, records[i].Present, records[i].NewURLs = at, present, newURLs
			return writeSeenState(dataDir, records)
		}
	}
	records = append(records, seenRecord{BoardID: boardID, CheckedAt: at, Present: present, NewURLs: newURLs})
	return writeSeenState(dataDir, records)
}

// LastChecks returns the stored last check of every board that has one.
func LastChecks(dataDir string) (map[string]BoardCheck, error) {
	records, err := readSeenState(dataDir)
	if err != nil {
		return nil, err
	}
	out := map[string]BoardCheck{}
	for _, rec := range records {
		if !rec.CheckedAt.IsZero() {
			out[rec.BoardID] = BoardCheck{CheckedAt: rec.CheckedAt, Present: rec.Present, NewURLs: rec.NewURLs}
		}
	}
	return out, nil
}

// SeenURLs returns boardID's persisted seen-URL set. existed is false when
// the board has no seen-state at all yet (never fetched, or its state was
// removed) — distinct from a board that has been fetched but had zero
// listings, which would return existed=true with an empty slice.
func SeenURLs(dataDir, boardID string) (urls []string, existed bool, err error) {
	records, err := readSeenState(dataDir)
	if err != nil {
		return nil, false, err
	}
	for _, rec := range records {
		if rec.BoardID == boardID {
			return rec.URLs, true, nil
		}
	}
	return nil, false, nil
}

// RecordSeen folds newURLs into boardID's persisted seen-set (union, never
// shrinks), creating the board's seen-state if it doesn't exist yet.
func RecordSeen(dataDir, boardID string, newURLs []string) error {
	records, err := readSeenState(dataDir)
	if err != nil {
		return err
	}

	for i, rec := range records {
		if rec.BoardID == boardID {
			records[i].URLs = unionStrings(rec.URLs, newURLs)
			return writeSeenState(dataDir, records)
		}
	}

	records = append(records, seenRecord{BoardID: boardID, URLs: unionStrings(nil, newURLs)})
	return writeSeenState(dataDir, records)
}

// RemoveSeenState deletes boardID's seen-state entirely (story 6), if
// present — a no-op, not an error, for a board with none.
func RemoveSeenState(dataDir, boardID string) error {
	records, err := readSeenState(dataDir)
	if err != nil {
		return err
	}

	kept := records[:0]
	for _, rec := range records {
		if rec.BoardID != boardID {
			kept = append(kept, rec)
		}
	}
	return writeSeenState(dataDir, kept)
}

func readSeenState(dataDir string) ([]seenRecord, error) {
	content, err := os.ReadFile(filepath.Join(dataDir, seenStateFile))
	if os.IsNotExist(err) {
		return []seenRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	var records []seenRecord
	if err := yaml.Unmarshal(content, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func writeSeenState(dataDir string, records []seenRecord) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	content, err := yaml.Marshal(records)
	if err != nil {
		return fmt.Errorf("marshaling tracked board seen-state: %w", err)
	}
	return atomicfile.WriteFile(filepath.Join(dataDir, seenStateFile), content, 0o644)
}

func unionStrings(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	result := make([]string, 0, len(a)+len(b))
	for _, s := range append(append([]string{}, a...), b...) {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
