package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func icsItem(id, company, title string, status tracking.Status, followUp string) tracking.ListingWithApplication {
	return tracking.ListingWithApplication{
		JobListing:  tracking.JobListing{ID: id, Company: company, Title: title},
		Application: tracking.Application{ID: "app-" + id, Status: status, FollowUpAt: followUp},
	}
}

func TestWriteFollowUpsICS_IncludesOnlyOpenDatedApplications(t *testing.T) {
	archived := icsItem("arc", "Arc", "Dev", tracking.StatusSent, "2026-11-05")
	archived.JobListing.Archived = true
	out := writeFollowUpsICS([]tracking.ListingWithApplication{
		icsItem("a", "Acme", "Dev", tracking.StatusSent, "2026-11-03"),
		icsItem("b", "Beta", "Dev", tracking.StatusRejected, "2026-11-03"),
		icsItem("c", "Cora", "Dev", tracking.StatusWithdrawn, "2026-11-03"),
		icsItem("d", "Dyn", "Dev", tracking.StatusSaved, ""),
		archived,
	}, "http://localhost:8080")
	if n := strings.Count(out, "BEGIN:VEVENT"); n != 1 {
		t.Fatalf("want 1 event, got %d:\n%s", n, out)
	}
	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n", "VERSION:2.0\r\n", "END:VCALENDAR\r\n",
		"UID:app-a@sumisura\r\n", "DTSTART;VALUE=DATE:20261103\r\n", "DTEND;VALUE=DATE:20261104\r\n",
		"SUMMARY:Follow-up: Dev @ Acme\r\n", `\nhttp://localhost:8080/jobs/a`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestWriteFollowUpsICS_EscapesAndFolds(t *testing.T) {
	long := strings.Repeat("è", 100)
	out := writeFollowUpsICS([]tracking.ListingWithApplication{
		icsItem("a", `Acme, Inc; \ co`, "Line1\nLine2"+long, tracking.StatusSent, "2026-11-03"),
	}, "")
	if !strings.Contains(out, `Acme\, Inc\; \\ co`) || !strings.Contains(out, `Line1\nLine2`) {
		t.Fatalf("not escaped:\n%s", out)
	}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Errorf("line over 75 octets (%d): %q", len(line), line)
		}
		if !utf8.ValidString(line) {
			t.Errorf("split a UTF-8 sequence: %q", line)
		}
	}
	if !strings.Contains(out, "\r\n ") {
		t.Errorf("expected folded lines")
	}
}

func TestExportFollowUpsICSHandler_StableAcrossDownloads(t *testing.T) {
	h := exportFollowUpsICSHandler(t.TempDir())
	get := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, "/api/applications/follow-ups.ics", nil))
		return rec
	}
	a, b := get(), get()
	if a.Code != 200 || !strings.HasPrefix(a.Header().Get("Content-Type"), "text/calendar") {
		t.Fatalf("code %d type %q", a.Code, a.Header().Get("Content-Type"))
	}
	if !strings.HasPrefix(a.Body.String(), "BEGIN:VCALENDAR") || a.Body.String() != b.Body.String() {
		t.Fatalf("unexpected body %q", a.Body.String())
	}
}
