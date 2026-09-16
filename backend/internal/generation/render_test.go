package generation

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// requireBinary skips t unless name is on PATH — for the smoke test below,
// which needs the real typst and pdftotext binaries rather than fakes
// (PRD's Testing Decisions: "skipped if either binary isn't on PATH,
// matching how other tests here would need to handle optional local
// tooling").
func requireBinary(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not on PATH, skipping", name)
	}
}

// TestParsabilityCheck_RealTypstAndPdftotextPipeline_ReportsOK is the PRD's
// thin integration smoke test: it renders a fixture cvData through the
// real typst compile -> pdftotext pipeline once, confirming the two tools
// are wired together correctly end to end. It is deliberately not where
// the comparison logic itself is exercised — see TestCheckParsability in
// parsability_test.go for that, against fixture strings instead of a real
// PDF.
func TestParsabilityCheck_RealTypstAndPdftotextPipeline_ReportsOK(t *testing.T) {
	requireBinary(t, "typst")
	requireBinary(t, "pdftotext")

	projectRoot := t.TempDir()
	copyTemplateFixture(t, projectRoot, "cv.typ")
	if err := os.MkdirAll(filepath.Join(projectRoot, "output", "smoke-test"), 0o755); err != nil {
		t.Fatal(err)
	}

	cv := cvData{
		Name:      "Jane Doe",
		Location:  "Milan, Italy",
		Email:     "jane@example.com",
		Phone:     "+39 000 000 000",
		LinkedIn:  "janedoe",
		GitHub:    "janedoe",
		Education: []masterdata.Education{},
		Experience: []cvExperience{
			{
				Employer: "Acme Corp",
				Role:     "Senior Engineer",
				Start:    "2022",
				End:      nil,
				Bullets:  []string{"Shipped things."},
			},
		},
		Projects:     []cvProject{},
		TechStack:    []string{},
		Publications: []masterdata.Publication{},
		Awards:       []masterdata.Award{},
		Activities:   []masterdata.Activity{},
		Languages:    []masterdata.Language{},
	}

	pdfRelPath, err := renderTypst(projectRoot, "template/cv.typ", "smoke-test", "data.json", "cv.pdf", cv)
	if err != nil {
		t.Fatalf("renderTypst: %v", err)
	}

	result := checkPDFParsability(filepath.Join(projectRoot, pdfRelPath), cvExpectedFields(cv))
	if result.Status != ParsabilityOK {
		t.Errorf("expected ParsabilityOK, got %+v", result)
	}
}

