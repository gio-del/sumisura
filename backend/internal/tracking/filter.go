package tracking

import (
	"strings"
	"time"
)

// FilterParams narrows a List result down for the Job Listings list view
// (issue #45). Every field is optional and independent — the zero value
// matches everything, so FilterListings(items, FilterParams{}) is a no-op.
type FilterParams struct {
	Status  Status
	Company string
	// Location matches a Job Listing's Location as a case-insensitive
	// substring, exactly as Company does (issue #206, story 58). Location
	// is free text as the board wrote it, so no attempt is made to
	// reconcile "Milan" with "Milano" — a substring is the honest match
	// for an unnormalized field. Empty matches everything, keeping the
	// zero value's "filters nothing" contract.
	Location  string
	SavedFrom *time.Time
	SavedTo   *time.Time
	// Archived selects which Job Listings to keep by their Archived flag
	// (issue #98). Its zero value is ArchivedAll, preserving the "zero value
	// matches everything" contract above; the API handler is what defaults
	// an absent query parameter to ArchivedExclude.
	Archived ArchivedView
}

// ArchivedView is which Job Listings a list view shows by their Archived
// flag (issue #98).
type ArchivedView string

const (
	// ArchivedAll keeps archived and non-archived Job Listings alike. It is
	// the zero value, so an empty FilterParams still filters nothing.
	ArchivedAll ArchivedView = ""
	// ArchivedExclude keeps only non-archived Job Listings — the Job
	// Listings list's default view.
	ArchivedExclude ArchivedView = "exclude"
	// ArchivedOnly keeps only archived Job Listings.
	ArchivedOnly ArchivedView = "only"
)

// FilterListings narrows items down to those matching every non-zero field
// of params. It's a pure, in-memory filter over an already-loaded List
// result — no new storage query mechanism, per ADR-0008's flat-file model.
func FilterListings(items []ListingWithApplication, params FilterParams) []ListingWithApplication {
	var result []ListingWithApplication
	for _, item := range items {
		if params.Archived == ArchivedExclude && item.JobListing.Archived {
			continue
		}
		if params.Archived == ArchivedOnly && !item.JobListing.Archived {
			continue
		}
		if params.Status != "" && item.Application.Status != params.Status {
			continue
		}
		if params.Company != "" && !strings.Contains(
			strings.ToLower(item.JobListing.Company),
			strings.ToLower(params.Company),
		) {
			continue
		}
		if params.Location != "" && !strings.Contains(
			strings.ToLower(item.JobListing.Location),
			strings.ToLower(params.Location),
		) {
			continue
		}
		if params.SavedFrom != nil || params.SavedTo != nil {
			savedAt, err := time.Parse(time.RFC3339Nano, item.JobListing.SavedAt)
			if err != nil {
				continue
			}
			if params.SavedFrom != nil && savedAt.Before(*params.SavedFrom) {
				continue
			}
			if params.SavedTo != nil && savedAt.After(endOfDay(*params.SavedTo)) {
				continue
			}
		}
		result = append(result, item)
	}
	return result
}

// endOfDay makes a date-only SavedTo bound inclusive of the whole day.
func endOfDay(d time.Time) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 999999999, d.Location())
}
