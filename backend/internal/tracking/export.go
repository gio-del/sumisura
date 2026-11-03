package tracking

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// exportDirs lists the tracking directories: Job Listings, Applications and
// Pending Captures, which have no backup/history path of their own.
var exportDirs = []string{jobsDir, applicationsDir, pendingCapturesDir}

// masterDataDirs and masterDataFiles are the Master Data paths under the
// data directory (ADR-0037, ADR-0038: gitignored, so an export is their
// only backup).
var (
	masterDataDirs  = []string{"experience", "projects", "cover-letter-snippets"}
	masterDataFiles = []string{"profile.yaml"}
)

// ExportScope selects what ExportData puts in the archive.
type ExportScope string

const (
	// ScopeAll is the full backup: tracking records plus Master Data.
	ScopeAll ExportScope = ""
	// ScopeTracking is the tracking records only.
	ScopeTracking ExportScope = "tracking"
)

// ExportData writes a zip archive to w, mirroring the on-disk layout under
// dataDir exactly. ScopeAll holds data/jobs/, data/applications/ and
// data/pending-captures/ plus the Master Data (profile.yaml, experience/,
// projects/, cover-letter-snippets/); ScopeTracking holds the first three
// only. Anything absent is omitted.
func ExportData(dataDir string, w io.Writer, scope ExportScope) error {
	zw := zip.NewWriter(w)

	dirs := append([]string{}, exportDirs...)
	if scope == ScopeAll {
		dirs = append(dirs, masterDataDirs...)
		for _, name := range masterDataFiles {
			if err := addExportFile(zw, dataDir, name); err != nil {
				return err
			}
		}
	}

	for _, dir := range dirs {
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

// addExportFile adds one top-level file under dataDir, skipping it when absent.
func addExportFile(zw *zip.Writer, dataDir, name string) error {
	content, err := os.ReadFile(filepath.Join(dataDir, name))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("export: reading %s: %w", name, err)
	}
	f, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("export: adding %s: %w", name, err)
	}
	if _, err := f.Write(content); err != nil {
		return fmt.Errorf("export: writing %s: %w", name, err)
	}
	return nil
}
