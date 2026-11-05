package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

func TestCreateJobListing_StatedRAL_WritesFilesAndCreatesSavedApplication(t *testing.T) {
	dataDir := seedDataDir(t)
	estimateCalled := false
	client := &fakeGenerationClient{
		estimateRAL: func(ctx context.Context, jobDescription string) (generation.RALRange, error) {
			estimateCalled = true
			return generation.RALRange{Source: generation.RALSourceNA}, nil
		},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: client}))
	defer server.Close()

	payload := map[string]any{
		"company":        "Acme Corp",
		"url":            "https://acme.example/jobs/42",
		"jobDescription": "Go backend engineer. Salary: €50,000 - €60,000.",
	}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	listing, ok := result["jobListing"].(map[string]any)
	if !ok {
		t.Fatalf("expected a jobListing object, got %v", result["jobListing"])
	}
	id, ok := listing["id"].(string)
	if !ok || id == "" {
		t.Fatalf("expected a generated jobListing id, got %v", listing["id"])
	}
	if listing["company"] != "Acme Corp" {
		t.Errorf("expected company Acme Corp, got %v", listing["company"])
	}
	if listing["source"] != "manual" {
		t.Errorf("expected source manual, got %v", listing["source"])
	}
	ral, ok := listing["ral"].(map[string]any)
	if !ok || ral["source"] != "stated" {
		t.Fatalf("expected stated RAL parsed from the job description, got %v", listing["ral"])
	}
	if estimateCalled {
		t.Error("expected EstimateRAL not to be called when a RAL figure is stated in the job description")
	}

	application, ok := result["application"].(map[string]any)
	if !ok {
		t.Fatalf("expected an application object, got %v", result["application"])
	}
	if application["id"] != id {
		t.Errorf("expected application id to match jobListing id %q, got %v", id, application["id"])
	}
	if application["jobListingId"] != id {
		t.Errorf("expected application jobListingId %q, got %v", id, application["jobListingId"])
	}
	if application["status"] != "saved" {
		t.Errorf("expected application status saved, got %v", application["status"])
	}

	jobFile, err := os.ReadFile(filepath.Join(dataDir, "jobs", id+".md"))
	if err != nil {
		t.Fatalf("expected job listing file to exist: %v", err)
	}
	if !bytes.Contains(jobFile, []byte("Acme Corp")) || !bytes.Contains(jobFile, []byte("Go backend engineer")) {
		t.Errorf("expected job listing file to contain company and job description, got:\n%s", jobFile)
	}

	applicationFile, err := os.ReadFile(filepath.Join(dataDir, "applications", id+".md"))
	if err != nil {
		t.Fatalf("expected application file to exist: %v", err)
	}
	if !bytes.Contains(applicationFile, []byte("saved")) {
		t.Errorf("expected application file to record status saved, got:\n%s", applicationFile)
	}
}

func TestCreateJobListing_TitleCarriesThrough(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	payload := map[string]any{
		"title":          "Senior Backend Engineer",
		"company":        "Acme Corp",
		"jobDescription": "Go backend engineer, remote friendly.",
	}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	listing := result["jobListing"].(map[string]any)
	if listing["title"] != "Senior Backend Engineer" {
		t.Errorf("expected title to carry through, got %v", listing["title"])
	}
}

func TestCreateJobListing_NoTitle_OmitsTitleField(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	payload := map[string]any{"company": "Acme Corp", "jobDescription": "Go backend engineer."}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	listing := result["jobListing"].(map[string]any)
	if _, present := listing["title"]; present {
		t.Errorf("expected no title field when none was given, got %v", listing["title"])
	}
}

