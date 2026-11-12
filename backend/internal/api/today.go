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
		writeJSON(w, http.StatusOK, todayResponse{
			Today:           tracking.BuildToday(items, time.Now(), tracking.DefaultStaleThreshold),
			TrackedBoards:   len(boards),
			PendingCaptures: len(captures),
		})
	}
}
