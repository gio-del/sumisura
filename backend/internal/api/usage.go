package api

import (
	"net/http"

	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// usageResponse is GET /api/usage's body: the usage totals, unchanged and
// flattened into the top level, plus a completeness signal. It wraps
// generation.GenerationUsage rather than extending it because that struct
// is also persisted on every GenerationRecord, where the signal would be
// meaningless (issue #102).
type usageResponse struct {
	generation.GenerationUsage
	Incomplete       bool   `json:"incomplete,omitempty"`
	IncompleteReason string `json:"incompleteReason,omitempty"`
}

// getUsageHandler serves the running-total Claude API usage/cost across
// every Generation and every standalone (non-Generation) call this dataDir
// has recorded (issue #39: cost/usage visibility, story 11), flagged as
// incomplete when some of that usage is known to be missing (issue #102).
func getUsageHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		total, err := tracking.TotalUsage(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, usageResponse{
			GenerationUsage:  total.Usage,
			Incomplete:       total.IncompleteReason != "",
			IncompleteReason: total.IncompleteReason,
		})
	}
}
