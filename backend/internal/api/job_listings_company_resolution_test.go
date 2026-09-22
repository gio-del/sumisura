package api_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// application reads one Application, so a test can check that a replaced
// record kept its Status and history.
func (s lookupServer) application(t *testing.T, id string) map[string]any {
	t.Helper()
	resp, err := http.Get(s.url + "/api/job-listings/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reading %s: got %d", id, resp.StatusCode)
	}
	return decodeBody(t, resp)["application"].(map[string]any)
}

// The same-company decision on the extension capture route (issue #206,
// stories 13-25): before a save at a company already being chased, the
// backend raises the siblings itself, and the client answers with a
// resolution. The warning never blocks — it asks.

const extensionPath = "/api/job-listings/from-extension"

// capture posts an extension capture, optionally carrying a resolution.
func (s lookupServer) capture(t *testing.T, company, title, url string, resolution map[string]any) *http.Response {
	t.Helper()
	payload := map[string]any{
		"company": company, "title": title, "url": url,
		"description": title + " at " + company + ".",
	}
	if resolution != nil {
		payload["resolution"] = resolution
	}
	return postJSON(t, s.url+extensionPath, payload)
}

func assertCompanyGate(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected the company decision raised with 409, got %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["reason"] != "company-has-listings" {
		t.Fatalf("expected reason company-has-listings, got %v", body)
	}
	return body
}

func (s lookupServer) listingIDs(t *testing.T, query string) []string {
	t.Helper()
	resp, err := http.Get(s.url + "/api/job-listings" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var rows []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row["jobListing"].(map[string]any)["id"].(string)
	}
	return ids
}

