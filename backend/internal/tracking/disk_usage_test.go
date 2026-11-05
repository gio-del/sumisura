package tracking

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeApp writes a listing plus an Application with the given status and
// Generations (slug -> createdAt), and the output dirs for them.
func writeAppWithGenerations(t *testing.T, dataDir, projectRoot, id, status, statusUpdatedAt string, gens [][2]string) {
	t.Helper()
	writeListingPair(t, dataDir, id, id, "Engineer")
	var b strings.Builder
	b.WriteString("jobListingId: " + id + "\nstatus: " + status + "\n")
	if statusUpdatedAt != "" {
		b.WriteString("statusUpdatedAt: \"" + statusUpdatedAt + "\"\n")
	}
	b.WriteString("method:\n    kind: unresolved\n    value: \"\"\ngenerations:\n")
	for _, g := range gens {
		b.WriteString("    - slug: " + g[0] + "\n      createdAt: \"" + g[1] + "\"\n      cvPath: output/" + g[0] + "/cv.pdf\n")
		writeOutputDir(t, projectRoot, g[0], "cv.pdf")
	}
	if err := os.WriteFile(filepath.Join(dataDir, applicationsDir, id+".md"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGetDiskUsage_Superseded(t *testing.T) {
	dataDir, projectRoot := seedGenerationIndex(t)
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)

	writeAppWithGenerations(t, dataDir, projectRoot, "open", "applied", "2026-01-01T00:00:00Z", [][2]string{
		{"open-old", "2026-09-01T10:00:00Z"},
		{"open-new", "2026-09-02T10:00:00Z"},
	})
	writeAppWithGenerations(t, dataDir, projectRoot, "recent", "rejected", "2026-10-20T00:00:00Z", [][2]string{
		{"recent-gen", "2026-10-01T10:00:00Z"},
	})
	writeAppWithGenerations(t, dataDir, projectRoot, "old", "withdrawn", "2026-06-01T00:00:00Z", [][2]string{
		{"old-gen", "2026-05-01T10:00:00Z"},
	})
	writeOutputDir(t, projectRoot, "default-20260916-062819", "cv.pdf")

	usage, err := GetDiskUsage(dataDir, projectRoot, 90, now)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"open-old": true, "open-new": false, "recent-gen": false, "old-gen": true,
		"default-20260916-062819": false,
	}
	if len(usage.Generations) != len(want) {
		t.Fatalf("got %d rows: %+v", len(usage.Generations), usage.Generations)
	}
	for _, g := range usage.Generations {
		if g.Superseded != want[g.Slug] {
			t.Errorf("%s: superseded=%v, want %v", g.Slug, g.Superseded, want[g.Slug])
		}
		if g.Bytes != 1 {
			t.Errorf("%s: bytes=%d, want 1", g.Slug, g.Bytes)
		}
	}
	if usage.TotalBytes != 5 {
		t.Errorf("totalBytes=%d, want 5", usage.TotalBytes)
	}
}
