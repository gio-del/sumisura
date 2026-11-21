package api

import (
	"net/http"
	"time"

	"github.com/gio-del/sumisura/backend/internal/atsboard"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// todayResponse is GET /api/today: the aggregation plus the "Da guardare"
// counts. Everything is read from stored records; no board is fetched.
type todayResponse struct {
	tracking.Today
	TrackedBoards   int `json:"trackedBoards"`
	PendingCaptures int `json:"pendingCaptures"`
	// NewListings is the number of listings found new at the boards' last
	// checks and not saved since; LastBoardCheck is the most recent check.
	// Both are absent when no check is stored.
	NewListings    *int       `json:"newListings,omitempty"`
	LastBoardCheck *time.Time `json:"lastBoardCheck,omitempty"`
}

func getTodayHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		boards, err := atsboard.ListTrackedBoards(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		captures, err := tracking.ListPendingCaptures(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		checks, err := atsboard.LastChecks(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		saved := map[string]bool{}
		for _, it := range items {
			if it.JobListing.URL != "" {
				saved[it.JobListing.URL] = true
			}
		}
		var newCount *int
		var last *time.Time
		for _, b := range boards {
			c, ok := checks[b.ID]
			if !ok {
				continue
			}
			if newCount == nil {
				newCount = new(int)
			}
			for _, u := range c.NewURLs {
				if !saved[u] {
					*newCount++
				}
			}
			if last == nil || c.CheckedAt.After(*last) {
				t := c.CheckedAt
				last = &t
			}
		}
		resp := todayResponse{
			Today:           tracking.BuildToday(items, time.Now(), tracking.DefaultStaleThreshold),
			TrackedBoards:   len(boards),
			PendingCaptures: len(captures),
			NewListings:     newCount,
			LastBoardCheck:  last,
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
