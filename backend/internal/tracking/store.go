package tracking

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/recordversion"
	"gopkg.in/yaml.v3"
)

const (
	jobsDir         = "jobs"
	applicationsDir = "applications"
)

// ErrValidation marks a SaveRequest that can't be saved.
var ErrValidation = errors.New("validation failed")

// SaveRequest is the input to Save: a Company name plus a Job Description,
// pasted verbatim or fetched from a URL — mirroring
// generation.GenerateRequest's JobDescription/JobDescriptionURL shape
// (story 1).
type SaveRequest struct {
	Title   string
	Company string
	// Location is where the role is, free text from the source. Optional
	// everywhere: a board that doesn't say, or a save path with nowhere to
	// read it from, simply leaves it empty (issue #206).
	Location          string
	URL               string
	JobDescription    string
	JobDescriptionURL string
	// LogoURL is the source's Company Logo image URL (browser-extension
	// capture only — ATS/manual save paths never populate it), downloaded
	// best-effort by Save (ADR-0013).
	LogoURL string
	// ListingSalaryText is LinkedIn's own salary-insight badge text
	// (browser-extension capture only — ATS/manual save paths have no
	// equivalent structured field, same as LogoURL), fed to RAL Range
	// resolution alongside the Job Description (ADR-0014).
	ListingSalaryText string
}

type rawJobListingFrontmatter struct {
	// SchemaVersion is omitempty so a legacy Job Listing rewritten by an
	// edit keeps reading as legacy (no key) until MigrateRecords runs.
	SchemaVersion int    `yaml:"schemaVersion,omitempty"`
	Title         string `yaml:"title,omitempty"`
	Company       string `yaml:"company"`
	// Location is omitempty so a Job Listing saved without one renders
	// byte-identically to a file written before the field existed.
	Location           string              `yaml:"location,omitempty"`
	URL                string              `yaml:"url,omitempty"`
	Source             string              `yaml:"source"`
	SavedAt            string              `yaml:"savedAt"`
	RAL                generation.RALRange `yaml:"ral"`
	Logo               string              `yaml:"logo,omitempty"`
	FreshnessStatus    FreshnessStatus     `yaml:"freshnessStatus,omitempty"`
	FreshnessCheckedAt string              `yaml:"freshnessCheckedAt,omitempty"`
	// Archived is omitempty so a Job Listing that was never archived (or
	// was unarchived) renders byte-identically to a file written before
	// issue #98, and only archived records carry the key.
	Archived bool `yaml:"archived,omitempty"`
}

type rawApplication struct {
	SchemaVersion   int                `yaml:"schemaVersion,omitempty"`
	JobListingID    string             `yaml:"jobListingId"`
	Status          Status             `yaml:"status"`
	StatusUpdatedAt string             `yaml:"statusUpdatedAt,omitempty"`
	Method          ApplicationMethod  `yaml:"method"`
	Contact         *Contact           `yaml:"contact,omitempty"`
	Generations     []GenerationRecord `yaml:"generations,omitempty"`
	StatusHistory   []StatusChange     `yaml:"statusHistory,omitempty"`
	// Notes is omitempty so an Application without Notes renders exactly as
	// a file written before issue #96, with no stray empty key.
	Notes []Note `yaml:"notes,omitempty"`
}

