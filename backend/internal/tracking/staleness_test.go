package tracking_test

import (
	"testing"
	"time"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func TestIsStale(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	old := now.Add(-15 * 24 * time.Hour).Format(time.RFC3339Nano)
	recent := now.Add(-1 * 24 * time.Hour).Format(time.RFC3339Nano)

	cases := []struct {
		name            string
		status          tracking.Status
		statusUpdatedAt string
		want            bool
	}{
		{"sent past threshold", tracking.StatusSent, old, true},
		{"interviewing past threshold", tracking.StatusInterviewing, old, true},
		{"sent under threshold", tracking.StatusSent, recent, false},
		{"interviewing under threshold", tracking.StatusInterviewing, recent, false},
		{"saved past threshold ignored", tracking.StatusSaved, old, false},
		{"tailoring past threshold ignored", tracking.StatusTailoring, old, false},
		{"rejected past threshold ignored", tracking.StatusRejected, old, false},
		{"offer past threshold ignored", tracking.StatusOffer, old, false},
		{"empty timestamp", tracking.StatusSent, "", false},
		{"unparseable timestamp", tracking.StatusSent, "not-a-timestamp", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := tracking.IsStale(c.status, c.statusUpdatedAt, now, tracking.DefaultStaleThreshold)
			if got != c.want {
				t.Errorf("IsStale(%q, %q, now, threshold) = %v, want %v", c.status, c.statusUpdatedAt, got, c.want)
			}
		})
	}
}

func TestDefaultStaleThreshold_Is14Days(t *testing.T) {
	if tracking.DefaultStaleThreshold != 14*24*time.Hour {
		t.Errorf("expected DefaultStaleThreshold to be 14 days, got %v", tracking.DefaultStaleThreshold)
	}
}

func TestIsStaleWithFollowUp(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	old := now.Add(-40 * 24 * time.Hour).Format(time.RFC3339Nano)
	fresh := now.Add(-time.Hour).Format(time.RFC3339Nano)

	tests := []struct {
		name       string
		status     tracking.Status
		updatedAt  string
		followUpAt string
		want       bool
	}{
		{"future follow-up overrides an old status change", tracking.StatusSent, old, "2026-11-15", false},
		{"today is not yet overdue", tracking.StatusSent, old, "2026-10-29", false},
		{"past follow-up is stale despite a fresh status change", tracking.StatusSent, fresh, "2026-10-28", true},
		{"past follow-up applies to Saved", tracking.StatusSaved, fresh, "2026-10-01", true},
		{"past follow-up applies to Tailoring", tracking.StatusTailoring, "", "2026-10-01", true},
		{"past follow-up applies to Interviewing", tracking.StatusInterviewing, fresh, "2026-10-01", true},
		{"Rejected is never stale", tracking.StatusRejected, old, "2026-10-01", false},
		{"Offer is never stale", tracking.StatusOffer, old, "2026-10-01", false},
		{"no follow-up keeps the threshold rule", tracking.StatusSent, old, "", true},
		{"unparseable follow-up falls back to the threshold", tracking.StatusSent, fresh, "soon", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tracking.IsStaleWithFollowUp(tt.status, tt.updatedAt, tt.followUpAt, now, tracking.DefaultStaleThreshold); got != tt.want {
				t.Errorf("IsStaleWithFollowUp = %v, want %v", got, tt.want)
			}
		})
	}
}
