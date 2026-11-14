package api_test

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

func packageFixture(t *testing.T, files ...string) (serverURL, id string, projectRoot, dataDir string) {
	t.Helper()
	projectRoot, dataDir = seedProjectRoot(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, ProjectRoot: projectRoot, GenerationClient: &fakeGenerationClient{}}))
	t.Cleanup(server.Close)
	id = saveJobListing(t, server.URL, "Acme")
	return server.URL, id, projectRoot, dataDir
}

func recordGen(t *testing.T, serverURL, id, slug string, letter bool) {
	t.Helper()
	payload := map[string]any{"slug": slug, "cvPath": "output/" + slug + "/cv.pdf"}
	if letter {
		payload["coverLetterPath"] = "output/" + slug + "/cover-letter.pdf"
	}
	resp := postJSON(t, serverURL+"/api/applications/"+id+"/generations", payload)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("recording generation: %d", resp.StatusCode)
	}
}

func writeOutput(t *testing.T, root, slug string, names ...string) {
	t.Helper()
	dir := filepath.Join(root, "output", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("pdf "+slug+" "+n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func zipContents(t *testing.T, resp *http.Response) map[string]string {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

func TestApplicationPackage_WithLetter_ZipsLatestGenerationNamedAfterProfile(t *testing.T) {
	url, id, root, _ := packageFixture(t)
	writeOutput(t, root, "acme-old", "cv.pdf", "cover-letter.pdf")
	writeOutput(t, root, "acme-new", "cv.pdf", "cover-letter.pdf")
	recordGen(t, url, id, "acme-old", true)
	recordGen(t, url, id, "acme-new", true)

	resp, err := http.Get(url + "/api/applications/" + id + "/package.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("got %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	got := zipContents(t, resp)
	var names []string
	for n := range got {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "Test-User-CV.pdf" || names[1] != "Test-User-Lettera.pdf" {
		t.Fatalf("names = %v", names)
	}
	if got["Test-User-CV.pdf"] != "pdf acme-new cv.pdf" {
		t.Errorf("zip holds %q, want the latest generation's CV", got["Test-User-CV.pdf"])
	}
}

func TestApplicationPackage_WithoutLetter_ZipsOnlyTheCV(t *testing.T) {
	url, id, root, _ := packageFixture(t)
	writeOutput(t, root, "acme-a", "cv.pdf")
	recordGen(t, url, id, "acme-a", false)

	resp, _ := http.Get(url + "/api/applications/" + id + "/package.zip")
	defer resp.Body.Close()
	got := zipContents(t, resp)
	if len(got) != 1 || got["Test-User-CV.pdf"] == "" {
		t.Fatalf("zip = %v", got)
	}
}

func TestApplicationPackage_UnsafeProfileName_YieldsSafeFileNames(t *testing.T) {
	url, id, root, dataDir := packageFixture(t)
	if err := os.WriteFile(filepath.Join(dataDir, "profile.yaml"), []byte("name: \"../Rossi / Élodie\\\\x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeOutput(t, root, "acme-a", "cv.pdf")
	recordGen(t, url, id, "acme-a", false)

	resp, _ := http.Get(url + "/api/applications/" + id + "/package.zip")
	defer resp.Body.Close()
	got := zipContents(t, resp)
	if got["Rossi-Élodie-x-CV.pdf"] == "" {
		t.Fatalf("zip = %v", got)
	}
}

func TestApplicationPackage_NoGeneration_Returns404(t *testing.T) {
	url, id, _, _ := packageFixture(t)
	resp, _ := http.Get(url + "/api/applications/" + id + "/package.zip")
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestApplicationPackage_FilesGone_Returns404(t *testing.T) {
	url, id, _, _ := packageFixture(t)
	recordGen(t, url, id, "acme-gone", true)
	resp, _ := http.Get(url + "/api/applications/" + id + "/package.zip")
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("got %d", resp.StatusCode)
	}
}

func TestApplicationPackage_UnknownApplication_Returns404(t *testing.T) {
	url, _, _, _ := packageFixture(t)
	resp, _ := http.Get(url + "/api/applications/nope/package.zip")
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("got %d", resp.StatusCode)
	}
}