func TestCreateJobListing_InfersApplicationMethodFromJobDescription(t *testing.T) {
	dataDir := seedDataDir(t)
	client := &fakeGenerationClient{
		inferApplicationMethod: func(ctx context.Context, jobDescription string) (tracking.ApplicationMethod, error) {
			if jobDescription != "Email your CV to jobs@acme.example." {
				t.Errorf("expected job description to reach the client, got %q", jobDescription)
			}
			return tracking.ApplicationMethod{Kind: tracking.MethodEmail, Value: "jobs@acme.example"}, nil
		},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: client}))
	defer server.Close()

	payload := map[string]any{"company": "Acme Corp", "jobDescription": "Email your CV to jobs@acme.example."}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	application := result["application"].(map[string]any)
	method, ok := application["method"].(map[string]any)
	if !ok {
		t.Fatalf("expected an application method object, got %v", application["method"])
	}
	if method["kind"] != "email" || method["value"] != "jobs@acme.example" {
		t.Errorf("expected inferred email method, got %v", method)
	}
}

func TestCreateJobListing_NoStatedRAL_CallsClientEstimateRAL(t *testing.T) {
	dataDir := seedDataDir(t)
	estimateCalled := false
	client := &fakeGenerationClient{
		estimateRAL: func(ctx context.Context, jobDescription string) (generation.RALRange, error) {
			estimateCalled = true
			min, max := 40000, 50000
			return generation.RALRange{Min: &min, Max: &max, Currency: "EUR", Source: generation.RALSourceEstimated}, nil
		},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: client}))
	defer server.Close()

	payload := map[string]any{"company": "Acme Corp", "jobDescription": "Go backend engineer, remote."}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if !estimateCalled {
		t.Error("expected EstimateRAL to be called when no RAL figure is stated in the job description")
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	listing := result["jobListing"].(map[string]any)
	ral := listing["ral"].(map[string]any)
	if ral["source"] != "estimated" {
		t.Errorf("expected estimated RAL, got %v", ral["source"])
	}
}

func TestCreateJobListing_RALEstimationFails_StillSavesWithUnresolvedRAL(t *testing.T) {
	dataDir := seedDataDir(t)
	inferCalled := false
	client := &fakeGenerationClient{
		estimateRAL: func(ctx context.Context, jobDescription string) (generation.RALRange, error) {
			return generation.RALRange{}, errors.New("claude api unreachable")
		},
		inferApplicationMethod: func(ctx context.Context, jobDescription string) (tracking.ApplicationMethod, error) {
			inferCalled = true
			return tracking.ApplicationMethod{Kind: tracking.MethodOther}, nil
		},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: client}))
	defer server.Close()

	payload := map[string]any{"company": "Acme Corp", "jobDescription": "Go backend engineer, remote."}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if !inferCalled {
		t.Error("expected Application Method inference to still run when RAL estimation fails")
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	listing := result["jobListing"].(map[string]any)
	ral := listing["ral"].(map[string]any)
	if ral["source"] != "unresolved" {
		t.Errorf("expected unresolved RAL, got %v", ral["source"])
	}
	application := result["application"].(map[string]any)
	method := application["method"].(map[string]any)
	if method["kind"] != "other" {
		t.Errorf("expected Application Method to still resolve normally, got %v", method)
	}

	id := listing["id"].(string)
	jobFile, err := os.ReadFile(filepath.Join(dataDir, "jobs", id+".md"))
	if err != nil {
		t.Fatalf("expected job listing file to exist: %v", err)
	}
	if !bytes.Contains(jobFile, []byte("unresolved")) {
		t.Errorf("expected job listing file to record RAL source unresolved, got:\n%s", jobFile)
	}
}

func TestCreateJobListing_MethodInferenceFails_StillSavesWithUnresolvedMethod(t *testing.T) {
	dataDir := seedDataDir(t)
	client := &fakeGenerationClient{
		inferApplicationMethod: func(ctx context.Context, jobDescription string) (tracking.ApplicationMethod, error) {
			return tracking.ApplicationMethod{}, errors.New("claude api unreachable")
		},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: client}))
	defer server.Close()

	payload := map[string]any{"company": "Acme Corp", "jobDescription": "Go backend engineer. Salary: €50,000 - €60,000."}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	listing := result["jobListing"].(map[string]any)
	ral := listing["ral"].(map[string]any)
	if ral["source"] != "stated" {
		t.Errorf("expected RAL to still resolve normally (stated), got %v", ral["source"])
	}
	application := result["application"].(map[string]any)
	method := application["method"].(map[string]any)
	if method["kind"] != "unresolved" {
		t.Errorf("expected unresolved Application Method, got %v", method)
	}

	id := listing["id"].(string)
	applicationFile, err := os.ReadFile(filepath.Join(dataDir, "applications", id+".md"))
	if err != nil {
		t.Fatalf("expected application file to exist: %v", err)
	}
	if !bytes.Contains(applicationFile, []byte("unresolved")) {
		t.Errorf("expected application file to record method kind unresolved, got:\n%s", applicationFile)
	}
}

