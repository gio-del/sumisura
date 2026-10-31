package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// maxImportTextBytes bounds a pasted CV; a long CV is a few tens of KB.
const maxImportTextBytes = 256 << 10

// entryProposer is the optional Claude capability behind the Master Data
// import preview. It is a separate interface, not part of tracking.Client,
// so the other Claude-facing code is untouched by it.
type entryProposer interface {
	ProposeEntries(ctx context.Context, cvText string) ([]masterdata.Entry, error)
}

type importPreviewRequest struct {
	Text string `json:"text"`
}

// importDraftWire is one proposed Entry with the bullets that could not be
// found in the pasted text.
type importDraftWire struct {
	Entry     masterdata.Entry `json:"entry"`
	NotInText []string         `json:"notInText"`
}

// importPreviewWire is the wire shape of POST /api/master-data/import/preview.
type importPreviewWire struct {
	Drafts  []importDraftWire `json:"drafts"`
	Skipped int               `json:"skipped"`
}

// importPreviewHandler asks Claude for draft Entries out of the pasted CV
// text and checks every proposed bullet against that text. It writes
// nothing: saving a draft is a separate, per-draft request.
func importPreviewHandler(dataDir string, client any) http.HandlerFunc {
	proposer, _ := client.(entryProposer)
	return func(w http.ResponseWriter, r *http.Request) {
		var req importPreviewRequest
		r.Body = http.MaxBytesReader(w, r.Body, maxImportTextBytes+1024)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Text) == "" {
			http.Error(w, "text is required", http.StatusBadRequest)
			return
		}
		if len(req.Text) > maxImportTextBytes {
			http.Error(w, "text is too long", http.StatusRequestEntityTooLarge)
			return
		}
		if proposer == nil {
			http.Error(w, "this Claude provider does not support importing", http.StatusNotImplemented)
			return
		}

		proposed, err := proposer.ProposeEntries(r.Context(), req.Text)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		entries, skipped := masterdata.DraftEntries(dataDir, proposed)
		drafts := make([]importDraftWire, 0, len(entries))
		for _, e := range entries {
			notInText := []string{}
			for _, b := range e.Bullets {
				if !bulletInText(req.Text, b) {
					notInText = append(notInText, b)
				}
			}
			drafts = append(drafts, importDraftWire{Entry: entryForWire(e), NotInText: notInText})
		}
		writeJSON(w, http.StatusOK, importPreviewWire{Drafts: drafts, Skipped: skipped})
	}
}

// bulletInText reports whether bullet is found in the pasted text: as a
// whitespace- and case-insensitive substring, or else grounded in it by the
// same check Generation runs on rewritten bullets.
func bulletInText(text, bullet string) bool {
	norm := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	if strings.Contains(norm(text), norm(bullet)) {
		return true
	}
	return len(generation.CheckGroundedness([]string{text}, bullet)) == 0
}

// importSaveHandler writes one approved draft under the id the preview gave
// it, and answers 409 when that id already exists.
func importSaveHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var entry masterdata.Entry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
			return
		}
		created, err := masterdata.CreateEntryWithID(dataDir, entry)
		if errors.Is(err, masterdata.ErrEntryExists) {
			http.Error(w, "an entry with this id already exists", http.StatusConflict)
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
		attachEntryVersion(&created, dataDir)
		writeJSON(w, http.StatusCreated, entryForWire(created))
	}
}
