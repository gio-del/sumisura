package tracking

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/gio-del/sumisura/backend/internal/generation"
)

// IndexedGeneration is one row of the "every CV generated so far" index
// (issue #173): a Generation recorded against an Application, or an
// output/<slug>/ directory that no record mentions.
//
// The two halves exist because a Generation is only recorded when it was
// made for a tracked Application. A Default Mode run — from the app or from
// the tailor-cv skill — writes output/<slug>/ and nothing else, so reading
// records alone would report an empty history while PDFs sit on disk.
type IndexedGeneration struct {
	Slug string `json:"slug"`
	// CreatedAt is RFC3339. For a recorded Generation it is the record's
	// own timestamp; for an unrecorded directory it is decoded from the
	// slug's -yyyymmdd-hhmmss suffix (the naming rule Render enforces),
	// which is the only date the directory carries that the user chose.
	CreatedAt string `json:"createdAt"`
	// Recorded distinguishes "this Generation is tracked against an
	// Application" from "this is a directory we found". An unrecorded row
	// has no Application, groundedness or language — absent because nothing
	// ever recorded them, not because they were empty.
	Recorded        bool                           `json:"recorded"`
	ApplicationID   string                         `json:"applicationId,omitempty"`
	Company         string                         `json:"company,omitempty"`
	JobTitle        string                         `json:"jobTitle,omitempty"`
	CVPath          string                         `json:"cvPath,omitempty"`
	CoverLetterPath string                         `json:"coverLetterPath,omitempty"`
	Language        string                         `json:"language,omitempty"`
	Groundedness    *generation.GroundednessResult `json:"groundedness,omitempty"`
	// HasCV/HasCoverLetter are checked on disk at read time. A recorded
	// Generation whose output/ directory has since been deleted is an
	// ordinary state, not an error (see CLAUDE.md on output/), and the UI
	// shows the row without working links rather than hiding it.
	HasCV          bool `json:"hasCv"`
	HasCoverLetter bool `json:"hasCoverLetter"`
}

// generationSlugTimestamp matches the -yyyymmdd-hhmmss stamp Render appends
// to every output directory name, optionally followed by the -2, -3, …
// disambiguator it adds when that name is taken (issue #105).
var generationSlugTimestamp = regexp.MustCompile(`-(\d{8})-(\d{6})(?:-\d+)?$`)

// ListGenerations returns every Generation known to this installation,
// newest first: the ones recorded against Applications under dataDir, plus
// every output/<slug>/ directory under projectRoot that no record claims.
func ListGenerations(dataDir, projectRoot string) ([]IndexedGeneration, error) {
	listings, err := List(dataDir)
	if err != nil {
		return nil, err
	}

	outputDir := filepath.Join(projectRoot, "output")
	var index []IndexedGeneration
	recorded := map[string]bool{}

	for _, l := range listings {
		for _, g := range l.Application.Generations {
			recorded[g.Slug] = true
			cv, cover := generationFilesOnDisk(outputDir, g.Slug)
			index = append(index, IndexedGeneration{
				Slug:            g.Slug,
				CreatedAt:       g.CreatedAt,
				Recorded:        true,
				ApplicationID:   l.Application.ID,
				Company:         l.JobListing.Company,
				JobTitle:        l.JobListing.Title,
				CVPath:          g.CVPath,
				CoverLetterPath: g.CoverLetterPath,
				Language:        g.Language,
				Groundedness:    g.Groundedness,
				HasCV:           cv,
				HasCoverLetter:  cover,
			})
		}
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || recorded[e.Name()] {
			continue
		}
		cv, cover := generationFilesOnDisk(outputDir, e.Name())
		if !cv && !cover {
			// Neither PDF: a half-written or hand-made directory, not a
			// Generation anyone can open. Listing it would only offer
			// dead links.
			continue
		}
		index = append(index, IndexedGeneration{
			Slug:           e.Name(),
			CreatedAt:      createdAtFromSlug(e.Name()),
			Recorded:       false,
			HasCV:          cv,
			HasCoverLetter: cover,
		})
	}

	// Newest first, with the slug as a stable tiebreaker so two Generations
	// stamped in the same second keep a deterministic order.
	sort.SliceStable(index, func(i, j int) bool {
		if index[i].CreatedAt != index[j].CreatedAt {
			return index[i].CreatedAt > index[j].CreatedAt
		}
		return index[i].Slug > index[j].Slug
	})
	return index, nil
}

func generationFilesOnDisk(outputDir, slug string) (cv, coverLetter bool) {
	if _, err := os.Stat(filepath.Join(outputDir, slug, "cv.pdf")); err == nil {
		cv = true
	}
	if _, err := os.Stat(filepath.Join(outputDir, slug, "cover-letter.pdf")); err == nil {
		coverLetter = true
	}
	return cv, coverLetter
}

// createdAtFromSlug decodes Render's -yyyymmdd-hhmmss stamp. A directory
// that doesn't carry one (renamed by hand, or written before the scheme
// existed) gets an empty date rather than a guessed one: the file mtime
// would say when it was last touched, not when it was generated.
func createdAtFromSlug(slug string) string {
	m := generationSlugTimestamp.FindStringSubmatch(slug)
	if m == nil {
		return ""
	}
	t, err := time.Parse("20060102150405", m[1]+m[2])
	if err != nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
