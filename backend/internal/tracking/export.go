package tracking

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// exportDirs lists the data directories with no backup/history path of
// their own (unlike Master Data, which is git-tracked), per ADR-0008.
var exportDirs = []string{jobsDir, applicationsDir, pendingCapturesDir}

// ExportData writes a zip archive of the Job Listing, Application and
// Pending Capture flat files (data/jobs/, data/applications/,
// data/pending-captures/) to w, mirroring their on-disk
// layout exactly. Master Data is intentionally excluded — it already has
// git as a backup/history mechanism.
func ExportData(dataDir string, w io.Writer) error {
	zw := zip.NewWriter(w)

	for _, dir := range exportDirs {
		fullDir := filepath.Join(dataDir, dir)
		info, err := os.Stat(fullDir)
		if os.IsNotExist(err) {
			// Nothing has been saved under this directory yet (e.g. a
			// brand-new install with zero Job Listings/Applications) —
			// that's not a failure, it just contributes nothing to the
			// archive.
			continue
		}
		if err != nil {
			return fmt.Errorf("export: reading %s: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("export: %s is not a directory", dir)
		}

		err = filepath.WalkDir(fullDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			rel, err := filepath.Rel(dataDir, path)
			if err != nil {
				return err
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("export: reading %s: %w", rel, err)
			}

			f, err := zw.Create(filepath.ToSlash(rel))
			if err != nil {
				return fmt.Errorf("export: adding %s: %w", rel, err)
			}
			if _, err := f.Write(content); err != nil {
				return fmt.Errorf("export: writing %s: %w", rel, err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return zw.Close()
}
