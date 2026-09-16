package tracking

import (
	"os"
	"path/filepath"
	"testing"
)

// seedGenerationIndex builds a projectRoot with a data dir and an output
// dir, and returns both paths.
func seedGenerationIndex(t *testing.T) (dataDir, projectRoot string) {
	t.Helper()
	projectRoot = t.TempDir()
	dataDir = filepath.Join(projectRoot, "data")
	for _, dir := range []string{jobsDir, applicationsDir} {
		if err := os.MkdirAll(filepath.Join(dataDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "output"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dataDir, projectRoot
}

// writeOutputDir creates output/<slug>/ with the named files in it.
func writeOutputDir(t *testing.T, projectRoot, slug string, files ...string) {
	t.Helper()
	dir := filepath.Join(projectRoot, "output", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// writeListingPair writes a Job Listing and its 1:1 Application straight to
// disk. Save() would need a Claude client and an HTTP doer for RAL research,
// which this index has nothing to do with.
func writeListingPair(t *testing.T, dataDir, id, company, title string) string {
	t.Helper()
	listing := "---\n" +
		"title: " + title + "\n" +
		"company: " + company + "\n" +
		"source: manual\n" +
		"savedAt: \"2026-01-01T00:00:00Z\"\n" +
		"---\n\nA job description.\n"
	application := "jobListingId: " + id + "\nstatus: saved\nmethod:\n    kind: unresolved\n    value: \"\"\n"
	if err := os.WriteFile(filepath.Join(dataDir, jobsDir, id+".md"), []byte(listing), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, applicationsDir, id+".md"), []byte(application), 0o644); err != nil {
		t.Fatal(err)
	}
	return id
}

// A Default Mode run records nothing — it only writes output/<slug>/. The
// index has to surface it anyway, or the page reports an empty history while
// PDFs sit on disk (issue #173).
func TestListGenerations_IncludesUnrecordedOutputDirectories(t *testing.T) {
	dataDir, projectRoot := seedGenerationIndex(t)
	writeOutputDir(t, projectRoot, "default-20260916-062819", "cv.pdf")

	index, err := ListGenerations(dataDir, projectRoot)
	if err != nil {
		t.Fatalf("ListGenerations: %v", err)
	}
	if len(index) != 1 {
		t.Fatalf("expected 1 generation, got %d: %+v", len(index), index)
	}
	got := index[0]
	if got.Recorded {
		t.Errorf("an output directory with no record must not read as recorded")
	}
	if !got.HasCV || got.HasCoverLetter {
		t.Errorf("expected cv.pdf only, got hasCv=%v hasCoverLetter=%v", got.HasCV, got.HasCoverLetter)
	}
	// The slug's own stamp is the only date it carries; the mtime would say
	// when it was last touched instead.
	if want := "2026-09-16T06:28:19Z"; got.CreatedAt != want {
		t.Errorf("expected createdAt %q decoded from the slug, got %q", want, got.CreatedAt)
	}
}

// A disambiguated slug (-2 when the name was taken) still carries a
// decodable timestamp.
func TestListGenerations_DecodesDisambiguatedSlugTimestamp(t *testing.T) {
	dataDir, projectRoot := seedGenerationIndex(t)
	writeOutputDir(t, projectRoot, "acme-corp-20260911-143022-2", "cv.pdf")

	index, err := ListGenerations(dataDir, projectRoot)
	if err != nil {
		t.Fatalf("ListGenerations: %v", err)
	}
	if want := "2026-09-11T14:30:22Z"; index[0].CreatedAt != want {
		t.Errorf("expected createdAt %q, got %q", want, index[0].CreatedAt)
	}
}

// A directory with no PDF in it is not something the user can open, so it
// is not offered as a row with dead links.
func TestListGenerations_SkipsDirectoriesWithNoPDF(t *testing.T) {
	dataDir, projectRoot := seedGenerationIndex(t)
	writeOutputDir(t, projectRoot, "half-written-20260916-000000", "selection.json")

	index, err := ListGenerations(dataDir, projectRoot)
	if err != nil {
		t.Fatalf("ListGenerations: %v", err)
	}
	if len(index) != 0 {
		t.Errorf("expected no generations, got %+v", index)
	}
}

// A recorded Generation carries its Application's company and job title, and
// is not listed twice when its output directory is also on disk.
func TestListGenerations_RecordedGenerationCarriesItsApplication(t *testing.T) {
	dataDir, projectRoot := seedGenerationIndex(t)
	id := writeListingPair(t, dataDir, "acme-corp", "Acme Corp", "Senior Data Engineer")
	writeOutputDir(t, projectRoot, "acme-corp-20260911-143022", "cv.pdf", "cover-letter.pdf")
	if _, err := RecordGeneration(dataDir, id, GenerationRecord{
		Slug:            "acme-corp-20260911-143022",
		CreatedAt:       "2026-09-11T14:30:22Z",
		CVPath:          "output/acme-corp-20260911-143022/cv.pdf",
		CoverLetterPath: "output/acme-corp-20260911-143022/cover-letter.pdf",
		Language:        "en",
	}); err != nil {
		t.Fatalf("RecordGeneration: %v", err)
	}

	index, err := ListGenerations(dataDir, projectRoot)
	if err != nil {
		t.Fatalf("ListGenerations: %v", err)
	}
	if len(index) != 1 {
		t.Fatalf("expected the recorded Generation exactly once, got %d: %+v", len(index), index)
	}
	got := index[0]
	if !got.Recorded || got.Company != "Acme Corp" || got.JobTitle != "Senior Data Engineer" {
		t.Errorf("expected the Application's company/job title on the row, got %+v", got)
	}
	if got.ApplicationID != id {
		t.Errorf("expected applicationId %q, got %q", id, got.ApplicationID)
	}
	if !got.HasCV || !got.HasCoverLetter {
		t.Errorf("expected both PDFs present, got %+v", got)
	}
}

// Records may outlive their files (see CLAUDE.md on output/): the row stays,
// with the missing files reported rather than the Generation hidden.
func TestListGenerations_RecordedGenerationWithNoFilesLeft(t *testing.T) {
	dataDir, projectRoot := seedGenerationIndex(t)
	id := writeListingPair(t, dataDir, "gone-ltd", "Gone Ltd", "Data Engineer")
	if _, err := RecordGeneration(dataDir, id, GenerationRecord{
		Slug: "gone-ltd-20260101-000000", CreatedAt: "2026-01-01T00:00:00Z",
		CVPath: "output/gone-ltd-20260101-000000/cv.pdf",
	}); err != nil {
		t.Fatalf("RecordGeneration: %v", err)
	}

	index, err := ListGenerations(dataDir, projectRoot)
	if err != nil {
		t.Fatalf("ListGenerations: %v", err)
	}
	if len(index) != 1 {
		t.Fatalf("expected the record to still be listed, got %+v", index)
	}
	if index[0].HasCV {
		t.Errorf("expected hasCv=false for a deleted output directory, got %+v", index[0])
	}
}

// Newest first, whichever half of the index a row came from.
func TestListGenerations_NewestFirstAcrossBothSources(t *testing.T) {
	dataDir, projectRoot := seedGenerationIndex(t)
	id := writeListingPair(t, dataDir, "acme-corp", "Acme Corp", "Data Engineer")
	writeOutputDir(t, projectRoot, "acme-corp-20260601-120000", "cv.pdf")
	if _, err := RecordGeneration(dataDir, id, GenerationRecord{
		Slug: "acme-corp-20260601-120000", CreatedAt: "2026-06-01T12:00:00Z",
	}); err != nil {
		t.Fatalf("RecordGeneration: %v", err)
	}
	writeOutputDir(t, projectRoot, "default-20260916-062819", "cv.pdf")
	writeOutputDir(t, projectRoot, "default-20260101-090000", "cv.pdf")

	index, err := ListGenerations(dataDir, projectRoot)
	if err != nil {
		t.Fatalf("ListGenerations: %v", err)
	}
	want := []string{"default-20260916-062819", "acme-corp-20260601-120000", "default-20260101-090000"}
	if len(index) != len(want) {
		t.Fatalf("expected %d generations, got %d: %+v", len(want), len(index), index)
	}
	for i, slug := range want {
		if index[i].Slug != slug {
			t.Errorf("position %d: expected %q, got %q", i, slug, index[i].Slug)
		}
	}
}

// A fresh install has neither applications nor an output directory.
func TestListGenerations_EmptyInstall(t *testing.T) {
	dir := t.TempDir()
	index, err := ListGenerations(filepath.Join(dir, "data"), dir)
	if err != nil {
		t.Fatalf("ListGenerations: %v", err)
	}
	if len(index) != 0 {
		t.Errorf("expected no generations, got %+v", index)
	}
}
