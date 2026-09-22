package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

// POST /api/job-listings/capture-lookup (issue #206): what the extension
// card asks before the user clicks Save — is this posting tracked, and does
// this company have other roles I am already chasing. Read-only.

const lookupPath = "/api/job-listings/capture-lookup"

type lookupServer struct {
	url string
}

func newLookupServer(t *testing.T) lookupServer {
	t.Helper()
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	t.Cleanup(server.Close)
	return lookupServer{url: server.URL}
}

// save puts a posting in the corpus the way the extension does. It always
// answers the same-company question with save-anyway, since it is setup:
// the decision itself is what the resolution tests drive explicitly.
func (s lookupServer) save(t *testing.T, company, title, url string) string {
	t.Helper()
	resp := postJSON(t, s.url+"/api/job-listings/from-extension", map[string]any{
		"company": company, "title": title, "url": url,
		"description": title + " at " + company + ".",
		"resolution":  map[string]any{"kind": "save-anyway"},
	})
	defer resp.Body.Close()
	return createdListingID(t, resp)
}

func (s lookupServer) archive(t *testing.T, id string) {
	t.Helper()
	resp := postJSON(t, s.url+"/api/job-listings/"+id+"/archive", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("archiving %s: got %d", id, resp.StatusCode)
	}
}

func (s lookupServer) moveTo(t *testing.T, id string, status string) {
	t.Helper()
	resp := patchJSON(t, s.url+"/api/applications/"+id+"/status", map[string]any{"status": status})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("moving %s to %s: got %d", id, status, resp.StatusCode)
	}
}

func (s lookupServer) lookup(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	resp := postJSON(t, s.url+lookupPath, payload)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from the lookup, got %d", resp.StatusCode)
	}
	return decodeBody(t, resp)
}

func TestCaptureLookup_UntrackedPosting_ReportsNothingTracked(t *testing.T) {
	s := newLookupServer(t)

	body := s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/view/4012345678/", "company": "Acme"})

	if body["tracked"] != nil {
		t.Errorf("expected tracked null for a posting never saved, got %v", body["tracked"])
	}
	company, ok := body["company"].(map[string]any)
	if !ok {
		t.Fatalf("expected a company object even with no siblings, got %v", body)
	}
	if listings, _ := company["listings"].([]any); len(listings) != 0 {
		t.Errorf("expected no sibling listings, got %v", listings)
	}
}

func TestCaptureLookup_TrackedPosting_ReportsItAndTheLegalStatusMoves(t *testing.T) {
	s := newLookupServer(t)
	id := s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")

	// Found from the search-pane URL, not the one it was saved from.
	body := s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/search/?currentJobId=4012345678", "company": "Acme"})

	tracked, ok := body["tracked"].(map[string]any)
	if !ok {
		t.Fatalf("expected the posting reported as tracked, got %v", body)
	}
	if tracked["id"] != id {
		t.Errorf("expected id %q, got %v", id, tracked["id"])
	}
	if tracked["title"] != "Backend Engineer" {
		t.Errorf("expected the Job Title, got %v", tracked["title"])
	}
	if tracked["status"] != "saved" {
		t.Errorf("expected status saved, got %v", tracked["status"])
	}
	if tracked["archived"] != false {
		t.Errorf("expected archived false, got %v", tracked["archived"])
	}
	if tracked["savedAt"] == nil || tracked["savedAt"] == "" {
		t.Errorf("expected a saved date, got %v", tracked["savedAt"])
	}
	// Story 44: only the moves the state machine actually allows.
	if got := stringsOf(t, tracked["allowedTransitions"]); !equalStrings(got, []string{"tailoring", "withdrawn"}) {
		t.Errorf("expected the moves legal from saved, got %v", got)
	}
}

func TestCaptureLookup_AllowedTransitions_FollowTheStatusAtEachStage(t *testing.T) {
	s := newLookupServer(t)
	s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")
	s.moveTo(t, "acme", "tailoring")
	s.moveTo(t, "acme", "sent")

	body := s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/view/4012345678/", "company": "Acme"})
	tracked := body["tracked"].(map[string]any)

	if tracked["status"] != "sent" {
		t.Fatalf("expected status sent, got %v", tracked["status"])
	}
	if got := stringsOf(t, tracked["allowedTransitions"]); !equalStrings(got, []string{"interviewing", "rejected", "withdrawn"}) {
		t.Errorf("expected the moves legal from sent, got %v", got)
	}
}

func TestCaptureLookup_TrackedButArchived_SaysSo(t *testing.T) {
	s := newLookupServer(t)
	id := s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")
	s.archive(t, id)

	body := s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/view/4012345678/", "company": "Acme"})

	tracked, ok := body["tracked"].(map[string]any)
	if !ok {
		t.Fatalf("expected an archived posting still reported as tracked, got %v", body)
	}
	if tracked["archived"] != true {
		t.Errorf("expected archived true, got %v", tracked["archived"])
	}
}

