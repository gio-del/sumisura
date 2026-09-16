package generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// ErrInvalidRenderRequest marks a Render request that can't be compiled:
// a malformed slug, or approved content citing an Entry Master Data no
// longer has.
var ErrInvalidRenderRequest = errors.New("invalid render request")

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// now is Render's clock, used to timestamp each Generation's output
// directory. An unexported package-level variable (not a RenderRequest
// field, which is decoded straight from a JSON body) so tests can pin it
// and assert an exact directory name.
var now = time.Now

// outputTimestampLayout is the UTC timestamp appended to a Generation's
// label: sortable, readable, and digits-and-dashes only, so the derived
// slug still satisfies slugRe (and the file-serving endpoint's copy of it).
const outputTimestampLayout = "20060102-150405"

// RenderRequest is the approved (and possibly user-edited) content from
// Text Review, ready to compile — see CONTEXT.md's Render entry. Slug is a
// kebab-case *label* for this Generation (e.g. the company applied to, or
// "default"), not the final directory name: Render derives a unique
// output/<label>-<UTC yyyymmdd-hhmmss>/ directory from it (issue #105) and
// returns that as RenderResult.Slug, so a later Generation with the same
// label never overwrites an earlier one's files.
type RenderRequest struct {
	Slug        string
	Selection   SelectionResult
	CoverLetter *CoverLetterResult

	// Language is the confirmed target language from Text Review
	// (GenerateResult.Language, possibly overridden). Empty is treated as
	// DefaultLanguage, so requests from before this field existed still
	// render correctly.
	Language string
}

// RenderResult names the produced PDF(s), relative to outputDir, plus the
// CV's page count so Visual Review can flag overflow (story 10), and each
// produced PDF's ATS-parsability check result so Visual Review can flag a
// bad text-layer extraction alongside it (PRD "PDF ATS-parsability
// check", story 5) — CoverLetterParsability is only set when req.CoverLetter
// was, matching CoverLetterPath.
//
// Slug is the directory Render actually wrote — derived from, and not
// equal to, RenderRequest.Slug — and is what a GenerationRecord must store.
type RenderResult struct {
	Slug                   string             `json:"slug"`
	CVPath                 string             `json:"cvPath"`
	CoverLetterPath        string             `json:"coverLetterPath,omitempty"`
	CVPageCount            int                `json:"cvPageCount"`
	CVParsability          ParsabilityResult  `json:"cvParsability"`
	CoverLetterParsability *ParsabilityResult `json:"coverLetterParsability,omitempty"`
}

type cvExperience struct {
	Employer string   `json:"employer"`
	Role     string   `json:"role"`
	Client   string   `json:"client,omitempty"`
	Location string   `json:"location,omitempty"`
	Start    string   `json:"start"`
	End      *string  `json:"end"`
	Bullets  []string `json:"bullets"`
}

type cvProject struct {
	Name    string   `json:"name"`
	Repo    string   `json:"repo,omitempty"`
	Start   string   `json:"start"`
	End     *string  `json:"end"`
	Bullets []string `json:"bullets"`
}

type cvData struct {
	Name         string                   `json:"name"`
	Lang         string                   `json:"lang"`
	Location     string                   `json:"location"`
	Email        string                   `json:"email"`
	Phone        string                   `json:"phone"`
	LinkedIn     string                   `json:"linkedin"`
	GitHub       string                   `json:"github"`
	Education    []masterdata.Education   `json:"education"`
	Experience   []cvExperience           `json:"experience"`
	Projects     []cvProject              `json:"projects"`
	TechStack    []string                 `json:"tech_stack"`
	Publications []masterdata.Publication `json:"publications"`
	// Certifications is a Static Section like the rest (issue #167): it is
	// carried through from profile.yaml untouched, never selected or
	// rewritten.
	Certifications []masterdata.Certification `json:"certifications"`
	Awards         []masterdata.Award         `json:"awards"`
	Activities     []masterdata.Activity      `json:"activities"`
	Languages      []masterdata.Language      `json:"languages"`
}

// emptySliceIfNil turns a nil slice into an empty one so it marshals as []
// rather than null — see the Static Sections comment in Render.
// TechStackMaxTags caps the derived Tech Stack line. Collecting every tag
// of every selected Entry does not survive a real career: four engagements
// and two projects produced 67 tags over six lines, which by itself pushed
// the render to a second page (issue #169). A list that contains everything
// also says nothing about what the candidate is strong in.
const TechStackMaxTags = 30

// deriveTechStack flattens the selected Entries' tags into the Tech Stack
// line: deduplicated, capped, and taken round-robin so every selected Entry
// contributes before any one of them fills the line.
//
// Selection has already ordered the Entries by relevance to the Job
// Description, and each Entry lists its own tags most-important-first, so
// taking one tag per Entry per pass keeps both orderings visible instead of
// letting the first two Entries spend the whole budget.
func deriveTechStack(tagsPerEntry [][]string) []string {
	seen := map[string]bool{}
	stack := []string{}
	for round := 0; len(stack) < TechStackMaxTags; round++ {
		progressed := false
		for _, tags := range tagsPerEntry {
			if round >= len(tags) {
				continue
			}
			progressed = true
			tag := tags[round]
			if seen[tag] {
				continue
			}
			seen[tag] = true
			stack = append(stack, tag)
			if len(stack) == TechStackMaxTags {
				return stack
			}
		}
		if !progressed {
			break
		}
	}
	return stack
}

func emptySliceIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

type coverLetterData struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	LinkedIn string `json:"linkedin"`
	GitHub   string `json:"github"`
	Body     string `json:"body"`
}

// Render compiles req into a Tailored CV PDF (and, if req.CoverLetter is
// set, a Cover Letter PDF) under a fresh projectRoot/output/<slug>/, where
// <slug> is derived from req.Slug by claimOutputDir, invoking the
// typst CLI exactly as the tailor-cv skill does (see CLAUDE.md), so it
// needs projectRoot to contain both template/ and output/ — the same
// "project root" the skill's own `--root .` invocation assumes.
func Render(projectRoot, dataDir string, req RenderRequest) (RenderResult, error) {
	if !slugRe.MatchString(req.Slug) {
		return RenderResult{}, fmt.Errorf("%w: slug must be kebab-case (e.g. \"acme-corp\"), got %q", ErrInvalidRenderRequest, req.Slug)
	}

	profile, err := masterdata.GetProfile(dataDir)
	if err != nil {
		return RenderResult{}, fmt.Errorf("loading profile: %w", err)
	}
	entries, err := masterdata.ListEntries(dataDir)
	if err != nil {
		return RenderResult{}, fmt.Errorf("loading master data: %w", err)
	}
	entriesByID := make(map[string]masterdata.Entry, len(entries))
	for _, e := range entries {
		entriesByID[e.ID] = e
	}

	cv, err := assembleCVData(profile, entriesByID, req.Selection)
	if err != nil {
		return RenderResult{}, err
	}
	cv.Lang = NormalizeLanguage(req.Language)

	slug, err := claimOutputDir(projectRoot, req.Slug, now())
	if err != nil {
		return RenderResult{}, err
	}
	outputDir := filepath.Join(projectRoot, "output", slug)

	cvRelPath, err := renderTypst(projectRoot, "template/cv.typ", slug, "data.json", "cv.pdf", cv)
	if err != nil {
		return RenderResult{}, err
	}
	pageCount, err := countPDFPages(filepath.Join(projectRoot, cvRelPath))
	if err != nil {
		return RenderResult{}, fmt.Errorf("counting rendered pages: %w", err)
	}
	cvParsability := checkPDFParsability(filepath.Join(projectRoot, cvRelPath), cvExpectedFields(cv))

	result := RenderResult{Slug: slug, CVPath: cvRelPath, CVPageCount: pageCount, CVParsability: cvParsability}

	if req.CoverLetter != nil {
		cl := coverLetterData{
			Name:     profile.Name,
			Location: profile.Location,
			Email:    profile.Email,
			Phone:    profile.Phone,
			LinkedIn: profile.LinkedIn,
			GitHub:   profile.GitHub,
			Body:     req.CoverLetter.Body,
		}
		clRelPath, err := renderTypst(projectRoot, "template/cover-letter.typ", slug, "cover-letter-data.json", "cover-letter.pdf", cl)
		if err != nil {
			return RenderResult{}, err
		}
		result.CoverLetterPath = clRelPath
		clParsability := checkPDFParsability(filepath.Join(projectRoot, clRelPath), coverLetterExpectedFields(cl))
		result.CoverLetterParsability = &clParsability

		// A plain-text copy alongside the PDF, so it can be downloaded as
		// either (story 11) without re-deriving it from the PDF.
		txtPath := filepath.Join(outputDir, "cover-letter.txt")
		if err := os.WriteFile(txtPath, []byte(req.CoverLetter.Body), 0o644); err != nil {
			return RenderResult{}, fmt.Errorf("writing cover letter text: %w", err)
		}
	}

	return result, nil
}

// maxOutputDirAttempts bounds claimOutputDir's -2, -3, ... disambiguation
// so a pathological output/ can't spin it forever.
const maxOutputDirAttempts = 1000

// claimOutputDir creates and returns the slug of a brand-new
// projectRoot/output/<slug>/ directory for one Generation: <label>-<UTC
// yyyymmdd-hhmmss>, or, if that directory already exists (two renders in
// the same second, or a directory left by an earlier run or the tailor-cv
// skill), the first free <label>-<timestamp>-N for N = 2, 3, .... Each
// candidate is claimed with a single os.Mkdir, which fails if the
// directory exists, so Render never writes into another Generation's
// directory — not even under concurrent renders. This is the one place the
// uniqueness rule lives (issue #105); SKILL.md step 5 mirrors it in prose.
func claimOutputDir(projectRoot, label string, at time.Time) (string, error) {
	outputRoot := filepath.Join(projectRoot, "output")
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	base := label + "-" + at.UTC().Format(outputTimestampLayout)
	for n := 1; n <= maxOutputDirAttempts; n++ {
		slug := base
		if n > 1 {
			slug = base + "-" + strconv.Itoa(n)
		}
		err := os.Mkdir(filepath.Join(outputRoot, slug), 0o755)
		if err == nil {
			return slug, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("creating output directory: %w", err)
		}
	}
	return "", fmt.Errorf("creating output directory: no free name for %q after %d attempts", base, maxOutputDirAttempts)
}

