package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

var generationSlugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// generationFileContentTypes allowlists the files Render produces under
// output/<slug>/ that may be served/downloaded (story 10's PDF preview,
// story 11's downloads) — never an arbitrary path under projectRoot.
var generationFileContentTypes = map[string]string{
	"cv.pdf":           "application/pdf",
	"cover-letter.pdf": "application/pdf",
	"cover-letter.txt": "text/plain; charset=utf-8",
}

// listGenerationsHandler answers the "every CV generated so far" index
// (issue #173). It needs both roots: the records live under dataDir, the
// output directories under projectRoot.
func listGenerationsHandler(dataDir, projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		index, err := tracking.ListGenerations(dataDir, projectRoot)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, emptyIfNil(index))
	}
}

// deleteGenerationHandler removes one Generation's output/<slug>/
// directory — the PDFs and the assembled JSON beside them.
//
// It deletes derived artifacts only (ADR-0018): a Generation recorded
// against an Application keeps its record, which is the durable trace of
// what was sent, at what cost and with what groundedness verdict, and which
// the Generated CVs page then shows with its files reported missing. An
// unrecorded Generation has no record to keep, so deleting its directory
// removes it entirely.
//
// Deleting a directory that is already gone succeeds: the caller asked for
// it not to be there.
func deleteGenerationHandler(projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		// The same pattern the file-serving route validates against. It
		// admits no dot, slash or empty segment, so the join below cannot
		// escape output/ — never delete a path built from an unchecked slug.
		if !generationSlugRe.MatchString(slug) {
			http.NotFound(w, r)
			return
		}

		if err := os.RemoveAll(filepath.Join(projectRoot, "output", slug)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func getGenerationFileHandler(projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		file := r.PathValue("file")

		contentType, allowed := generationFileContentTypes[file]
		if !allowed || !generationSlugRe.MatchString(slug) {
			http.NotFound(w, r)
			return
		}

		path := filepath.Join(projectRoot, "output", slug, file)
		if _, err := os.Stat(path); err != nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, path)
	}
}

// renderGenerationHandler is a pass-through to generation.Render. The
// request's slug is only a label: Render derives the unique output
// directory from it (issue #105) and the response's slug names the one it
// actually wrote, which is what callers must record and serve from.
func renderGenerationHandler(dataDir, projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req generation.RenderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}

		result, err := generation.Render(projectRoot, dataDir, req)
		if errors.Is(err, generation.ErrInvalidRenderRequest) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func createGenerationHandler(dataDir string, client generation.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req generation.GenerateRequest
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
				return
			}
		}

		result, err := generation.Generate(r.Context(), dataDir, client, req)
		if errors.Is(err, generation.ErrInvalidSelection) {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// previewGenerationHandler serves the Selection-only preview (see the
// "Dry-run Selection preview" PRD): same request shape as
// createGenerationHandler, but never calls Rewrite, Cover Letter drafting,
// RAL Range estimation, or Render, and nothing about the response is ever
// persisted against an Application.
func previewGenerationHandler(dataDir string, client generation.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req generation.GenerateRequest
		if r.Body != nil && r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
				return
			}
		}

		result, err := generation.Preview(r.Context(), dataDir, client, req)
		// A preview has no GenerationRecord to carry its usage, so it goes
		// to the standalone usage log — drained now, even on failure (the
		// call was still billed), so it can't leak into the next Generation.
		tracking.RecordStandaloneUsage(dataDir, client)
		if errors.Is(err, generation.ErrInvalidSelection) {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}
