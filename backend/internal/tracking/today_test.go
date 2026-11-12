package tracking

import (
	"testing"
	"time"
)

func todayItem(id string, status Status, mod func(*ListingWithApplication)) ListingWithApplication {
	item := ListingWithApplication{
		JobListing:  JobListing{ID: id, Company: "Co " + id},
		Application: Application{JobListingID: id, Status: status, StatusUpdatedAt: "2026-11-01T09:00:00Z"},
	}
	if mod != nil {
		mod(&item)
	}
	return item
}

func TestBuildToday(t *testing.T) {
	now := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	old := "2026-09-01T09:00:00Z"
	items := []ListingWithApplication{
		todayItem("past", StatusSent, func(i *ListingWithApplication) { i.Application.FollowUpAt = "2026-11-03" }),
		todayItem("today", StatusSent, func(i *ListingWithApplication) { i.Application.FollowUpAt = "2026-11-06" }),
		todayItem("future", StatusSent, func(i *ListingWithApplication) { i.Application.FollowUpAt = "2026-11-07" }),
		todayItem("rej", StatusRejected, func(i *ListingWithApplication) {
			i.Application.FollowUpAt = "2026-11-01"
			i.Application.StatusUpdatedAt = old
		}),
		todayItem("wd", StatusWithdrawn, func(i *ListingWithApplication) { i.Application.StatusUpdatedAt = old }),
		todayItem("idle", StatusSent, func(i *ListingWithApplication) { i.Application.StatusUpdatedAt = old }),
		todayItem("archived", StatusSent, func(i *ListingWithApplication) {
			i.Application.StatusUpdatedAt = old
			i.JobListing.Archived = true
		}),
		todayItem("ivs", StatusInterviewing, func(i *ListingWithApplication) {
			i.Application.Interviews = []Interview{
				{At: "2026-11-05T10:00"}, {At: "2026-11-09T15:00"}, {At: "2026-11-16T10:00"},
			}
		}),
	}
	got := BuildToday(items, now, DefaultStaleThreshold)

	ids := func(rows []TodayApplication) []string {
		var out []string
		for _, r := range rows {
			out = append(out, r.JobListingID)
		}
		return out
	}
	if g := ids(got.FollowUps); len(g) != 2 || g[0] != "past" || g[1] != "today" {
		t.Errorf("follow-ups = %v, want [past today]", g)
	}
	if g := ids(got.Stale); len(g) != 1 || g[0] != "idle" {
		t.Errorf("stale = %v, want [idle]", g)
	}
	if len(got.Interviews) != 1 || got.Interviews[0].At != "2026-11-09T15:00" {
		t.Errorf("interviews = %+v, want only the one in 3 days", got.Interviews)
	}
}

func TestBuildToday_EmptyHasNonNilSlices(t *testing.T) {
	got := BuildToday(nil, time.Now(), DefaultStaleThreshold)
	if got.FollowUps == nil || got.Interviews == nil || got.Stale == nil {
		t.Fatalf("slices must be non-nil: %+v", got)
	}
}