func TestCreateJobListing_BothRALAndMethodFail_BothMarkedUnresolved(t *testing.T) {
	dataDir := seedDataDir(t)
	client := &fakeGenerationClient{
		estimateRAL: func(ctx context.Context, jobDescription string) (generation.RALRange, error) {
			return generation.RALRange{}, errors.New("claude api unreachable")
		},
		inferApplicationMethod: func(ctx context.Context, jobDescription string) (tracking.ApplicationMethod, error) {
			return tracking.ApplicationMethod{}, errors.New("claude api unreachable")
		},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: client}))
	defer server.Close()

	payload := map[string]any{"company": "Acme Corp", "jobDescription": "Go backend engineer, remote."}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	listing := result["jobListing"].(map[string]any)
	ral := listing["ral"].(map[string]any)
	if ral["source"] != "unresolved" {
		t.Errorf("expected unresolved RAL, got %v", ral["source"])
	}
	application := result["application"].(map[string]any)
	method := application["method"].(map[string]any)
	if method["kind"] != "unresolved" {
		t.Errorf("expected unresolved Application Method, got %v", method)
	}
}

func TestCreateJobListing_JobDescriptionURL_FetchesText(t *testing.T) {
	dataDir := seedDataDir(t)
	jdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body><p>Go backend engineer.</p></body></html>"))
	}))
	defer jdServer.Close()

	client := &fakeGenerationClient{}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: client}))
	defer server.Close()

	payload := map[string]any{"company": "Acme Corp", "jobDescriptionUrl": jdServer.URL}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	listing := result["jobListing"].(map[string]any)
	if listing["jobDescription"] != "Go backend engineer." {
		t.Errorf("expected fetched+stripped job description, got %v", listing["jobDescription"])
	}
}

func TestCreateJobListing_MissingCompany_Returns400AndNoFilesCreated(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	payload := map[string]any{"jobDescription": "Go backend engineer."}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	assertNoFilesIn(t, filepath.Join(dataDir, "jobs"))
}

func TestCreateJobListing_MissingJobDescription_Returns400AndNoFilesCreated(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	payload := map[string]any{"company": "Acme Corp"}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	assertNoFilesIn(t, filepath.Join(dataDir, "jobs"))
}

// Confirms POST /api/job-listings (the ATS-browse and manual-paste save
// path) downloads and persists a Company Logo when the request carries a
// logoUrl, mirroring what the extension-capture path already does — issue
// #42 extends this same save path to ATS-sourced listings (fixturePNG is
// defined in job_listings_extension_test.go, same package).
func TestCreateJobListing_LogoURLPresent_DownloadsAndPersistsLogo(t *testing.T) {
	dataDir := seedDataDir(t)
	doer := fakeATSDoer{do: func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://boards.greenhouse.io/acme/logo.png" {
			t.Errorf("expected a request to the logo URL, got %s", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(fixturePNG)),
		}, nil
	}}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}, ATSHTTPDoer: doer}))
	defer server.Close()

	payload := map[string]any{
		"company":        "Acme Corp",
		"url":            "https://boards.greenhouse.io/acme/jobs/1",
		"jobDescription": "Go backend engineer.",
		"logoUrl":        "https://boards.greenhouse.io/acme/logo.png",
	}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	listing := result["jobListing"].(map[string]any)
	logo, ok := listing["logo"].(string)
	if !ok || logo == "" {
		t.Fatalf("expected a non-empty logo filename, got %v", listing["logo"])
	}

	logoBytes, err := os.ReadFile(filepath.Join(dataDir, "jobs", logo))
	if err != nil {
		t.Fatalf("expected the downloaded logo file to exist on disk: %v", err)
	}
	if !bytes.Equal(logoBytes, fixturePNG) {
		t.Errorf("expected the downloaded logo file to contain the fetched bytes")
	}
}

