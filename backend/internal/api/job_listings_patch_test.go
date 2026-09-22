package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

// PATCH /api/job-listings/{id} (issue #206, stories 61-70): the fields a
// user can reasonably be expected to correct become correctable, and the
// two that identity and derivation rest on stay fixed.

func (s lookupServer) patchListing(t *testing.T, id string, body map[string]any) *http.Response {
	t.Helper()
	return patchJSON(t, s.url+"/api/job-listings/"+id, body)
}

func (s lookupServer) savedListing(t *testing.T) string {
	t.Helper()
	resp := postJSON(t, s.url+"/api/job-listings", map[string]any{
		"company": "Acme", "title": "Backend Engineer", "location": "Milan",
		"url": "https://www.linkedin.com/jobs/view/4012345678/", "jobDescription": "Build Go services.",
	})
	defer resp.Body.Close()
	return createdListingID(t, resp)
}

// Stories 61-63: each field individually.
func TestPatchJobListing_CorrectsEachFieldOnItsOwn(t *testing.T) {
	cases := []struct{ field, value string }{
		{"title", "Senior Backend Engineer"},
		{"company", "Acme Rockets"},
		{"location", "Remote (EU)"},
	}
	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			s := newLookupServer(t)
			id := s.savedListing(t)

			resp := s.patchListing(t, id, map[string]any{tc.field: tc.value})
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected the correction to apply, got %d", resp.StatusCode)
			}
			if got := decodeBody(t, resp)["jobListing"].(map[string]any)[tc.field]; got != tc.value {
				t.Errorf("expected %s on the response, got %v", tc.field, got)
			}
			// Story 70: corrected fields show up wherever the listing appears.
			if got := s.jobListing(t, id)[tc.field]; got != tc.value {
				t.Errorf("expected %s persisted, got %v", tc.field, got)
			}
		})
	}
}

func TestPatchJobListing_LeavesEveryFieldItWasNotGiven(t *testing.T) {
	s := newLookupServer(t)
	id := s.savedListing(t)
	before := s.jobListing(t, id)

	resp := s.patchListing(t, id, map[string]any{"title": "Staff Engineer"})
	defer resp.Body.Close()

	after := s.jobListing(t, id)
	for _, field := range []string{"company", "location", "url", "jobDescription", "savedAt", "source", "freshnessStatus"} {
		if after[field] != before[field] {
			t.Errorf("expected %s untouched, went from %v to %v", field, before[field], after[field])
		}
	}
	if after["title"] != "Staff Engineer" {
		t.Errorf("expected the title corrected, got %v", after["title"])
	}
}

// Stories 67, 68: identity and derivation stay fixed.
func TestPatchJobListing_RefusesTheURLAndTheJobDescription(t *testing.T) {
	for _, field := range []string{"url", "jobDescription"} {
		t.Run(field, func(t *testing.T) {
			s := newLookupServer(t)
			id := s.savedListing(t)
			before := s.jobListing(t, id)

			resp := s.patchListing(t, id, map[string]any{field: "something else"})
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected editing %s refused with 400, got %d", field, resp.StatusCode)
			}
			if after := s.jobListing(t, id); after[field] != before[field] {
				t.Errorf("expected %s untouched, got %v", field, after[field])
			}
		})
	}
}

// A Job Listing without a Company is not a Job Listing, so the correction
// cannot empty it.
func TestPatchJobListing_RefusesAnEmptyCompany(t *testing.T) {
	s := newLookupServer(t)
	id := s.savedListing(t)

	resp := s.patchListing(t, id, map[string]any{"company": "   "})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected an empty company refused with 400, got %d", resp.StatusCode)
	}
	if got := s.jobListing(t, id)["company"]; got != "Acme" {
		t.Errorf("expected the company untouched, got %v", got)
	}
}

// A Job Title or Location, unlike a Company, may legitimately be cleared.
func TestPatchJobListing_ClearsATitleOrLocationWhenAskedTo(t *testing.T) {
	s := newLookupServer(t)
	id := s.savedListing(t)

	resp := s.patchListing(t, id, map[string]any{"title": "", "location": ""})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected clearing to be allowed, got %d", resp.StatusCode)
	}
	after := s.jobListing(t, id)
	if after["title"] != nil && after["title"] != "" {
		t.Errorf("expected the title cleared, got %v", after["title"])
	}
	if after["location"] != nil && after["location"] != "" {
		t.Errorf("expected the location cleared, got %v", after["location"])
	}
}

