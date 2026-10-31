package api

import (
	"time"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// attachStaleEntries populates StaleEntries on each of application's
// GenerationRecords (issue #52, story 2, 8), read-time only — never
// persisted, since it's a mechanical timestamp comparison against Master
// Data's current state, not durable history. A record with no stored
// EntryIDs (persisted before this field existed) is left untouched: story
// 10's "unknown, not stale" degrade.
func attachStaleEntries(application *tracking.Application, dataDir, projectRoot string) {
	for i := range application.Generations {
		record := &application.Generations[i]
		if len(record.EntryIDs) == 0 {
			continue
		}

		createdAt, err := time.Parse(time.RFC3339Nano, record.CreatedAt)
		if err != nil {
			continue
		}

		staleIDs := tracking.StaleEntries(record.EntryIDs, createdAt, func(entryID string) (time.Time, bool) {
			lm, err := masterdata.EntryLastModified(projectRoot, dataDir, entryID)
			if err != nil || lm.At == "" {
				return time.Time{}, false
			}
			t, err := time.Parse(time.RFC3339Nano, lm.At)
			if err != nil {
				return time.Time{}, false
			}
			return t, true
		})

		for _, entryID := range staleIDs {
			record.StaleEntries = append(record.StaleEntries, entryLabel(dataDir, entryID))
		}
	}
}

// entryLabel names a Master Data Entry by employer/client + role, per
// CONTEXT.md's Entry shape (issue #52, story 3) — falling back to the raw
// id if the Entry can no longer be read (deleted since the Generation ran).
func entryLabel(dataDir, entryID string) string {
	entry, err := masterdata.GetEntry(dataDir, entryID)
	if err != nil {
		return entryID
	}

	name := entry.Employer
	if name == "" {
		name = entry.Client
	}
	if name == "" {
		name = entry.Name
	}
	if entry.Role != "" {
		if name != "" {
			return name + " – " + entry.Role
		}
		return entry.Role
	}
	if name != "" {
		return name
	}
	return entryID
}
