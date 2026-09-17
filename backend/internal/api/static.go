package api

import (
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Go's built-in MIME table has no entry for the web app manifest, and the
// release image has no /etc/mime.types to fall back on, so without this
// the manifest would go out as text/plain (issue #185).
func init() {
	if err := mime.AddExtensionType(".webmanifest", "application/manifest+json"); err != nil {
		log.Printf("api: registering the .webmanifest MIME type: %v", err)
	}
}

// staticHandler serves the production frontend build from dir (ADR-0039):
// hashed assets with a long cache, index.html with none, and every
// unmatched path falling back to index.html so client-side routes like
// /generations survive a reload or a shared link.
//
// It is only ever reached for non-/api paths: NewRouter registers the API
// routes (and an /api/ catch-all) on the same mux, and Go's ServeMux
// prefers the more specific pattern.
func staticHandler(dir string) http.Handler {
	fileServer := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		clean := path.Clean("/" + r.URL.Path)
		file := filepath.Join(dir, filepath.FromSlash(clean))

		// filepath.Join already removes any ".." segments, but a path that
		// still escapes dir (a symlink, a crafted prefix) must not be
		// served: fall back to the app shell rather than reading outside
		// the build directory.
		if !strings.HasPrefix(file, filepath.Clean(dir)+string(os.PathSeparator)) && clean != "/" {
			serveIndex(w, r, index)
			return
		}

		info, err := os.Stat(file)
		if err != nil || info.IsDir() {
			// No such file: this is a client-side route (/jobs/acme,
			// /generations, …), which only the SPA can resolve.
			serveIndex(w, r, index)
			return
		}

		// Vite fingerprints everything under assets/, so those URLs can
		// never mean different bytes later and are safe to cache hard.
		// index.html is the opposite: it names the current hashed bundles,
		// so a cached copy would pin a browser to an old release.
		if strings.HasPrefix(clean, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, index string) {
	if _, err := os.Stat(index); err != nil {
		// The image always ships an index.html; a missing one means the
		// static directory is misconfigured, and saying so beats serving a
		// blank 404 that looks like an app bug.
		http.Error(w, "frontend build not found: set STATIC_DIR to the directory holding index.html", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, index)
}

// apiNotFoundHandler answers an unrouted /api/ path. Without it the SPA
// fallback would return index.html for a mistyped API call, so a broken
// request would look like a 200 with HTML in it — the hardest possible
// shape to debug from the client side.
func apiNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	if _, err := w.Write([]byte(`{"error":"no such API route"}`)); err != nil {
		log.Printf("api: writing 404 response body: %v", err)
	}
}
