package tracking

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gio-del/sumisura/backend/internal/atomicfile"
	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/postingkey"
	"gopkg.in/yaml.v3"
)

// MigrationOptions configures MigrateRecords.
type MigrationOptions struct {
	// DryRun computes and reports every change without writing any file.
	DryRun bool
}

// RecordKind names which of the two record types a migration touched.
type RecordKind string

const (
	RecordJobListing  RecordKind = "job listing"
	RecordApplication RecordKind = "application"
)

// MigrationReport is what MigrateRecords did, or in a dry run would do.
type MigrationReport struct {
	DryRun bool
	// Scanned counts every Job Listing and Application file read.
	Scanned int
	// Migrated names each record that was (or would be) rewritten, in the
	// order it was scanned: Job Listings first, then Applications.
	Migrated []RecordMigration
	// Inconsistencies are problems found alongside the migration that it
	// neither fixes nor halts on, such as a Job Listing with no Application.
	Inconsistencies []Inconsistency
	// DuplicatePostings are Posting Keys held by more than one Job Listing
	// — a corpus written before ADR-0042 made one posting exactly one Job
	// Listing. They are reported, never resolved: picking which Status
	// history, Notes and Generations survive is the user's call, so no
	// -write run ever merges, archives or deletes one (issue #206).
	DuplicatePostings []DuplicatePosting
}

// Pending reports whether the corpus still needs attention: a record not
// at CurrentSchemaVersion, or a posting held by more than one Job Listing.
// The second kind is never resolved by -write, so a completed write run
// can legitimately still report pending work.
func (r MigrationReport) Pending() bool {
	return len(r.Migrated) > 0 || len(r.DuplicatePostings) > 0
}

// DuplicatePosting is one posting held by more than one Job Listing.
type DuplicatePosting struct {
	// PostingKey is the shared identity, in postingkey.Key's text form
	// (e.g. "linkedin:4012345678").
	PostingKey string
	// Listings are the records holding it, oldest saved first, so the
	// first is the original and the rest are what came after.
	Listings []DuplicateListing
}

// DuplicateListing is one of the Job Listings holding a duplicated
// posting: enough to decide which to keep without opening the files.
type DuplicateListing struct {
	// Path is relative to the data directory, slash-separated.
	Path     string
	ID       string
	Title    string
	SavedAt  string
	Archived bool
	// Status is the Application's Status, empty when the Job Listing has
	// no Application file (reported separately as an Inconsistency).
	Status Status
}

// RecordMigration is one record brought to CurrentSchemaVersion.
type RecordMigration struct {
	// Path is relative to the data directory, slash-separated
	// ("jobs/acme.md").
	Path        string
	Kind        RecordKind
	FromVersion int
	ToVersion   int
	// Backfilled are the fields written because their value is provably
	// recoverable from what is already on disk.
	Backfilled []BackfilledField
	// Unknowable are the fields deliberately left empty because nothing on
	// disk records them. They stay empty for good.
	Unknowable []UnknowableField
}

// BackfilledField is a field MigrateRecords filled, and the value it wrote.
type BackfilledField struct {
	Field string
	Value string
}

// UnknowableField is a field MigrateRecords refused to fill, and why.
type UnknowableField struct {
	Field  string
	Reason string
}

// Inconsistency is a problem with a record MigrateRecords reports but does
// not act on.
type Inconsistency struct {
	Path    string
	Problem string
}

// migrationWrite is a planned rewrite of one file.
type migrationWrite struct {
	path    string
	content []byte
}

