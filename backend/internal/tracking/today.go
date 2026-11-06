package tracking

import (
	"sort"
	"time"
)

// TodayInterviewWindow is how far ahead the Today view looks for interviews.
const TodayInterviewWindow = 7 * 24 * time.Hour

// TodayApplication is one row of the Today view's Follow-up and Ferme
// blocks: enough to name the Application and link to its Job Listing.
type TodayApplication struct {
	JobListingID    string `json:"jobListingId"`
	Title           string `json:"title,omitempty"`
	Company         string `json:"company"`
	Status          Status `json:"status"`
	FollowUpAt      string `json:"followUpAt,omitempty"`
	StatusUpdatedAt string `json:"statusUpdatedAt,omitempty"`
}

// TodayInterview is one upcoming interview of the Today view.
type TodayInterview struct {
	JobListingID string `json:"jobListingId"`
	Title        string `json:"title,omitempty"`
	Company      string `json:"company"`
	At           string `json:"at"`
	Note         string `json:"note,omitempty"`
	Location     string `json:"location,omitempty"`
}

// Today is the read-only "what needs action" view built from existing
// records. Slices are never nil so the JSON carries empty arrays.
type Today struct {
	FollowUps  []TodayApplication `json:"followUps"`
	Interviews []TodayInterview   `json:"interviews"`
	Stale      []TodayApplication `json:"stale"`
}

// BuildToday computes the Today view. Archived Job Listings and Applications
// in a final Status (Rejected, Withdrawn, Offer) never appear. An
// Application with a follow-up day is listed under FollowUps once that day
// is today or past; one without is listed under Stale when IsStale says so
// against threshold. Interviews are those from now up to
// TodayInterviewWindow ahead, soonest first.
func BuildToday(items []ListingWithApplication, now time.Time, threshold time.Duration) Today {
	result := Today{FollowUps: []TodayApplication{}, Interviews: []TodayInterview{}, Stale: []TodayApplication{}}
	y, m, d := now.Date()
	startOfDay := time.Date(y, m, d, 0, 0, 0, 0, now.Location())

	for _, item := range items {
		a := item.Application
		if item.JobListing.Archived {
			continue
		}
		switch a.Status {
		case StatusRejected, StatusWithdrawn, StatusOffer:
			continue
		}
		row := TodayApplication{
			JobListingID: item.JobListing.ID, Title: item.JobListing.Title, Company: item.JobListing.Company,
			Status: a.Status, FollowUpAt: a.FollowUpAt, StatusUpdatedAt: a.StatusUpdatedAt,
		}
		if due, err := time.ParseInLocation(FollowUpDateLayout, a.FollowUpAt, now.Location()); err == nil {
			if !due.After(startOfDay) {
				result.FollowUps = append(result.FollowUps, row)
			}
		} else if IsStale(a.Status, a.StatusUpdatedAt, now, threshold) {
			result.Stale = append(result.Stale, row)
		}

		for _, iv := range a.Interviews {
			at, err := time.ParseInLocation(InterviewAtLayout, iv.At, now.Location())
			if err != nil || at.Before(now) || at.After(now.Add(TodayInterviewWindow)) {
				continue
			}
			result.Interviews = append(result.Interviews, TodayInterview{
				JobListingID: item.JobListing.ID, Title: item.JobListing.Title, Company: item.JobListing.Company,
				At: iv.At, Note: iv.Note, Location: iv.Location,
			})
		}
	}

	sort.SliceStable(result.FollowUps, func(i, j int) bool { return result.FollowUps[i].FollowUpAt < result.FollowUps[j].FollowUpAt })
	sort.SliceStable(result.Stale, func(i, j int) bool { return result.Stale[i].StatusUpdatedAt < result.Stale[j].StatusUpdatedAt })
	sort.SliceStable(result.Interviews, func(i, j int) bool { return result.Interviews[i].At < result.Interviews[j].At })
	return result
}
