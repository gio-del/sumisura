package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/postingkey"
	"github.com/gio-del/sumisura/backend/internal/recordversion"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// defaultRALCurrency mirrors generation.ParseStatedRAL's own default: RAL
// is an Italian-market term, so a ral_min/ral_max filter with no explicit
// ral_currency is assumed EUR rather than left ambiguous.
const defaultRALCurrency = "EUR"

// parseRALFilter reads the optional ral_min/ral_max/ral_currency query
// params (issue #51). active is false when neither ral_min nor ral_max is
// present, meaning no RAL filter should be applied at all — a listing
// missing a comparable figure is only ever excluded by an active filter,
// never by an absent one.
func parseRALFilter(q url.Values) (min, max int, currency string, active bool, err error) {
	minStr, maxStr := q.Get("ral_min"), q.Get("ral_max")
	if minStr == "" && maxStr == "" {
		return 0, 0, "", false, nil
	}

	min = 0
	if minStr != "" {
		if min, err = strconv.Atoi(minStr); err != nil {
			return 0, 0, "", false, fmt.Errorf("invalid ral_min: %w", err)
		}
	}
	max = math.MaxInt32
	if maxStr != "" {
		if max, err = strconv.Atoi(maxStr); err != nil {
			return 0, 0, "", false, fmt.Errorf("invalid ral_max: %w", err)
		}
	}
	currency = q.Get("ral_currency")
	if currency == "" {
		currency = defaultRALCurrency
	}
	return min, max, currency, true, nil
}

// LogoURL lets the ATS-browse save path (frontend/src/pages/AtsBrowsePage.tsx)
// pass through a Company Logo it already knows about from atsboard.Listing
// (issue #42) — the manual-paste save path simply never sets it.
type saveJobListingRequest struct {
	Title             string `json:"title"`
	Company           string `json:"company"`
	Location          string `json:"location"`
	URL               string `json:"url"`
	JobDescription    string `json:"jobDescription"`
	JobDescriptionURL string `json:"jobDescriptionUrl"`
	LogoURL           string `json:"logoUrl"`
}

type saveJobListingResponse struct {
	JobListing  tracking.JobListing  `json:"jobListing"`
	Application tracking.Application `json:"application"`
	// DuplicateWarning is set when this save looks like a role already
	// tracked under a different Job Listing (issue #43) — a non-blocking
	// hint, never a reason the save above was rejected.
	DuplicateWarning *tracking.DuplicateMatch `json:"duplicateWarning,omitempty"`
	// CompletedPendingCaptureID is the Pending Capture this save completed:
	// the same posting shared earlier from a phone and waiting in To
	// complete (issue #183). Absent when nothing was pending for it.
	CompletedPendingCaptureID string `json:"completedPendingCaptureId,omitempty"`
	// ArchivedJobListingID is the role a replace resolution archived in the
	// same action (issue #206, story 17). Absent on every other save.
	ArchivedJobListingID string `json:"archivedJobListingId,omitempty"`
	// ArchiveFailed reports a replace whose save succeeded but whose
	// archive did not, so the user is never left believing they
	// consolidated something they didn't (story 20). Surfaced rather than
	// swallowed, unlike completePendingCaptureBestEffort — a stale inbox
	// entry is a nuisance, a role still active in the pipeline is a wrong
	// answer to "what am I chasing?".
	ArchiveFailed bool `json:"archiveFailed,omitempty"`
}

// findDuplicateWarningBestEffort checks the just-saved listing against every
// other tracked Job Listing and returns a warning if one looks like a
// likely duplicate. It never fails the save: a listing-read error here is
// swallowed (nil warning) rather than surfaced as a 500, since the save
// itself already succeeded by the time this runs.
func findDuplicateWarningBestEffort(dataDir string, saved tracking.JobListing) *tracking.DuplicateMatch {
	all, err := tracking.List(dataDir)
	if err != nil {
		return nil
	}
	existing := make([]tracking.JobListing, 0, len(all))
	for _, lwa := range all {
		existing = append(existing, lwa.JobListing)
	}
	match, found := tracking.FindLikelyDuplicate(saved, existing)
	if !found {
		return nil
	}
	return &match
}

