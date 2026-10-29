package api_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
)

// seedGapsDataDir tracks three Job Listings (one archived) against one Entry
// tagged Go: Terraform recurs uncovered, Go recurs covered, and the archived
// listing's Pulumi must not count.
func seedGapsDataDir(t *testing.T) string {
	t.Helper()
	dir := seedTagLintDataDir(t)
	listing := func(id, archived, description string) {
		writeFile(t, filepath.Join(dir, "jobs", id+".md"),
			"---\nschemaVersion: 1\ncompany: "+id+"\nsource: manual\nsavedAt: \"2025-01-01T00:00:00Z\"\nral:\n  source: n/a\nfreshnessStatus: not-yet-checked\narchived: "+archived+"\n---\n\n"+description+"\n")
		writeFile(t, filepath.Join(dir, "applications", id+".md"),
			"schemaVersion: 1\njobListingId: "+id+"\nstatus: saved\nmethod:\n  kind: other\n")
	}
	listing("alpha", "false", "Terraform and Go. Pulumi nice to have.")
	listing("beta", "false", "TERRAFORM, go.")
	listing("gamma", "true", "Pulumi and Terraform.")
	return dir
}

func TestGapsRoute_ReportsUncoveredFirstAndSkipsArchived(t *testing.T) {
	server := newSimpleServer(t, seedGapsDataDir(t))
	body := call(t, http.MethodGet, server.URL+"/api/master-data/gaps", nil, http.StatusOK)

	var report struct {
		Gaps []struct {
			Term       string   `json:"term"`
			Count      int      `json:"count"`
			ListingIDs []string `json:"listingIds"`
			Covered    bool     `json:"covered"`
		} `json:"gaps"`
	}
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Gaps) != 2 {
		t.Fatalf("gaps = %+v, want terraform and go only", report.Gaps)
	}
	if g := report.Gaps[0]; g.Term != "terraform" || g.Covered || g.Count != 2 || len(g.ListingIDs) != 2 {
		t.Errorf("first = %+v, want uncovered terraform in 2 listings", g)
	}
	if g := report.Gaps[1]; g.Term != "go" || !g.Covered {
		t.Errorf("second = %+v, want covered go", g)
	}
}

func TestGapsRoute_EmptyWithoutListingsOrMasterData(t *testing.T) {
	server := newSimpleServer(t, t.TempDir())
	body := call(t, http.MethodGet, server.URL+"/api/master-data/gaps", nil, http.StatusOK)
	if string(body) != "{\"gaps\":[]}\n" && string(body) != "{\"gaps\":[]}" {
		t.Errorf("body = %q, want an empty gaps list", body)
	}
}
