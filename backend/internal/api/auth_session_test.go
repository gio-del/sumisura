package api_test

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

// newTokenServer starts a router in token mode (LAN_AUTH_TOKEN "s3cret")
// over a seeded data dir, with one Generation file and one Company Logo on
// disk so the routes a browser loads on its own can be exercised.
func newTokenServer(t *testing.T, token string) (*httptest.Server, string) {
	t.Helper()
	dataDir := seedDataDir(t)
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, "output", "acme"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "output", "acme", "cv.pdf"), []byte("%PDF-1.7"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, LANAuthToken: token}))
	t.Cleanup(server.Close)
	return server, projectRoot
}

func exchangeToken(t *testing.T, server *httptest.Server, token string, header http.Header) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/auth/session", strings.NewReader(`{"token":"`+token+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func accessCookieFrom(t *testing.T, resp *http.Response) *http.Cookie {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == "sumisura_access" {
			return c
		}
	}
	t.Fatalf("no sumisura_access cookie set; Set-Cookie: %v", resp.Header.Values("Set-Cookie"))
	return nil
}

func getWithCookie(t *testing.T, url string, cookie *http.Cookie) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestAuthSession_CorrectToken_SetsHardenedAccessCookie(t *testing.T) {
	server, _ := newTokenServer(t, "s3cret")

	resp := exchangeToken(t, server, "s3cret", nil)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	c := accessCookieFrom(t, resp)
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge < 300*24*3600 {
		t.Fatalf("cookie attributes not hardened: %+v", c)
	}
	if c.Secure {
		t.Fatal("cookie marked Secure on a plain-HTTP request with no forwarded proto")
	}
	if strings.Contains(c.Value, "s3cret") {
		t.Fatal("cookie carries the raw token")
	}
}

func TestAuthSession_BehindHTTPSProxy_SetsSecureCookie(t *testing.T) {
	server, _ := newTokenServer(t, "s3cret")

	resp := exchangeToken(t, server, "s3cret", http.Header{"X-Forwarded-Proto": {"https"}})

	if c := accessCookieFrom(t, resp); !c.Secure {
		t.Fatalf("expected Secure cookie behind an HTTPS proxy, got %+v", c)
	}
}

func TestAuthSession_OverTLS_SetsSecureCookie(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewTLSServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, LANAuthToken: "s3cret"}))
	defer server.Close()
	client := server.Client()
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // test server's self-signed cert

	resp, err := client.Post(server.URL+"/api/auth/session", "application/json", strings.NewReader(`{"token":"s3cret"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if c := accessCookieFrom(t, resp); !c.Secure {
		t.Fatalf("expected Secure cookie over TLS, got %+v", c)
	}
}

func TestAuthSession_WrongToken_401AndNoCookie(t *testing.T) {
	server, _ := newTokenServer(t, "s3cret")

	resp := exchangeToken(t, server, "nope", nil)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	if len(resp.Cookies()) != 0 {
		t.Fatalf("expected no cookie, got %v", resp.Cookies())
	}
}

func TestAuthSession_DefaultMode_NoOp(t *testing.T) {
	server, _ := newTokenServer(t, "")

	resp := exchangeToken(t, server, "anything", nil)

	if resp.StatusCode != http.StatusNoContent || len(resp.Cookies()) != 0 {
		t.Fatalf("expected a no-op 204 with no cookie, got %d %v", resp.StatusCode, resp.Cookies())
	}
}

// TestAuthSession_CookieOpensBrowserLoadedRoutes is the reason the cookie
// exists: the routes a browser requests by itself (a PDF link or iframe, a
// Company Logo img) carry no custom header, and must work on the cookie.
func TestAuthSession_CookieOpensBrowserLoadedRoutes(t *testing.T) {
	server, _ := newTokenServer(t, "s3cret")
	cookie := accessCookieFrom(t, exchangeToken(t, server, "s3cret", nil))

	for _, path := range []string{"/api/generations/acme/cv.pdf", "/api/job-listings", "/api/master-data/entries"} {
		if resp := getWithCookie(t, server.URL+path, nil); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s without cookie: expected 401, got %d", path, resp.StatusCode)
		}
		if resp := getWithCookie(t, server.URL+path, cookie); resp.StatusCode != http.StatusOK {
			t.Fatalf("%s with cookie: expected 200, got %d", path, resp.StatusCode)
		}
	}
}

func TestAuthSession_RotatedToken_LocksOutOldCookie(t *testing.T) {
	oldServer, _ := newTokenServer(t, "s3cret")
	cookie := accessCookieFrom(t, exchangeToken(t, oldServer, "s3cret", nil))
	newServer, _ := newTokenServer(t, "n3w")

	if resp := getWithCookie(t, newServer.URL+"/api/job-listings", cookie); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for a cookie from the old token, got %d", resp.StatusCode)
	}
}

func TestAuthSession_Delete_ExpiresCookie(t *testing.T) {
	server, _ := newTokenServer(t, "s3cret")
	req, err := http.NewRequest(http.MethodDelete, server.URL+"/api/auth/session", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if c := accessCookieFrom(t, resp); c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("expected an expired, empty cookie, got %+v", c)
	}
}

func TestAuthStatus(t *testing.T) {
	tokenServer, _ := newTokenServer(t, "s3cret")
	cookie := accessCookieFrom(t, exchangeToken(t, tokenServer, "s3cret", nil))
	defaultServer, _ := newTokenServer(t, "")

	for _, tc := range []struct {
		name   string
		url    string
		cookie *http.Cookie
		want   string
	}{
		{"default mode", defaultServer.URL, nil, `{"required":false,"authenticated":true}`},
		{"token mode, no access", tokenServer.URL, nil, `{"required":true,"authenticated":false}`},
		{"token mode, cookie", tokenServer.URL, cookie, `{"required":true,"authenticated":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tc.url+"/api/auth/status", nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var got, want any
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != http.StatusOK || !jsonEqual(got, want) {
				t.Fatalf("expected 200 %s, got %d %v", tc.want, resp.StatusCode, got)
			}
		})
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// TestExtensionCapture_TokenMode_PreflightOpenPostGated is issue #195: the
// extension sends the token in a header, so its CORS preflight must allow
// that header and answer without a token, while the capture itself stays
// gated.
func TestExtensionCapture_TokenMode_PreflightOpenPostGated(t *testing.T) {
	server, _ := newTokenServer(t, "s3cret")

	req, err := http.NewRequest(http.MethodOptions, server.URL+"/api/job-listings/from-extension", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || !strings.Contains(resp.Header.Get("Access-Control-Allow-Headers"), "X-Sumisura-Token") {
		t.Fatalf("preflight: got %d, Allow-Headers %q", resp.StatusCode, resp.Header.Get("Access-Control-Allow-Headers"))
	}

	post, err := http.Post(server.URL+"/api/job-listings/from-extension", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	post.Body.Close()
	if post.StatusCode != http.StatusUnauthorized {
		t.Fatalf("capture without token: expected 401, got %d", post.StatusCode)
	}
}
