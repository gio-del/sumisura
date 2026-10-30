package api

import (
	"net/http"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// gapWire is the wire shape of one recurring term of the Job Descriptions.
type gapWire struct {
	Term       string   `json:"term"`
	Count      int      `json:"count"`
	ListingIDs []string `json:"listingIds"`
	Covered    bool     `json:"covered"`
}

// gapsReport is the wire shape of GET /api/master-data/gaps's body.
type gapsReport struct {
	Gaps []gapWire `json:"gaps"`
}

// gapsHandler recomputes the Gaps report on every call from the non-archived
// Job Listings and the current Entries. Read-only: it never writes Master
// Data and makes no Claude call.
func gapsHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := masterdata.ListEntries(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tracked, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var listings []masterdata.GapListing
		for _, t := range tracked {
			if t.JobListing.Archived {
				continue
			}
			listings = append(listings, masterdata.GapListing{ID: t.JobListing.ID, JobDescription: t.JobListing.JobDescription})
		}

		gaps := masterdata.ComputeGaps(listings, entries)
		out := make([]gapWire, 0, len(gaps))
		for _, g := range gaps {
			out = append(out, gapWire{Term: g.Term, Count: len(g.ListingIDs), ListingIDs: g.ListingIDs, Covered: g.Covered})
		}
		writeJSON(w, http.StatusOK, gapsReport{Gaps: out})
	}
}