// Save resolves req's Job Description (required — its absence blocks the
// save via ErrValidation, same as a missing Company), then attempts RAL
// Range resolution and Application Method inference independently and
// best-effort: either failing sets that field Unresolved rather than
// discarding the save (stories 1-4, 14) — only Company/Job Description
// validation and the one-Job-Listing-per-posting refusal (ErrDuplicate,
// issue #206) still block writing the Job Listing and its linked
// Application (Status Saved, "saving a Job Listing immediately creates its
// Application", story 2).
//
// The Posting Key check runs before the Job Description is resolved and
// before any Claude call, so a refused save costs neither a fetch nor a
// token and leaves the corpus untouched.
func Save(ctx context.Context, dataDir string, client Client, doer HTTPDoer, req SaveRequest) (JobListing, Application, error) {
	if strings.TrimSpace(req.Company) == "" {
		return JobListing{}, Application{}, fmt.Errorf("%w: company is required", ErrValidation)
	}

	existing, duplicate, err := findByPostingKey(dataDir, req.URL)
	if err != nil {
		return JobListing{}, Application{}, err
	}
	if duplicate {
		return JobListing{}, Application{}, &DuplicatePostingError{Existing: existing}
	}

	jobDescription, err := generation.ResolveJobDescription(ctx, req.JobDescription, req.JobDescriptionURL)
	if err != nil {
		return JobListing{}, Application{}, err
	}
	if jobDescription == "" {
		return JobListing{}, Application{}, fmt.Errorf("%w: jobDescription or jobDescriptionUrl is required", ErrValidation)
	}

	ral := resolveRALBestEffort(ctx, jobDescription, req.ListingSalaryText, client)
	method := resolveApplicationMethodBestEffort(ctx, jobDescription, client)
	RecordStandaloneUsage(dataDir, client)

	jobsFullDir := filepath.Join(dataDir, jobsDir)
	if err := os.MkdirAll(jobsFullDir, 0o755); err != nil {
		return JobListing{}, Application{}, err
	}
	slug := uniqueSlug(jobsFullDir, slugify(req.Company))
	logo := downloadLogoBestEffort(ctx, doer, req.LogoURL, jobsFullDir, slug)

	listing := JobListing{
		SchemaVersion:   CurrentSchemaVersion,
		ID:              slug,
		Title:           req.Title,
		Company:         req.Company,
		Location:        strings.TrimSpace(req.Location),
		URL:             req.URL,
		Source:          SourceManual,
		SavedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		JobDescription:  jobDescription,
		RAL:             ral,
		Logo:            logo,
		FreshnessStatus: FreshnessNotYetChecked,
	}
	if err := atomicfile.WriteFile(filepath.Join(jobsFullDir, slug+".md"), renderJobListing(listing), 0o644); err != nil {
		return JobListing{}, Application{}, err
	}

	applicationsFullDir := filepath.Join(dataDir, applicationsDir)
	if err := os.MkdirAll(applicationsFullDir, 0o755); err != nil {
		return JobListing{}, Application{}, err
	}
	application := Application{
		SchemaVersion:   CurrentSchemaVersion,
		ID:              slug,
		JobListingID:    slug,
		Status:          StatusSaved,
		StatusUpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Method:          method,
		StatusHistory:   []StatusChange{{Status: StatusSaved, ChangedAt: time.Now().UTC()}},
	}
	if err := atomicfile.WriteFile(filepath.Join(applicationsFullDir, slug+".md"), renderApplication(application), 0o644); err != nil {
		return JobListing{}, Application{}, err
	}

	return listing, application, nil
}

// List reads every Job Listing under dataDir/jobs, paired with its 1:1
// Application, newest-saved first — the pipeline-at-a-glance view (story 3).
func List(dataDir string) ([]ListingWithApplication, error) {
	jobsFullDir := filepath.Join(dataDir, jobsDir)
	files, err := os.ReadDir(jobsFullDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var result []ListingWithApplication
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		slug := strings.TrimSuffix(f.Name(), ".md")

		listing, err := getJobListing(dataDir, slug)
		if err != nil {
			return nil, fmt.Errorf("reading job listing %s: %w", f.Name(), err)
		}
		application, err := getApplication(dataDir, slug)
		if err != nil {
			return nil, fmt.Errorf("reading application for job listing %s: %w", slug, err)
		}
		result = append(result, ListingWithApplication{JobListing: listing, Application: application})
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].JobListing.SavedAt > result[j].JobListing.SavedAt
	})
	return result, nil
}

// GetJobListing reads a single Job Listing by id, so the FE can prefill a
// Generation from an existing Job Listing's Job Description (story 11).
func GetJobListing(dataDir, id string) (JobListing, error) {
	return getJobListing(dataDir, id)
}

// Get reads a single Job Listing by id paired with its 1:1 Application —
// the same pairing List builds per row and Resolve returns, for the Job
// Listing detail page (issue #94). A missing Job Listing surfaces as
// os.ErrNotExist exactly as GetJobListing does.
func Get(dataDir, id string) (ListingWithApplication, error) {
	listing, err := getJobListing(dataDir, id)
	if err != nil {
		return ListingWithApplication{}, err
	}
	application, err := getApplication(dataDir, id)
	if err != nil {
		return ListingWithApplication{}, err
	}
	return ListingWithApplication{JobListing: listing, Application: application}, nil
}

// Delete removes a Job Listing and its 1:1 Application together (story 9):
// the Company Logo file (if any), the Application file, then the Job
// Listing file — in that order so a partial failure never leaves the Job
// Listing behind without having tried to clean up what it owns. A missing
// Logo or Application file is tolerated (the Job Listing file is still
// removed); only the final removal's error is returned, so callers can
// errors.Is(err, os.ErrNotExist) exactly like masterdata.DeleteEntry.
func Delete(dataDir, id string) error {
	return DeleteIfMatch(dataDir, id, "", "")
}

