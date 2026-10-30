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

	// JobDescription is the one this Generation was tailored to, for the
	// ATS Report's term coverage (issue #198). Empty — Default Mode — leaves
	// term coverage out.
	JobDescription string
}

// RenderResult names the produced PDF(s), relative to outputDir, plus the
// CV's page count so Visual Review can flag overflow (story 10), and the
// ATS Reports of the produced PDFs so Visual Review can flag a bad
// text-layer extraction alongside it (issues #49, #198) — the Cover
// Letter's is only set when req.CoverLetter was, matching CoverLetterPath.
// The same reports are written to output/<slug>/ats-report.json.
//
// Slug is the directory Render actually wrote — derived from, and not
// equal to, RenderRequest.Slug — and is what a GenerationRecord must store.
type RenderResult struct {
	Slug            string     `json:"slug"`
	CVPath          string     `json:"cvPath"`
	CoverLetterPath string     `json:"coverLetterPath,omitempty"`
	CVPageCount     int        `json:"cvPageCount"`
	ATSReports      ATSReports `json:"atsReports"`
}

type cvExperience struct {
	Employer string `json:"employer"`
	// Kind is the Entry's resolved shape (ADR-0045). The template groups
	// Client Engagements under one employer header and leaves an In-house
	// Role as a row of its own, so it must be told which it is rather than
	// guessing from whether a client happens to be named.
	Kind   string `json:"kind"`
	Role   string `json:"role"`
	Client string `json:"client,omitempty"`
	// ClientLabel replaces Client on the CV and Title names the engagement
	// (issue #232); EmployerDisplay and EmployerRole come from
	// profile.yaml's employers and head the employer block.
	ClientLabel     string  `json:"client_label,omitempty"`
	Title           string  `json:"title,omitempty"`
	Group           string  `json:"group,omitempty"`
	EmployerDisplay string  `json:"employer_display,omitempty"`
	EmployerRole    string  `json:"employer_role,omitempty"`
	Location        string  `json:"location,omitempty"`
	Start           string  `json:"start"`
	End             *string `json:"end"`
	// EmployerStart/EmployerEnd are the Employer Tenure this engagement
	// falls in, derived from every Entry of that employer in Master Data
	// and not only the Selected ones: dropping a Client Engagement from a
	// Generation must narrow what the CV shows, never the employment it
	// claims. EmployerEnd is nil while the employment is ongoing.
	EmployerStart string   `json:"employer_start"`
	EmployerEnd   *string  `json:"employer_end"`
	Bullets       []string `json:"bullets"`
	// Tags is the row printed under the engagement (issue #232).
	Tags []string `json:"tags"`
}

type cvProject struct {
	Name    string   `json:"name"`
	Repo    string   `json:"repo,omitempty"`
	Start   string   `json:"start"`
	End     *string  `json:"end"`
	Bullets []string `json:"bullets"`
	Tags    []string `json:"tags"`
}

