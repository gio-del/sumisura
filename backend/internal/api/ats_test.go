package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

type fakeATSDoer struct {
	do func(*http.Request) (*http.Response, error)
}

func (f fakeATSDoer) Do(req *http.Request) (*http.Response, error) {
	return f.do(req)
}

func jsonATSResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}
}

const fixtureGreenhouseResponse = `{
  "jobs": [
    {
      "id": 1,
      "title": "Backend Engineer",
      "location": {"name": "Remote"},
      "absolute_url": "https://boards.greenhouse.io/acme/jobs/1",
      "content": "<p>Join us.</p>"
    }
  ]
}`

func TestListAtsListings_Greenhouse_ReturnsNormalizedListings(t *testing.T) {
	dataDir := seedDataDir(t)
	doer := fakeATSDoer{do: func(req *http.Request) (*http.Response, error) {
		return jsonATSResponse(http.StatusOK, fixtureGreenhouseResponse), nil
	}}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: doer}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/ats/greenhouse/acme/listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var listings []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&listings); err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0]["title"] != "Backend Engineer" {
		t.Errorf("expected title Backend Engineer, got %v", listings[0]["title"])
	}
	if listings[0]["url"] != "https://boards.greenhouse.io/acme/jobs/1" {
		t.Errorf("expected url from absolute_url, got %v", listings[0]["url"])
	}
}

func TestListAtsListings_MarksAlreadySavedListingsByURL(t *testing.T) {
	dataDir := seedDataDir(t)
	doer := fakeATSDoer{do: func(req *http.Request) (*http.Response, error) {
		return jsonATSResponse(http.StatusOK, fixtureGreenhouseResponse), nil
	}}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: doer}))
	defer server.Close()

	saveResp := postJSON(t, server.URL+"/api/job-listings", map[string]any{
		"company":        "Acme Corp",
		"url":            "https://boards.greenhouse.io/acme/jobs/1",
		"jobDescription": "Already saved role.",
	})
	saveResp.Body.Close()
	if saveResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected save to succeed with 201, got %d", saveResp.StatusCode)
	}

	resp, err := http.Get(server.URL + "/api/ats/greenhouse/acme/listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var listings []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&listings); err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(listings))
	}
	if listings[0]["alreadySaved"] != true {
		t.Errorf("expected the previously-saved listing to be marked alreadySaved, got %v", listings[0]["alreadySaved"])
	}
}

func TestListAtsListings_UntrackedBoard_NeverMarksNewOrCreatesSeenState(t *testing.T) {
	dataDir := seedDataDir(t)
	doer := fakeATSDoer{do: func(req *http.Request) (*http.Response, error) {
		return jsonATSResponse(http.StatusOK, fixtureGreenhouseResponse), nil
	}}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: doer}))
	defer server.Close()

	// Story 10: a one-off browse of a board never tracked shouldn't require
	// or create seen-state.
	resp, err := http.Get(server.URL + "/api/ats/greenhouse/acme/listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var listings []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&listings); err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 || listings[0]["new"] != false {
		t.Errorf("expected the untracked board's listing to not be flagged new, got %v", listings)
	}
}

func TestListAtsListings_TrackedBoard_FirstFetch_SeedsBaselineNoneNew(t *testing.T) {
	dataDir := seedDataDir(t)
	doer := fakeATSDoer{do: func(req *http.Request) (*http.Response, error) {
		return jsonATSResponse(http.StatusOK, fixtureGreenhouseResponse), nil
	}}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: doer}))
	defer server.Close()

	trackResp := postJSON(t, server.URL+"/api/ats/tracked-boards", map[string]any{"provider": "greenhouse", "slug": "acme"})
	trackResp.Body.Close()

	// Story 5: tracking a board with an existing backlog shouldn't report
	// that backlog as new on the very first fetch.
	resp, err := http.Get(server.URL + "/api/ats/greenhouse/acme/listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var listings []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&listings); err != nil {
		t.Fatal(err)
	}
	if len(listings) != 1 || listings[0]["new"] != false {
		t.Errorf("expected no listings marked new on the first fetch of a freshly-tracked board, got %v", listings)
	}
}

