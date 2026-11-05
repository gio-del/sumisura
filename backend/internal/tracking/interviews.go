package tracking

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// UpdateApplicationInterviewsIfMatch replaces the Application's interviews
// with the given list, or clears them when it is empty (issue #283),
// refusing the write with recordversion.ErrMismatch when version no longer
// matches the file on disk. An empty version writes unconditionally. Every
// At must be a YYYY-MM-DDTHH:MM local date and time; the list is stored
// soonest first.
func UpdateApplicationInterviewsIfMatch(dataDir, id string, interviews []Interview, version string) (Application, error) {
	cleaned := make([]Interview, 0, len(interviews))
	for i, iv := range interviews {
		iv.At = strings.TrimSpace(iv.At)
		iv.Note = strings.TrimSpace(iv.Note)
		iv.Location = strings.TrimSpace(iv.Location)
		if _, err := time.Parse(InterviewAtLayout, iv.At); err != nil {
			return Application{}, fmt.Errorf("%w: interviews[%d].at must be a YYYY-MM-DDTHH:MM date and time", ErrValidation, i)
		}
		cleaned = append(cleaned, iv)
	}
	sort.SliceStable(cleaned, func(i, j int) bool { return cleaned[i].At < cleaned[j].At })
	if len(cleaned) == 0 {
		cleaned = nil
	}

	application, err := getApplication(dataDir, id)
	if err != nil {
		return Application{}, err
	}
	application.Interviews = cleaned

	return writeApplicationIfMatch(dataDir, id, application, version)
}

// NextInterview returns the soonest interview at or after now (local time
// of now), if any.
func NextInterview(interviews []Interview, now time.Time) (Interview, bool) {
	for _, iv := range interviews {
		t, err := time.ParseInLocation(InterviewAtLayout, iv.At, now.Location())
		if err == nil && !t.Before(now) {
			return iv, true
		}
	}
	return Interview{}, false
}
