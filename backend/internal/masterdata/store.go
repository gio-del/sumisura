package masterdata

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
	"github.com/gio-del/sumisura/backend/internal/recordversion"
	"gopkg.in/yaml.v3"
)

// ErrValidation marks errors returned when an Entry fails validation before
// being written to disk (story 9).
var ErrValidation = errors.New("validation failed")

// entryDirs maps each Entry directory (matching the real layout under
// data/) to the Entry Type it holds.
var entryDirs = map[string]string{
	"experience": TypeExperience,
	"projects":   TypeProject,
}

// typeDirs is the inverse of entryDirs, used to pick the directory a new
// Entry of a given Type is created under.
var typeDirs = map[string]string{
	TypeExperience: "experience",
	TypeProject:    "projects",
}

type rawFrontmatter struct {
	Employer    string   `yaml:"employer,omitempty"`
	Kind        string   `yaml:"kind,omitempty"`
	Role        string   `yaml:"role,omitempty"`
	Client      string   `yaml:"client,omitempty"`
	ClientLabel string   `yaml:"client_label,omitempty"`
	Title       string   `yaml:"title,omitempty"`
	Group       string   `yaml:"group,omitempty"`
	Name        string   `yaml:"name,omitempty"`
	Location    string   `yaml:"location,omitempty"`
	Start       string   `yaml:"start"`
	End         *string  `yaml:"end,omitempty"`
	Flagship    bool     `yaml:"flagship,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
	Repo        string   `yaml:"repo,omitempty"`
}

// ListEntries reads every Entry under dataDir/experience and
// dataDir/projects.
func ListEntries(dataDir string) ([]Entry, error) {
	var entries []Entry
	for dir, entryType := range entryDirs {
		dirEntries, err := listDir(dataDir, dir, entryType)
		if err != nil {
			return nil, err
		}
		entries = append(entries, dirEntries...)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

// GetEntry reads a single Entry by its id ("<dir>/<slug>", e.g.
// "experience/example-client-a").
func GetEntry(dataDir, id string) (Entry, error) {
	dir, slug, ok := splitID(id)
	if !ok {
		return Entry{}, fmt.Errorf("invalid entry id %q", id)
	}
	entryType, ok := entryDirs[dir]
	if !ok {
		return Entry{}, fmt.Errorf("invalid entry id %q", id)
	}

	content, err := os.ReadFile(filepath.Join(dataDir, dir, slug+".md"))
	if err != nil {
		return Entry{}, err
	}
	return parseEntry(dir, entryType, slug, content)
}

// UpdateEntry validates entry and, if valid, writes it back to the same file
// GetEntry would read for id, replacing its frontmatter and bullets in
// place. On validation failure the file is left untouched.
func UpdateEntry(dataDir, id string, entry Entry) (Entry, error) {
	return UpdateEntryIfMatch(dataDir, id, entry, "")
}

// UpdateEntryIfMatch is UpdateEntry, refusing the write with
// recordversion.ErrMismatch when version no longer matches the file on
// disk — something else (the tailor-cv skill, another tab, a hand edit)
// wrote the Entry since the caller read it. An empty version writes
// unconditionally, exactly as UpdateEntry always has.
//
// The comparison happens immediately before the write, not in the caller,
// so the window between checking and writing is as small as a
// single-process app can make it (issue #89).
func UpdateEntryIfMatch(dataDir, id string, entry Entry, version string) (Entry, error) {
	dir, slug, ok := splitID(id)
	if !ok {
		return Entry{}, fmt.Errorf("invalid entry id %q", id)
	}
	entryType, ok := entryDirs[dir]
	if !ok {
		return Entry{}, fmt.Errorf("invalid entry id %q", id)
	}

	path := filepath.Join(dataDir, dir, slug+".md")
	if _, err := os.Stat(path); err != nil {
		return Entry{}, err
	}

	entry.ID = dir + "/" + slug
	entry.Type = entryType

	if err := Validate(entryType, entry); err != nil {
		return Entry{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	if err := recordversion.Check(path, version); err != nil {
		return Entry{}, err
	}

	if err := atomicfile.WriteFile(path, renderEntry(entry), 0o644); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// EntryVersion returns the version token of the Entry file GetEntry would
// read for id — the opaque value a later conditional write is checked
// against.
func EntryVersion(dataDir, id string) (string, error) {
	dir, slug, ok := splitID(id)
	if !ok {
		return "", fmt.Errorf("invalid entry id %q", id)
	}
	if _, ok := entryDirs[dir]; !ok {
		return "", fmt.Errorf("invalid entry id %q", id)
	}
	return recordversion.Of(filepath.Join(dataDir, dir, slug+".md"))
}

// CreateEntry validates entry and, if valid, writes it to a new file under
// dataDir, generating a slug (and thus id) from its content. The directory
// is picked from entry.Type.
func CreateEntry(dataDir string, entry Entry) (Entry, error) {
	dir, ok := typeDirs[entry.Type]
	if !ok {
		return Entry{}, fmt.Errorf("%w: unknown entry type %q", ErrValidation, entry.Type)
	}

	if err := Validate(entry.Type, entry); err != nil {
		return Entry{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	fullDir := filepath.Join(dataDir, dir)
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return Entry{}, err
	}

	slug := uniqueSlug(fullDir, slugify(slugSource(entry)))
	entry.ID = dir + "/" + slug

	if err := atomicfile.WriteFile(filepath.Join(fullDir, slug+".md"), renderEntry(entry), 0o644); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

func slugSource(e Entry) string {
	switch e.Type {
	case TypeExperience:
		if e.Client != "" {
			return e.Employer + "-" + e.Client
		}
		return e.Employer
	default:
		return e.Name
	}
}

func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// uniqueSlug appends -2, -3, ... to base until it no longer collides with an
// existing file in fullDir.
func uniqueSlug(fullDir, base string) string {
	if base == "" {
		base = "entry"
	}
	slug := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(fullDir, slug+".md")); os.IsNotExist(err) {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

// DeleteEntry removes the file GetEntry would read for id.
func DeleteEntry(dataDir, id string) error {
	return DeleteEntryIfMatch(dataDir, id, "")
}

// DeleteEntryIfMatch is DeleteEntry, refusing with
// recordversion.ErrMismatch when version no longer matches the file on
// disk: acting destructively on content the caller is no longer seeing is
// the same hazard as overwriting it (issue #89, story 18). An empty
// version deletes unconditionally.
func DeleteEntryIfMatch(dataDir, id, version string) error {
	dir, slug, ok := splitID(id)
	if !ok {
		return fmt.Errorf("invalid entry id %q", id)
	}
	if _, ok := entryDirs[dir]; !ok {
		return fmt.Errorf("invalid entry id %q", id)
	}
	path := filepath.Join(dataDir, dir, slug+".md")
	if err := recordversion.Check(path, version); err != nil {
		return err
	}
	return os.Remove(path)
}

func renderEntry(e Entry) []byte {
	raw := rawFrontmatter{
		Employer:    e.Employer,
		Kind:        e.Kind,
		Role:        e.Role,
		Client:      e.Client,
		ClientLabel: e.ClientLabel,
		Title:       e.Title,
		Group:       e.Group,
		Name:        e.Name,
		Location:    e.Location,
		Start:       e.Start,
		End:         e.End,
		Flagship:    e.Flagship,
		Tags:        e.Tags,
		Repo:        e.Repo,
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	fmBytes, _ := yaml.Marshal(raw) //nolint:errcheck // raw is a plain struct of strings/slices; yaml.Marshal cannot fail on it
	buf.Write(fmBytes)
	buf.WriteString("---\n\n")
	for _, bullet := range e.Bullets {
		buf.WriteString("- ")
		buf.WriteString(bullet)
		buf.WriteString("\n")
	}
	return buf.Bytes()
}

func listDir(dataDir, dir, entryType string) ([]Entry, error) {
	fullDir := filepath.Join(dataDir, dir)
	files, err := os.ReadDir(fullDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		slug := strings.TrimSuffix(f.Name(), ".md")
		content, err := os.ReadFile(filepath.Join(fullDir, f.Name()))
		if err != nil {
			return nil, err
		}
		entry, err := parseEntry(dir, entryType, slug, content)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", f.Name(), err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func parseEntry(dir, entryType, slug string, content []byte) (Entry, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return Entry{}, err
	}

	var raw rawFrontmatter
	if err := yaml.Unmarshal(fm, &raw); err != nil {
		return Entry{}, err
	}

	return Entry{
		ID:          dir + "/" + slug,
		Type:        entryType,
		Employer:    raw.Employer,
		Kind:        raw.Kind,
		Client:      raw.Client,
		ClientLabel: raw.ClientLabel,
		Title:       raw.Title,
		Group:       raw.Group,
		Role:        raw.Role,
		Name:        raw.Name,
		Location:    raw.Location,
		Start:       raw.Start,
		End:         raw.End,
		Flagship:    raw.Flagship,
		Tags:        raw.Tags,
		Repo:        raw.Repo,
		Bullets:     extractBullets(body),
	}, nil
}

func splitID(id string) (dir, slug string, ok bool) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
