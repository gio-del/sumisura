package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

// lanAuthHeader is the custom header LAN-reachable mode requires on every
// /api/* request once a shared-secret token is configured (see the
// LAN-reachable mode PRD, issue #57). Unused in default (localhost-only)
// mode.
const lanAuthHeader = "X-Sumisura-Token"

// accessCookieName is the cookie a browser holds instead of the header
// (issue #181). A browser cannot attach a custom header to the requests it
// makes on its own — a PDF link, the PDF preview iframe, a Company Logo
// <img> — so a header-only gate left those broken on every device in token
// mode. The header stays the way in for non-browser clients (the extension,
// an iOS Shortcut).
const accessCookieName = "sumisura_access"

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

// accessCookieValue is what the access cookie holds for a configured token:
// an HMAC of a fixed label keyed by the token, never the token itself, so
// the secret is not stored in the browser. It is derived rather than
// random, so there is no session store — and rotating LAN_AUTH_TOKEN
// changes the value, which locks out every device holding the old cookie
// with no extra step.
func accessCookieValue(token string) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte("sumisura-access-cookie-v1"))
	return hex.EncodeToString(mac.Sum(nil))
}

// checkLANAccess is the full gate decision for a request: the header check
// above, or else an access cookie matching the configured token. With no
// token configured it allows everything, exactly as checkLANToken does.
func checkLANAccess(configuredToken, presentedHeader, presentedCookie string) bool {
	if checkLANToken(configuredToken, presentedHeader) {
		return true
	}
	if presentedCookie == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(accessCookieValue(configuredToken)), []byte(presentedCookie)) == 1
}

// requestLANAccess reads the header and cookie off r and applies
// checkLANAccess.
func requestLANAccess(configuredToken string, r *http.Request) bool {
	cookie := ""
	if c, err := r.Cookie(accessCookieName); err == nil {
		cookie = c.Value
	}
	return checkLANAccess(configuredToken, r.Header.Get(lanAuthHeader), cookie)
}

// ungatedAPIPaths are the /api routes that must answer without a token:
// they are how a browser finds out a token is needed and exchanges one for
// the access cookie.
var ungatedAPIPaths = map[string]bool{
	"/api/auth/status":  true,
	"/api/auth/session": true,
}

// requireLANToken wraps next with LAN mode's auth gate: a request whose
// X-Sumisura-Token header doesn't match lanAuthToken, and which carries no
// matching access cookie either, gets 401 instead of reaching next. Callers only wrap with this when lanAuthToken is
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
		if ungatedAPIPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		if !requestLANAccess(lanAuthToken, r) {
			http.Error(w, "missing or invalid LAN auth token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
