package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

// seedStaticDir writes a minimal production frontend build: an index.html
// and one hashed asset, the shape `npm run build` produces.
func seedStaticDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html><title>Sumisura</title>")
	writeFile(t, filepath.Join(dir, "assets", "index-a1b2c3d4.js"), "console.log('app')")
	return dir
}

func staticServer(t *testing.T, staticDir string) *httptest.Server {
	t.Helper()
	_, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{
		DataDir: dataDir, StaticDir: staticDir, GenerationClient: &fakeGenerationClient{},
	}))
	t.Cleanup(server.Close)
	return server
}

func get(t *testing.T, url string) (int, string, http.Header) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body), resp.Header
}

func TestStatic_ServesTheAppShellAtRoot(t *testing.T) {
	server := staticServer(t, seedStaticDir(t))

	code, body, header := get(t, server.URL+"/")

	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if !strings.Contains(body, "<title>Sumisura</title>") {
		t.Errorf("expected the app shell, got %q", body)
	}
	// index.html names the current hashed bundles, so a cached copy would
	// pin a browser to an old release.
	if got := header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("expected index.html served no-cache, got %q", got)
	}
}

// Vite fingerprints everything under assets/, so those URLs can never mean
// different bytes later.
func TestStatic_HashedAssetsAreCachedHard(t *testing.T) {
	server := staticServer(t, seedStaticDir(t))

	code, body, header := get(t, server.URL+"/assets/index-a1b2c3d4.js")

	if code != http.StatusOK || !strings.Contains(body, "console.log") {
		t.Fatalf("expected the asset, got %d %q", code, body)
	}
	if got := header.Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("expected an immutable cache header, got %q", got)
	}
}

// A client-side route is not a file on disk; only the SPA can resolve it,
// so a reload or a shared link must still get the app.
func TestStatic_ClientRoutesFallBackToTheAppShell(t *testing.T) {
	server := staticServer(t, seedStaticDir(t))

	for _, path := range []string{"/generations", "/jobs/acme-corp", "/profile"} {
		code, body, _ := get(t, server.URL+path)
		if code != http.StatusOK || !strings.Contains(body, "<title>Sumisura</title>") {
			t.Errorf("%s: expected the app shell, got %d %q", path, code, body)
		}
	}
}

// The static handler must never shadow the API, and an unrouted /api path
// must not come back as the app shell with a 200 — the hardest possible
// failure to debug from the client.
func TestStatic_DoesNotShadowTheAPI(t *testing.T) {
	server := staticServer(t, seedStaticDir(t))

	code, body, _ := get(t, server.URL+"/api/healthz")
	if code != http.StatusOK || !strings.Contains(body, `"status":"ok"`) {
		t.Errorf("expected the health route, got %d %q", code, body)
	}

	code, body, header := get(t, server.URL+"/api/does-not-exist")
	if code != http.StatusNotFound {
		t.Errorf("expected 404 for an unrouted API path, got %d", code)
	}
	if ct := header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected a JSON 404, got Content-Type %q", ct)
	}
	if strings.Contains(body, "<title>") {
		t.Errorf("an unrouted API path must not return the app shell, got %q", body)
	}
}

// With no StaticDir — the dev compose file's configuration — nothing is
// served at / at all, and the API is unchanged.
func TestStatic_DisabledByDefault(t *testing.T) {
	_, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	if code, _, _ := get(t, server.URL+"/"); code != http.StatusNotFound {
		t.Errorf("expected 404 at / with no StaticDir, got %d", code)
	}
	if code, _, _ := get(t, server.URL+"/api/healthz"); code != http.StatusOK {
		t.Errorf("expected the API to work with no StaticDir, got %d", code)
	}
}

// LAN mode's token gates the API, which carries the user's data. The
// frontend build is the same bytes for every install and a browser cannot
// attach a custom header to the document request that loads it, so gating
// it would break LAN mode in a browser while protecting nothing.
func TestStatic_LANModeGatesTheAPIButNotTheAppShell(t *testing.T) {
	_, dataDir := seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{
		DataDir: dataDir, StaticDir: seedStaticDir(t), LANAuthToken: "secret",
		GenerationClient: &fakeGenerationClient{},
	}))
	defer server.Close()

	if code, _, _ := get(t, server.URL+"/"); code != http.StatusOK {
		t.Errorf("expected the app shell to load without a token, got %d", code)
	}
	if code, _, _ := get(t, server.URL+"/assets/index-a1b2c3d4.js"); code != http.StatusOK {
		t.Errorf("expected assets to load without a token, got %d", code)
	}
	if code, _, _ := get(t, server.URL+"/api/master-data/entries"); code != http.StatusUnauthorized {
		t.Errorf("expected the API to still require the token, got %d", code)
	}

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/master-data/entries", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Sumisura-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected the API to accept the token, got %d", resp.StatusCode)
	}
}

// A path that tries to climb out of the build directory gets the app
// shell, never a file from elsewhere on disk.
func TestStatic_RefusesToServeOutsideTheBuildDirectory(t *testing.T) {
	staticDir := seedStaticDir(t)
	secret := filepath.Join(filepath.Dir(staticDir), "secret.txt")
	if err := os.WriteFile(secret, []byte("PRIVATE"), 0o644); err != nil {
		t.Fatal(err)
	}
	server := staticServer(t, staticDir)

	for _, path := range []string{"/../secret.txt", "/..%2fsecret.txt", "/assets/../../secret.txt"} {
		_, body, _ := get(t, server.URL+path)
		if strings.Contains(body, "PRIVATE") {
			t.Errorf("%s: served a file from outside the build directory", path)
		}
	}
}

// TestStatic_PWAFilesServedFreshWithTheRightType covers what installing the
// app needs (issue #185): the manifest with its registered type, and the
// service worker revalidated on every load so an update reaches devices.
func TestStatic_PWAFilesServedFreshWithTheRightType(t *testing.T) {
	dir := seedStaticDir(t)
	writeFile(t, filepath.Join(dir, "manifest.webmanifest"), `{"name":"Sumisura"}`)
	writeFile(t, filepath.Join(dir, "sw.js"), "self.addEventListener('fetch', () => {})")
	server := staticServer(t, dir)

	status, _, header := get(t, server.URL+"/manifest.webmanifest")
	if status != http.StatusOK || header.Get("Content-Type") != "application/manifest+json" || header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("manifest: got %d, Content-Type %q, Cache-Control %q", status, header.Get("Content-Type"), header.Get("Cache-Control"))
	}
	status, _, header = get(t, server.URL+"/sw.js")
	if status != http.StatusOK || !strings.Contains(header.Get("Content-Type"), "javascript") || header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("service worker: got %d, Content-Type %q, Cache-Control %q", status, header.Get("Content-Type"), header.Get("Cache-Control"))
	}
	status, body, _ := get(t, server.URL+"/share?text=hello")
	if status != http.StatusOK || !strings.Contains(body, "<title>Sumisura</title>") {
		t.Fatalf("share target: expected the app shell, got %d %q", status, body)
	}
}