// DeleteIfMatch is Delete, refusing with recordversion.ErrMismatch — and
// removing nothing — when jobListingVersion no longer matches the Job
// Listing file or applicationVersion no longer matches the Application
// file. Both are checked because the delete destroys both: a Status that
// moved on since the list was loaded must stop it just as an edited Job
// Listing does (issue #89, story 19). An empty token skips its check; a
// missing Job Listing file is os.ErrNotExist (not-found outranks
// conflict), while an Application already gone has nothing left to lose
// and is tolerated exactly as Delete tolerates it.
func DeleteIfMatch(dataDir, id, jobListingVersion, applicationVersion string) error {
	if err := recordversion.Check(filepath.Join(dataDir, jobsDir, id+".md"), jobListingVersion); err != nil {
		return err
	}
	if err := recordversion.Check(applicationPath(dataDir, id), applicationVersion); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	listing, err := getJobListing(dataDir, id)
	if err == nil && listing.Logo != "" {
		if rmErr := os.Remove(filepath.Join(dataDir, jobsDir, listing.Logo)); rmErr != nil && !os.IsNotExist(rmErr) {
			return rmErr
		}
	}

	if rmErr := os.Remove(filepath.Join(dataDir, applicationsDir, id+".md")); rmErr != nil && !os.IsNotExist(rmErr) {
		return rmErr
	}

	return os.Remove(filepath.Join(dataDir, jobsDir, id+".md"))
}

func getJobListing(dataDir, slug string) (JobListing, error) {
	content, err := os.ReadFile(filepath.Join(dataDir, jobsDir, slug+".md"))
	if err != nil {
		return JobListing{}, err
	}
	return parseJobListing(slug, content)
}

func parseJobListing(slug string, content []byte) (JobListing, error) {
	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return JobListing{}, err
	}
	var raw rawJobListingFrontmatter
	if err := yaml.Unmarshal(fm, &raw); err != nil {
		return JobListing{}, err
	}
	if err := checkSchemaVersion(raw.SchemaVersion); err != nil {
		return JobListing{}, fmt.Errorf("job listing %s: %w", slug, err)
	}
	// A legacy Job Listing may predate FreshnessStatus (issue #59), so it
	// reads as FreshnessNotYetChecked — exactly what MigrateRecords writes
	// down for it. A current record always carries the field (Save writes
	// it, migration backfills it), so it is read as stored (issue #100).
	freshnessStatus := raw.FreshnessStatus
	if freshnessStatus == "" && raw.SchemaVersion == LegacySchemaVersion {
		freshnessStatus = FreshnessNotYetChecked
	}
	return JobListing{
		SchemaVersion:      raw.SchemaVersion,
		ID:                 slug,
		Title:              raw.Title,
		Company:            raw.Company,
		Location:           raw.Location,
		URL:                raw.URL,
		Source:             raw.Source,
		SavedAt:            raw.SavedAt,
		JobDescription:     strings.TrimSpace(string(body)),
		RAL:                raw.RAL,
		Logo:               raw.Logo,
		FreshnessStatus:    freshnessStatus,
		FreshnessCheckedAt: raw.FreshnessCheckedAt,
		Archived:           raw.Archived,
	}, nil
}

// applicationPath is the file every Application mutation writes — the
// Application's own file, never its Job Listing's, even though the two
// share an id (issue #89).
func applicationPath(dataDir, id string) string {
	return filepath.Join(dataDir, applicationsDir, id+".md")
}

// writeApplicationIfMatch writes application back to disk unless version
// no longer matches what is there, in which case it returns
// recordversion.ErrMismatch and writes nothing. The comparison sits
// immediately before the write, inside the store, so the window between
// checking and writing is as small as a single-process app can make it.
// An empty version writes unconditionally (issue #89).
func writeApplicationIfMatch(dataDir, id string, application Application, version string) (Application, error) {
	path := applicationPath(dataDir, id)
	if err := recordversion.Check(path, version); err != nil {
		return Application{}, err
	}
	if err := atomicfile.WriteFile(path, renderApplication(application), 0o644); err != nil {
		return Application{}, err
	}
	return application, nil
}

// ApplicationVersion returns the version token of the Application file for
// id, computed from that file alone.
func ApplicationVersion(dataDir, id string) (string, error) {
	return recordversion.Of(applicationPath(dataDir, id))
}