// Stories 13, 14: the roles I already track at this company, with what I
// need to answer "do I want this one too?".
func TestCaptureLookup_CompanyWithOtherRoles_ListsThemWithStatuses(t *testing.T) {
	s := newLookupServer(t)
	other := s.save(t, "Acme", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")
	s.moveTo(t, other, "tailoring")

	body := s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/view/4012345678/", "company": "Acme"})

	listings := objectsOf(t, body["company"].(map[string]any)["listings"])
	if len(listings) != 1 {
		t.Fatalf("expected the company's other role listed, got %v", listings)
	}
	sibling := listings[0]
	if sibling["id"] != other || sibling["title"] != "Platform Engineer" || sibling["status"] != "tailoring" {
		t.Errorf("expected the sibling's id, title and status, got %v", sibling)
	}
	if sibling["savedAt"] == nil || sibling["savedAt"] == "" {
		t.Errorf("expected the sibling's saved date, got %v", sibling["savedAt"])
	}
}

// Story 21: formatting and legal suffixes don't split a company in two.
func TestCaptureLookup_CompanyNameFormatting_StillMatches(t *testing.T) {
	s := newLookupServer(t)
	s.save(t, "Acme Inc.", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")

	for _, spelling := range []string{"Acme", "ACME, Inc", "acme inc"} {
		body := s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/view/4012345678/", "company": spelling})
		if listings := objectsOf(t, body["company"].(map[string]any)["listings"]); len(listings) != 1 {
			t.Errorf("company %q: expected the sibling matched, got %v", spelling, listings)
		}
	}
}

// Stories 22, 23: a company worked through and closed out stops interrupting.
func TestCaptureLookup_ArchivedSiblings_AreExcluded(t *testing.T) {
	s := newLookupServer(t)
	archived := s.save(t, "Acme", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")
	s.archive(t, archived)
	active := s.save(t, "Acme", "Staff Engineer", "https://www.linkedin.com/jobs/view/4088888888/")

	body := s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/view/4012345678/", "company": "Acme"})

	listings := objectsOf(t, body["company"].(map[string]any)["listings"])
	if len(listings) != 1 || listings[0]["id"] != active {
		t.Errorf("expected only the active sibling, got %v", listings)
	}
}

func TestCaptureLookup_CompanyWhollyArchived_HasNoSiblings(t *testing.T) {
	s := newLookupServer(t)
	archived := s.save(t, "Acme", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")
	s.archive(t, archived)

	body := s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/view/4012345678/", "company": "Acme"})

	if listings := objectsOf(t, body["company"].(map[string]any)["listings"]); len(listings) != 0 {
		t.Errorf("expected no interruption from a wholly archived company, got %v", listings)
	}
}

// The tracked posting is not its own sibling: the card shows it on the
// state line already.
func TestCaptureLookup_TrackedPosting_IsExcludedFromItsOwnSiblings(t *testing.T) {
	s := newLookupServer(t)
	id := s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")
	s.save(t, "Acme", "Platform Engineer", "https://www.linkedin.com/jobs/view/4099999999/")

	body := s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/view/4012345678/", "company": "Acme"})

	for _, sibling := range objectsOf(t, body["company"].(map[string]any)["listings"]) {
		if sibling["id"] == id {
			t.Errorf("expected the tracked listing excluded from its own siblings, got %v", sibling)
		}
	}
}

// Story 49: a results page is one request, not one per row.
func TestCaptureLookup_BatchForm_AnswersTrackedStateForEachURL(t *testing.T) {
	s := newLookupServer(t)
	id := s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")

	body := s.lookup(t, map[string]any{"urls": []string{
		"https://www.linkedin.com/jobs/search/?currentJobId=4012345678",
		"https://www.linkedin.com/jobs/view/4099999999/",
	}})

	results := objectsOf(t, body["results"])
	if len(results) != 2 {
		t.Fatalf("expected one result per url, got %v", results)
	}
	if results[0]["url"] != "https://www.linkedin.com/jobs/search/?currentJobId=4012345678" {
		t.Errorf("expected each result to echo its url, got %v", results[0]["url"])
	}
	tracked, ok := results[0]["tracked"].(map[string]any)
	if !ok {
		t.Fatalf("expected the first url tracked, got %v", results[0])
	}
	if tracked["id"] != id || tracked["status"] != "saved" {
		t.Errorf("expected the badge's id and status, got %v", tracked)
	}
	if results[1]["tracked"] != nil {
		t.Errorf("expected the second url untracked, got %v", results[1]["tracked"])
	}
}

func TestCaptureLookup_BatchForm_OverTheCapIsRefused(t *testing.T) {
	s := newLookupServer(t)
	urls := make([]string, 201)
	for i := range urls {
		urls[i] = "https://www.linkedin.com/jobs/view/40000000" + string(rune('0'+i%10)) + "/"
	}

	resp := postJSON(t, s.url+lookupPath, map[string]any{"urls": urls})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected an over-cap batch refused with 400, got %d", resp.StatusCode)
	}
}

func TestCaptureLookup_WritesNothing(t *testing.T) {
	s := newLookupServer(t)
	s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")

	s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/view/4012345678/", "company": "Acme"})

	listed, err := http.Get(s.url + "/api/job-listings?archived=all")
	if err != nil {
		t.Fatal(err)
	}
	defer listed.Body.Close()
	var rows []map[string]any
	if err := json.NewDecoder(listed.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("expected the lookup to write nothing, found %d listings", len(rows))
	}
}

// The extension calls this cross-origin, so it needs the same CORS headers
// and preflight the capture route carries.
func TestCaptureLookup_CarriesCORSHeadersAndAnswersPreflight(t *testing.T) {
	s := newLookupServer(t)

	resp := postJSON(t, s.url+lookupPath, map[string]any{"url": "https://example.com/x", "company": "Acme"})
	defer resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected the response to carry CORS, got %q", got)
	}

	req, err := http.NewRequest(http.MethodOptions, s.url+lookupPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer preflight.Body.Close()
	if preflight.StatusCode != http.StatusNoContent {
		t.Errorf("expected the preflight answered with 204, got %d", preflight.StatusCode)
	}
	if got := preflight.Header.Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("expected the preflight to name the allowed methods")
	}
}

// stringsOf and objectsOf unwrap decoded JSON arrays.
func stringsOf(t *testing.T, v any) []string {
	t.Helper()
	items, ok := v.([]any)
	if !ok {
		t.Fatalf("expected an array, got %v", v)
	}
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.(string)
	}
	return out
}

func objectsOf(t *testing.T, v any) []map[string]any {
	t.Helper()
	if v == nil {
		return nil
	}
	items, ok := v.([]any)
	if !ok {
		t.Fatalf("expected an array, got %v", v)
	}
	out := make([]map[string]any, len(items))
	for i, item := range items {
		out[i] = item.(map[string]any)
	}
	return out
}

// Location is captured and kept (issue #206, stories 55-57): the extension
// sends it, and it survives to the record and to the list row.
func TestCapture_Location_IsPersistedAndListed(t *testing.T) {
	s := newLookupServer(t)

	resp := postJSON(t, s.url+"/api/job-listings/from-extension", map[string]any{
		"company": "Acme", "title": "Backend Engineer", "location": "Milan, Lombardy, Italy",
		"url": "https://www.linkedin.com/jobs/view/4012345678/", "description": "A backend role.",
	})
	defer resp.Body.Close()
	id := createdListingID(t, resp)

	detail := s.jobListing(t, id)
	if detail["location"] != "Milan, Lombardy, Italy" {
		t.Errorf("expected the location on the record, got %v", detail["location"])
	}

	listed, err := http.Get(s.url + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer listed.Body.Close()
	var rows []map[string]any
	if err := json.NewDecoder(listed.Body).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if got := rows[0]["jobListing"].(map[string]any)["location"]; got != "Milan, Lombardy, Italy" {
		t.Errorf("expected the location on the list row, got %v", got)
	}
}

// Story 59: the manual path isn't a second-class one.
func TestCreateJobListing_Location_IsPersisted(t *testing.T) {
	s := newLookupServer(t)

	resp := postJSON(t, s.url+"/api/job-listings", map[string]any{
		"company": "Acme", "title": "Backend Engineer", "location": "Remote",
		"jobDescription": "A backend role.",
	})
	defer resp.Body.Close()
	id := createdListingID(t, resp)

	if got := s.jobListing(t, id)["location"]; got != "Remote" {
		t.Errorf("expected the location kept from the manual form, got %v", got)
	}
}

// Story 60: a listing saved without one is an ordinary empty value.
func TestCreateJobListing_NoLocation_OmitsTheField(t *testing.T) {
	s := newLookupServer(t)

	resp := postJSON(t, s.url+"/api/job-listings", map[string]any{
		"company": "Acme", "jobDescription": "A backend role.",
	})
	defer resp.Body.Close()
	id := createdListingID(t, resp)

	if got, present := s.jobListing(t, id)["location"]; present && got != "" {
		t.Errorf("expected no location, got %v", got)
	}
}

// jobListing reads one Job Listing's detail response.
func (s lookupServer) jobListing(t *testing.T, id string) map[string]any {
	t.Helper()
	resp, err := http.Get(s.url + "/api/job-listings/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reading %s: got %d", id, resp.StatusCode)
	}
	return decodeBody(t, resp)["jobListing"].(map[string]any)
}
