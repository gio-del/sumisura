package tracking

import (
	"fmt"
	"sort"
)

// Priority is how much the user wants to apply to a Job Listing (issue
// #280). Empty means not decided.
type Priority string

const (
	PriorityNone   Priority = ""
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// ParsePriority validates a priority value; empty is valid and means none.
func ParsePriority(s string) (Priority, error) {
	switch p := Priority(s); p {
	case PriorityNone, PriorityHigh, PriorityMedium, PriorityLow:
		return p, nil
	}
	return "", fmt.Errorf("%w: invalid priority %q (want high, medium, low or empty)", ErrValidation, s)
}

// rank orders priorities high first, undecided last.
func (p Priority) rank() int {
	switch p {
	case PriorityHigh:
		return 0
	case PriorityMedium:
		return 1
	case PriorityLow:
		return 2
	}
	return 3
}

// SortListingsByPriority returns a copy of items with higher priorities
// first and undecided last. The sort is stable, so the incoming order holds
// within a priority.
func SortListingsByPriority(items []ListingWithApplication) []ListingWithApplication {
	out := make([]ListingWithApplication, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].JobListing.Priority.rank() < out[j].JobListing.Priority.rank()
	})
	return out
}
