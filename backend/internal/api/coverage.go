package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// coveredTermWire is one Job Description term some Entry carries.
type coveredTermWire struct {
	Term     string   `json:"term"`
	EntryIDs []string `json:"entryIds"`
}

// coverageReport is the wire shape of GET /api/job-listings/{id}/coverage.
type coverageReport struct {
	Covered   []coveredTermWire `json:"covered"`
	Uncovered []string          `json:"uncovered"`
}

// coverageHandler reports how far the Master Data covers one Job Listing's
// Job Description. Read-only, deterministic, no Claude call.
func coverageHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		listing, err := tracking.Get(dataDir, r.PathValue("id"))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "job listing not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entries, err := masterdata.ListEntries(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cov := masterdata.ComputeCoverage(listing.JobListing.JobDescription, entries)
		out := coverageReport{Covered: make([]coveredTermWire, 0, len(cov.Covered)), Uncovered: cov.Uncovered}
		for _, c := range cov.Covered {
			out.Covered = append(out.Covered, coveredTermWire{Term: c.Term, EntryIDs: c.EntryIDs})
		}
		writeJSON(w, http.StatusOK, out)
	}
}