// copyTemplateFixture copies the repo's real template/<name> into
// projectRoot/template/<name>, mirroring internal/api's copyTemplate
// helper — Render's smoke test needs the real template, not a
// reimplementation, so a change to cv.typ's structure is caught here too.
func copyTemplateFixture(t *testing.T, projectRoot, name string) {
	t.Helper()
	src := filepath.Join("..", "..", "..", "template", name)
	content, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("reading real template %s: %v", src, err)
	}
	dst := filepath.Join(projectRoot, "template", name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedRenderProject builds a temp project root holding a minimal Master
// Data profile (no Entries) plus the real template/*.typ files, so Render
// can be driven end to end against the real typst binary without touching
// the repo's own output/ directory.
func seedRenderProject(t *testing.T) (projectRoot, dataDir string) {
	t.Helper()
	projectRoot = t.TempDir()
	dataDir = filepath.Join(projectRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := "name: Jane Doe\nlocation: Milan, Italy\nemail: jane@example.com\nphone: \"+39 000 000 000\"\nlinkedin: janedoe\ngithub: janedoe\n" +
		"education: []\npublications: []\nawards: []\nactivities: []\nlanguages: []\n"
	if err := os.WriteFile(filepath.Join(dataDir, "profile.yaml"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	copyTemplateFixture(t, projectRoot, "cv.typ")
	copyTemplateFixture(t, projectRoot, "cover-letter.typ")
	return projectRoot, dataDir
}

// fixClock pins Render's clock to at for the rest of t.
func fixClock(t *testing.T, at time.Time) {
	t.Helper()
	prev := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = prev })
}

func renderLabel(t *testing.T, projectRoot, dataDir, label string) RenderResult {
	t.Helper()
	result, err := Render(projectRoot, dataDir, RenderRequest{Slug: label})
	if err != nil {
		t.Fatalf("Render(%q): %v", label, err)
	}
	return result
}

func TestRender_DerivesDirectoryFromLabelPlusUTCTimestamp(t *testing.T) {
	requireBinary(t, "typst")
	projectRoot, dataDir := seedRenderProject(t)
	// A non-UTC clock, to prove the timestamp is rendered in UTC.
	fixClock(t, time.Date(2026, 9, 11, 16, 30, 22, 0, time.FixedZone("CEST", 2*60*60)))

	result := renderLabel(t, projectRoot, dataDir, "acme-corp")

	const want = "acme-corp-20260911-143022"
	if result.Slug != want {
		t.Errorf("expected returned slug %q, got %q", want, result.Slug)
	}
	if result.CVPath != filepath.Join("output", want, "cv.pdf") {
		t.Errorf("expected CVPath under output/%s/, got %q", want, result.CVPath)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "output", want, "cv.pdf")); err != nil {
		t.Errorf("expected cv.pdf in output/%s/: %v", want, err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "output", "acme-corp")); !os.IsNotExist(err) {
		t.Errorf("expected no bare output/acme-corp/ directory, got err=%v", err)
	}
}

func TestRender_ReturnedSlugSatisfiesServingEndpointPattern(t *testing.T) {
	requireBinary(t, "typst")
	projectRoot, dataDir := seedRenderProject(t)
	fixClock(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

	// The file-serving endpoint validates slugs against this same
	// kebab-case pattern; a derived slug that failed it would be
	// unservable.
	kebab := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	for i := 0; i < 2; i++ { // the second render also exercises a disambiguated slug
		result := renderLabel(t, projectRoot, dataDir, "default")
		if !kebab.MatchString(result.Slug) {
			t.Errorf("returned slug %q is not kebab-case", result.Slug)
		}
	}
}

// readCV returns the bytes of output/<slug>/cv.pdf, failing t if absent.
func readCV(t *testing.T, projectRoot, slug string) []byte {
	t.Helper()
	pdf, err := os.ReadFile(filepath.Join(projectRoot, "output", slug, "cv.pdf"))
	if err != nil {
		t.Fatalf("expected output/%s/cv.pdf to exist: %v", slug, err)
	}
	return pdf
}

// TestRender_SameLabelTwice_KeepsBothGenerations is the regression test
// for issue #105: a later Generation with the same label used to
// overwrite the earlier one's files while the earlier record still
// pointed there.
func TestRender_SameLabelTwice_KeepsBothGenerations(t *testing.T) {
	requireBinary(t, "typst")
	projectRoot, dataDir := seedRenderProject(t)

	fixClock(t, time.Date(2026, 9, 11, 14, 30, 22, 0, time.UTC))
	first := renderLabel(t, projectRoot, dataDir, "acme-corp")
	firstPDF := readCV(t, projectRoot, first.Slug)

	fixClock(t, time.Date(2026, 9, 11, 15, 11, 40, 0, time.UTC))
	second := renderLabel(t, projectRoot, dataDir, "acme-corp")

	if first.Slug != "acme-corp-20260911-143022" || second.Slug != "acme-corp-20260911-151140" {
		t.Fatalf("expected slugs acme-corp-20260911-143022 and acme-corp-20260911-151140, got %q and %q", first.Slug, second.Slug)
	}
	readCV(t, projectRoot, second.Slug)
	if got := readCV(t, projectRoot, first.Slug); !bytes.Equal(got, firstPDF) {
		t.Error("expected the first Generation's cv.pdf to be left untouched by the second")
	}
}

func TestRender_SameLabelSameSecond_DisambiguatesInsteadOfOverwriting(t *testing.T) {
	requireBinary(t, "typst")
	projectRoot, dataDir := seedRenderProject(t)
	fixClock(t, time.Date(2026, 9, 11, 14, 30, 22, 0, time.UTC))

	first := renderLabel(t, projectRoot, dataDir, "acme-corp")
	firstPDF := readCV(t, projectRoot, first.Slug)
	second := renderLabel(t, projectRoot, dataDir, "acme-corp")
	third := renderLabel(t, projectRoot, dataDir, "acme-corp")

	want := []string{"acme-corp-20260911-143022", "acme-corp-20260911-143022-2", "acme-corp-20260911-143022-3"}
	got := []string{first.Slug, second.Slug, third.Slug}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("render %d: expected slug %q, got %q", i+1, want[i], got[i])
		}
		readCV(t, projectRoot, got[i])
	}
	if !bytes.Equal(readCV(t, projectRoot, first.Slug), firstPDF) {
		t.Error("expected the first Generation's cv.pdf to be left untouched")
	}
}