// MigrateRecords brings every Job Listing and Application under dataDir to
// CurrentSchemaVersion (issue #100, ADR-0034), backfilling only what is
// mechanically recoverable and reporting what is not.
//
// It reads and plans every record before writing any, so an unparseable
// record, or one stamped at a version this build does not know, halts the
// whole run with an error naming the file and leaves every file untouched.
// Records already at CurrentSchemaVersion are never rewritten, so running
// it again is a no-op. Each rewrite goes through a temporary file and a
// rename. A missing or empty dataDir is not an error.
//
// A Job Listing's Markdown body (its Job Description) is kept byte for
// byte, and keys the migration does not know about are kept too: the
// frontmatter or document is edited as a YAML node tree, never round-tripped
// through the typed record.
func MigrateRecords(dataDir string, opts MigrationOptions) (MigrationReport, error) {
	report := MigrationReport{DryRun: opts.DryRun}

	listingSlugs, err := recordSlugs(filepath.Join(dataDir, jobsDir))
	if err != nil {
		return report, err
	}
	applicationSlugs, err := recordSlugs(filepath.Join(dataDir, applicationsDir))
	if err != nil {
		return report, err
	}
	hasApplication := make(map[string]bool, len(applicationSlugs))
	for _, slug := range applicationSlugs {
		hasApplication[slug] = true
	}

	var writes []migrationWrite
	savedAt := make(map[string]string, len(listingSlugs))
	// byPostingKey groups Job Listings by the posting they point at, so a
	// posting held by more than one can be reported (issue #206).
	byPostingKey := make(map[string][]DuplicateListing, len(listingSlugs))
	var postingKeyOrder []string
	for _, slug := range listingSlugs {
		rel := path.Join(jobsDir, slug+".md")
		content, err := os.ReadFile(filepath.Join(dataDir, filepath.FromSlash(rel)))
		if err != nil {
			return report, fmt.Errorf("%s: %w", rel, err)
		}
		report.Scanned++

		migrated, raw, change, err := migrateJobListing(content)
		if err != nil {
			return report, fmt.Errorf("%s: %w", rel, err)
		}
		listingSavedAt := raw.SavedAt
		savedAt[slug] = listingSavedAt
		// A URL that yields no Posting Key has no identity to collide on,
		// exactly as Save never refuses one (ADR-0042).
		if key, ok := postingkey.Of(raw.URL); ok {
			text := key.String()
			if _, seen := byPostingKey[text]; !seen {
				postingKeyOrder = append(postingKeyOrder, text)
			}
			byPostingKey[text] = append(byPostingKey[text], DuplicateListing{
				Path: rel, ID: slug, Title: raw.Title, SavedAt: listingSavedAt, Archived: raw.Archived,
			})
		}
		if !hasApplication[slug] {
			report.Inconsistencies = append(report.Inconsistencies, Inconsistency{
				Path:    rel,
				Problem: fmt.Sprintf("no Application file at %s", path.Join(applicationsDir, slug+".md")),
			})
		}
		if change == nil {
			continue
		}
		change.Path = rel
		report.Migrated = append(report.Migrated, *change)
		writes = append(writes, migrationWrite{path: filepath.Join(dataDir, filepath.FromSlash(rel)), content: migrated})
	}

	applicationStatus := make(map[string]Status, len(applicationSlugs))
	for _, slug := range applicationSlugs {
		rel := path.Join(applicationsDir, slug+".md")
		content, err := os.ReadFile(filepath.Join(dataDir, filepath.FromSlash(rel)))
		if err != nil {
			return report, fmt.Errorf("%s: %w", rel, err)
		}
		report.Scanned++

		listingSavedAt, hasListing := savedAt[slug]
		if !hasListing {
			report.Inconsistencies = append(report.Inconsistencies, Inconsistency{
				Path:    rel,
				Problem: fmt.Sprintf("no Job Listing file at %s", path.Join(jobsDir, slug+".md")),
			})
		}
		migrated, status, change, err := migrateApplication(content, listingSavedAt, hasListing)
		if err != nil {
			return report, fmt.Errorf("%s: %w", rel, err)
		}
		applicationStatus[slug] = status
		if change == nil {
			continue
		}
		change.Path = rel
		report.Migrated = append(report.Migrated, *change)
		writes = append(writes, migrationWrite{path: filepath.Join(dataDir, filepath.FromSlash(rel)), content: migrated})
	}

	report.DuplicatePostings = collectDuplicatePostings(byPostingKey, postingKeyOrder, applicationStatus)

	if opts.DryRun {
		return report, nil
	}
	for _, w := range writes {
		if err := writeFileAtomic(w.path, w.content); err != nil {
			return report, err
		}
	}
	return report, nil
}