// renderTypst writes data as JSON to output/<slug>/<dataFile>, then invokes
// `typst compile --root projectRoot template/<template> output/<slug>/<pdfFile>
// --input data=output/<slug>/<dataFile>`, exactly the tailor-cv skill's
// invocation (see CLAUDE.md), and returns the PDF's path relative to
// projectRoot.
func renderTypst(projectRoot, templateRelPath, slug, dataFile, pdfFile string, data any) (string, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshaling render data: %w", err)
	}

	dataRelPath := filepath.Join("output", slug, dataFile)
	if err := os.WriteFile(filepath.Join(projectRoot, dataRelPath), encoded, 0o644); err != nil {
		return "", fmt.Errorf("writing render data: %w", err)
	}

	pdfRelPath := filepath.Join("output", slug, pdfFile)
	cmd := exec.Command("typst", "compile",
		"--root", projectRoot,
		filepath.Join(projectRoot, templateRelPath),
		filepath.Join(projectRoot, pdfRelPath),
		"--input", "data="+dataRelPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("typst compile failed: %w\n%s", err, out)
	}
	return pdfRelPath, nil
}

func assembleCVData(profile masterdata.Profile, entriesByID map[string]masterdata.Entry, selection SelectionResult) (cvData, error) {
	cv := cvData{
		Name:       profile.Name,
		Location:   profile.Location,
		Email:      profile.Email,
		Phone:      profile.Phone,
		LinkedIn:   profile.LinkedIn,
		GitHub:     profile.GitHub,
		Education:  emptySliceIfNil(profile.Education),
		Experience: []cvExperience{},
		Projects:   []cvProject{},
		TechStack:  []string{},
		// The Static Sections are emitted as [] rather than null when
		// absent: template/cv.typ guards each one with `.len() > 0`, and
		// `none.len()` is a hard typst error — so a profile with no awards
		// (or no certifications) would fail the render instead of simply
		// omitting the section.
		Publications:   emptySliceIfNil(profile.Publications),
		Certifications: emptySliceIfNil(profile.Certifications),
		Awards:         emptySliceIfNil(profile.Awards),
		Activities:     emptySliceIfNil(profile.Activities),
		Languages:      emptySliceIfNil(profile.Languages),
	}

	var selectedTags [][]string
	for _, se := range selection.Entries {
		if len(se.Bullets) == 0 {
			continue
		}
		entry, ok := entriesByID[se.EntryID]
		if !ok {
			return cvData{}, fmt.Errorf("%w: approved content cites unknown entry id %q", ErrInvalidRenderRequest, se.EntryID)
		}

		bullets := make([]string, len(se.Bullets))
		for i, b := range se.Bullets {
			bullets[i] = b.Rewritten
		}

		switch entry.Type {
		case masterdata.TypeExperience:
			cv.Experience = append(cv.Experience, cvExperience{
				Employer: entry.Employer,
				Role:     entry.Role,
				Client:   entry.Client,
				Location: entry.Location,
				Start:    entry.Start,
				End:      entry.End,
				Bullets:  bullets,
			})
		case masterdata.TypeProject:
			cv.Projects = append(cv.Projects, cvProject{
				Name:    entry.Name,
				Repo:    entry.Repo,
				Start:   entry.Start,
				End:     entry.End,
				Bullets: bullets,
			})
		}

		selectedTags = append(selectedTags, entry.Tags)
	}
	cv.TechStack = deriveTechStack(selectedTags)

	return cv, nil
}

// pagesRe matches a PDF's page tree dictionary, e.g. "/Type/Pages/Count 3"
// or "/Count 3/Type/Pages" — typst writes this uncompressed.
var pagesRe = regexp.MustCompile(`/Type\s*/Pages[^<>]{0,200}?/Count\s+(\d+)|/Count\s+(\d+)[^<>]{0,200}?/Type\s*/Pages`)

// countPDFPages reads a rendered PDF's page count directly from its object
// structure — good enough for asserting "the CV is one page" (per the
// PRD's Testing Decisions) without a PDF parsing dependency.
func countPDFPages(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	m := pagesRe.FindSubmatch(content)
	if m == nil {
		return 0, fmt.Errorf("could not find a page count in %s", path)
	}
	for _, group := range m[1:] {
		if len(group) > 0 {
			return strconv.Atoi(string(group))
		}
	}
	return 0, fmt.Errorf("could not find a page count in %s", path)
}
