package api

import (
	"archive/zip"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// packageFileBase turns the profile name into a file-name stem that is safe
// on every platform: letters and digits are kept (accents included), any run
// of anything else becomes one hyphen. Empty when nothing usable is left.
func packageFileBase(name string) string {
	var sb strings.Builder
	pendingHyphen := false
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if pendingHyphen && sb.Len() > 0 {
				sb.WriteByte('-')
			}
			pendingHyphen = false
			sb.WriteRune(r)
		} else {
			pendingHyphen = true
		}
	}
	return sb.String()
}

// getApplicationPackageHandler streams a zip of the Application's latest
// recorded Generation as the files to send: the CV and, when there is one,
// the Cover Letter, named after the profile owner. Records may outlive
// their files (ADR-0018), so a missing directory or CV is a 404.
func getApplicationPackageHandler(dataDir, projectRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, err := tracking.Get(dataDir, r.PathValue("id"))
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "application not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, tracking.ErrValidation) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		gens := item.Application.Generations
		if len(gens) == 0 {
			http.Error(w, "this application has no generation yet", http.StatusNotFound)
			return
		}
		latest := gens[len(gens)-1]
		// The same pattern the file route validates: no dot, slash or empty
		// segment, so the join cannot leave output/.
		if !generationSlugRe.MatchString(latest.Slug) {
			http.Error(w, "the latest generation's files are no longer on disk", http.StatusNotFound)
			return
		}
		dir := filepath.Join(projectRoot, "output", latest.Slug)
		cv, err := os.ReadFile(filepath.Join(dir, "cv.pdf"))
		if err != nil {
			http.Error(w, "the latest generation's files are no longer on disk", http.StatusNotFound)
			return
		}
		letter, letterErr := os.ReadFile(filepath.Join(dir, "cover-letter.pdf"))

		base := ""
		if profile, err := masterdata.GetProfile(dataDir); err == nil {
			base = packageFileBase(profile.Name)
		}
		cvName, letterName := "CV.pdf", "Lettera.pdf"
		if base != "" {
			cvName, letterName = base+"-CV.pdf", base+"-Lettera.pdf"
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="package.zip"`)
		zw := zip.NewWriter(w)
		files := []struct {
			name string
			data []byte
		}{{cvName, cv}}
		if letterErr == nil {
			files = append(files, struct {
				name string
				data []byte
			}{letterName, letter})
		}
		for _, f := range files {
			fw, err := zw.Create(f.name)
			if err != nil {
				return
			}
			if _, err := fw.Write(f.data); err != nil {
				return
			}
		}
		if err := zw.Close(); err != nil {
			return
		}
	}
}