// migrateJobListing returns content rewritten at CurrentSchemaVersion and
// what changed, or a nil change when the Job Listing is already current.
// It also returns the Job Listing's savedAt, which the paired Application's
// migration may backfill from.
func migrateJobListing(content []byte) ([]byte, rawJobListingFrontmatter, *RecordMigration, error) {
	fm, closingAndBody, err := locateFrontmatter(content)
	if err != nil {
		return nil, rawJobListingFrontmatter{}, nil, err
	}
	// Parsing the node tree first also rejects a frontmatter that is not a
	// mapping, which the typed decode below would not always notice.
	doc, mapping, err := parseMapping(fm)
	if err != nil {
		return nil, rawJobListingFrontmatter{}, nil, err
	}
	var raw rawJobListingFrontmatter
	if err := yaml.Unmarshal(fm, &raw); err != nil {
		return nil, rawJobListingFrontmatter{}, nil, err
	}
	if err := checkSchemaVersion(raw.SchemaVersion); err != nil {
		return nil, rawJobListingFrontmatter{}, nil, err
	}
	if raw.SchemaVersion == CurrentSchemaVersion {
		return nil, raw, nil, nil
	}

	change := &RecordMigration{Kind: RecordJobListing, FromVersion: raw.SchemaVersion, ToVersion: CurrentSchemaVersion}

	// location has nothing on disk to recover it from: a board's own
	// wording is not in the Job Description reliably enough to re-derive,
	// and a guess written into the record would read afterwards as a fact
	// (ADR-0034). It is named here instead.
	if raw.Location == "" {
		change.Unknowable = append(change.Unknowable, UnknowableField{
			Field:  "location",
			Reason: "added after this record was written; nothing on disk records where the role was",
		})
	}

	// Every reader already treats a missing freshnessStatus as
	// not-yet-checked; writing it down is recovery, not estimation.
	if raw.FreshnessStatus == "" {
		if err := setMappingValue(mapping, "freshnessStatus", FreshnessNotYetChecked); err != nil {
			return nil, rawJobListingFrontmatter{}, nil, err
		}
		change.Backfilled = append(change.Backfilled, BackfilledField{Field: "freshnessStatus", Value: string(FreshnessNotYetChecked)})
	}
	stampSchemaVersion(mapping)

	encoded, err := yaml.Marshal(doc)
	if err != nil {
		return nil, rawJobListingFrontmatter{}, nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(encoded)
	buf.Write(closingAndBody)
	return buf.Bytes(), raw, change, nil
}

// migrateApplication returns content rewritten at CurrentSchemaVersion and
// what changed, or a nil change when the Application is already current.
// listingSavedAt is the paired Job Listing's savedAt (hasListing false when
// there is no Job Listing file).
//
// Only an Application still at Status Saved gets statusUpdatedAt and a
// statusHistory backfilled: no Status moves back to Saved, so it has
// provably never transitioned and its Job Listing's saved date is the date
// its Status was set. Past Saved, the transitions were never written down,
// and inventing them would make time-in-stage (stats.go) and the stale
// nudges (staleness.go) confidently wrong — so those stay empty and are
// reported. Existing Generations are left legacy, with their absent
// Selection/Snippet/usage/language fields reported the same way.
func migrateApplication(content []byte, listingSavedAt string, hasListing bool) ([]byte, Status, *RecordMigration, error) {
	doc, mapping, err := parseMapping(content)
	if err != nil {
		return nil, "", nil, err
	}
	var raw rawApplication
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, "", nil, err
	}
	if err := checkApplicationSchemaVersions(raw); err != nil {
		return nil, raw.Status, nil, err
	}
	if raw.SchemaVersion == CurrentSchemaVersion {
		return nil, raw.Status, nil, nil
	}

	change := &RecordMigration{Kind: RecordApplication, FromVersion: raw.SchemaVersion, ToVersion: CurrentSchemaVersion}
	unknowable := func(field, reason string) {
		change.Unknowable = append(change.Unknowable, UnknowableField{Field: field, Reason: reason})
	}

	missingUpdatedAt := raw.StatusUpdatedAt == ""
	missingHistory := len(raw.StatusHistory) == 0
	if missingUpdatedAt || missingHistory {
		reason := ""
		var savedTime time.Time
		switch {
		case raw.Status != StatusSaved:
			reason = fmt.Sprintf("Status is %s: the transitions since Saved were never recorded, and inventing them would skew time-in-stage and stale nudges", raw.Status)
		case !hasListing:
			reason = "Status is Saved, but there is no paired Job Listing to take the saved date from"
		default:
			parsed, parseErr := time.Parse(time.RFC3339Nano, listingSavedAt)
			if parseErr != nil {
				reason = fmt.Sprintf("Status is Saved, but the Job Listing's savedAt %q is not a valid timestamp", listingSavedAt)
			}
			savedTime = parsed.UTC()
		}

		if missingUpdatedAt {
			if reason != "" {
				unknowable("statusUpdatedAt", reason)
			} else {
				if err := setMappingValueAfter(mapping, "statusUpdatedAt", listingSavedAt, "status"); err != nil {
					return nil, raw.Status, nil, err
				}
				change.Backfilled = append(change.Backfilled, BackfilledField{Field: "statusUpdatedAt", Value: listingSavedAt})
			}
		}
		if missingHistory {
			if reason != "" {
				unknowable("statusHistory", reason)
			} else {
				history := []StatusChange{{Status: StatusSaved, ChangedAt: savedTime}}
				if err := setMappingValue(mapping, "statusHistory", history); err != nil {
					return nil, raw.Status, nil, err
				}
				change.Backfilled = append(change.Backfilled, BackfilledField{Field: "statusHistory", Value: fmt.Sprintf("[%s at %s]", StatusSaved, listingSavedAt)})
			}
		}
	}

	for i, g := range raw.Generations {
		if !g.IsLegacy() {
			continue
		}
		const reason = "recorded before Generations carried this field; nothing on disk records it"
		prefix := fmt.Sprintf("generations[%d].", i)
		if len(g.SourceSnippetIDs) == 0 {
			unknowable(prefix+"sourceSnippetIds", reason)
		}
		if len(g.EntryIDs) == 0 {
			unknowable(prefix+"entryIds", reason)
		}
		if reflect.DeepEqual(g.Usage, generation.GenerationUsage{}) {
			unknowable(prefix+"usage", reason)
		}
		if g.Language == "" {
			unknowable(prefix+"language", reason)
		}
	}

	stampSchemaVersion(mapping)
	encoded, err := yaml.Marshal(doc)
	if err != nil {
		return nil, raw.Status, nil, err
	}
	return encoded, raw.Status, change, nil
}

