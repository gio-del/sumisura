package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/recordversion"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

type updateApplicationStatusRequest struct {
	Status tracking.Status `json:"status"`
}

// attachApplicationVersion populates application.Version, the read-time
// token a later conditional write is checked against (issue #89) —
// computed from the Application's own file, not its Job Listing's.
func attachApplicationVersion(application *tracking.Application, dataDir string) {
	version, err := tracking.ApplicationVersion(dataDir, application.ID)
	if err != nil {
		return
	}
	application.Version = version
}

func updateApplicationStatusHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req updateApplicationStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}

		application, err := tracking.UpdateApplicationStatusIfMatch(dataDir, id, req.Status, requestVersion(r))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "application not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, recordversion.ErrMismatch) {
			writeConflict(w, "Application")
			return
		}
		if errors.Is(err, tracking.ErrInvalidTransition) || errors.Is(err, tracking.ErrValidation) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attachApplicationVersion(&application, dataDir)
		writeJSON(w, http.StatusOK, application)
	}
}

type recordGenerationRequest struct {
	Slug             string                         `json:"slug"`
	CVPath           string                         `json:"cvPath"`
	CoverLetterPath  string                         `json:"coverLetterPath"`
	SourceSnippetIDs []string                       `json:"sourceSnippetIds"`
	Usage            generation.GenerationUsage     `json:"usage"`
	Language         string                         `json:"language"`
	Groundedness     *generation.GroundednessResult `json:"groundedness"`
	ATSReports       *generation.ATSReports         `json:"atsReports"`
	EntryIDs         []string                       `json:"entryIds"`
}

// recordApplicationGenerationHandler records a Generation the FE already
// rendered (via POST /api/generations/render) against the Application
// identified by id (story 11) — the linking step, since the render
// pipeline itself has no notion of which Application it's for.
func recordApplicationGenerationHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req recordGenerationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Slug) == "" {
			http.Error(w, "slug is required", http.StatusBadRequest)
			return
		}

		record := tracking.GenerationRecord{
			Slug:             req.Slug,
			CreatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
			CVPath:           req.CVPath,
			CoverLetterPath:  req.CoverLetterPath,
			SourceSnippetIDs: req.SourceSnippetIDs,
			Usage:            req.Usage,
			Language:         req.Language,
			Groundedness:     req.Groundedness,
			ATSReports:       req.ATSReports,
			EntryIDs:         req.EntryIDs,
		}
		application, err := tracking.RecordGeneration(dataDir, id, record)
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "application not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attachApplicationVersion(&application, dataDir)
		writeJSON(w, http.StatusCreated, application)
	}
}

// updateApplicationContactHandler saves a Contact to the Application
// identified by id (story 7) — the explicit confirmation step for both a
// manual entry and an accepted Claude suggestion.
func updateApplicationContactHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req tracking.Contact
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}

		application, err := tracking.UpdateApplicationContactIfMatch(dataDir, id, req, requestVersion(r))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "application not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, recordversion.ErrMismatch) {
			writeConflict(w, "Application")
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
		attachApplicationVersion(&application, dataDir)
		writeJSON(w, http.StatusOK, application)
	}
}

type mailtoResponse struct {
	URI string `json:"uri"`
}

// getApplicationMailtoHandler returns the mailto: draft for the
// Application identified by id (story 9) — the user's own email client
// opens it ready to send; nothing is sent automatically.
func getApplicationMailtoHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		uri, err := tracking.GetMailtoURI(dataDir, id)
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "application not found", http.StatusNotFound)
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
		writeJSON(w, http.StatusOK, mailtoResponse{URI: uri})
	}
}

func updateApplicationMethodHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req tracking.ApplicationMethod
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}

		application, err := tracking.UpdateApplicationMethodIfMatch(dataDir, id, req, requestVersion(r))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "application not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, recordversion.ErrMismatch) {
			writeConflict(w, "Application")
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
		attachApplicationVersion(&application, dataDir)
		writeJSON(w, http.StatusOK, application)
	}
}

// getApplicationsStatsHandler serves the Application funnel/stats view's
// data (issue #36): counts per Status, stage-to-stage conversion rates,
// and time-in-stage, computed from the same tracking.List every other
// Application read already uses — no separate stats-only data store.
func getApplicationsStatsHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		listings, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Archived Job Listings (issue #98) deliberately still count:
		// archiving is a view concern, never a history concern.
		applications := make([]tracking.Application, len(listings))
		for i, l := range listings {
			applications[i] = l.Application
		}
		writeJSON(w, http.StatusOK, statsForWire(tracking.ComputeStats(applications)))
	}
}

// applicationGroupResponse is one Status group as GET /api/applications
// returns it: its items are list rows (a Job Listing summary without the Job
// Description text, paired with its Application), never whole records.
type applicationGroupResponse struct {
	Status tracking.Status                    `json:"status"`
	Count  int                                `json:"count"`
	Items  []jobListingSummaryWithApplication `json:"items"`
}

type applicationGroupsResponse struct {
	Total  int                        `json:"total"`
	Groups []applicationGroupResponse `json:"groups"`
}

// listApplicationsHandler serves the Applications view (issue #95): every
// tracked Application grouped under its Status, most overdue first within
// each group. It is wired exactly like the stats handler: load every record
// once with tracking.List, hand the loaded set to a pure function
// (tracking.GroupApplications), serialize the result.
//
// Archived Job Listings are left out by default and selected with the same
// archived=exclude|only|all parameter GET /api/job-listings takes: archiving
// is how the user takes a Job Listing off their plate, and this view is the
// plate. Stats still counts them, since it is about history, not today.
func listApplicationsHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		archived, err := parseArchivedView(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		listings, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		listings = tracking.FilterListings(listings, tracking.FilterParams{Archived: archived})

		grouped := tracking.GroupApplications(listings, time.Now(), tracking.DefaultStaleThreshold)
		response := applicationGroupsResponse{
			Total:  grouped.Total,
			Groups: make([]applicationGroupResponse, len(grouped.Groups)),
		}
		for i, group := range grouped.Groups {
			items := make([]jobListingSummaryWithApplication, len(group.Items))
			for j, item := range group.Items {
				// A row's Status move presents the Application's token, so
				// the grouped view carries both tokens exactly as the Job
				// Listings list rows do (issue #89).
				attachApplicationVersion(&item.Application, dataDir)
				attachJobListingVersion(&item.JobListing, dataDir)
				items[j] = summarizeListing(item)
			}
			response.Groups[i] = applicationGroupResponse{Status: group.Status, Count: group.Count, Items: items}
		}
		writeJSON(w, http.StatusOK, response)
	}
}