// Regression check: omitting logoUrl behaves exactly as before this issue
// — no doer call, no logo field on the saved listing.
func TestCreateJobListing_NoLogoURL_SavesWithoutLogo(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	payload := map[string]any{
		"company":        "Acme Corp",
		"jobDescription": "Go backend engineer.",
	}
	resp := postJSON(t, server.URL+"/api/job-listings", payload)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	listing := result["jobListing"].(map[string]any)
	if logo, present := listing["logo"]; present {
		t.Errorf("expected no logo field when logoUrl is omitted, got %v", logo)
	}
}

func TestListJobListings_ReturnsEachWithItsApplicationNewestFirst(t *testing.T) {
	dataDir := seedDataDir(t)
	client := &fakeGenerationClient{}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: client}))
	defer server.Close()

	postJSON(t, server.URL+"/api/job-listings", map[string]any{
		"company":        "First Corp",
		"jobDescription": "Role one. Salary: €40,000.",
	}).Body.Close()
	postJSON(t, server.URL+"/api/job-listings", map[string]any{
		"company":        "Second Corp",
		"jobDescription": "Role two. Salary: €50,000.",
	}).Body.Close()

	resp, err := http.Get(server.URL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var results []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 job listings, got %d", len(results))
	}

	first := results[0]["jobListing"].(map[string]any)
	if first["company"] != "Second Corp" {
		t.Errorf("expected the most recently saved listing (Second Corp) first, got %v", first["company"])
	}

	application := results[0]["application"].(map[string]any)
	if application["status"] != "saved" {
		t.Errorf("expected application status saved, got %v", application["status"])
	}
	if application["jobListingId"] != first["id"] {
		t.Errorf("expected application jobListingId to match jobListing id, got %v vs %v", application["jobListingId"], first["id"])
	}

	ral := first["ral"].(map[string]any)
	if ral["source"] != "stated" {
		t.Errorf("expected stated RAL on the list view, got %v", ral["source"])
	}
}

func TestListJobListings_NoneSaved_ReturnsEmptyArray(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var results []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected no job listings, got %v", results)
	}
}

// getJobListingDetail reads the detail endpoint's {jobListing, application}
// pair — the shape the Job Listing detail page needs to render a record
// without a second request (issue #94).
func getJobListingDetail(t *testing.T, serverURL, id string) map[string]any {
	t.Helper()
	resp, err := http.Get(serverURL + "/api/job-listings/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestGetJobListing_ReturnsSavedRecordWithItsApplication(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	result := getJobListingDetail(t, server.URL, id)

	listing, ok := result["jobListing"].(map[string]any)
	if !ok {
		t.Fatalf("expected a jobListing object, got %v", result["jobListing"])
	}
	if listing["company"] != "Acme Corp" {
		t.Errorf("expected company Acme Corp, got %v", listing["company"])
	}

	application, ok := result["application"].(map[string]any)
	if !ok {
		t.Fatalf("expected an application object, got %v", result["application"])
	}
	if application["jobListingId"] != id {
		t.Errorf("expected application jobListingId %q, got %v", id, application["jobListingId"])
	}
	if application["status"] != "saved" {
		t.Errorf("expected status saved, got %v", application["status"])
	}
	if _, ok := application["method"].(map[string]any); !ok {
		t.Errorf("expected an application method, got %v", application["method"])
	}
	if _, ok := application["statusHistory"].([]any); !ok {
		t.Errorf("expected a statusHistory, got %v", application["statusHistory"])
	}
}

func TestGetJobListing_AfterContactAndGeneration_CarriesTheWholeApplication(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	contactResp := patchJSON(t, server.URL+"/api/applications/"+id+"/contact", map[string]any{
		"name":  "Dana Recruiter",
		"email": "dana@acme.example",
	})
	contactResp.Body.Close()

	genResp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":   "acme-corp",
		"cvPath": "output/acme-corp/cv.pdf",
	})
	genResp.Body.Close()

	application := getJobListingDetail(t, server.URL, id)["application"].(map[string]any)

	contact, ok := application["contact"].(map[string]any)
	if !ok || contact["email"] != "dana@acme.example" {
		t.Errorf("expected the confirmed Contact on the detail response, got %v", application["contact"])
	}
	generations, ok := application["generations"].([]any)
	if !ok || len(generations) != 1 {
		t.Fatalf("expected 1 generation record, got %v", application["generations"])
	}
	if generations[0].(map[string]any)["slug"] != "acme-corp" {
		t.Errorf("expected the recorded Generation's slug, got %v", generations[0])
	}
}

