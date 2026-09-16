package masterdata

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
	"github.com/gio-del/sumisura/backend/internal/recordversion"
	"gopkg.in/yaml.v3"
)

// snippetDir is the directory under dataDir holding Cover Letter Snippet
// files, per ADR-0003's per-Entry-file pattern extended to Snippets (see
// CONTEXT.md's Cover Letter Snippet entry).
const snippetDir = "cover-letter-snippets"

// iso639_1 matches the two-letter language codes a Snippet's lang may carry
// — the same alphabet generation.NormalizeLanguage speaks.
var iso639_1 = regexp.MustCompile(`^[a-z]{2}$`)

// Snippet is a reusable Cover Letter paragraph (opening, why-this-company,
// closing, ...), stored one-per-file like an Entry.
type Snippet struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Lang is the ISO 639-1 code this Snippet is written in, empty when it
	// is unmarked. It exists so a bilingual library can be selected from
	// rather than translated: machine-translating the user's own vetted
	// prose produces exactly the generated-sounding letter a Snippet
	// library is there to avoid (issue #168).
	Lang string   `json:"lang,omitempty"`
	Tags []string `json:"tags"`
	Body string   `json:"body"`

	// Version is the Snippet file's version token, populated by the API
	// layer (via SnippetVersion) on reads and never persisted — see
	// Entry.Version and issue #89.
	Version string `json:"version,omitempty"`
}

type rawSnippetFrontmatter struct {
	Kind string   `yaml:"kind"`
	Lang string   `yaml:"lang,omitempty"`
	Tags []string `yaml:"tags,omitempty"`
}

// ListSnippets reads every Snippet under dataDir/cover-letter-snippets.
func ListSnippets(dataDir string) ([]Snippet, error) {
	fullDir := filepath.Join(dataDir, snippetDir)
	files, err := os.ReadDir(fullDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var snippets []Snippet
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		slug := strings.TrimSuffix(f.Name(), ".md")
		content, err := os.ReadFile(filepath.Join(fullDir, f.Name()))
		if err != nil {
			return nil, err
		}
		snippet, err := parseSnippet(slug, content)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", f.Name(), err)
		}
		snippets = append(snippets, snippet)
	}
	sort.Slice(snippets, func(i, j int) bool { return snippets[i].ID < snippets[j].ID })
	return snippets, nil
}

// GetSnippet reads a single Snippet by its id (the file's slug).
func GetSnippet(dataDir, id string) (Snippet, error) {
	content, err := os.ReadFile(filepath.Join(dataDir, snippetDir, id+".md"))
	if err != nil {
		return Snippet{}, err
	}
	return parseSnippet(id, content)
}

func parseSnippet(slug string, content []byte) (Snippet, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return Snippet{}, err
	}

	var raw rawSnippetFrontmatter
	if err := yaml.Unmarshal(fm, &raw); err != nil {
		return Snippet{}, err
	}

	return Snippet{
		ID:   slug,
		Kind: raw.Kind,
		Lang: raw.Lang,
		Tags: raw.Tags,
		Body: strings.TrimSpace(string(body)),
	}, nil
}

// CreateSnippet validates snippet and, if valid, writes it to a new file
// under dataDir/cover-letter-snippets, generating a slug (and thus id) from
// its kind.
func CreateSnippet(dataDir string, snippet Snippet) (Snippet, error) {
	if err := ValidateSnippet(snippet); err != nil {
		return Snippet{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	fullDir := filepath.Join(dataDir, snippetDir)
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return Snippet{}, err
	}

	slug := uniqueSlug(fullDir, slugify(snippet.Kind))
	snippet.ID = slug

	if err := atomicfile.WriteFile(filepath.Join(fullDir, slug+".md"), renderSnippet(snippet), 0o644); err != nil {
		return Snippet{}, err
	}
	return snippet, nil
}

// UpdateSnippet validates snippet and, if valid, writes it back to the same
// file GetSnippet would read for id. On validation failure the file is left
// untouched.
func UpdateSnippet(dataDir, id string, snippet Snippet) (Snippet, error) {
	return UpdateSnippetIfMatch(dataDir, id, snippet, "")
}

// UpdateSnippetIfMatch is UpdateSnippet, refusing the write with
// recordversion.ErrMismatch when version no longer matches the file on
// disk (issue #89, story 13). An empty version writes unconditionally.
func UpdateSnippetIfMatch(dataDir, id string, snippet Snippet, version string) (Snippet, error) {
	path := filepath.Join(dataDir, snippetDir, id+".md")
	if _, err := os.Stat(path); err != nil {
		return Snippet{}, err
	}

	snippet.ID = id

	if err := ValidateSnippet(snippet); err != nil {
		return Snippet{}, fmt.Errorf("%w: %v", ErrValidation, err)
	}

	if err := recordversion.Check(path, version); err != nil {
		return Snippet{}, err
	}

	if err := atomicfile.WriteFile(path, renderSnippet(snippet), 0o644); err != nil {
		return Snippet{}, err
	}
	return snippet, nil
}

// SnippetVersion returns the version token of the Snippet file GetSnippet
// would read for id.
func SnippetVersion(dataDir, id string) (string, error) {
	return recordversion.Of(filepath.Join(dataDir, snippetDir, id+".md"))
}

// DeleteSnippet removes the file GetSnippet would read for id.
func DeleteSnippet(dataDir, id string) error {
	return DeleteSnippetIfMatch(dataDir, id, "")
}

// DeleteSnippetIfMatch is DeleteSnippet, refusing with
// recordversion.ErrMismatch when version no longer matches the file on
// disk (issue #89, story 18).
func DeleteSnippetIfMatch(dataDir, id, version string) error {
	path := filepath.Join(dataDir, snippetDir, id+".md")
	if err := recordversion.Check(path, version); err != nil {
		return err
	}
	return os.Remove(path)
}

// ValidateSnippet checks that a Snippet's required fields are present before
// it is written to disk.
func ValidateSnippet(s Snippet) error {
	if s.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if strings.TrimSpace(s.Body) == "" {
		return fmt.Errorf("body is required")
	}
	// lang is optional — an unmarked Snippet is language-agnostic, which is
	// what every Snippet written before this field was one. When set it must
	// be an ISO 639-1 code, so it can be compared with the target language
	// the Generation resolved.
	if s.Lang != "" && !iso639_1.MatchString(s.Lang) {
		return fmt.Errorf("lang must be a two-letter ISO 639-1 code (e.g. \"en\", \"it\"), got %q", s.Lang)
	}
	return nil
}

func renderSnippet(s Snippet) []byte {
	raw := rawSnippetFrontmatter{Kind: s.Kind, Lang: s.Lang, Tags: s.Tags}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	fmBytes, _ := yaml.Marshal(raw) //nolint:errcheck // raw is a plain struct of strings/slices; yaml.Marshal cannot fail on it
	buf.Write(fmBytes)
	buf.WriteString("---\n\n")
	buf.WriteString(s.Body)
	buf.WriteString("\n")
	return buf.Bytes()
}
