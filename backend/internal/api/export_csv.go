package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// applicationsCSVHeader is the fixed column set of the Applications CSV
// export. Notes and Contact are left out on purpose: they hold other
// people's personal data.
var applicationsCSVHeader = []string{
	"company", "title", "location", "status", "archived", "saved_at",
	"status_updated_at", "ral_min", "ral_max", "ral_currency", "ral_source",
	"application_method", "url", "generations",
}

// csvSafe defuses spreadsheet formula injection: a cell a spreadsheet would
// read as a formula gets a leading apostrophe.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func optionalInt(n *int) string {
	if n == nil {
		return ""
	}
	return strconv.Itoa(*n)
}

// writeApplicationsCSV writes one row per Application (archived included),
// RFC 4180 quoted, UTF-8 with a BOM so Excel detects the encoding.
func writeApplicationsCSV(w interface{ Write([]byte) (int, error) }, items []tracking.ListingWithApplication) error {
	if _, err := w.Write([]byte("\xEF\xBB\xBF")); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	if err := cw.Write(applicationsCSVHeader); err != nil {
		return err
	}
	for _, item := range items {
		l, a := item.JobListing, item.Application
		row := []string{
			csvSafe(l.Company),
			csvSafe(l.Title),
			csvSafe(l.Location),
			csvSafe(string(a.Status)),
			strconv.FormatBool(l.Archived),
			csvSafe(l.SavedAt),
			csvSafe(a.StatusUpdatedAt),
			optionalInt(l.RAL.Min),
			optionalInt(l.RAL.Max),
			csvSafe(l.RAL.Currency),
			csvSafe(string(l.RAL.Source)),
			csvSafe(string(a.Method.Kind)),
			csvSafe(l.URL),
			strconv.Itoa(len(a.Generations)),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func exportApplicationsCSVHandler(dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := tracking.List(dataDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var sb strings.Builder
		if err := writeApplicationsCSV(&sb, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		filename := fmt.Sprintf("sumisura-applications-%s.csv", time.Now().UTC().Format("20060102T150405Z"))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		_, _ = w.Write([]byte(sb.String()))
	}
}