// listJobListingsJSON reads GET /api/job-listings decoded generically, so a
// test can assert on which fields the list response does and does not carry.
func listJobListingsJSON(t *testing.T, serverURL string) []map[string]any {
	t.Helper()
	resp, err := http.Get(serverURL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var listed []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	return listed
}

// The list returns a summary of each Job Listing (issue #97): the Job
// Description text stays behind the detail endpoint, so the list payload
// does not grow by a full posting with every save.
func TestListJobListings_OmitsTheJobDescriptionText(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	saveJobListing(t, server.URL, "Acme Corp")

	listed := listJobListingsJSON(t, server.URL)
	if len(listed) != 1 {
		t.Fatalf("expected 1 job listing, got %d", len(listed))
	}
	listing := listed[0]["jobListing"].(map[string]any)
	if description, present := listing["jobDescription"]; present {
		t.Errorf("expected the list response to omit jobDescription, got %v", description)
	}
}

func TestListJobListings_ReportsWhetherEachListingHasAJobDescription(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	withDescription := saveJobListing(t, server.URL, "Acme Corp")
	withoutDescription := saveJobListing(t, server.URL, "Beta Inc")
	// The save endpoints require a Job Description, but a Job Listing file
	// edited by hand can be left with an empty body: cut it after the
	// frontmatter's closing delimiter.
	path := filepath.Join(dataDir, "jobs", withoutDescription+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	closing := []byte("\n---\n")
	end := bytes.Index(content[len("---"):], closing)
	if end == -1 {
		t.Fatalf("expected frontmatter in %s, got %q", path, content)
	}
	if err := os.WriteFile(path, content[:len("---")+end+len(closing)], 0o644); err != nil {
		t.Fatal(err)
	}

	hasJobDescription := map[string]any{}
	for _, row := range listJobListingsJSON(t, server.URL) {
		listing := row["jobListing"].(map[string]any)
		hasJobDescription[listing["id"].(string)] = listing["hasJobDescription"]
	}
	if hasJobDescription[withDescription] != true {
		t.Errorf("expected hasJobDescription true for %s, got %v", withDescription, hasJobDescription[withDescription])
	}
	if hasJobDescription[withoutDescription] != false {
		t.Errorf("expected hasJobDescription false for %s, got %v", withoutDescription, hasJobDescription[withoutDescription])
	}
}

func TestListJobListings_SummaryKeepsEveryFieldTheListRenders(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/api/job-listings", map[string]any{
		"title":          "Backend Engineer",
		"company":        "Acme Corp",
		"url":            "https://acme.example/jobs/1",
		"jobDescription": "Some role. Salary: €40,000.",
	})
	resp.Body.Close()

	listing := listJobListingsJSON(t, server.URL)[0]["jobListing"].(map[string]any)
	want := map[string]any{
		"id":              "acme-corp",
		"title":           "Backend Engineer",
		"company":         "Acme Corp",
		"url":             "https://acme.example/jobs/1",
		"source":          "manual",
		"freshnessStatus": "not-yet-checked",
	}
	for field, value := range want {
		if listing[field] != value {
			t.Errorf("expected %s %v on the list summary, got %v", field, value, listing[field])
		}
	}
	if savedAt, _ := listing["savedAt"].(string); savedAt == "" {
		t.Errorf("expected savedAt on the list summary, got %v", listing["savedAt"])
	}
	if ral, ok := listing["ral"].(map[string]any); !ok || ral["source"] != "stated" {
		t.Errorf("expected the stated RAL Range on the list summary, got %v", listing["ral"])
	}
}

