package tracking

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
)

// maxRestoreEntryBytes bounds one archive entry, so a crafted zip cannot
// exhaust memory. Records are small text files.
const maxRestoreEntryBytes = 64 << 20

// RestoreOptions controls RestoreData.
type RestoreOptions struct {
	// DryRun reports what would happen and writes nothing.
	DryRun bool
	// Overwrite replaces files that already exist; without it they are
	// left untouched and reported as skipped.
	Overwrite bool
}

// RestoreReport lists the archive's records by what happened (or, in a dry
// run, would happen) to each. Paths are slash-separated, relative to the
// data directory.
type RestoreReport struct {
	DryRun      bool
	Created     []string
	Overwritten []string
	Skipped     []string
}

// Pending reports whether applying the archive would change anything.
func (r RestoreReport) Pending() bool {
	return len(r.Created) > 0 || len(r.Overwritten) > 0
}

// RestoreData writes the records of an ExportData zip back under dataDir.
// Every entry is validated before the first write: a path outside
// jobs/, applications/ and pending-captures/ (absolute, containing "..",
// or a backslash), a duplicate entry or a non-regular file fails the whole
// restore with nothing written. Files are written with atomicfile.WriteFile.
func RestoreData(zipPath, dataDir string, opts RestoreOptions) (RestoreReport, error) {
	report := RestoreReport{DryRun: opts.DryRun}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return report, fmt.Errorf("restore: opening %s: %w", zipPath, err)
	}
	defer zr.Close() //nolint:errcheck // read-only archive; nothing to flush

	type record struct {
		name    string
		content []byte
	}
	var records []record
	seen := map[string]bool{}
	for _, f := range zr.File {
		if err := validateRestorePath(f.Name); err != nil {
			return report, err
		}
		if !f.Mode().IsRegular() {
			return report, fmt.Errorf("restore: %s is not a regular file", f.Name)
		}
		if seen[f.Name] {
			return report, fmt.Errorf("restore: %s appears twice in the archive", f.Name)
		}
		seen[f.Name] = true
		if f.UncompressedSize64 > maxRestoreEntryBytes {
			return report, fmt.Errorf("restore: %s is larger than %d bytes", f.Name, maxRestoreEntryBytes)
		}
		rc, err := f.Open()
		if err != nil {
			return report, fmt.Errorf("restore: reading %s: %w", f.Name, err)
		}
		content, err := io.ReadAll(io.LimitReader(rc, maxRestoreEntryBytes+1))
		rc.Close() //nolint:errcheck // read-only entry
		if err != nil {
			return report, fmt.Errorf("restore: reading %s: %w", f.Name, err)
		}
		if len(content) > maxRestoreEntryBytes {
			return report, fmt.Errorf("restore: %s is larger than %d bytes", f.Name, maxRestoreEntryBytes)
		}
		records = append(records, record{f.Name, content})
	}

	type step struct {
		record
		target string
	}
	var steps []step
	for _, r := range records {
		target := filepath.Join(dataDir, filepath.FromSlash(r.name))
		existing, err := os.ReadFile(target)
		switch {
		case err == nil:
			if !opts.Overwrite {
				report.Skipped = append(report.Skipped, r.name)
				continue
			}
			if bytes.Equal(existing, r.content) {
				report.Skipped = append(report.Skipped, r.name)
				continue
			}
			report.Overwritten = append(report.Overwritten, r.name)
		case errors.Is(err, os.ErrNotExist):
			report.Created = append(report.Created, r.name)
		default:
			return report, fmt.Errorf("restore: checking %s: %w", r.name, err)
		}
		steps = append(steps, step{r, target})
	}
	if opts.DryRun {
		return report, nil
	}

	for _, s := range steps {
		if err := os.MkdirAll(filepath.Dir(s.target), 0o755); err != nil {
			return report, fmt.Errorf("restore: creating directory for %s: %w", s.name, err)
		}
		if err := atomicfile.WriteFile(s.target, s.content, 0o644); err != nil {
			return report, fmt.Errorf("restore: writing %s: %w", s.name, err)
		}
	}
	return report, nil
}

// validateRestorePath accepts only "<exported dir>/<relative file path>"
// with no way out of the data directory.
func validateRestorePath(name string) error {
	unsafe := fmt.Errorf("restore: refusing unsafe path %q", name)
	if name == "" || strings.ContainsAny(name, "\\\x00") || strings.HasPrefix(name, "/") {
		return unsafe
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return unsafe
		}
	}
	if path.Clean(name) != name {
		return unsafe
	}
	top, _, ok := strings.Cut(name, "/")
	if ok {
		for _, d := range exportDirs {
			if top == d {
				return nil
			}
		}
	}
	return fmt.Errorf("restore: %q is not inside %s/", name, strings.Join(exportDirs, "/, "))
}