// collectDuplicatePostings turns the Posting Key grouping into the report,
// keeping only the keys held by more than one Job Listing and attaching
// each one's Application Status. Groups come back in the order their key
// was first seen, and listings within a group oldest saved first (ties
// broken by id), so two runs over an unchanged corpus print the same
// thing and the original is always named before its duplicates.
func collectDuplicatePostings(byKey map[string][]DuplicateListing, order []string, status map[string]Status) []DuplicatePosting {
	var duplicates []DuplicatePosting
	for _, key := range order {
		listings := byKey[key]
		if len(listings) < 2 {
			continue
		}
		withStatus := make([]DuplicateListing, len(listings))
		for i, l := range listings {
			l.Status = status[l.ID]
			withStatus[i] = l
		}
		sort.SliceStable(withStatus, func(i, j int) bool {
			if withStatus[i].SavedAt != withStatus[j].SavedAt {
				return withStatus[i].SavedAt < withStatus[j].SavedAt
			}
			return withStatus[i].ID < withStatus[j].ID
		})
		duplicates = append(duplicates, DuplicatePosting{PostingKey: key, Listings: withStatus})
	}
	return duplicates
}

// recordSlugs lists the record ids under dir: every regular *.md file,
// skipping dotfiles (a temporary file left by an interrupted write) and
// anything else, such as Company Logo images. A missing dir has none.
func recordSlugs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var slugs []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".md") {
			continue
		}
		slugs = append(slugs, strings.TrimSuffix(name, ".md"))
	}
	return slugs, nil
}

// parseMapping parses a YAML document whose root must be a mapping,
// returning the document node (for re-encoding) and the mapping itself.
func parseMapping(content []byte) (*yaml.Node, *yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("record is not a YAML mapping")
	}
	return &doc, doc.Content[0], nil
}

// stampSchemaVersion sets schemaVersion to CurrentSchemaVersion, as the
// first key so the version is the first thing a reader of the file sees.
func stampSchemaVersion(mapping *yaml.Node) {
	removeMappingKey(mapping, "schemaVersion")
	key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "schemaVersion"}
	value := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(CurrentSchemaVersion)}
	mapping.Content = append([]*yaml.Node{key, value}, mapping.Content...)
}

// setMappingValue sets key to value's YAML encoding, replacing an existing
// value in place or appending the key at the end.
func setMappingValue(mapping *yaml.Node, key string, value any) error {
	var encoded yaml.Node
	if err := encoded.Encode(value); err != nil {
		return err
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = &encoded
			return nil
		}
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &encoded)
	return nil
}

// setMappingValueAfter adds key (absent from mapping) directly after the
// key named after, or at the end when after is absent.
func setMappingValueAfter(mapping *yaml.Node, key string, value any, after string) error {
	if err := setMappingValue(mapping, key, value); err != nil {
		return err
	}
	n := len(mapping.Content)
	pair := []*yaml.Node{mapping.Content[n-2], mapping.Content[n-1]}
	for i := 0; i+1 < n-2; i += 2 {
		if mapping.Content[i].Value == after {
			rest := append(pair, mapping.Content[i+2:n-2]...)
			mapping.Content = append(mapping.Content[:i+2], rest...)
			return nil
		}
	}
	return nil
}

func removeMappingKey(mapping *yaml.Node, key string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

// writeFileAtomic replaces path with content via a temporary file in the
// same directory and a rename, so an interrupted write leaves either the
// old file or the new one, never a truncated one. It keeps the existing
// file's permissions. Narrow on purpose: once issue #88's shared atomic
// write helper lands, this should be replaced by it.
func writeFileAtomic(path string, content []byte) error {
	perm := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		perm = info.Mode().Perm()
	}
	return atomicfile.WriteFile(path, content, perm)
}
