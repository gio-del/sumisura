package tracking_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gio-del/sumisura/backend/internal/recordversion"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func TestUpdateApplicationInterviewsIfMatch(t *testing.T) {
	dataDir := t.TempDir()
	writeClosedV3(t, dataDir)

	got, err := tracking.UpdateApplicationInterviewsIfMatch(dataDir, "acme", []tracking.Interview{
		{At: "2026-11-12T10:00", Note: " second "},
		{At: "2026-11-10T14:30", Location: "Via Roma 1"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Interviews) != 2 || got.Interviews[0].At != "2026-11-10T14:30" || got.Interviews[1].Note != "second" {
		t.Fatalf("not sorted/trimmed: %+v", got.Interviews)
	}

	pair, err := tracking.Get(dataDir, "acme")
	if err != nil || len(pair.Application.Interviews) != 2 {
		t.Fatalf("not persisted: %v %+v", err, pair.Application.Interviews)
	}

	if _, err := tracking.UpdateApplicationInterviewsIfMatch(dataDir, "acme", []tracking.Interview{{At: "tomorrow"}}, ""); !errors.Is(err, tracking.ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
	if _, err := tracking.UpdateApplicationInterviewsIfMatch(dataDir, "acme", nil, "stale"); !errors.Is(err, recordversion.ErrMismatch) {
		t.Errorf("want ErrMismatch, got %v", err)
	}

	cleared, err := tracking.UpdateApplicationInterviewsIfMatch(dataDir, "acme", []tracking.Interview{}, "")
	if err != nil || len(cleared.Interviews) != 0 {
		t.Errorf("not cleared: %v %+v", err, cleared.Interviews)
	}
}

func TestNextInterview(t *testing.T) {
	now := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	iv, ok := tracking.NextInterview([]tracking.Interview{{At: "2026-11-01T10:00"}, {At: "2026-11-06T10:00"}}, now)
	if !ok || iv.At != "2026-11-06T10:00" {
		t.Errorf("got %+v %v", iv, ok)
	}
	if _, ok := tracking.NextInterview(nil, now); ok {
		t.Error("expected none")
	}
}
