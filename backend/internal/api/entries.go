package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
	"github.com/gio-del/sumisura/backend/internal/recordversion"
)

func listEntriesHandler(dataDir, projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := masterdata.ListEntries(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for i := range entries {
			attachLastModified(&entries[i], dataDir, projectRoot)
			attachEntryVersion(&entries[i], dataDir)
			entries[i] = entryForWire(entries[i])
		}
		writeJSON(w, http.StatusOK, entries)
	}
}

// listEntryProblemsHandler names the Entry files the list skipped (broken
// frontmatter, unreadable file) and why. It is a separate route so the list
// response keeps its shape; nothing is modified or deleted.
func listEntryProblemsHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, problems, err := masterdata.ListEntriesWithProblems(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if problems == nil {
			problems = []masterdata.EntryProblem{}
		}
		writeJSON(w, http.StatusOK, problems)
	}
}

func getEntryHandler(dataDir, projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		entry, err := masterdata.GetEntry(dataDir, id)
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "entry not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attachLastModified(&entry, dataDir, projectRoot)
		attachEntryVersion(&entry, dataDir)
		writeJSON(w, http.StatusOK, entryForWire(entry))
	}
}

// attachEntryVersion populates entry.Version, the read-time token a later
// conditional write is checked against (issue #89) — computed by the API
// layer, exactly like attachLastModified, and never persisted.
func attachEntryVersion(entry *masterdata.Entry, dataDir string) {
	version, err := masterdata.EntryVersion(dataDir, entry.ID)
	if err != nil {
		return
	}
	entry.Version = version
}

// attachLastModified populates entry.LastModified via a git-log lookup,
// leaving it nil (never erroring the request) if there's no history to
// report — see masterdata.EntryLastModified.
func attachLastModified(entry *masterdata.Entry, dataDir, projectRoot string) {
	lm, err := masterdata.EntryLastModified(projectRoot, dataDir, entry.ID)
	if err != nil || lm == (masterdata.LastModified{}) {
		return
	}
	entry.LastModified = &lm
}

func createEntryHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var entry masterdata.Entry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}

		created, err := masterdata.CreateEntry(dataDir, entry)
		if errors.Is(err, masterdata.ErrValidation) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attachEntryVersion(&created, dataDir)
		writeJSON(w, http.StatusCreated, entryForWire(created))
	}
}

func putEntryHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var entry masterdata.Entry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}

		updated, err := masterdata.UpdateEntryIfMatch(dataDir, id, entry, requestVersion(r))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "entry not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, recordversion.ErrMismatch) {
			writeConflict(w, "Entry")
			return
		}
		if errors.Is(err, masterdata.ErrValidation) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		attachEntryVersion(&updated, dataDir)
		writeJSON(w, http.StatusOK, entryForWire(updated))
	}
}

func deleteEntryHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		err := masterdata.DeleteEntryIfMatch(dataDir, id, requestVersion(r))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "entry not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, recordversion.ErrMismatch) {
			writeConflict(w, "Entry")
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status line is already on the wire, so the response can't be
		// turned into an error one — but a half-written body shouldn't look
		// like a successful reply either. Log it.
		log.Printf("api: encoding JSON response body: %v", err)
	}
}
