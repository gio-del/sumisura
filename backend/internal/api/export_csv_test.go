package api

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func csvItem(company, title string) tracking.ListingWithApplication {
	return tracking.ListingWithApplication{
		JobListing:  tracking.JobListing{Company: company, Title: title},
		Application: tracking.Application{Status: tracking.StatusSaved},
	}
}

func parseCSV(t *testing.T, body string) [][]string {
	t.Helper()
	if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
		t.Fatalf("missing UTF-8 BOM")
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, "\xEF\xBB\xBF"))).ReadAll()
	if err != nil {
		t.Fatalf("not valid CSV: %v", err)
	}
	return rows
}

func TestWriteApplicationsCSV_HeaderRowsAndQuoting(t *testing.T) {
	min, max := 40000, 50000
	archived := csvItem("Globex", "Dev")
	archived.JobListing.Archived = true
	acme := csvItem(`Acme, "Inc"`, "Line1\nLine2")
	acme.JobListing.RAL = generation.RALRange{Min: &min, Max: &max, Currency: "EUR", Source: "listing"}
	acme.Application.Generations = make([]tracking.GenerationRecord, 2)

	var sb strings.Builder
	if err := writeApplicationsCSV(&sb, []tracking.ListingWithApplication{acme, archived}); err != nil {
		t.Fatal(err)
	}
	rows := parseCSV(t, sb.String())
	if len(rows) != 3 {
		t.Fatalf("want header + 2 rows, got %d", len(rows))
	}
	if strings.Join(rows[0], ",") != strings.Join(applicationsCSVHeader, ",") {
		t.Fatalf("unexpected header %v", rows[0])
	}
	if rows[1][0] != `Acme, "Inc"` || rows[1][1] != "Line1\nLine2" {
		t.Errorf("special characters not round-tripped: %q", rows[1][:2])
	}
	if rows[1][7] != "40000" || rows[1][8] != "50000" || rows[1][9] != "EUR" || rows[1][13] != "2" {
		t.Errorf("RAL/generation columns wrong: %v", rows[1])
	}
	if rows[2][4] != "true" {
		t.Errorf("archived listing should be flagged, got %q", rows[2][4])
	}
}

func TestWriteApplicationsCSV_ProtectsFormulas(t *testing.T) {
	item := csvItem(`=HYPERLINK("http://evil","x")`, "+1")
	item.JobListing.Location = "-2"
	item.JobListing.URL = "@sum"

	var sb strings.Builder
	if err := writeApplicationsCSV(&sb, []tracking.ListingWithApplication{item}); err != nil {
		t.Fatal(err)
	}
	row := parseCSV(t, sb.String())[1]
	for i, want := range map[int]string{0: `'=HYPERLINK("http://evil","x")`, 1: "'+1", 2: "'-2", 12: "'@sum"} {
		if row[i] != want {
			t.Errorf("column %d = %q, want %q", i, row[i], want)
		}
	}
}

func TestWriteApplicationsCSV_ExcludesNotesAndContact(t *testing.T) {
	item := csvItem("Acme", "Dev")
	item.Application.Contact = &tracking.Contact{Name: "Jane Secret", Email: "jane@secret.example"}
	item.Application.Notes = []tracking.Note{{Body: "private remark"}}

	var sb strings.Builder
	if err := writeApplicationsCSV(&sb, []tracking.ListingWithApplication{item}); err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"Jane Secret", "jane@secret.example", "private remark"} {
		if strings.Contains(sb.String(), leak) {
			t.Errorf("CSV leaks %q", leak)
		}
	}
}
