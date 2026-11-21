package api

import (
	"net/http"

	"github.com/gio-del/sumisura/backend/internal/setupcheck"
)

// setupCheckHandler serves GET /api/setup-check: a read-only report on
// whether this installation is usable. No network, no Claude call, and the
// API key is only ever reported as set or not.
func setupCheckHandler(env setupcheck.Env) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, setupcheck.Check(env))
	}
}
