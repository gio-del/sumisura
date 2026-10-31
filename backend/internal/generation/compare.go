package generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// ErrGenerationNotFound means an output directory named in a comparison does
// not exist.
var ErrGenerationNotFound = errors.New("generation not found")

// CompareEntryRef names an Entry in a comparison by its title (project name,
// role or name, whichever the Entry has), keeping the id for linking.
type CompareEntryRef struct {
	EntryID string `json:"entryId"`
	Title   string `json:"title"`
}

// CompareBulletChange is one rewritten bullet that differs between the two
// Generations. A or B is empty when that side did not keep the bullet.
type CompareBulletChange struct {
	EntryID     string `json:"entryId"`
	Title       string `json:"title"`
	SourceIndex int    `json:"sourceIndex"`
	A           string `json:"a"`
	B           string `json:"b"`
}

// CompareTagChange is the Tag Row of an Entry both Generations kept, when the
// two differ.
type CompareTagChange struct {
	EntryID string   `json:"entryId"`
	Title   string   `json:"title"`
	A       []string `json:"a"`
	B       []string `json:"b"`
}

// CompareText is a piece of prose (About me, Cover Letter) on both sides.
type CompareText struct {
	A       string `json:"a"`
	B       string `json:"b"`
	Changed bool   `json:"changed"`
}

// CompareSide says what a Generation's directory could contribute.
type CompareSide struct {
	Slug string `json:"slug"`
	// SelectionAvailable is false for a legacy Generation with no readable
	// selection.json: its Entries, bullets, summary and tags are unknown.
	SelectionAvailable   bool `json:"selectionAvailable"`
	CoverLetterAvailable bool `json:"coverLetterAvailable"`
}

// Comparison is the read-only textual difference between two Generations.
// The selection-derived fields are empty when either side lacks its
// selection.json; Cover Letter is nil unless both sides have one.
type Comparison struct {
	A                CompareSide           `json:"a"`
	B                CompareSide           `json:"b"`
	EntriesAdded     []CompareEntryRef     `json:"entriesAdded"`
	EntriesRemoved   []CompareEntryRef     `json:"entriesRemoved"`
	RewrittenChanges []CompareBulletChange `json:"rewrittenChanges"`
	TagChanges       []CompareTagChange    `json:"tagChanges"`
	Summary          *CompareText          `json:"summary,omitempty"`
	CoverLetter      *CompareText          `json:"coverLetter,omitempty"`
}

type compareInput struct {
	side      CompareSide
	selection SelectionResult
	letter    string
}

func readCompareInput(projectRoot, slug string) (compareInput, error) {
	dir := filepath.Join(projectRoot, "output", slug)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return compareInput{}, fmt.Errorf("%w: %s", ErrGenerationNotFound, slug)
	}
	in := compareInput{side: CompareSide{Slug: slug}}
	if content, err := os.ReadFile(filepath.Join(dir, "selection.json")); err == nil {
		if json.Unmarshal(content, &in.selection) == nil {
			in.side.SelectionAvailable = true
		}
	}
	if content, err := os.ReadFile(filepath.Join(dir, "cover-letter.txt")); err == nil {
		in.letter = string(content)
		in.side.CoverLetterAvailable = true
	}
	return in, nil
}

// CompareGenerations reads two Generations' output directories and computes
// what changed between a and b. It never calls Claude and writes nothing.
// Entry titles come from Master Data, falling back to the id for an Entry
// that has since been deleted.
func CompareGenerations(dataDir, projectRoot, a, b string) (Comparison, error) {
	inA, err := readCompareInput(projectRoot, a)
	if err != nil {
		return Comparison{}, err
	}
	inB, err := readCompareInput(projectRoot, b)
	if err != nil {
		return Comparison{}, err
	}

	cmp := Comparison{
		A: inA.side, B: inB.side,
		EntriesAdded: []CompareEntryRef{}, EntriesRemoved: []CompareEntryRef{},
		RewrittenChanges: []CompareBulletChange{}, TagChanges: []CompareTagChange{},
	}
	if inA.side.CoverLetterAvailable && inB.side.CoverLetterAvailable {
		cmp.CoverLetter = &CompareText{A: inA.letter, B: inB.letter, Changed: inA.letter != inB.letter}
	}
	if !inA.side.SelectionAvailable || !inB.side.SelectionAvailable {
		return cmp, nil
	}

	titles := map[string]string{}
	if entries, err := masterdata.ListEntries(dataDir); err == nil {
		for _, e := range entries {
			titles[e.ID] = entryTitle(e)
		}
	}
	title := func(id string) string {
		if t := titles[id]; t != "" {
			return t
		}
		return id
	}

	byID := func(s SelectionResult) (map[string]SelectedEntry, []string) {
		m := map[string]SelectedEntry{}
		var order []string
		for _, e := range s.Entries {
			if _, seen := m[e.EntryID]; !seen {
				order = append(order, e.EntryID)
			}
			m[e.EntryID] = e
		}
		return m, order
	}
	entriesA, orderA := byID(inA.selection)
	entriesB, orderB := byID(inB.selection)

	for _, id := range orderB {
		if _, ok := entriesA[id]; !ok {
			cmp.EntriesAdded = append(cmp.EntriesAdded, CompareEntryRef{EntryID: id, Title: title(id)})
		}
	}
	for _, id := range orderA {
		eb, ok := entriesB[id]
		if !ok {
			cmp.EntriesRemoved = append(cmp.EntriesRemoved, CompareEntryRef{EntryID: id, Title: title(id)})
			continue
		}
		ea := entriesA[id]
		cmp.RewrittenChanges = append(cmp.RewrittenChanges, bulletChanges(id, title(id), ea, eb)...)
		if !sameStrings(ea.Tags, eb.Tags) {
			cmp.TagChanges = append(cmp.TagChanges, CompareTagChange{
				EntryID: id, Title: title(id), A: nonNil(ea.Tags), B: nonNil(eb.Tags),
			})
		}
	}

	cmp.Summary = &CompareText{
		A: inA.selection.Summary, B: inB.selection.Summary,
		Changed: inA.selection.Summary != inB.selection.Summary,
	}
	return cmp, nil
}

func entryTitle(e masterdata.Entry) string {
	for _, s := range []string{e.Title, e.Role, e.Name, e.Client, e.Employer} {
		if s != "" {
			return s
		}
	}
	return e.ID
}

// bulletChanges pairs the two sides' bullets by the source bullet they were
// rewritten from, so a reordered or dropped bullet shows as a change rather
// than shifting every following pair.
func bulletChanges(id, title string, a, b SelectedEntry) []CompareBulletChange {
	ra := map[int]string{}
	rb := map[int]string{}
	var idx []int
	for _, x := range a.Bullets {
		if _, ok := ra[x.SourceIndex]; !ok {
			idx = append(idx, x.SourceIndex)
		}
		ra[x.SourceIndex] = x.Rewritten
	}
	for _, x := range b.Bullets {
		if _, ok := ra[x.SourceIndex]; !ok {
			if _, dup := rb[x.SourceIndex]; !dup {
				idx = append(idx, x.SourceIndex)
			}
		}
		rb[x.SourceIndex] = x.Rewritten
	}
	sort.Ints(idx)
	var out []CompareBulletChange
	for _, i := range idx {
		ta, inA := ra[i]
		tb, inB := rb[i]
		if inA && inB && ta == tb {
			continue
		}
		out = append(out, CompareBulletChange{EntryID: id, Title: title, SourceIndex: i, A: ta, B: tb})
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
