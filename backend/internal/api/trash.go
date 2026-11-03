package api

import (
	"errors"
	"net/http"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// trashList is the wire shape of GET /api/master-data/trash's body.
type trashList struct {
	Items []masterdata.TrashItem `json:"items"`
}

// trashRestored is the wire shape of the restore route's body.
type trashRestored struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

func listTrashHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := masterdata.ListTrash(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, trashList{Items: items})
	}
}

func restoreTrashHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		items, err := masterdata.ListTrash(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		kind := ""
		for _, it := range items {
			if it.ID == id {
				kind = it.Kind
			}
		}
		restored, err := masterdata.RestoreTrash(dataDir, id)
		if errors.Is(err, masterdata.ErrTrashNotFound) {
			http.Error(w, "trashed item not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, trashRestored{ID: restored, Kind: kind})
	}
}

func purgeTrashHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := masterdata.PurgeTrash(dataDir, r.PathValue("id"))
		if errors.Is(err, masterdata.ErrTrashNotFound) {
			http.Error(w, "trashed item not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
