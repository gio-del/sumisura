package api

import (
	"github.com/gio-del/sumisura/backend/internal/masterdata"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// encoding/json sends a nil slice as null, but the frontend declares these
// list fields as arrays (frontend/src/api/types.ts) and maps over them
// unguarded. A nil slice here is ordinary — a Snippet or Entry file with no
// tags key, a profile.yaml without a Static Section (yaml omitempty drops an
// emptied one on write), no Applications to compute stats from — so each is
// turned into an empty one at the wire, leaving the domain types' own nil
// semantics alone (issue #99, caught by the contract fixtures).

func emptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func entryForWire(entry masterdata.Entry) masterdata.Entry {
	entry.Tags = emptyIfNil(entry.Tags)
	return entry
}

func snippetForWire(snippet masterdata.Snippet) masterdata.Snippet {
	snippet.Tags = emptyIfNil(snippet.Tags)
	return snippet
}

func profileForWire(profile masterdata.Profile) masterdata.Profile {
	profile.Education = emptyIfNil(profile.Education)
	profile.Publications = emptyIfNil(profile.Publications)
	profile.Certifications = emptyIfNil(profile.Certifications)
	profile.Awards = emptyIfNil(profile.Awards)
	profile.Activities = emptyIfNil(profile.Activities)
	profile.Languages = emptyIfNil(profile.Languages)
	return profile
}

func statsForWire(stats tracking.Stats) tracking.Stats {
	stats.Counts = emptyIfNil(stats.Counts)
	stats.Conversions = emptyIfNil(stats.Conversions)
	stats.TimeInStage = emptyIfNil(stats.TimeInStage)
	return stats
}