// The detail endpoint stays the one place a Job Description comes from, and
// carries it verbatim.
func TestGetJobListing_CarriesTheFullJobDescriptionVerbatim(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	description := "We are hiring.\n\n**Requirements**\n\n- Go\n- [Postgres](https://postgres.example)"
	id := saveListing(t, server.URL, "Acme Corp", description)

	listing := getJobListingDetail(t, server.URL, id)["jobListing"].(map[string]any)
	if listing["jobDescription"] != description {
		t.Errorf("expected the detail response to carry the Job Description verbatim, got %q", listing["jobDescription"])
	}
}

// The detail page and the list row show the same record; a divergence
// between the two responses would mean an affordance works in one place and
// not the other (issue #94's "the two cannot drift"). The one deliberate
// difference is the Job Description (issue #97): the list row reports only
// whether there is one.
func TestGetJobListing_MatchesWhatTheListEndpointReturnsForTheSameRecord(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	listed := listJobListingsJSON(t, server.URL)
	if len(listed) != 1 {
		t.Fatalf("expected 1 job listing, got %d", len(listed))
	}

	detail := getJobListingDetail(t, server.URL, id)
	detailListing := detail["jobListing"].(map[string]any)
	delete(detailListing, "jobDescription")
	detailListing["hasJobDescription"] = true
	// sameCompany is detail-only (issue #287).
	delete(detail, "sameCompany")
	if !reflect.DeepEqual(detail, listed[0]) {
		t.Errorf("detail response differs from the list entry for the same record:\ndetail: %v\nlist:   %v", detail, listed[0])
	}
}

func TestGetJobListing_UnknownID_Returns404(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/job-listings/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestSuggestContact_ResearchesAndDoesNotPersist(t *testing.T) {
	dataDir := seedDataDir(t)
	client := &fakeGenerationClient{
		suggestContact: func(ctx context.Context, company, jobDescription string) (tracking.Contact, error) {
			if company != "Acme Corp" {
				t.Errorf("expected company Acme Corp to reach the client, got %q", company)
			}
			return tracking.Contact{Name: "Jane Recruiter", Email: "jane@acme.example"}, nil
		},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: client}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/job-listings/"+id+"/suggest-contact", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var contact map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&contact); err != nil {
		t.Fatal(err)
	}
	if contact["name"] != "Jane Recruiter" || contact["email"] != "jane@acme.example" {
		t.Errorf("expected suggested contact, got %v", contact)
	}

	applicationContent, err := os.ReadFile(filepath.Join(dataDir, "applications", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(applicationContent, []byte("jane@acme.example")) {
		t.Errorf("expected suggestion NOT to be persisted without confirmation, got:\n%s", applicationContent)
	}
}

func TestSuggestContact_UnknownJobListing_Returns404(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/api/job-listings/does-not-exist/suggest-contact", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCreateJobListing_ClientRecordsUsage_LogsToStandaloneUsageLog(t *testing.T) {
	dataDir := seedDataDir(t)
	client := &fakeGenerationClientWithUsage{
		usage: []generation.CallUsage{
			{CallType: "ral_estimation", InputTokens: 10, OutputTokens: 5, EstimatedCostUSD: 0.001},
		},
	}
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: client}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/api/job-listings", map[string]any{
		"company":        "Acme Corp",
		"jobDescription": "Some role.",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	logged, err := tracking.ReadUsageLog(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(logged) != 1 || logged[0].CallType != "ral_estimation" {
		t.Fatalf("expected the standalone usage log to contain the RAL-estimation call, got %+v", logged)
	}
}

func TestReadUsageLog_NoLogYet_ReturnsNil(t *testing.T) {
	dataDir := t.TempDir()
	if got, err := tracking.ReadUsageLog(dataDir); got != nil || err != nil {
		t.Errorf("ReadUsageLog() on a fresh dataDir = %+v, %v, want nil, nil", got, err)
	}
}

func assertNoFilesIn(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files in %s, found %v", dir, entries)
	}
}
