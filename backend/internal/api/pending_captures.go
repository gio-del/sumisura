package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// addPendingCaptureRequest is what a share entry point sends (issue #182):
// the PWA share target forwards the share's url/text/title as-is, and an
// iOS Shortcut sends whatever the share sheet gave it. At least one of URL
// and Text must contain an http(s) link.
type addPendingCaptureRequest struct {
	URL   string `json:"url"`
	Text  string `json:"text"`
	Title string `json:"title"`
}

// addPendingCaptureResponse says what sharing did. Message is a ready-made
// sentence, so a client with no UI of its own (an iOS Shortcut's
// notification) can show it without branching on Outcome.
type addPendingCaptureResponse struct {
	Outcome        tracking.PendingCaptureOutcome `json:"outcome"`
	Message        string                         `json:"message"`
	PendingCapture *tracking.PendingCapture       `json:"pendingCapture,omitempty"`
	JobListingID   string                         `json:"jobListingId,omitempty"`
}

var pendingCaptureMessages = map[tracking.PendingCaptureOutcome]string{
	tracking.OutcomePending:        "Saved to To complete.",
	tracking.OutcomeAlreadyPending: "Already waiting in To complete.",
	tracking.OutcomeAlreadyTracked: "Already tracked as a Job Listing.",
}

// addPendingCaptureHandler answers 201 when a Pending Capture was written
// and 200 when nothing was (already pending, already tracked) — both are
// successful shares from the user's point of view.
func addPendingCaptureHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req addPendingCaptureRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		result, err := tracking.AddPendingCapture(dataDir, tracking.PendingCaptureInput{URL: req.URL, Text: req.Text, Title: req.Title})
		if errors.Is(err, tracking.ErrValidation) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		status := http.StatusOK
		if result.Outcome == tracking.OutcomePending {
			status = http.StatusCreated
		}
		writeJSON(w, status, addPendingCaptureResponse{
			Outcome:        result.Outcome,
			Message:        pendingCaptureMessages[result.Outcome],
			PendingCapture: result.PendingCapture,
			JobListingID:   result.JobListingID,
		})
	}
}

func listPendingCapturesHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		captures, err := tracking.ListPendingCaptures(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, emptyIfNil(captures))
	}
}

// deletePendingCaptureHandler dismisses a Pending Capture. No If-Match:
// a Pending Capture is never edited, only created and removed, so there is
// no lost update to protect against.
func deletePendingCaptureHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := tracking.DeletePendingCapture(dataDir, r.PathValue("id"))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "pending capture not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type completePendingCaptureRequest struct {
	Company        string `json:"company"`
	Title          string `json:"title"`
	JobDescription string `json:"jobDescription"`
}

// completePendingCaptureHandler turns a Pending Capture into a Job Listing
// with the Job Description the user pasted, answering exactly like a
// manual save (POST /api/job-listings), duplicate warning included.
func completePendingCaptureHandler(dataDir string, client tracking.Client, doer tracking.HTTPDoer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req completePendingCaptureRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		listing, application, err := tracking.CompletePendingCapture(r.Context(), dataDir, client, doer, r.PathValue("id"), tracking.CompletePendingCaptureRequest{
			Company:        req.Company,
			Title:          req.Title,
			JobDescription: req.JobDescription,
		})
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "pending capture not found", http.StatusNotFound)
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
		attachApplicationVersion(&application, dataDir)
		writeJSON(w, http.StatusCreated, saveJobListingResponse{
			JobListing:       listing,
			Application:      application,
			DuplicateWarning: findDuplicateWarningBestEffort(dataDir, listing),
		})
	}
}
