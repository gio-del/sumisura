package api

import (
	"encoding/json"
	"net/http"

	"github.com/gio-del/sumisura/backend/internal/postingkey"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// maxCaptureLookupURLs caps the batch form. A search-results page holds a
// couple of dozen rows and the extension sends the visible ones, so this
// is far above any honest request — it exists so one malformed caller
// can't make the backend read the corpus against an unbounded list.
const maxCaptureLookupURLs = 200

// captureLookupRequest is the extension card's question, in two forms
// (issue #206). The single form asks about the posting the user has open:
// is it tracked, and does this company have other roles. The batch form
// asks tracked-state only, for badging a results page in one request — a
// row needs a badge, not a decision.
type captureLookupRequest struct {
	URL     string   `json:"url"`
	Company string   `json:"company"`
	URLs    []string `json:"urls"`
}

// trackedPosting is what the card shows on its state line for a posting
// already tracked, plus the Status moves it may offer.
type trackedPosting struct {
	ID       string          `json:"id"`
	Title    string          `json:"title,omitempty"`
	SavedAt  string          `json:"savedAt"`
	Status   tracking.Status `json:"status"`
	Archived bool            `json:"archived"`
	// AllowedTransitions comes straight from the Status state machine, so
	// the card never holds a second definition of it (story 44). The
	// backend still re-validates every move, so a stale card cannot
	// corrupt anything (story 45).
	AllowedTransitions []tracking.Status `json:"allowedTransitions"`
}

// siblingListing is one other role tracked at the same company: enough to
// answer "do I want this one too?" without leaving the job board.
type siblingListing struct {
	ID      string          `json:"id"`
	Title   string          `json:"title,omitempty"`
	SavedAt string          `json:"savedAt"`
	Status  tracking.Status `json:"status"`
}

type companyLookup struct {
	Listings []siblingListing `json:"listings"`
}

type captureLookupResponse struct {
	Tracked *trackedPosting `json:"tracked"`
	Company companyLookup   `json:"company"`
}

// badgedPosting is the batch form's answer per row: only what a badge
// needs.
type badgedPosting struct {
	ID     string          `json:"id"`
	Status tracking.Status `json:"status"`
}

type captureLookupResult struct {
	URL     string         `json:"url"`
	Tracked *badgedPosting `json:"tracked"`
}

type captureLookupBatchResponse struct {
	Results []captureLookupResult `json:"results"`
}

// captureLookupCORSPreflightHandler answers the browser's CORS preflight
// for the lookup, for the same reason the capture route carries one: the
// caller runs on a moz-extension:// or chrome-extension:// origin, which
// is CORS-checked like any other cross-origin fetch.
func captureLookupCORSPreflightHandler(w http.ResponseWriter, r *http.Request) {
	captureJobListingCORSPreflightHandler(w, r)
}

// captureLookupHandler answers what the extension card needs to know
// before the user clicks Save. It is read-only: no record is created or
// changed, and no Claude call is made. A POST rather than a GET so the two
// extension routes are shaped identically and a company name needs no URL
// encoding.
func captureLookupHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		var req captureLookupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if len(req.URLs) > maxCaptureLookupURLs {
			http.Error(w, "too many urls in one lookup", http.StatusBadRequest)
			return
		}

		listings, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if len(req.URLs) > 0 {
			writeJSON(w, http.StatusOK, captureLookupBatchResponse{Results: badgeURLs(req.URLs, listings)})
			return
		}
		writeJSON(w, http.StatusOK, lookUpPosting(req.URL, req.Company, listings))
	}
}

// lookUpPosting answers the single form: the tracked state of this
// posting, and the company's other active roles.
func lookUpPosting(url, company string, listings []tracking.ListingWithApplication) captureLookupResponse {
	response := captureLookupResponse{Company: companyLookup{Listings: []siblingListing{}}}

	key, keyed := postingkey.Of(url)
	trackedID := ""
	if keyed {
		for _, l := range listings {
			if other, ok := postingkey.Of(l.JobListing.URL); ok && other == key {
				trackedID = l.JobListing.ID
				response.Tracked = &trackedPosting{
					ID:                 l.JobListing.ID,
					Title:              l.JobListing.Title,
					SavedAt:            l.JobListing.SavedAt,
					Status:             l.Application.Status,
					Archived:           l.JobListing.Archived,
					AllowedTransitions: tracking.AllowedTransitions(l.Application.Status),
				}
				break
			}
		}
	}

	for _, l := range listings {
		// Archived listings are excluded entirely: a company worked
		// through and closed out stops interrupting (stories 22, 23). The
		// tracked listing is excluded too — the card shows it on its own
		// state line, not as one of its siblings.
		if l.JobListing.Archived || l.JobListing.ID == trackedID {
			continue
		}
		if !tracking.SameCompany(l.JobListing.Company, company) {
			continue
		}
		response.Company.Listings = append(response.Company.Listings, siblingListing{
			ID:      l.JobListing.ID,
			Title:   l.JobListing.Title,
			SavedAt: l.JobListing.SavedAt,
			Status:  l.Application.Status,
		})
	}
	return response
}

// badgeURLs answers the batch form. It reads each row's own href and
// computes the Posting Key here, so that computation never moves into the
// extension. A URL with no Posting Key is simply untracked.
func badgeURLs(urls []string, listings []tracking.ListingWithApplication) []captureLookupResult {
	byKey := make(map[postingkey.Key]*badgedPosting, len(listings))
	for _, l := range listings {
		if key, ok := postingkey.Of(l.JobListing.URL); ok {
			if _, seen := byKey[key]; !seen {
				byKey[key] = &badgedPosting{ID: l.JobListing.ID, Status: l.Application.Status}
			}
		}
	}

	results := make([]captureLookupResult, len(urls))
	for i, url := range urls {
		results[i] = captureLookupResult{URL: url}
		if key, ok := postingkey.Of(url); ok {
			results[i].Tracked = byKey[key]
		}
	}
	return results
}