type cvData struct {
	Name         string                   `json:"name"`
	Lang         string                   `json:"lang"`
	Location     string                   `json:"location"`
	Email        string                   `json:"email"`
	Phone        string                   `json:"phone"`
	LinkedIn     string                   `json:"linkedin"`
	GitHub       string                   `json:"github"`
	LocationNote string                   `json:"location_note,omitempty"`
	Headline     string                   `json:"headline,omitempty"`
	Summary      string                   `json:"summary,omitempty"`
	Education    []masterdata.Education   `json:"education"`
	Experience   []cvExperience           `json:"experience"`
	Projects     []cvProject              `json:"projects"`
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
// inlineCode matches a Markdown `code span`, the one piece of markup a
// bullet reliably picks up: Master Data files are Markdown, so writing
// `server.json` is the natural instinct, and template/cv.typ prints what it
// is given literally (issue #170).
var inlineCode = regexp.MustCompile("`([^`]+)`")

// stripInlineMarkup unwraps code spans so the PDF shows the text rather
// than the backticks. Only backticks are touched: an asterisk or an
// underscore can be the author's own punctuation, and silently deleting one
// would change what the bullet says. Those are reported by the rendered-CV
// check instead (findMarkdownMarkup).
func stripInlineMarkup(s string) string {
	return inlineCode.ReplaceAllString(s, "$1")
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
	var tags []string
	for _, e := range entries {
		tags = append(tags, e.Tags...)
	}
	cvReport := buildATSReport(filepath.Join(projectRoot, cvRelPath), cvExpectedFields(cv), req.JobDescription, tags)

	result := RenderResult{Slug: slug, CVPath: cvRelPath, CVPageCount: pageCount, ATSReports: ATSReports{CV: cvReport}}

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
		clReport := buildATSReport(filepath.Join(projectRoot, clRelPath), coverLetterExpectedFields(cl), "", nil)
		result.ATSReports.CoverLetter = &clReport

		// A plain-text copy alongside the PDF, so it can be downloaded as
		// either (story 11) without re-deriving it from the PDF.
		txtPath := filepath.Join(outputDir, "cover-letter.txt")
		if err := os.WriteFile(txtPath, []byte(req.CoverLetter.Body), 0o644); err != nil {
			return RenderResult{}, fmt.Errorf("writing cover letter text: %w", err)
		}
	}

	if err := WriteATSReports(outputDir, result.ATSReports); err != nil {
		return RenderResult{}, fmt.Errorf("writing ATS report: %w", err)
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
	if err := validateTitles(profile, selection); err != nil {
		return cvData{}, fmt.Errorf("%w: %s", ErrInvalidRenderRequest, err)
	}
	cv := cvData{
		Name:         profile.Name,
		Location:     profile.Location,
		Email:        profile.Email,
		Phone:        profile.Phone,
		LinkedIn:     profile.LinkedIn,
		GitHub:       profile.GitHub,
		LocationNote: profile.LocationNote,
		Headline:     firstNonEmpty(selection.Headline, profile.Headline),
		Summary:      stripInlineMarkup(selection.Summary),
		Education:    emptySliceIfNil(profile.Education),
		Experience:   []cvExperience{},
		Projects:     []cvProject{},
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

	allEntries := make([]masterdata.Entry, 0, len(entriesByID))
	for _, e := range entriesByID {
		allEntries = append(allEntries, e)
	}

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
			bullets[i] = stripInlineMarkup(b.Rewritten)
		}
		tags := se.Tags
		if len(tags) == 0 {
			tags = firstTags(entry.Tags)
		}
		tags = emptySliceIfNil(tags)

		switch entry.Type {
		case masterdata.TypeExperience:
			item := cvExperience{
				Employer:        entry.Employer,
				Kind:            entry.Shape(),
				Role:            entry.Role,
				Client:          entry.Client,
				ClientLabel:     entry.ClientLabel,
				Group:           entry.Group,
				Title:           entry.Title,
				EmployerDisplay: profile.Employers[entry.Employer].Display,
				EmployerRole:    firstNonEmpty(selection.EmployerRoles[entry.Employer], profile.Employers[entry.Employer].Role),
				Location:        entry.Location,
				Start:           entry.Start,
				End:             entry.End,
				EmployerStart:   entry.Start,
				EmployerEnd:     entry.End,
				Bullets:         bullets,
				Tags:            tags,
			}
			if tenure, ok := masterdata.TenureOf(allEntries, entry); ok {
				item.EmployerStart, item.EmployerEnd = tenure.Start, tenure.End
			}
			cv.Experience = append(cv.Experience, item)
		case masterdata.TypeProject:
			cv.Projects = append(cv.Projects, cvProject{
				Name:    entry.Name,
				Repo:    entry.Repo,
				Start:   entry.Start,
				End:     entry.End,
				Bullets: bullets,
				Tags:    tags,
			})
		}
	}
	cv.Experience = orderExperience(cv.Experience)

	return cv, nil
}

// orderExperience keeps employer blocks in Selection's order — Selection
// decided which employer leads, and that is a relevance judgement — and,
// inside one block, the engagements in Selection's order too: they print
// without dates of their own (ADR-0048), so the most relevant leads. Entries
// sharing a Group stay together, at the place of the first one.
func orderExperience(items []cvExperience) []cvExperience {
	type group struct {
		key   string
		items []cvExperience
	}
	var groups []group
	index := make(map[string]int, len(items))

	for _, item := range items {
		key := item.Employer + "\x00" + item.EmployerStart
		if item.Kind == masterdata.KindInHouse {
			// An In-house Role is a job of its own and never joins a block,
			// so give it a key nothing else can share.
			key = fmt.Sprintf("in-house\x00%d", len(groups))
		}
		if at, ok := index[key]; ok {
			groups[at].items = append(groups[at].items, item)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, group{key: key, items: []cvExperience{item}})
	}

	ordered := make([]cvExperience, 0, len(items))
	for _, g := range groups {
		ordered = append(ordered, keepGroupsTogether(g.items)...)
	}
	return ordered
}

// keepGroupsTogether moves every engagement sharing a Group right after the
// first one of that Group, so the template can print them as one project
// (issue #232). Order is otherwise unchanged.
func keepGroupsTogether(items []cvExperience) []cvExperience {
	out := make([]cvExperience, 0, len(items))
	placed := make([]bool, len(items))
	for i, item := range items {
		if placed[i] {
			continue
		}
		placed[i] = true
		out = append(out, item)
		if item.Group == "" {
			continue
		}
		for j := i + 1; j < len(items); j++ {
			if !placed[j] && items[j].Group == item.Group {
				placed[j] = true
				out = append(out, items[j])
			}
		}
	}
	return out
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

// firstNonEmpty is the chosen title, or the profile's own when none was
// chosen (issue #238).
func firstNonEmpty(chosen, def string) string {
	if chosen != "" {
		return chosen
	}
	return def
}