func TestListAtsListings_TrackedBoard_SecondFetch_OnlyFlagsGenuinelyNewListings(t *testing.T) {
	dataDir := seedDataDir(t)
	fetchCount := 0
	doer := fakeATSDoer{do: func(req *http.Request) (*http.Response, error) {
		fetchCount++
		if fetchCount == 1 {
			return jsonATSResponse(http.StatusOK, fixtureGreenhouseResponse), nil
		}
		return jsonATSResponse(http.StatusOK, `{
		  "jobs": [
		    {"id": 1, "title": "Backend Engineer", "location": {"name": "Remote"}, "absolute_url": "https://boards.greenhouse.io/acme/jobs/1", "content": "<p>Join us.</p>"},
		    {"id": 2, "title": "New Role", "location": {"name": "Remote"}, "absolute_url": "https://boards.greenhouse.io/acme/jobs/2", "content": "<p>New.</p>"}
		  ]
		}`), nil
	}}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: doer}))
	defer server.Close()

	trackResp := postJSON(t, server.URL+"/api/ats/tracked-boards", map[string]any{"provider": "greenhouse", "slug": "acme"})
	trackResp.Body.Close()

	firstResp, err := http.Get(server.URL + "/api/ats/greenhouse/acme/listings")
	if err != nil {
		t.Fatal(err)
	}
	firstResp.Body.Close()

	secondResp, err := http.Get(server.URL + "/api/ats/greenhouse/acme/listings")
	if err != nil {
		t.Fatal(err)
	}
	defer secondResp.Body.Close()

	var listings []map[string]any
	if err := json.NewDecoder(secondResp.Body).Decode(&listings); err != nil {
		t.Fatal(err)
	}
	if len(listings) != 2 {
		t.Fatalf("expected 2 listings, got %d", len(listings))
	}
	byURL := map[string]map[string]any{}
	for _, l := range listings {
		byURL[l["url"].(string)] = l
	}
	if byURL["https://boards.greenhouse.io/acme/jobs/1"]["new"] != false {
		t.Errorf("expected the already-seen listing to not be flagged new, got %v", byURL["https://boards.greenhouse.io/acme/jobs/1"])
	}
	if byURL["https://boards.greenhouse.io/acme/jobs/2"]["new"] != true {
		t.Errorf("expected the genuinely new listing to be flagged new, got %v", byURL["https://boards.greenhouse.io/acme/jobs/2"])
	}
}

func TestListAtsListings_BoardNotFound_Returns404WithClearError(t *testing.T) {
	dataDir := seedDataDir(t)
	doer := fakeATSDoer{do: func(req *http.Request) (*http.Response, error) {
		return jsonATSResponse(http.StatusNotFound, `{}`), nil
	}}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: doer}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/ats/greenhouse/does-not-exist/listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	got := strings.ToLower(string(body))
	// Story 6: the message must let the user act on it — name the slug they
	// typed and suggest what to try next — not just say "not found".
	if !strings.Contains(got, "does-not-exist") {
		t.Errorf("expected the error to name the slug that wasn't found, got %q", body)
	}
	if !strings.Contains(got, "check the slug") && !strings.Contains(got, "different provider") {
		t.Errorf("expected the error to suggest checking the slug or trying a different provider, got %q", body)
	}
}

const todayBoardsSecond = `{"jobs": [
 {"id": 1, "title": "Backend Engineer", "location": {"name": "Remote"}, "absolute_url": "https://boards.greenhouse.io/acme/jobs/1", "content": "<p>x</p>"},
 {"id": 2, "title": "New Role", "location": {"name": "Remote"}, "absolute_url": "https://boards.greenhouse.io/acme/jobs/2", "content": "<p>x</p>"},
 {"id": 3, "title": "Other Role", "location": {"name": "Remote"}, "absolute_url": "https://boards.greenhouse.io/acme/jobs/3", "content": "<p>x</p>"}
]}`

func getToday(t *testing.T, url string) map[string]any {
	t.Helper()
	resp, err := http.Get(url + "/api/today")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestToday_NoStoredBoardCheck_OmitsNewListings(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()
	postJSON(t, server.URL+"/api/ats/tracked-boards", map[string]any{"provider": "greenhouse", "slug": "acme"}).Body.Close()

	today := getToday(t, server.URL)
	if _, ok := today["newListings"]; ok {
		t.Errorf("expected no newListings without a stored check, got %v", today)
	}
	if _, ok := today["lastBoardCheck"]; ok {
		t.Errorf("expected no lastBoardCheck without a stored check, got %v", today)
	}
	if today["trackedBoards"] != float64(1) {
		t.Errorf("expected trackedBoards 1, got %v", today["trackedBoards"])
	}
}

func TestToday_NewListingsFromLastCheck_DropWhenSaved(t *testing.T) {
	dataDir := seedDataDir(t)
	fetchCount := 0
	doer := fakeATSDoer{do: func(req *http.Request) (*http.Response, error) {
		fetchCount++
		if fetchCount == 1 {
			return jsonATSResponse(http.StatusOK, fixtureGreenhouseResponse), nil
		}
		return jsonATSResponse(http.StatusOK, todayBoardsSecond), nil
	}}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: doer}))
	defer server.Close()
	postJSON(t, server.URL+"/api/ats/tracked-boards", map[string]any{"provider": "greenhouse", "slug": "acme"}).Body.Close()

	for i := 0; i < 2; i++ {
		resp, err := http.Get(server.URL + "/api/ats/greenhouse/acme/listings")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	before := getToday(t, server.URL)
	if before["newListings"] != float64(2) {
		t.Fatalf("expected 2 new listings (2 and 3), got %v", before["newListings"])
	}
	if before["lastBoardCheck"] == nil {
		t.Fatalf("expected lastBoardCheck, got %v", before)
	}
	fetchesAfterChecks := fetchCount

	saved := postJSON(t, server.URL+"/api/job-listings", map[string]any{
		"company":        "Acme",
		"title":          "New Role",
		"url":            "https://boards.greenhouse.io/acme/jobs/2",
		"jobDescription": "New role.",
	})
	saved.Body.Close()

	after := getToday(t, server.URL)
	if after["newListings"] != float64(1) {
		t.Errorf("expected the count to drop to 1 once a listing is saved, got %v", after["newListings"])
	}
	if fetchCount != fetchesAfterChecks {
		t.Errorf("opening Today must not fetch any board")
	}
}
