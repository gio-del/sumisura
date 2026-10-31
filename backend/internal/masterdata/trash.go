package masterdata

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
)

// trashDir is the directory under dataDir holding the previous versions of
// Entries and Cover Letter Snippets that were deleted or overwritten. It is
// gitignored like the rest of Master Data and is never read by ListEntries,
// ListSnippets or the exports.
const trashDir = ".trash"

const (
	// MaxTrashVersions is how many versions of one item the trash keeps.
	MaxTrashVersions = 10
	// TrashRetention is how long a trashed version is kept.
	TrashRetention = 90 * 24 * time.Hour
)

// trashStampLayout is fixed-width, so a file name splits into slug and
// timestamp unambiguously whatever dashes the slug holds.
const trashStampLayout = "20060102T150405.000000000Z"

// ErrTrashNotFound marks a trash id that names no trashed version.
var ErrTrashNotFound = errors.New("trashed item not found")

// trashKinds are the Master Data directories the trash accepts.
var trashKinds = map[string]bool{
	"experience": true,
	"projects":   true,
	snippetDir:   true,
}

// TrashItem is one trashed version of an Entry or Cover Letter Snippet.
type TrashItem struct {
	// ID identifies the trashed version: "<dir>~<slug>-<UTC>".
	ID string `json:"id"`
	// Kind is "entry" or "snippet".
	Kind string `json:"kind"`
	// OriginalID is the id the item had in Master Data.
	OriginalID string `json:"originalId"`
	// Label is a short human description read from the content.
	Label     string `json:"label"`
	TrashedAt string `json:"trashedAt"`
	// Content is the raw file, for the user to look at before restoring.
	Content string `json:"content"`
}

// stash copies the previous content of an Entry (dir "experience"/"projects") or Snippet
// (dir "cover-letter-snippets") into the trash. now stamps the copy.
func stash(dataDir, dir, slug string, content []byte, now time.Time) error {
	if !trashKinds[dir] {
		return fmt.Errorf("invalid trash kind %q", dir)
	}
	target := filepath.Join(dataDir, trashDir, dir)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	name := slug + "-" + now.UTC().Format(trashStampLayout) + ".md"
	if err := atomicfile.WriteFile(filepath.Join(target, name), content, 0o644); err != nil {
		return err
	}
	return pruneTrash(dataDir, now)
}

// stashFile stashes the file at path as it is now. A file that does not
// exist is not an error to stash: there is nothing to keep.
func stashFile(dataDir, dir, slug, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return stash(dataDir, dir, slug, content, time.Now())
}

// stashIfChanged stashes the file at path unless next is byte-identical.
func stashIfChanged(dataDir, dir, slug, path string, next []byte) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bytes.Equal(content, next) {
		return nil
	}
	return stash(dataDir, dir, slug, content, time.Now())
}

type trashFile struct {
	dir, slug, path string
	at              time.Time
}

func readTrash(dataDir string) ([]trashFile, error) {
	var out []trashFile
	for dir := range trashKinds {
		files, err := os.ReadDir(filepath.Join(dataDir, trashDir, dir))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			name, ok := strings.CutSuffix(f.Name(), ".md")
			if f.IsDir() || !ok || len(name) <= len(trashStampLayout)+1 {
				continue
			}
			stampAt := len(name) - len(trashStampLayout)
			at, err := time.Parse(trashStampLayout, name[stampAt:])
			if err != nil || name[stampAt-1] != '-' {
				continue
			}
			out = append(out, trashFile{dir: dir, slug: name[:stampAt-1], path: filepath.Join(dataDir, trashDir, dir, f.Name()), at: at})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].at.Equal(out[j].at) {
			return out[i].at.After(out[j].at)
		}
		return out[i].path < out[j].path
	})
	return out, nil
}

// pruneTrash drops versions older than TrashRetention and, per item, all
// but the newest MaxTrashVersions.
func pruneTrash(dataDir string, now time.Time) error {
	files, err := readTrash(dataDir)
	if err != nil {
		return err
	}
	kept := map[string]int{}
	for _, f := range files { // newest first
		key := f.dir + "/" + f.slug
		kept[key]++
		if kept[key] > MaxTrashVersions || now.Sub(f.at) > TrashRetention {
			if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

// ListTrash returns every trashed version, newest first, after dropping the
// ones past the retention.
func ListTrash(dataDir string) ([]TrashItem, error) {
	if err := pruneTrash(dataDir, time.Now()); err != nil {
		return nil, err
	}
	files, err := readTrash(dataDir)
	if err != nil {
		return nil, err
	}
	items := []TrashItem{}
	for _, f := range files {
		content, err := os.ReadFile(f.path)
		if err != nil {
			return nil, err
		}
		items = append(items, trashItem(f, content))
	}
	return items, nil
}

func trashItem(f trashFile, content []byte) TrashItem {
	item := TrashItem{
		ID:        trashID(f),
		Kind:      "entry",
		TrashedAt: f.at.Format(time.RFC3339Nano),
		Content:   string(content),
		Label:     f.slug,
	}
	if f.dir == snippetDir {
		item.Kind = "snippet"
		item.OriginalID = f.slug
		if s, err := parseSnippet(f.slug, content); err == nil && s.Kind != "" {
			item.Label = s.Kind
		}
		return item
	}
	item.OriginalID = f.dir + "/" + f.slug
	if e, err := parseEntry(f.dir, entryDirs[f.dir], f.slug, content); err == nil {
		item.Label = firstNonEmpty(e.Name, joinNonEmpty(e.Employer, e.Client), f.slug)
	}
	return item
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func joinNonEmpty(a, b string) string {
	if a != "" && b != "" {
		return a + " / " + b
	}
	return a + b
}

func trashID(f trashFile) string {
	return f.dir + "~" + f.slug + "-" + f.at.Format(trashStampLayout)
}

func findTrash(dataDir, id string) (trashFile, error) {
	files, err := readTrash(dataDir)
	if err != nil {
		return trashFile{}, err
	}
	for _, f := range files {
		if trashID(f) == id {
			return f, nil
		}
	}
	return trashFile{}, ErrTrashNotFound
}

// RestoreTrash recreates the trashed version as a live Entry or Snippet under
// its original id, or under a suffixed one when that id is taken, then
// removes it from the trash. It returns the id the item now has.
func RestoreTrash(dataDir, id string) (string, error) {
	f, err := findTrash(dataDir, id)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(f.path)
	if err != nil {
		return "", err
	}
	fullDir := filepath.Join(dataDir, f.dir)
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return "", err
	}
	slug := uniqueSlug(fullDir, f.slug)
	if err := atomicfile.WriteFile(filepath.Join(fullDir, slug+".md"), content, 0o644); err != nil {
		return "", err
	}
	if err := os.Remove(f.path); err != nil {
		return "", err
	}
	if f.dir == snippetDir {
		return slug, nil
	}
	return f.dir + "/" + slug, nil
}

// PurgeTrash permanently deletes one trashed version.
func PurgeTrash(dataDir, id string) error {
	f, err := findTrash(dataDir, id)
	if err != nil {
		return err
	}
	return os.Remove(f.path)
}
