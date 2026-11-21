package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gio-del/sumisura/backend/internal/atsboard"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// listAtsListingsHandler fetches boardSlug's open roles from provider's
// public job board API and returns them normalized (story 1) and marked
// with whether each is already saved as a Job Listing (story 7), or a
// clear 404 if the board slug doesn't exist on that provider rather than a
// silent empty list (story 6).
func listAtsListingsHandler(dataDir string, atsHTTPDoer atsboard.HTTPDoer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := atsboard.Provider(r.PathValue("provider"))
		slug := r.PathValue("slug")

		listings, err := atsboard.Fetch(r.Context(), atsHTTPDoer, provider, slug)
		if errors.Is(err, atsboard.ErrBoardNotFound) {
			http.Error(w, fmt.Sprintf(
				"No %s board found for slug %q. Check the slug, or try a different provider.",
				provider, slug,
			), http.StatusNotFound)
			return
		}
		if errors.Is(err, atsboard.ErrUnknownProvider) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		existing, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		existingURLs := make([]string, 0, len(existing))
		for _, e := range existing {
			if e.JobListing.URL != "" {
				existingURLs = append(existingURLs, e.JobListing.URL)
			}
		}

		withSaved := atsboard.MarkAlreadySaved(listings, existingURLs)

		// The new-since-last-check digest only applies to boards the user
		// has deliberately tracked (story 10) — a one-off browse of an
		// untracked board never reads or writes seen-state.
		tracked, err := atsboard.ListTrackedBoards(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		boardID := atsboard.TrackedBoardID(provider, slug)
		isTracked := false
		for _, b := range tracked {
			if b.ID == boardID {
				isTracked = true
				break
			}
		}

		if !isTracked {
			digest := make([]atsboard.ListingDigest, len(withSaved))
			for i, ls := range withSaved {
				digest[i] = atsboard.ListingDigest{Listing: ls.Listing, AlreadySaved: ls.AlreadySaved}
			}
			writeJSON(w, http.StatusOK, digest)
			return
		}

		seenURLs, existed, err := atsboard.SeenURLs(dataDir, boardID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		listingURLs := make([]string, len(listings))
		for i, l := range listings {
			listingURLs[i] = l.URL
		}
		effectiveSeen := seenURLs
		if !existed {
			// First-ever fetch of a freshly-tracked board (story 5): seed
			// the baseline from what's visible now, so the existing
			// backlog isn't reported as new.
			effectiveSeen = listingURLs
		}
		withNew := atsboard.MarkNewSinceLastCheck(listings, effectiveSeen)

		// Fetching updates the board's seen-state (story 3): fold in both
		// what's visible now and every already-saved URL (story 4), so a
		// listing saved outside revisiting this board never later shows
		// as new.
		if err := atsboard.RecordSeen(dataDir, boardID, append(listingURLs, existingURLs...)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var newURLs []string
		for _, l := range withNew {
			if l.New {
				newURLs = append(newURLs, l.URL)
			}
		}
		if err := atsboard.RecordCheck(dataDir, boardID, time.Now().UTC(), listingURLs, newURLs); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, atsboard.CombineDigest(withSaved, withNew))
	}
}