// JobListingVersion returns the version token of the Job Listing file for
// id — what a Job Listing delete is checked against, distinct from the
// Application's own token.
func JobListingVersion(dataDir, id string) (string, error) {
	return recordversion.Of(filepath.Join(dataDir, jobsDir, id+".md"))
}

func getApplication(dataDir, slug string) (Application, error) {
	content, err := os.ReadFile(filepath.Join(dataDir, applicationsDir, slug+".md"))
	if err != nil {
		return Application{}, err
	}
	var raw rawApplication
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return Application{}, err
	}
	if err := checkApplicationSchemaVersions(raw); err != nil {
		return Application{}, fmt.Errorf("application %s: %w", slug, err)
	}
	sortNotesNewestFirst(raw.Notes)
	return Application{
		SchemaVersion:   raw.SchemaVersion,
		ID:              slug,
		JobListingID:    raw.JobListingID,
		Status:          raw.Status,
		StatusUpdatedAt: raw.StatusUpdatedAt,
		Method:          raw.Method,
		Contact:         raw.Contact,
		IsStale:         IsStale(raw.Status, raw.StatusUpdatedAt, time.Now(), DefaultStaleThreshold),
		Generations:     raw.Generations,
		StatusHistory:   raw.StatusHistory,
		Notes:           raw.Notes,
	}, nil
}

// splitFrontmatter splits a Job Listing file's content into its YAML
// frontmatter and Markdown body (its Job Description), mirroring
// masterdata's Entry file shape (ADR-0003 extended to Job Listings by
// ADR-0008).
func splitFrontmatter(content []byte) (frontmatter, body []byte, err error) {
	frontmatter, closingAndBody, err := locateFrontmatter(content)
	if err != nil {
		return nil, nil, err
	}
	after := closingAndBody[len("---"):]
	after = bytes.TrimPrefix(after, []byte("\r\n"))
	after = bytes.TrimPrefix(after, []byte("\n"))
	return frontmatter, after, nil
}

// locateFrontmatter finds a Job Listing file's frontmatter and returns it
// alongside the untouched remainder of the file, starting at the closing
// "---" delimiter — so MigrateRecords can rewrite the frontmatter while
// keeping the Job Description body byte for byte.
func locateFrontmatter(content []byte) (frontmatter, closingAndBody []byte, err error) {
	trimmed := bytes.TrimLeft(content, "\n")
	if !bytes.HasPrefix(trimmed, []byte("---")) {
		return nil, nil, fmt.Errorf("missing frontmatter delimiter")
	}
	rest := trimmed[len("---"):]
	rest = bytes.TrimPrefix(rest, []byte("\r\n"))
	rest = bytes.TrimPrefix(rest, []byte("\n"))

	idx := bytes.Index(rest, []byte("\n---"))
	if idx == -1 {
		return nil, nil, fmt.Errorf("missing closing frontmatter delimiter")
	}
	return rest[:idx], rest[idx+1:], nil
}

func renderJobListing(l JobListing) []byte {
	raw := rawJobListingFrontmatter{
		SchemaVersion:      l.SchemaVersion,
		Title:              l.Title,
		Company:            l.Company,
		Location:           l.Location,
		URL:                l.URL,
		Source:             l.Source,
		SavedAt:            l.SavedAt,
		RAL:                l.RAL,
		Logo:               l.Logo,
		FreshnessStatus:    l.FreshnessStatus,
		FreshnessCheckedAt: l.FreshnessCheckedAt,
		Archived:           l.Archived,
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	fmBytes, _ := yaml.Marshal(raw) //nolint:errcheck // raw is a plain struct of strings/slices; yaml.Marshal cannot fail on it
	buf.Write(fmBytes)
	buf.WriteString("---\n\n")
	buf.WriteString(l.JobDescription)
	buf.WriteString("\n")
	return buf.Bytes()
}

func renderApplication(a Application) []byte {
	raw := rawApplication{
		SchemaVersion:   a.SchemaVersion,
		JobListingID:    a.JobListingID,
		Status:          a.Status,
		StatusUpdatedAt: a.StatusUpdatedAt,
		Method:          a.Method,
		Contact:         a.Contact,
		Generations:     a.Generations,
		StatusHistory:   a.StatusHistory,
		Notes:           a.Notes,
	}
	out, _ := yaml.Marshal(raw) //nolint:errcheck // raw is a plain struct of strings/slices; yaml.Marshal cannot fail on it
	return out
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
// existing Job Listing file in fullDir.
func uniqueSlug(fullDir, base string) string {
	if base == "" {
		base = "job"
	}
	slug := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(fullDir, slug+".md")); os.IsNotExist(err) {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}
