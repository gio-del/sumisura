package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// icsEscape escapes a TEXT value per RFC 5545 §3.3.11.
func icsEscape(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	r := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`)
	return r.Replace(s)
}

// icsFold writes one content line, folded at 75 octets (RFC 5545 §3.1) without
// splitting a UTF-8 sequence; continuation lines start with one space.
func icsFold(sb *strings.Builder, line string) {
	limit := 75
	for len(line) > limit {
		cut := limit
		for cut > 0 && line[cut]&0xC0 == 0x80 {
			cut--
		}
		sb.WriteString(line[:cut])
		sb.WriteString("\r\n ")
		line = line[cut:]
		limit = 74
	}
	sb.WriteString(line)
	sb.WriteString("\r\n")
}

// applicationOpen reports whether an Application is neither final
// (Rejected, Withdrawn, Offer) nor on an archived listing.
func applicationOpen(item tracking.ListingWithApplication) bool {
	switch item.Application.Status {
	case tracking.StatusRejected, tracking.StatusWithdrawn, tracking.StatusOffer:
		return false
	}
	return !item.JobListing.Archived
}

// followUpOpen reports whether an Application belongs in the follow-up feed:
// not final (Rejected, Withdrawn, Offer), not on an archived listing, and
// carrying a parseable followUpAt.
func followUpOpen(item tracking.ListingWithApplication) (time.Time, bool) {
	a := item.Application
	switch a.Status {
	case tracking.StatusRejected, tracking.StatusWithdrawn, tracking.StatusOffer:
		return time.Time{}, false
	}
	if item.JobListing.Archived {
		return time.Time{}, false
	}
	day, err := time.Parse(tracking.FollowUpDateLayout, a.FollowUpAt)
	if err != nil {
		return time.Time{}, false
	}
	return day, true
}

// writeFollowUpsICS writes a VCALENDAR with one all-day VEVENT per open
// Application that has a follow-up date. UIDs derive from the Application id
// and DTSTAMP from the follow-up day, so the same data yields the same file.
// baseURL (may be empty) prefixes the link to the Job Listing page.
func writeFollowUpsICS(items []tracking.ListingWithApplication, baseURL string, now time.Time) string {
	var sb strings.Builder
	icsFold(&sb, "BEGIN:VCALENDAR")
	icsFold(&sb, "VERSION:2.0")
	icsFold(&sb, "PRODID:-//Sumisura//Follow-ups//EN")
	icsFold(&sb, "CALSCALE:GREGORIAN")
	for _, item := range items {
		l, a := item.JobListing, item.Application
		summary := l.Title
		if l.Company != "" {
			summary += " @ " + l.Company
		}
		desc := "Status: " + string(a.Status)
		if baseURL != "" {
			desc += "\n" + baseURL + "/jobs/" + l.ID
		}
		if applicationOpen(item) {
			writeInterviewEvents(&sb, a, summary, desc, now)
		}
		day, ok := followUpOpen(item)
		if !ok {
			continue
		}
		icsFold(&sb, "BEGIN:VEVENT")
		icsFold(&sb, "UID:"+icsEscape(a.ID)+"@sumisura")
		icsFold(&sb, "DTSTAMP:"+day.Format("20060102")+"T000000Z")
		icsFold(&sb, "DTSTART;VALUE=DATE:"+day.Format("20060102"))
		icsFold(&sb, "DTEND;VALUE=DATE:"+day.AddDate(0, 0, 1).Format("20060102"))
		icsFold(&sb, "SUMMARY:Follow-up: "+icsEscape(summary))
		icsFold(&sb, "DESCRIPTION:"+icsEscape(desc))
		icsFold(&sb, "END:VEVENT")
	}
	icsFold(&sb, "END:VCALENDAR")
	return sb.String()
}

// writeInterviewEvents writes one timed VEVENT (floating local time, one
// hour) for every interview of a that is not before now. The UID derives
// from the Application id and the interview's index, DTSTAMP from its start.
func writeInterviewEvents(sb *strings.Builder, a tracking.Application, summary, desc string, now time.Time) {
	for i, iv := range a.Interviews {
		start, err := time.ParseInLocation(tracking.InterviewAtLayout, iv.At, now.Location())
		if err != nil || start.Before(now) {
			continue
		}
		d := desc
		if iv.Note != "" {
			d = iv.Note + "\n" + d
		}
		icsFold(sb, "BEGIN:VEVENT")
		icsFold(sb, fmt.Sprintf("UID:%s-interview-%d@sumisura", icsEscape(a.ID), i))
		icsFold(sb, "DTSTAMP:"+start.Format("20060102T150400")+"Z")
		icsFold(sb, "DTSTART:"+start.Format("20060102T150400"))
		icsFold(sb, "DTEND:"+start.Add(time.Hour).Format("20060102T150400"))
		icsFold(sb, "SUMMARY:Interview: "+icsEscape(summary))
		if iv.Location != "" {
			icsFold(sb, "LOCATION:"+icsEscape(iv.Location))
		}
		icsFold(sb, "DESCRIPTION:"+icsEscape(d))
		icsFold(sb, "END:VEVENT")
	}
}

func exportFollowUpsICSHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		scheme := "http"
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		baseURL := ""
		if r.Host != "" {
			baseURL = fmt.Sprintf("%s://%s", scheme, r.Host)
		}
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="sumisura-follow-ups.ics"`)
		if _, err := w.Write([]byte(writeFollowUpsICS(items, baseURL, time.Now()))); err != nil {
			return
		}
	}
}