// Story 24: the warning can't be skipped by a client that didn't look first.
func TestCapture_CompanyHasListings_NoResolution_AsksAndWritesNothing(t *testing.T) {
	s := newLookupServer(t)
	sibling := s.save(t, "Acme", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")
	s.moveTo(t, sibling, "tailoring")

	resp := s.capture(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/", nil)
	defer resp.Body.Close()
	body := assertCompanyGate(t, resp)

	// Story 14: the roles, with what I need to decide.
	listings := objectsOf(t, body["company"].(map[string]any)["listings"])
	if len(listings) != 1 {
		t.Fatalf("expected the sibling listed, got %v", listings)
	}
	if listings[0]["id"] != sibling || listings[0]["title"] != "Platform Engineer" || listings[0]["status"] != "tailoring" {
		t.Errorf("expected the sibling's id, title and status, got %v", listings[0])
	}
	if listings[0]["savedAt"] == nil || listings[0]["savedAt"] == "" {
		t.Errorf("expected the sibling's saved date, got %v", listings[0]["savedAt"])
	}

	if got := s.listingIDs(t, "?archived=all"); len(got) != 1 {
		t.Errorf("expected nothing written while the decision is pending, got %v", got)
	}
}

func TestCapture_CompanyWithNoListings_SavesWithoutAsking(t *testing.T) {
	s := newLookupServer(t)
	s.save(t, "Globex", "Frontend Engineer", "https://www.linkedin.com/jobs/view/4099999999/")

	resp := s.capture(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected a first role at a new company to save straight away, got %d", resp.StatusCode)
	}
}

// Story 23: a company whose listings are all archived never interrupts.
func TestCapture_CompanyWhollyArchived_SavesWithoutAsking(t *testing.T) {
	s := newLookupServer(t)
	archived := s.save(t, "Acme", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")
	s.archive(t, archived)

	resp := s.capture(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected a wholly archived company not to interrupt, got %d", resp.StatusCode)
	}
}

// Stories 15, 16: pursuing several roles at one company is normal.
func TestCapture_SaveAnyway_AddsTheRoleAlongsideTheOthers(t *testing.T) {
	s := newLookupServer(t)
	sibling := s.save(t, "Acme", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")

	resp := s.capture(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/",
		map[string]any{"kind": "save-anyway"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected save-anyway to save, got %d", resp.StatusCode)
	}

	ids := s.listingIDs(t, "")
	if len(ids) != 2 {
		t.Errorf("expected both roles active, got %v", ids)
	}
	for _, id := range ids {
		if id == sibling {
			return
		}
	}
	t.Errorf("expected the existing role untouched, got %v", ids)
}

// The Posting Key gate is not skippable: save-anyway answers the company
// question only.
func TestCapture_SaveAnyway_StillRefusesADuplicatePosting(t *testing.T) {
	s := newLookupServer(t)
	existing := s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")

	resp := s.capture(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/search/?currentJobId=4012345678",
		map[string]any{"kind": "save-anyway"})
	defer resp.Body.Close()
	if got := refusedListing(t, resp); got["id"] != existing {
		t.Errorf("expected the Posting Key gate to still refuse, naming %q, got %v", existing, got["id"])
	}
}

// Stories 17, 18: replacing takes one step and destroys nothing.
func TestCapture_Replace_SavesTheNewRoleAndArchivesTheOld(t *testing.T) {
	s := newLookupServer(t)
	old := s.save(t, "Acme", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")
	s.moveTo(t, old, "tailoring")

	resp := s.capture(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/",
		map[string]any{"kind": "replace", "jobListingId": old})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected replace to save the new role, got %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["archivedJobListingId"] != old {
		t.Errorf("expected the response to name what it archived, got %v", body["archivedJobListingId"])
	}
	if body["archiveFailed"] != nil {
		t.Errorf("expected no archive failure reported, got %v", body["archiveFailed"])
	}

	if active := s.listingIDs(t, ""); len(active) != 1 || active[0] == old {
		t.Errorf("expected only the new role active, got %v", active)
	}
	// Story 18: archived, not deleted — its Application and history survive.
	if all := s.listingIDs(t, "?archived=all"); len(all) != 2 {
		t.Errorf("expected the replaced role kept on disk, got %v", all)
	}
	replaced := s.application(t, old)
	if replaced["status"] != "tailoring" {
		t.Errorf("expected the replaced Application's Status intact, got %v", replaced["status"])
	}
	if history, _ := replaced["statusHistory"].([]any); len(history) != 2 {
		t.Errorf("expected the replaced Application's Status history intact, got %v", history)
	}
}

// Story 19: a stale choice fails cleanly rather than half-applying.
func TestCapture_Replace_BadTarget_IsRefusedWithNothingWritten(t *testing.T) {
	s := newLookupServer(t)
	sibling := s.save(t, "Acme", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")
	otherCompany := s.save(t, "Globex", "Frontend Engineer", "https://www.linkedin.com/jobs/view/4088888888/")
	archived := s.save(t, "Acme", "Staff Engineer", "https://www.linkedin.com/jobs/view/4077777777/")
	s.archive(t, archived)

	cases := []struct{ name, target string }{
		{"a listing that no longer exists", "deleted-long-ago"},
		{"a listing at another company", otherCompany},
		{"a listing already archived", archived},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := s.listingIDs(t, "?archived=all")

			resp := s.capture(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/",
				map[string]any{"kind": "replace", "jobListingId": tc.target})
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("expected a stale replace target refused with 409, got %d", resp.StatusCode)
			}

			if after := s.listingIDs(t, "?archived=all"); len(after) != len(before) {
				t.Errorf("expected nothing written, went from %v to %v", before, after)
			}
			if active := s.listingIDs(t, ""); len(active) != 2 {
				t.Errorf("expected nothing archived, got active %v", active)
			}
		})
	}
	_ = sibling
}

// Story 8, on the wire: the case where I did mean to pick that listing
// back up takes one click, and saves nothing new.
func TestCapture_UnarchiveExisting_BringsItBackAndSavesNothing(t *testing.T) {
	s := newLookupServer(t)
	existing := s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")
	s.archive(t, existing)

	resp := s.capture(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/",
		map[string]any{"kind": "unarchive-existing", "jobListingId": existing})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected unarchive-existing to answer 200, got %d", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	listing := body["jobListing"].(map[string]any)
	if listing["id"] != existing || listing["archived"] != false {
		t.Errorf("expected the unarchived listing back, got %v", listing)
	}

	if all := s.listingIDs(t, "?archived=all"); len(all) != 1 {
		t.Errorf("expected nothing new saved, got %v", all)
	}
	if active := s.listingIDs(t, ""); len(active) != 1 || active[0] != existing {
		t.Errorf("expected the listing active again, got %v", active)
	}
}

func TestCapture_UnknownResolutionKind_IsRefused(t *testing.T) {
	s := newLookupServer(t)

	resp := s.capture(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/",
		map[string]any{"kind": "merge"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected an unknown resolution refused with 400, got %d", resp.StatusCode)
	}
}

// The company gate is the extension handler's alone: the manual and ATS
// save paths keep today's post-save fuzzy warning (a deliberate, temporary
// asymmetry the issue records).
func TestCreateJobListing_CompanyHasListings_SavesWithoutAsking(t *testing.T) {
	s := newLookupServer(t)
	s.save(t, "Acme", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")

	resp := postJSON(t, s.url+"/api/job-listings", map[string]any{
		"company": "Acme", "title": "Backend Engineer",
		"url": "https://www.linkedin.com/jobs/view/4012345678/", "jobDescription": "A backend role.",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected the manual save path unchanged, got %d", resp.StatusCode)
	}
}