func TestRender_ComputedDirectoryAlreadyExists_DoesNotWriteIntoIt(t *testing.T) {
	requireBinary(t, "typst")
	projectRoot, dataDir := seedRenderProject(t)
	fixClock(t, time.Date(2026, 9, 11, 14, 30, 22, 0, time.UTC))

	// A directory left behind by some earlier run, e.g. the skill's.
	existing := filepath.Join(projectRoot, "output", "acme-corp-20260911-143022")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(existing, "cv.pdf")
	if err := os.WriteFile(sentinel, []byte("an earlier Generation's CV"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := renderLabel(t, projectRoot, dataDir, "acme-corp")

	if result.Slug != "acme-corp-20260911-143022-2" {
		t.Errorf("expected a disambiguated slug, got %q", result.Slug)
	}
	if got, _ := os.ReadFile(sentinel); string(got) != "an earlier Generation's CV" {
		t.Errorf("expected the pre-existing cv.pdf to be untouched, got %d bytes", len(got))
	}
	entries, err := os.ReadDir(existing)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected nothing new written into the pre-existing directory, found %d files", len(entries))
	}
}

func TestRender_WithCoverLetter_WritesEverythingIntoOneDirectory(t *testing.T) {
	requireBinary(t, "typst")
	projectRoot, dataDir := seedRenderProject(t)
	fixClock(t, time.Date(2026, 9, 11, 14, 30, 22, 0, time.UTC))

	result, err := Render(projectRoot, dataDir, RenderRequest{
		Slug:        "acme-corp",
		CoverLetter: &CoverLetterResult{Body: "Dear Hiring Manager,\n\nI'm excited to apply."},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	dir := filepath.Join("output", result.Slug)
	if result.CVPath != filepath.Join(dir, "cv.pdf") {
		t.Errorf("expected CVPath in %s, got %q", dir, result.CVPath)
	}
	if result.CoverLetterPath != filepath.Join(dir, "cover-letter.pdf") {
		t.Errorf("expected CoverLetterPath in %s, got %q", dir, result.CoverLetterPath)
	}
	for _, name := range []string{"cv.pdf", "cover-letter.pdf", "cover-letter.txt"} {
		if _, err := os.Stat(filepath.Join(projectRoot, dir, name)); err != nil {
			t.Errorf("expected %s in %s: %v", name, dir, err)
		}
	}
	outputs, err := os.ReadDir(filepath.Join(projectRoot, "output"))
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 1 {
		t.Errorf("expected exactly one Generation directory under output/, found %d", len(outputs))
	}
}

// A profile.yaml that simply omits a Static Section — no `awards:` key at
// all, as opposed to `awards: []` — must still render. The Go zero value is
// a nil slice, which marshals to JSON null, and template/cv.typ guards each
// section with `.len() > 0`, so an unguarded null used to fail the whole
// render with "type none has no method `len`" rather than just omitting the
// section (found while adding Certifications, issue #167).
func TestRender_ProfileWithNoStaticSections(t *testing.T) {
	requireBinary(t, "typst")
	projectRoot := t.TempDir()
	dataDir := filepath.Join(projectRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := "name: Jane Doe\nlocation: Milan, Italy\nemail: jane@example.com\nphone: \"+39 000 000 000\"\nlinkedin: janedoe\ngithub: janedoe\n"
	if err := os.WriteFile(filepath.Join(dataDir, "profile.yaml"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	copyTemplateFixture(t, projectRoot, "cv.typ")
	copyTemplateFixture(t, projectRoot, "cover-letter.typ")

	renderLabel(t, projectRoot, dataDir, "sparse-profile")
}

// Certifications reach the rendered PDF's data file like any other Static
// Section (issue #167).
func TestRender_CarriesCertificationsThrough(t *testing.T) {
	requireBinary(t, "typst")
	projectRoot, dataDir := seedRenderProject(t)
	profile := "name: Jane Doe\nlocation: Milan, Italy\nemail: jane@example.com\nphone: \"+39 000 000 000\"\nlinkedin: janedoe\ngithub: janedoe\n" +
		"certifications:\n  - title: SnowPro Core\n    issuer: Snowflake\n    date: \"2025-10\"\n"
	if err := os.WriteFile(filepath.Join(dataDir, "profile.yaml"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}

	result := renderLabel(t, projectRoot, dataDir, "certs")

	data, err := os.ReadFile(filepath.Join(projectRoot, "output", result.Slug, "data.json"))
	if err != nil {
		t.Fatalf("reading data.json: %v", err)
	}
	if !strings.Contains(string(data), "SnowPro Core") {
		t.Errorf("expected the certification in data.json, got:\n%s", data)
	}
}

// The Tech Stack line is derived, and "every tag of every selected Entry"
// does not survive a real career: it produced 67 tags over six lines and
// pushed the render onto a second page (issue #169).
func TestDeriveTechStack_CapsTheLine(t *testing.T) {
	var many []string
	for i := 0; i < 50; i++ {
		many = append(many, fmt.Sprintf("tag-%02d", i))
	}

	got := deriveTechStack([][]string{many})

	if len(got) != TechStackMaxTags {
		t.Fatalf("expected the line capped at %d, got %d", TechStackMaxTags, len(got))
	}
	if got[0] != "tag-00" {
		t.Errorf("expected the Entry's own order kept, got %v", got[:3])
	}
}

// Round-robin, so a long-tagged first Entry cannot spend the whole budget
// and hide every later Entry's stack.
func TestDeriveTechStack_EveryEntryContributesBeforeAnyOneFillsTheLine(t *testing.T) {
	var first []string
	for i := 0; i < 40; i++ {
		first = append(first, fmt.Sprintf("first-%02d", i))
	}
	second := []string{"Snowflake", "Kafka"}

	got := deriveTechStack([][]string{first, second})

	if !slices.Contains(got, "Snowflake") || !slices.Contains(got, "Kafka") {
		t.Errorf("expected the second Entry's tags on the line, got %v", got)
	}
	// One from each, alternating, while both still have tags to give.
	if want := []string{"first-00", "Snowflake", "first-01", "Kafka", "first-02"}; !slices.Equal(got[:5], want) {
		t.Errorf("expected round-robin %v, got %v", want, got[:5])
	}
}

func TestDeriveTechStack_DeduplicatesAcrossEntries(t *testing.T) {
	got := deriveTechStack([][]string{{"Python", "AWS"}, {"Python", "Snowflake"}})

	if want := []string{"Python", "AWS", "Snowflake"}; !slices.Equal(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

// Under the cap, nothing is dropped or reordered beyond the round-robin.
func TestDeriveTechStack_ShortSelectionKeepsEverything(t *testing.T) {
	got := deriveTechStack([][]string{{"Go", "Typst"}, {"React"}})

	if want := []string{"Go", "React", "Typst"}; !slices.Equal(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}
