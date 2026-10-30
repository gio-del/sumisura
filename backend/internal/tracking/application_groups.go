package tracking

import (
	"sort"
	"time"
)

// applicationGroupOrder is the order the Applications view (issue #95)
// presents its Status groups in: the pipeline, active Statuses first and the
// terminal ones last. It is a presentation order and deliberately a separate
// declaration from stats.go's allStatuses/stageRank, which serve conversion
// math (Rejected and Offer share a rank there, and Withdrawn is absent).
var applicationGroupOrder = []Status{
	StatusSaved,
	StatusTailoring,
	StatusSent,
	StatusInterviewing,
	StatusOffer,
	StatusRejected,
	StatusWithdrawn,
}

// ApplicationGroup is every Application currently in Status, ordered so the
// one most overdue for attention comes first.
type ApplicationGroup struct {
	Status Status                   `json:"status"`
	Count  int                      `json:"count"`
	Items  []ListingWithApplication `json:"items"`
}

// ApplicationGroups is the Applications view's data: one group per Status in
// applicationGroupOrder, empty groups included, so a reader never has to
// know the Status vocabulary to fill in a missing one.
type ApplicationGroups struct {
	Total  int                `json:"total"`
	Groups []ApplicationGroup `json:"groups"`
}

// GroupApplications groups already-loaded Job Listing + Application pairs
// under their Status, mirroring ComputeStats: no storage or HTTP dependency,
// and now and threshold injected so staleness is deterministic.
//
// Within a group, stale Applications come first, then least-recently-changed
// first. Staleness is IsStale, recomputed against now and threshold and
// written back onto each item's Application so the flag a row shows always
// agrees with where it was sorted. An absent or unparseable StatusUpdatedAt
// sorts to the end of its group, never the front; ties keep the input order.
// An Application whose Status is not one of the known Statuses belongs to no
// group and is left out.
func GroupApplications(items []ListingWithApplication, now time.Time, threshold time.Duration) ApplicationGroups {
	byStatus := make(map[Status][]ListingWithApplication, len(applicationGroupOrder))
	for _, item := range items {
		item.Application.IsStale = IsStaleWithFollowUp(item.Application.Status, item.Application.StatusUpdatedAt, item.Application.FollowUpAt, now, threshold)
		byStatus[item.Application.Status] = append(byStatus[item.Application.Status], item)
	}

	result := ApplicationGroups{Groups: make([]ApplicationGroup, 0, len(applicationGroupOrder))}
	for _, status := range applicationGroupOrder {
		groupItems := byStatus[status]
		if groupItems == nil {
			groupItems = []ListingWithApplication{}
		}
		sortByAttention(groupItems)
		result.Groups = append(result.Groups, ApplicationGroup{Status: status, Count: len(groupItems), Items: groupItems})
		result.Total += len(groupItems)
	}
	return result
}

// sortByAttention orders one group in place: stale first, then oldest
// parseable StatusUpdatedAt first, then records with no usable timestamp.
func sortByAttention(items []ListingWithApplication) {
	changedAt := make(map[string]time.Time, len(items))
	for _, item := range items {
		if t, err := time.Parse(time.RFC3339Nano, item.Application.StatusUpdatedAt); err == nil {
			changedAt[item.Application.ID] = t
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].Application, items[j].Application
		if a.IsStale != b.IsStale {
			return a.IsStale
		}
		aAt, aKnown := changedAt[a.ID]
		bAt, bKnown := changedAt[b.ID]
		if aKnown != bKnown {
			return aKnown
		}
		return aKnown && aAt.Before(bAt)
	})
}
