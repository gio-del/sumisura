package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"regexp"
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

// lanAuth is the installation's access configuration: the shared token
// (LAN_AUTH_TOKEN), and optionally a username and password a browser can
// sign in with instead of typing the token (LAN_AUTH_USER and
// LAN_AUTH_PASSWORD, both or neither). The password is for people, who keep
// it in a password manager; the token stays the way in for non-browser
// clients (the extension, the skill, an iOS Shortcut), which send it in the
// header. A zero lanAuth is default mode: no gate at all.
type lanAuth struct {
	token    string
	user     string
	password string
}

// passwordLogin reports whether browsers sign in with a username and
// password. It needs the token too: the password only adds a way to obtain
// the access cookie, it never opens the gate on its own.
func (a lanAuth) passwordLogin() bool {
	return a.token != "" && a.user != "" && a.password != ""
}

// cookieKey keys the access cookie. With password sign-in it covers the
// username and password as well as the token, so rotating any of them signs
// every browser out; header clients only ever depend on the token.
func (a lanAuth) cookieKey() string {
	if a.passwordLogin() {
		return a.token + "\x00" + a.user + "\x00" + a.password
	}
	return a.token
}

// checkLANPassword compares a presented username and password with the
// configured ones in constant time, both always compared so a wrong username
// takes as long as a wrong password.
func checkLANPassword(a lanAuth, user, password string) bool {
	if !a.passwordLogin() {
		return false
	}
	userOK := subtle.ConstantTimeCompare([]byte(a.user), []byte(user))
	passwordOK := subtle.ConstantTimeCompare([]byte(a.password), []byte(password))
	return userOK&passwordOK == 1
}

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

// accessCookieValue is what the access cookie holds for a cookie key (the
// token, or the token plus the sign-in credentials, see lanAuth.cookieKey):
// an HMAC of a fixed label keyed by it, never the secret itself, so
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
// above, or else an access cookie matching the configured credentials. With
// no token configured it allows everything, exactly as checkLANToken does.
func checkLANAccess(a lanAuth, presentedHeader, presentedCookie string) bool {
	if checkLANToken(a.token, presentedHeader) {
		return true
	}
	if presentedCookie == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(accessCookieValue(a.cookieKey())), []byte(presentedCookie)) == 1
}

// requestLANAccess reads the header and cookie off r and applies
// checkLANAccess.
func requestLANAccess(a lanAuth, r *http.Request) bool {
	cookie := ""
	if c, err := r.Cookie(accessCookieName); err == nil {
		cookie = c.Value
	}
	return checkLANAccess(a, r.Header.Get(lanAuthHeader), cookie)
}

// ungatedAPIPaths are the /api routes that must answer without a token:
// they are how a browser finds out a token is needed and exchanges one for
// the access cookie.
var ungatedAPIPaths = map[string]bool{
	"/api/auth/status":  true,
	"/api/auth/session": true,
}

// ungatedPreflightPaths are the /api routes whose CORS preflight must
// answer without a token. The extension calls both cross-origin from a
// moz-extension:// or chrome-extension:// origin, and a browser strips
// custom headers from a preflight, so the OPTIONS can never carry one.
// The requests they clear — the capture POST and the lookup POST — are
// gated as usual.
var ungatedPreflightPaths = map[string]bool{
	"/api/job-listings/from-extension": true,
	"/api/job-listings/capture-lookup": true,
}

// applicationStatusPath matches the Status route, whose path carries a
// record id and so cannot be listed literally above.
var applicationStatusPath = regexp.MustCompile(`^/api/applications/[^/]+/status$`)

func isUngatedPreflight(path string) bool {
	return ungatedPreflightPaths[path] || applicationStatusPath.MatchString(path)
}

// requireLANToken wraps next with LAN mode's auth gate: a request whose
// X-Sumisura-Token header doesn't match the token, and which carries no
// matching access cookie either, gets 401 instead of reaching next. Callers only wrap with this when the token is
// non-empty (see RouterConfig.LANAuthToken) — default mode never wraps a
// handler with it at all.
func requireLANToken(a lanAuth, next http.Handler) http.Handler {
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
		// A CORS preflight can never carry the token (browsers strip custom
		// headers from it), so the extension routes' preflights must answer
		// without one; the requests they clear are still gated (#195).
		if ungatedAPIPaths[r.URL.Path] || (r.Method == http.MethodOptions && isUngatedPreflight(r.URL.Path)) {
			next.ServeHTTP(w, r)
			return
		}
		if !requestLANAccess(a, r) {
			http.Error(w, "missing or invalid LAN auth token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