// captureJobListingRequest is what a browser extension's content script can
// trivially read off a job posting page it's already viewing (PRD "Browser
// Extension (LinkedIn Capture)", story 2). Location was accepted and
// discarded until issue #206 gave tracking.JobListing a Location field; it
// is now persisted like Title and LogoURL.
type captureJobListingRequest struct {
	Title       string `json:"title"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	URL         string `json:"url"`
	Description string `json:"description"`
	LogoURL     string `json:"logoUrl"`
	// ListingSalaryText is LinkedIn's own salary-insight badge text,
	// captured separately from Description (ADR-0014) — empty when the
	// listing has no such badge, which resolveRALBestEffort treats
	// exactly as today (Job-Description-text-only resolution).
	ListingSalaryText string `json:"listingSalaryText"`
	// Resolution is the client's answer to the same-company question
	// (issue #206). Absent means "no decision yet", which is what raises
	// the company-has-listings 409 — so the warning cannot be skipped by a
	// client that did not look first (story 24).
	Resolution *captureResolution `json:"resolution"`
}

// captureResolution is what the card sends once the user has decided.
type captureResolution struct {
	Kind         string `json:"kind"`
	JobListingID string `json:"jobListingId"`
}

const (
	// resolutionSaveAnyway adds the role alongside the company's existing
	// ones. It answers the company question only — the Posting Key gate
	// lives in tracking.Save and is not skippable.
	resolutionSaveAnyway = "save-anyway"
	// resolutionReplace saves the new role and archives one existing role
	// at the same company, in one action. Archive, never delete, so the
	// replaced Application's Status history, Notes and Generations survive
	// and the choice is reversible (story 18).
	resolutionReplace = "replace"
	// resolutionUnarchiveExisting brings a listing back from the archive
	// and saves nothing. It answers a duplicate-posting refusal whose
	// match turned out to be archived (story 8), so it is handled before
	// the Posting Key gate — which would otherwise refuse it again.
	resolutionUnarchiveExisting = "unarchive-existing"
)

// companyHasListingsReason is the machine-readable tag on the
// same-company question.
const companyHasListingsReason = "company-has-listings"

// companyConflictResponse asks the client to decide before a save at a
// company already being chased. Nothing is written and no Claude call is
// made while the question stands.
type companyConflictResponse struct {
	Reason  string        `json:"reason"`
	Message string        `json:"message"`
	Company companyLookup `json:"company"`
}

// parseArchivedView reads the archived query parameter (issue #98) shared by
// GET /api/job-listings and GET /api/applications. An absent parameter means
// the safe default, non-archived only. The default lives here rather than in
// tracking.FilterParams, whose zero value must keep matching everything.
func parseArchivedView(query url.Values) (tracking.ArchivedView, error) {
	switch archived := query.Get("archived"); archived {
	case "", string(tracking.ArchivedExclude):
		return tracking.ArchivedExclude, nil
	case string(tracking.ArchivedOnly):
		return tracking.ArchivedOnly, nil
	case "all":
		return tracking.ArchivedAll, nil
	default:
		return "", fmt.Errorf("invalid archived: %q (want exclude, only or all)", archived)
	}
}

// parseJobListingsFilter reads the optional status/company/savedFrom/savedTo
// query parameters (issue #45) and archived (issue #98), returning a
// descriptive error for any value that can't be parsed rather than silently
// ignoring it.
func parseJobListingsFilter(query url.Values) (tracking.FilterParams, error) {
	var params tracking.FilterParams

	archived, err := parseArchivedView(query)
	if err != nil {
		return params, err
	}
	params.Archived = archived

	if status := query.Get("status"); status != "" {
		switch tracking.Status(status) {
		case tracking.StatusSaved, tracking.StatusTailoring, tracking.StatusSent,
			tracking.StatusInterviewing, tracking.StatusRejected, tracking.StatusOffer,
			tracking.StatusWithdrawn:
			params.Status = tracking.Status(status)
		default:
			return params, fmt.Errorf("invalid status: %q", status)
		}
	}

	params.Company = query.Get("company")
	params.Location = query.Get("location")

	if raw := query.Get("savedFrom"); raw != "" {
		from, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return params, fmt.Errorf("invalid savedFrom date: %q", raw)
		}
		params.SavedFrom = &from
	}

	if raw := query.Get("savedTo"); raw != "" {
		to, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return params, fmt.Errorf("invalid savedTo date: %q", raw)
		}
		params.SavedTo = &to
	}

	return params, nil
}

func listJobListingsHandler(dataDir, projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := parseJobListingsFilter(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		listings, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		listings = tracking.FilterListings(listings, filter)

		q := r.URL.Query()
		if min, max, currency, active, err := parseRALFilter(q); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		} else if active {
			listings = tracking.FilterListingsByRAL(listings, min, max, currency)
		}

		if q.Get("sort") == "ral" {
			order := tracking.SortOrderAsc
			if q.Get("order") == "desc" {
				order = tracking.SortOrderDesc
			}
			listings = tracking.SortListingsByRAL(listings, order)
		}

		// An Application and its Job Listing are two separate files
		// sharing one id, so the combined shape carries both tokens: the
		// Application patches check the Application's, the Job Listing
		// delete checks the Job Listing's (issue #89).
		summaries := make([]jobListingSummaryWithApplication, len(listings))
		for i := range listings {
			attachStaleEntries(&listings[i].Application, dataDir, projectRoot)
			attachApplicationVersion(&listings[i].Application, dataDir)
			attachJobListingVersion(&listings[i].JobListing, dataDir)
			summaries[i] = summarizeListing(listings[i])
		}
		writeJSON(w, http.StatusOK, summaries)
	}
}

// jobListingSummary is a Job Listing as GET /api/job-listings returns it
// (issue #97): every field the list view renders, and only whether the Job
// Listing holds a Job Description rather than the text itself, which can be
// several kilobytes per posting. GET /api/job-listings/{id} stays the one
// place the Job Description text comes from.
type jobListingSummary struct {
	SchemaVersion      int                      `json:"schemaVersion"`
	ID                 string                   `json:"id"`
	Title              string                   `json:"title,omitempty"`
	Company            string                   `json:"company"`
	Location           string                   `json:"location,omitempty"`
	URL                string                   `json:"url,omitempty"`
	Source             string                   `json:"source"`
	SavedAt            string                   `json:"savedAt"`
	HasJobDescription  bool                     `json:"hasJobDescription"`
	RAL                generation.RALRange      `json:"ral"`
	Logo               string                   `json:"logo,omitempty"`
	FreshnessStatus    tracking.FreshnessStatus `json:"freshnessStatus"`
	FreshnessCheckedAt string                   `json:"freshnessCheckedAt,omitempty"`
	Archived           bool                     `json:"archived"`
	// Version is the Job Listing file's version token (issue #89), carried
	// over from the whole record so a list row can still guard a delete.
	Version string `json:"version,omitempty"`
}

// jobListingSummaryWithApplication is one list row: a Job Listing summary
// paired with its whole Application, Generation history and stale-Entry
// information included, since list rows read both.
type jobListingSummaryWithApplication struct {
	JobListing  jobListingSummary    `json:"jobListing"`
	Application tracking.Application `json:"application"`
}

// summarizeListing projects a whole record down to its list row. It runs
// after filtering and sorting, none of which read the Job Description.
func summarizeListing(l tracking.ListingWithApplication) jobListingSummaryWithApplication {
	listing := l.JobListing
	return jobListingSummaryWithApplication{
		JobListing: jobListingSummary{
			SchemaVersion:      listing.SchemaVersion,
			ID:                 listing.ID,
			Title:              listing.Title,
			Company:            listing.Company,
			Location:           listing.Location,
			URL:                listing.URL,
			Source:             listing.Source,
			SavedAt:            listing.SavedAt,
			HasJobDescription:  listing.JobDescription != "",
			RAL:                listing.RAL,
			Logo:               listing.Logo,
			FreshnessStatus:    listing.FreshnessStatus,
			FreshnessCheckedAt: listing.FreshnessCheckedAt,
			Archived:           listing.Archived,
			Version:            listing.Version,
		},
		Application: l.Application,
	}
}

// getJobListingHandler returns a Job Listing paired with its 1:1
// Application — the list endpoint's row shape, including the read-time
// stale-Entry attachment, but with the whole Job Listing and its Job
// Description text rather than a summary (issue #97) — so the Job Listing detail page
// (issue #94) can render Status, Application Method, Contact, Generation
// history and the stale-Entry notice from one request. projectRoot is what
// that attachment compares Master Data modification times against, exactly
// as in listJobListingsHandler.
func getJobListingHandler(dataDir, projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		listing, err := tracking.Get(dataDir, id)
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "job listing not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attachStaleEntries(&listing.Application, dataDir, projectRoot)
		attachApplicationVersion(&listing.Application, dataDir)
		attachJobListingVersion(&listing.JobListing, dataDir)
		writeJSON(w, http.StatusOK, listing)
	}
}

// attachJobListingVersion populates listing.Version, the read-time token
// the Job Listing delete is checked against (issue #89).
func attachJobListingVersion(listing *tracking.JobListing, dataDir string) {
	version, err := tracking.JobListingVersion(dataDir, listing.ID)
	if err != nil {
		return
	}
	listing.Version = version
}

// getJobListingLogoHandler serves the Company Logo file tracking.Save
// downloaded for the Job Listing identified by id (story 8), 404 when the
// listing doesn't exist or has no logo (story 9). Content-Type is left to
// http.ServeFile's own extension-based sniffing, since the file on disk
// already carries the extension downloadLogoBestEffort picked for it.
func getJobListingLogoHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		listing, err := tracking.GetJobListing(dataDir, id)
		if errors.Is(err, os.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if listing.Logo == "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dataDir, "jobs", listing.Logo))
	}
}

// deleteJobListingHandler removes a Job Listing and its 1:1 Application
// together (story 9), mirroring deleteEntryHandler's shape exactly.
func deleteJobListingHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := tracking.DeleteIfMatch(dataDir, id, requestVersion(r), strings.TrimSpace(r.Header.Get(applicationVersionHeader)))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "job listing not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, recordversion.ErrMismatch) {
			writeConflict(w, "Job Listing or its Application")
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// suggestContactHandler researches a Contact suggestion for the Job
// Listing identified by id (story 7). It never persists — the FE must
// PATCH /api/applications/{id}/contact to save it once the user confirms.
func suggestContactHandler(dataDir string, client tracking.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		contact, err := tracking.SuggestContact(r.Context(), dataDir, client, id)
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "job listing not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, contact)
	}
}

// resolveJobListingHandler re-attempts RAL Range resolution and/or
// Application Method inference for the Job Listing identified by id,
// whichever is currently Unresolved (stories 9-13), returning the same
// {jobListing, application} shape Save's endpoints return. 404 when id
// doesn't exist (story 17), matching the existing GetJobListing/
// suggest-contact pattern.
func resolveJobListingHandler(dataDir string, client tracking.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		listing, application, err := tracking.Resolve(r.Context(), dataDir, client, id)
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "job listing not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attachJobListingVersion(&listing, dataDir)
		attachApplicationVersion(&application, dataDir)
		writeJSON(w, http.StatusOK, saveJobListingResponse{JobListing: listing, Application: application})
	}
}

// createJobListingHandler backs both the manual-paste save path and the
// ATS-browse save path — the latter may supply a LogoURL (issue #42, story
// 4-9), downloaded via doer the same way the extension-capture path
// already does (see captureJobListingFromExtensionHandler).
func createJobListingHandler(dataDir string, client tracking.Client, doer tracking.HTTPDoer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveJobListingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}

		listing, application, err := tracking.Save(r.Context(), dataDir, client, doer, tracking.SaveRequest{
			Title:             req.Title,
			Company:           req.Company,
			Location:          req.Location,
			URL:               req.URL,
			JobDescription:    req.JobDescription,
			JobDescriptionURL: req.JobDescriptionURL,
			LogoURL:           req.LogoURL,
		})
		if handleSaveError(w, err) {
			return
		}
		attachJobListingVersion(&listing, dataDir)
		attachApplicationVersion(&application, dataDir)
		writeJSON(w, http.StatusCreated, saveJobListingResponse{
			JobListing:                listing,
			Application:               application,
			DuplicateWarning:          findDuplicateWarningBestEffort(dataDir, listing),
			CompletedPendingCaptureID: completePendingCaptureBestEffort(dataDir, listing),
		})
	}
}

// checkFreshnessResponse is check-freshness's response shape — just the
// updated Job Listing, unlike resolve's {jobListing, application} pair,
// since freshness never touches the Application record. The correction
// route (PATCH) answers with it too, for the same reason: correcting a Job
// Listing never touches its Application.
type checkFreshnessResponse struct {
	JobListing tracking.JobListing `json:"jobListing"`
}

// checkFreshnessHandler triggers an on-demand freshness check (issue #59,
// "Job Description link-rot / staleness check") against the Job Listing
// identified by id's own source URL, using the same injectable HTTP doer
// already threaded through for ATS sourcing (a real *http.Client in
// production — see RouterConfig.ATSHTTPDoer — a fixture doer in tests). 404 when id
// doesn't exist, matching resolve/suggest-contact's existing pattern.
func checkFreshnessHandler(dataDir string, doer tracking.HTTPDoer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		listing, err := tracking.CheckFreshness(r.Context(), dataDir, doer, id)
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "job listing not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attachJobListingVersion(&listing, dataDir)
		writeJSON(w, http.StatusOK, checkFreshnessResponse{JobListing: listing})
	}
}

// archiveJobListingResponse is archive/unarchive's response shape — just the
// updated Job Listing, like check-freshness's, since archiving never
// touches the Application record.
type archiveJobListingResponse struct {
	JobListing tracking.JobListing `json:"jobListing"`
}

// setJobListingArchivedHandler backs both POST /api/job-listings/{id}/archive
// (archived true) and /unarchive (archived false) (issue #98). Both are
// idempotent and 404 when id doesn't exist, matching the other id-addressed
// Job Listing routes. Both honour an optional If-Match carrying the Job
// Listing's version token, since they rewrite the Job Listing file (issue
// #89), and answer with the fresh token.
func setJobListingArchivedHandler(dataDir string, archived bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		listing, err := tracking.SetArchivedIfMatch(dataDir, id, archived, requestVersion(r))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "job listing not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, recordversion.ErrMismatch) {
			writeConflict(w, "Job Listing")
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attachJobListingVersion(&listing, dataDir)
		writeJSON(w, http.StatusOK, archiveJobListingResponse{JobListing: listing})
	}
}

// captureJobListingCORSPreflightHandler answers the browser's CORS preflight
// for the extension's ingestion endpoint. The caller runs on a
// moz-extension:// or chrome-extension:// origin, which is CORS-checked
// like any other cross-origin fetch on at least some Firefox versions,
// regardless of the extension's declared host_permissions (observed in manual
// verification), so this endpoint must answer preflight and carry CORS
// headers itself rather than relying on that exemption.
func captureJobListingCORSPreflightHandler(w http.ResponseWriter, r *http.Request) {
	corsPreflight("POST")(w, r)
}

// corsPreflight answers a preflight for one of the routes the extension
// calls cross-origin. If-Match is allowed because the card presents the
// Application's version token on a Status move, exactly as the frontend
// does (issue #89).
func corsPreflight(method string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", method+", OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, If-Match, "+lanAuthHeader)
		w.WriteHeader(http.StatusNoContent)
	}
}

// captureJobListingFromExtensionHandler is the browser extension's ingestion
// endpoint (story 3): it normalizes a page capture into tracking.SaveRequest
// and reuses PRD 3's Save path exactly, the same way PRD 4's ATS browse view
// reuses POST /api/job-listings from the frontend.
func captureJobListingFromExtensionHandler(dataDir string, client tracking.Client, doer tracking.HTTPDoer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		var req captureJobListingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		kind := ""
		if req.Resolution != nil {
			kind = req.Resolution.Kind
		}
		switch kind {
		case "", resolutionSaveAnyway, resolutionReplace, resolutionUnarchiveExisting:
		default:
			http.Error(w, "unknown resolution kind: "+kind, http.StatusBadRequest)
			return
		}

		// Bringing an archived match back answers a duplicate-posting
		// refusal, so it runs before the Posting Key gate would refuse it
		// again — and saves nothing.
		if kind == resolutionUnarchiveExisting {
			unarchiveExistingListing(w, dataDir, req.Resolution.JobListingID)
			return
		}

		listings, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// The Posting Key refusal is the more specific answer, so it wins
		// over the company question: asking someone to decide about a
		// company's other roles, only to refuse the save they then ask
		// for, would be a wasted round trip and a misleading one.
		// tracking.Save re-checks it regardless — that is where the
		// invariant lives (ADR-0042).
		if existing, duplicate := findTrackedPosting(listings, req.URL); duplicate {
			writeDuplicatePostingConflict(w, existing)
			return
		}

		siblings := activeSiblings(listings, req.Company, "")
		if kind == "" && len(siblings) > 0 {
			writeJSON(w, http.StatusConflict, companyConflictResponse{
				Reason:  companyHasListingsReason,
				Message: "You already track other roles at this company.",
				Company: companyLookup{Listings: siblings},
			})
			return
		}

		// A replace target is validated before anything is written, so a
		// stale choice fails cleanly instead of half-applying (story 19).
		var replacing string
		if kind == resolutionReplace {
			target, ok := validReplaceTarget(listings, req.Resolution.JobListingID, req.Company)
			if !ok {
				writeJSON(w, http.StatusConflict, saveConflictResponse{
					Reason:  replaceTargetUnavailableReason,
					Message: "The Job Listing you chose to replace can no longer be replaced. Reload and choose again.",
				})
				return
			}
			replacing = target
		}

		listing, application, err := tracking.Save(r.Context(), dataDir, client, doer, tracking.SaveRequest{
			Title:             req.Title,
			Company:           req.Company,
			Location:          req.Location,
			URL:               req.URL,
			JobDescription:    req.Description,
			LogoURL:           req.LogoURL,
			ListingSalaryText: req.ListingSalaryText,
		})
		if handleSaveError(w, err) {
			return
		}

		archived, archiveFailed := "", false
		if replacing != "" {
			if _, err := tracking.SetArchived(dataDir, replacing, true); err != nil {
				log.Printf("api: archiving replaced job listing %s: %v", replacing, err)
				archiveFailed = true
			} else {
				archived = replacing
			}
		}

		attachJobListingVersion(&listing, dataDir)
		attachApplicationVersion(&application, dataDir)
		writeJSON(w, http.StatusCreated, saveJobListingResponse{
			JobListing:                listing,
			Application:               application,
			DuplicateWarning:          findDuplicateWarningBestEffort(dataDir, listing),
			CompletedPendingCaptureID: completePendingCaptureBestEffort(dataDir, listing),
			ArchivedJobListingID:      archived,
			ArchiveFailed:             archiveFailed,
		})
	}
}

// replaceTargetUnavailableReason marks a replace whose chosen target is no
// longer one the user could have chosen — deleted, already archived, or at
// another company. The card was stale; nothing is written.
const replaceTargetUnavailableReason = "replace-target-unavailable"

// unarchiveExistingListing brings one Job Listing back from the archive
// and answers with it. It writes no new record.
func unarchiveExistingListing(w http.ResponseWriter, dataDir, id string) {
	listing, err := tracking.SetArchived(dataDir, id, false)
	if errors.Is(err, os.ErrNotExist) {
		http.Error(w, "job listing not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pair, err := tracking.Get(dataDir, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	application := pair.Application
	attachJobListingVersion(&listing, dataDir)
	attachApplicationVersion(&application, dataDir)
	writeJSON(w, http.StatusOK, saveJobListingResponse{JobListing: listing, Application: application})
}

// findTrackedPosting returns the Job Listing already holding rawURL's
// posting, archived or not.
func findTrackedPosting(listings []tracking.ListingWithApplication, rawURL string) (tracking.JobListing, bool) {
	key, ok := postingkey.Of(rawURL)
	if !ok {
		return tracking.JobListing{}, false
	}
	for _, l := range listings {
		if other, ok := postingkey.Of(l.JobListing.URL); ok && other == key {
			return l.JobListing, true
		}
	}
	return tracking.JobListing{}, false
}

// activeSiblings are the non-archived roles tracked at company, excluding
// excludeID. Archived listings are left out entirely: a company worked
// through and closed out stops interrupting (stories 22, 23).
func activeSiblings(listings []tracking.ListingWithApplication, company, excludeID string) []siblingListing {
	siblings := []siblingListing{}
	for _, l := range listings {
		if l.JobListing.Archived || l.JobListing.ID == excludeID {
			continue
		}
		if !tracking.SameCompany(l.JobListing.Company, company) {
			continue
		}
		siblings = append(siblings, siblingListing{
			ID:      l.JobListing.ID,
			Title:   l.JobListing.Title,
			SavedAt: l.JobListing.SavedAt,
			Status:  l.Application.Status,
		})
	}
	return siblings
}

// validReplaceTarget checks the listing the user chose to replace is still
// one they could have chosen: it exists, it is not already archived, and
// it belongs to the company being saved. Anything else means the card was
// stale, and the save is refused with nothing written.
func validReplaceTarget(listings []tracking.ListingWithApplication, id, company string) (string, bool) {
	for _, l := range listings {
		if l.JobListing.ID != id {
			continue
		}
		if l.JobListing.Archived || !tracking.SameCompany(l.JobListing.Company, company) {
			return "", false
		}
		return id, true
	}
	return "", false
}

// completePendingCaptureBestEffort removes the Pending Capture for the
// posting just saved, if any. Best-effort like the duplicate warning: the
// Job Listing is already on disk, so a failure here only leaves a stale
// entry in To complete for the user to dismiss.
func completePendingCaptureBestEffort(dataDir string, listing tracking.JobListing) string {
	id, err := tracking.RemovePendingCaptureFor(dataDir, listing.URL)
	if err != nil {
		log.Printf("api: completing pending capture for %s: %v", listing.URL, err)
	}
	return id
}

// duplicatePostingReason is the machine-readable tag on the refusal, so a
// client can tell "this posting is already tracked" apart from any other
// 409 the API may answer with.
const duplicatePostingReason = "duplicate-posting"

// existingJobListingRef is the Job Listing a duplicate-posting refusal
// names: enough to recognise it, link to it and decide what to do, without
// shipping the whole record (and its Job Description) back on an error.
type existingJobListingRef struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	Company string `json:"company"`
	SavedAt string `json:"savedAt"`
	// Archived is what lets a client additionally offer to unarchive the
	// match (issue #206, story 8). It never changes the message: an
	// archived match reads exactly like any other refusal (story 7).
	Archived bool `json:"archived"`
}

// saveConflictResponse is the body of a refused save. Every save path
// answers with it, so a client has one shape to read whichever door it
// came in through (story 9).
type saveConflictResponse struct {
	Reason   string                 `json:"reason"`
	Message  string                 `json:"message"`
	Existing *existingJobListingRef `json:"existing,omitempty"`
}

// writeDuplicatePostingConflict answers a save refused because the posting
// is already tracked. Shared by every route that calls tracking.Save, so
// the refusal is a property of the record rather than of the route.
func writeDuplicatePostingConflict(w http.ResponseWriter, existing tracking.JobListing) {
	writeJSON(w, http.StatusConflict, saveConflictResponse{
		Reason:  duplicatePostingReason,
		Message: "This posting is already saved as a Job Listing.",
		Existing: &existingJobListingRef{
			ID:       existing.ID,
			Title:    existing.Title,
			Company:  existing.Company,
			SavedAt:  existing.SavedAt,
			Archived: existing.Archived,
		},
	})
}

// handleSaveError maps the errors tracking.Save can return to their HTTP
// answers, and reports whether it handled one. Every save path routes
// through it so validation and the duplicate refusal stay identical
// across them.
func handleSaveError(w http.ResponseWriter, err error) bool {
	var duplicate *tracking.DuplicatePostingError
	switch {
	case err == nil:
		return false
	case errors.As(err, &duplicate):
		writeDuplicatePostingConflict(w, duplicate.Existing)
	case errors.Is(err, tracking.ErrValidation):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return true
}

// patchJobListingRequest is a correction (issue #206, stories 61-70).
// Every field is a pointer so "leave it alone" differs from "set it to
// empty": clearing a Job Title is a real correction, and a field nobody
// sent must never be overwritten.
//
// URL and JobDescription are present only to be refused. Silently ignoring
// them would leave a client believing an edit had applied; the record's
// identity and the text every Generation was derived from stay fixed, and
// saying so is more useful than saying nothing.
type patchJobListingRequest struct {
	Title    *string          `json:"title"`
	Company  *string          `json:"company"`
	Location *string          `json:"location"`
	RAL      *patchRALRange   `json:"ral"`
	URL      *json.RawMessage `json:"url"`
	JobDesc  *json.RawMessage `json:"jobDescription"`
}

// patchRALRange is a figure the user entered themselves. It carries no
// source: the backend stamps RALSourceManual, so a client cannot claim a
// figure was stated by the posting (story 65).
type patchRALRange struct {
	Min      int    `json:"min"`
	Max      int    `json:"max"`
	Currency string `json:"currency"`
}

// patchJobListingHandler applies a correction to a Job Listing, honouring
// an optional If-Match like every other record-writing route (issue #89).
func patchJobListingHandler(dataDir, projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req patchJobListingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if req.URL != nil {
			http.Error(w, "a Job Listing's url cannot be corrected: it is the record's identity, and one Job Listing exists per posting", http.StatusBadRequest)
			return
		}
		if req.JobDesc != nil {
			http.Error(w, "a Job Listing's jobDescription cannot be corrected: every recorded Generation was tailored to the text it holds", http.StatusBadRequest)
			return
		}

		correction := tracking.JobListingCorrection{
			Title:    req.Title,
			Company:  req.Company,
			Location: req.Location,
		}
		if req.RAL != nil {
			min, max := req.RAL.Min, req.RAL.Max
			correction.RAL = &generation.RALRange{Min: &min, Max: &max, Currency: req.RAL.Currency}
			normalized := tracking.NormalizeManualRAL(*correction.RAL)
			correction.RAL = &normalized
		}

		listing, err := tracking.CorrectJobListingIfMatch(dataDir, r.PathValue("id"), correction, requestVersion(r))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "job listing not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, recordversion.ErrMismatch) {
			writeConflict(w, "Job Listing")
			return
		}
		if errors.Is(err, tracking.ErrValidation) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attachJobListingVersion(&listing, dataDir)
		writeJSON(w, http.StatusOK, checkFreshnessResponse{JobListing: listing})
	}
}
