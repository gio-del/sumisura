package masterdata

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LastModified is the most recent commit's timestamp (RFC3339) and subject
// line for an Entry's file. When git has no history for the file (Master
// Data is gitignored, or the release image has no repository) At is the
// file's mtime and Subject stays empty; the zero value means the file
// does not exist either.
type LastModified struct {
	At      string `json:"at,omitempty"`
	Subject string `json:"subject,omitempty"`
}

// EntryLastModified looks up the last commit that touched an Entry's file,
// by shelling out to `git log` against projectRoot — mirroring the
// exec.Command shell-out precedent already used for `typst compile` in the
// render path (ADR-0012). Git history is preferred since it survives
// clone/checkout; when git finds nothing (uncommitted or gitignored file,
// path outside the repository, no repository at all) the file's mtime is
// used as an approximation, with an empty Subject. A file that cannot be
// stat'ed is reported as the zero LastModified with a nil error.
func EntryLastModified(projectRoot, dataDir, id string) (LastModified, error) {
	dir, slug, ok := splitID(id)
	if !ok {
		return LastModified{}, fmt.Errorf("invalid entry id %q", id)
	}
	if _, ok := entryDirs[dir]; !ok {
		return LastModified{}, fmt.Errorf("invalid entry id %q", id)
	}

	path := filepath.Join(dataDir, dir, slug+".md")

	if lm := gitLastModified(projectRoot, path); lm.At != "" {
		return lm, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return LastModified{}, nil
	}
	return LastModified{At: info.ModTime().UTC().Format(time.RFC3339Nano)}, nil
}

func gitLastModified(projectRoot, path string) LastModified {
	cmd := exec.Command("git", "-C", projectRoot, "log", "-1", "--format=%aI%x00%s", "--", path)
	out, err := cmd.Output()
	if err != nil {
		// Not a repo, path outside the repo, or any other git failure: no
		// history to report, not an error the caller needs to handle.
		return LastModified{}
	}

	line := strings.TrimRight(string(out), "\n")
	if line == "" {
		// Valid path inside a repo, but never committed.
		return LastModified{}
	}

	parts := strings.SplitN(line, "\x00", 2)
	lm := LastModified{At: parts[0]}
	if len(parts) == 2 {
		lm.Subject = parts[1]
	}
	return lm
}
