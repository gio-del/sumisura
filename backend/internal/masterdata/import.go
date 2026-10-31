package masterdata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
)

// ErrEntryExists is returned by CreateEntryWithID when the id already names
// an Entry: importing never overwrites existing Master Data.
var ErrEntryExists = errors.New("entry already exists")

// DraftEntries validates entries proposed by an import and gives each a free
// id, without writing anything. An entry that fails Validate is dropped and
// counted in skipped; ids are unique among the drafts and against the files
// already under dataDir.
func DraftEntries(dataDir string, entries []Entry) (drafts []Entry, skipped int) {
	taken := map[string]bool{}
	for _, e := range entries {
		dir, ok := typeDirs[e.Type]
		if !ok || Validate(e.Type, e) != nil {
			skipped++
			continue
		}
		base := slugify(slugSource(e))
		if base == "" {
			base = "entry"
		}
		slug := base
		for i := 2; ; i++ {
			_, err := os.Stat(filepath.Join(dataDir, dir, slug+".md"))
			if os.IsNotExist(err) && !taken[dir+"/"+slug] {
				break
			}
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		e.ID = dir + "/" + slug
		e.Version, e.LastModified = "", nil
		taken[e.ID] = true
		drafts = append(drafts, e)
	}
	return drafts, skipped
}

// CreateEntryWithID validates entry and writes it under the id it carries
// (as proposed by DraftEntries), refusing with ErrEntryExists when a file is
// already there.
func CreateEntryWithID(dataDir string, entry Entry) (Entry, error) {
	dir, ok := typeDirs[entry.Type]
	if !ok {
		return Entry{}, fmt.Errorf("%w: unknown entry type %q", ErrValidation, entry.Type)
	}
	if err := Validate(entry.Type, entry); err != nil {
		return Entry{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	slug, found := strings.CutPrefix(entry.ID, dir+"/")
	if !found || slug == "" || slugify(slug) != slug {
		return Entry{}, fmt.Errorf("%w: id must be %s/<slug> for a %s entry, got %q", ErrValidation, dir, entry.Type, entry.ID)
	}

	fullDir := filepath.Join(dataDir, dir)
	path := filepath.Join(fullDir, slug+".md")
	if _, err := os.Stat(path); err == nil {
		return Entry{}, ErrEntryExists
	} else if !os.IsNotExist(err) {
		return Entry{}, err
	}
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return Entry{}, err
	}
	entry.Version, entry.LastModified = "", nil
	if err := atomicfile.WriteFile(path, renderEntry(entry), 0o644); err != nil {
		return Entry{}, err
	}
	return entry, nil
}
