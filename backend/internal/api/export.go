package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func exportDataHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var scope tracking.ExportScope
		switch r.URL.Query().Get("scope") {
		case "":
		case string(tracking.ScopeTracking):
			scope = tracking.ScopeTracking
		default:
			http.Error(w, `scope must be "tracking" or omitted`, http.StatusBadRequest)
			return
		}
		filename := fmt.Sprintf("sumisura-export-%s.zip", time.Now().UTC().Format("20060102T150405Z"))

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		if err := tracking.ExportData(dataDir, w, scope); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
