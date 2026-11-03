package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestPatchJobListing_PriorityRoundTripsAndRejectsOthers(t *testing.T) {
	s := newLookupServer(t)
	id := s.savedListing(t)

	resp := s.patchListing(t, id, map[string]any{"priority": "high"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := s.jobListing(t, id)["priority"]; got != "high" {
		t.Errorf("expected high persisted, got %v", got)
	}

	resp = s.patchListing(t, id, map[string]any{"priority": "urgent"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for an unknown priority, got %d", resp.StatusCode)
	}

	resp = s.patchListing(t, id, map[string]any{"priority": ""})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 clearing, got %d", resp.StatusCode)
	}
	if got, present := s.jobListing(t, id)["priority"]; present && got != "" {
		t.Errorf("expected priority cleared, got %v", got)
	}
}

func TestPatchJobListing_PriorityHonoursIfMatch(t *testing.T) {
	s := newLookupServer(t)
	id := s.savedListing(t)

	resp := doJSON(t, http.MethodPatch, s.url+"/api/job-listings/"+id, map[string]any{"priority": "low"}, "stale-token")
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 on a stale token, got %d", resp.StatusCode)
	}
}

func TestListJobListings_FiltersAndSortsByPriority(t *testing.T) {
	s := newLookupServer(t)
	a := s.savedListing(t)
	resp := postJSON(t, s.url+"/api/job-listings", map[string]any{
		"company": "Beta", "title": "Dev", "url": "https://www.linkedin.com/jobs/view/4099999999/", "jobDescription": "Other.",
	})
	b := createdListingID(t, resp)
	resp.Body.Close()
	r := s.patchListing(t, b, map[string]any{"priority": "high"})
	r.Body.Close()
	r = s.patchListing(t, a, map[string]any{"priority": "low"})
	r.Body.Close()

	idsAt := func(query string) []string {
		resp, err := http.Get(s.url + "/api/job-listings" + query)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for %q, got %d", query, resp.StatusCode)
		}
		var rows []struct {
			JobListing struct {
				ID       string `json:"id"`
				Priority string `json:"priority"`
			} `json:"jobListing"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, r := range rows {
			out = append(out, r.JobListing.ID)
		}
		return out
	}

	sorted := idsAt("?sort=priority")
	if len(sorted) < 2 || sorted[0] != b || sorted[1] != a {
		t.Errorf("expected high (%s) then low (%s) first, got %v", b, a, sorted)
	}
	if only := idsAt("?priority=low"); len(only) != 1 || only[0] != a {
		t.Errorf("expected only %s, got %v", a, only)
	}
	bad, err := http.Get(s.url + "/api/job-listings?priority=urgent")
	if err != nil {
		t.Fatal(err)
	}
	bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", bad.StatusCode)
	}
}