// Stories 64, 65: a figure learned in a conversation is recorded, and
// labelled as the user's own.
func TestPatchJobListing_RALEnteredByHand_IsLabelledManual(t *testing.T) {
	s := newLookupServer(t)
	id := s.savedListing(t)

	resp := s.patchListing(t, id, map[string]any{"ral": map[string]any{"min": 55000, "max": 65000, "currency": "EUR"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected the RAL Range accepted, got %d", resp.StatusCode)
	}

	ral := s.jobListing(t, id)["ral"].(map[string]any)
	if ral["source"] != "manual" {
		t.Errorf("expected source manual, never shown as though Claude estimated it, got %v", ral["source"])
	}
	if ral["min"] != float64(55000) || ral["max"] != float64(65000) || ral["currency"] != "EUR" {
		t.Errorf("expected the figure kept verbatim, got %v", ral)
	}
}

// A client must not be able to claim a source it did not earn.
func TestPatchJobListing_RALSourceIsAlwaysManual_WhateverTheClientSays(t *testing.T) {
	s := newLookupServer(t)
	id := s.savedListing(t)

	resp := s.patchListing(t, id, map[string]any{
		"ral": map[string]any{"min": 55000, "max": 65000, "currency": "EUR", "source": "stated"},
	})
	defer resp.Body.Close()

	if got := s.jobListing(t, id)["ral"].(map[string]any)["source"]; got != "manual" {
		t.Errorf("expected source manual whatever was sent, got %v", got)
	}
}

func TestPatchJobListing_RefusesARALThatIsNotARange(t *testing.T) {
	cases := []struct {
		name string
		ral  map[string]any
	}{
		{"max below min", map[string]any{"min": 65000, "max": 55000}},
		{"a negative figure", map[string]any{"min": -1, "max": 55000}},
		{"no figure at all", map[string]any{"min": 0, "max": 0}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newLookupServer(t)
			id := s.savedListing(t)

			resp := s.patchListing(t, id, map[string]any{"ral": tc.ral})
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected %s refused with 400, got %d", tc.name, resp.StatusCode)
			}
		})
	}
}

// Story 66: a retry can't overwrite the most reliable figure on the record.
func TestPatchJobListing_ManualRAL_SurvivesResolve(t *testing.T) {
	s := newLookupServer(t)
	id := s.savedListing(t)

	entered := s.patchListing(t, id, map[string]any{"ral": map[string]any{"min": 55000, "max": 65000, "currency": "EUR"}})
	entered.Body.Close()

	resolved := postJSON(t, s.url+"/api/job-listings/"+id+"/resolve", map[string]any{})
	defer resolved.Body.Close()
	if resolved.StatusCode != http.StatusOK {
		t.Fatalf("expected resolve to run, got %d", resolved.StatusCode)
	}

	ral := s.jobListing(t, id)["ral"].(map[string]any)
	if ral["source"] != "manual" || ral["min"] != float64(55000) {
		t.Errorf("expected the entered figure left alone by re-resolution, got %v", ral)
	}
}

// Story 69: an edit is refused if someone changed the record while I had
// it open.
func TestPatchJobListing_StaleVersionToken_IsRefused(t *testing.T) {
	s := newLookupServer(t)
	id := s.savedListing(t)
	stale := s.jobListing(t, id)["version"].(string)

	first := s.patchListing(t, id, map[string]any{"title": "Changed by someone else"})
	first.Body.Close()

	req := patchWithVersion(t, s.url+"/api/job-listings/"+id, map[string]any{"title": "Changed by me"}, stale)
	defer req.Body.Close()
	if req.StatusCode != http.StatusConflict {
		t.Fatalf("expected a stale edit refused with 409, got %d", req.StatusCode)
	}
	if got := s.jobListing(t, id)["title"]; got != "Changed by someone else" {
		t.Errorf("expected the other change kept, got %v", got)
	}
}

func TestPatchJobListing_CurrentVersionToken_IsAccepted(t *testing.T) {
	s := newLookupServer(t)
	id := s.savedListing(t)
	current := s.jobListing(t, id)["version"].(string)

	resp := patchWithVersion(t, s.url+"/api/job-listings/"+id, map[string]any{"title": "Staff Engineer"}, current)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected the edit to apply, got %d", resp.StatusCode)
	}
	// The response carries a fresh token, so the page can edit again
	// without reloading.
	if got := decodeBody(t, resp)["jobListing"].(map[string]any)["version"]; got == current || got == "" {
		t.Errorf("expected a fresh version token on the response, got %v", got)
	}
}

func TestPatchJobListing_UnknownListing_Is404(t *testing.T) {
	s := newLookupServer(t)

	resp := s.patchListing(t, "no-such-listing", map[string]any{"title": "x"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// patchWithVersion sends a conditional correction, presenting the version
// token the caller read (issue #89's pattern).
func patchWithVersion(t *testing.T, url string, payload any, version string) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
