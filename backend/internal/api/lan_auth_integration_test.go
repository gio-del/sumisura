package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

// TestLANAuth_TokenConfigured_RejectsRequestWithoutToken confirms the
// token-check is actually wired into the request path, not just tested in
// isolation: a router built with a LAN auth token configured must reject a
// request that doesn't carry it.
func TestLANAuth_TokenConfigured_RejectsRequestWithoutToken(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, LANAuthToken: "s3cret"}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestLANAuth_NoTokenConfigured_RequestSucceedsUnauthenticated confirms
// default (localhost-only) mode is completely unaffected: with no LAN auth
// token configured, the same request path works with no token at all.
func TestLANAuth_NoTokenConfigured_RequestSucceedsUnauthenticated(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, LANAuthToken: ""}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestLANAuth_AcceptsRenamedHeaderOnly pins the clean break made when the
// project was renamed to Sumisura: the LAN token travels in X-Sumisura-Token,
// and the pre-rename X-CV-Reporter-Token is not accepted as a fallback. The
// rename happened before the first release, so there is no installed client to
// keep working — this test exists so nobody "helpfully" re-adds the old name.
func TestLANAuth_AcceptsRenamedHeaderOnly(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, LANAuthToken: "s3cret"}))
	defer server.Close()

	for _, tc := range []struct {
		name   string
		header string
		want   int
	}{
		{name: "current header", header: "X-Sumisura-Token", want: http.StatusOK},
		{name: "pre-rename header", header: "X-CV-Reporter-Token", want: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, server.URL+"/api/healthz", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set(tc.header, "s3cret")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.want {
				t.Fatalf("%s: expected %d, got %d", tc.header, tc.want, resp.StatusCode)
			}
		})
	}
}

// TestLANAuth_CaptureLookup_PreflightUngatedButPOSTGated pins the split
// the extension's second cross-origin route needs (issue #206): a browser
// strips custom headers from a CORS preflight, so the OPTIONS must answer
// without a token — but the POST it clears is gated like any other /api
// request. The capture route has had this since #195; the lookup joins it.
func TestLANAuth_CaptureLookup_PreflightUngatedButPOSTGated(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{
		DataDir: dataDir, GenerationClient: &fakeGenerationClient{}, LANAuthToken: "s3cret",
	}))
	defer server.Close()

	for _, path := range []string{"/api/job-listings/from-extension", "/api/job-listings/capture-lookup"} {
		req, err := http.NewRequest(http.MethodOptions, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		preflight, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		preflight.Body.Close()
		if preflight.StatusCode != http.StatusNoContent {
			t.Errorf("%s: expected the preflight answered without a token, got %d", path, preflight.StatusCode)
		}

		posted := postJSON(t, server.URL+path, map[string]any{"url": "https://example.com/x", "company": "Acme"})
		posted.Body.Close()
		if posted.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: expected the POST still gated, got %d", path, posted.StatusCode)
		}
	}
}
