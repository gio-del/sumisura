package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// lanAuthHeader is the custom header LAN-reachable mode requires on every
// /api/* request once a shared-secret token is configured (see the
// LAN-reachable mode PRD, issue #57). Unused in default (localhost-only)
// mode.
const lanAuthHeader = "X-Sumisura-Token"

// checkLANToken is the pure token-check at LAN mode's auth gate: given the
// configured shared secret and the token an incoming request presented, it
// reports whether the request should be allowed through. An empty
// configuredToken means LAN mode's auth gate is off (default/localhost-only
// mode) and every request is allowed — the check must stay a no-op there so
// local usage never requires configuring a secret that doesn't apply to it.
// Comparison is constant-time so a mismatched token doesn't leak timing
// information about how much of it is correct.
func checkLANToken(configuredToken, presentedToken string) bool {
	if configuredToken == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(configuredToken), []byte(presentedToken)) == 1
}

// requireLANToken wraps next with LAN mode's auth gate: a request whose
// X-Sumisura-Token header doesn't match lanAuthToken gets 401 instead of
// reaching next. Callers only wrap with this when lanAuthToken is
// non-empty (see RouterConfig.LANAuthToken) — default mode never wraps a
// handler with it at all.
func requireLANToken(lanAuthToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The gate is scoped to /api/: the frontend build is the app's own
		// HTML, CSS and JS, identical for every install and holding none of
		// the user's data, and a browser cannot attach a custom header to
		// the document request that loads it. Gating it would make LAN mode
		// unusable from a browser while protecting nothing (ADR-0039).
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if !checkLANToken(lanAuthToken, r.Header.Get(lanAuthHeader)) {
			http.Error(w, "missing or invalid LAN auth token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
