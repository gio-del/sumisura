package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// accessCookieMaxAge is how long a device stays signed in after entering
// the token once. Long on purpose: the phone should keep working for months,
// and the way to revoke every device is rotating LAN_AUTH_TOKEN, not expiry.
const accessCookieMaxAge = 365 * 24 * time.Hour

// authStatus is GET /api/auth/status's body: whether this installation
// requires access at all, whether this request already carries valid access
// (header or cookie), and whether a browser signs in with a username and
// password rather than the token. The frontend reads it to decide between
// the app and the sign-in screen, and which form that screen shows.
type authStatus struct {
	Required      bool `json:"required"`
	Authenticated bool `json:"authenticated"`
	PasswordLogin bool `json:"passwordLogin"`
}

func authStatusHandler(a lanAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, authStatus{
			Required:      a.token != "",
			Authenticated: requestLANAccess(a, r),
			PasswordLogin: a.passwordLogin(),
		})
	}
}

// createAuthSessionRequest carries either the token or, with password
// sign-in configured, a username and password.
type createAuthSessionRequest struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// createAuthSessionHandler exchanges the access token, or the username and
// password when those are configured, for the access cookie (issue #181).
// With no token configured there is nothing to exchange: it answers 204 and
// sets nothing, so a stale sign-in screen can't fail against a server that
// stopped requiring one.
func createAuthSessionHandler(a lanAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.token == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		var req createAuthSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.Username != "" || req.Password != "" {
			if !checkLANPassword(a, req.Username, req.Password) {
				http.Error(w, "invalid username or password", http.StatusUnauthorized)
				return
			}
		} else if !checkLANToken(a.token, req.Token) {
			http.Error(w, "invalid access token", http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, accessCookie(r, accessCookieValue(a.cookieKey()), int(accessCookieMaxAge.Seconds())))
		w.WriteHeader(http.StatusNoContent)
	}
}

// deleteAuthSessionHandler is "Forget this device": it expires the access
// cookie. It needs no token — clearing your own cookie grants nothing.
func deleteAuthSessionHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, accessCookie(r, "", -1))
	w.WriteHeader(http.StatusNoContent)
}

// accessCookie builds the access cookie. HttpOnly keeps it away from page
// scripts. SameSite=Strict is the CSRF defence: a single-user tool whose
// writes are JSON bodies needs nothing more. Secure is set whenever the
// request reached us over HTTPS — directly, or through a TLS-terminating
// proxy on the same machine such as `tailscale serve`, which reports it in
// X-Forwarded-Proto. Trusting that header unconditionally is safe here: a
// client that lies about it only makes its own cookie unusable over HTTP.
func accessCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     accessCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
	}
}
