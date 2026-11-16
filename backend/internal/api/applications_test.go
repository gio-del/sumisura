package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gio-del/sumisura/backend/internal/api"
)

func patchJSON(t *testing.T, url string, payload any) *http.Response {
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func saveJobListing(t *testing.T, serverURL, company string) string {
	t.Helper()
	resp := postJSON(t, serverURL+"/api/job-listings", map[string]any{
		"company":        company,
		"jobDescription": "Some role. Salary: €40,000.",
	})
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result["jobListing"].(map[string]any)["id"].(string)
}

func TestUpdateApplicationStatus_AllowedTransition_WritesFileAndReturns200(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "tailoring"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	if application["status"] != "tailoring" {
		t.Errorf("expected status tailoring, got %v", application["status"])
	}

	content, err := os.ReadFile(filepath.Join(dataDir, "applications", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("tailoring")) {
		t.Errorf("expected application file to record status tailoring, got:\n%s", content)
	}
}

func TestUpdateApplicationStatus_AllowedTransition_AppendsStatusHistory(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "tailoring"})
	defer resp.Body.Close()

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	history, ok := application["statusHistory"].([]any)
	if !ok {
		t.Fatalf("expected statusHistory array in response, got %v", application["statusHistory"])
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 statusHistory entries (saved, tailoring), got %d: %v", len(history), history)
	}
	first := history[0].(map[string]any)
	second := history[1].(map[string]any)
	if first["status"] != "saved" {
		t.Errorf("expected first history entry status saved, got %v", first["status"])
	}
	if first["changedAt"] == "" || first["changedAt"] == nil {
		t.Errorf("expected first history entry to carry a non-empty changedAt, got %v", first["changedAt"])
	}
	if second["status"] != "tailoring" {
		t.Errorf("expected second history entry status tailoring, got %v", second["status"])
	}
	if second["changedAt"] == "" || second["changedAt"] == nil {
		t.Errorf("expected second history entry to carry a non-empty changedAt, got %v", second["changedAt"])
	}
}

func TestUpdateApplicationStatus_DisallowedTransition_DoesNotAppendStatusHistory(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "offer"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	content, err := os.ReadFile(filepath.Join(dataDir, "applications", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("statusHistory")) {
		t.Fatalf("expected application file to still carry the initial statusHistory entry, got:\n%s", content)
	}
	if bytes.Contains(content, []byte("status: offer")) {
		t.Errorf("did not expect a rejected transition to appear in the stored application, got:\n%s", content)
	}
}

func TestUpdateApplicationStatus_DisallowedTransition_Returns400AndLeavesFileUnchanged(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "offer"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	content, err := os.ReadFile(filepath.Join(dataDir, "applications", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("saved")) {
		t.Errorf("expected application file to still record status saved, got:\n%s", content)
	}
}

func TestUpdateApplicationStatus_UnknownApplication_Returns404(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp := patchJSON(t, server.URL+"/api/applications/does-not-exist/status", map[string]any{"status": "tailoring"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateApplicationStatus_FullHappyPathToOffer(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	for _, status := range []string{"tailoring", "sent", "interviewing", "offer"} {
		resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": status})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("transitioning to %q: expected 200, got %d", status, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestUpdateApplicationStatus_ReopenFromRejectedToInterviewing_Returns200(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	for _, status := range []string{"tailoring", "sent", "rejected"} {
		resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": status})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("transitioning to %q: expected 200, got %d", status, resp.StatusCode)
		}
		resp.Body.Close()
	}

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "interviewing"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reopening to interviewing: expected 200, got %d", resp.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	if application["status"] != "interviewing" {
		t.Errorf("expected status interviewing, got %v", application["status"])
	}
}

func TestUpdateApplicationStatus_WithdrawnThenReopenToInterviewing_RoundTrips(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "withdrawn"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("withdrawing: expected 200, got %d", resp.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	if application["status"] != "withdrawn" {
		t.Errorf("expected status withdrawn, got %v", application["status"])
	}

	content, err := os.ReadFile(filepath.Join(dataDir, "applications", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("withdrawn")) {
		t.Errorf("expected application file to record status withdrawn, got:\n%s", content)
	}

	reopenResp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "interviewing"})
	defer reopenResp.Body.Close()

	if reopenResp.StatusCode != http.StatusOK {
		t.Fatalf("reopening from withdrawn: expected 200, got %d", reopenResp.StatusCode)
	}

	var reopened map[string]any
	if err := json.NewDecoder(reopenResp.Body).Decode(&reopened); err != nil {
		t.Fatal(err)
	}
	if reopened["status"] != "interviewing" {
		t.Errorf("expected status interviewing, got %v", reopened["status"])
	}
}

func TestUpdateApplicationStatus_WithdrawnToOffer_Returns400(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "withdrawn"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("withdrawing: expected 200, got %d", resp.StatusCode)
	}

	invalidResp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "offer"})
	defer invalidResp.Body.Close()

	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 moving Withdrawn directly to offer, got %d", invalidResp.StatusCode)
	}
}

func TestSaveJobListing_SetsStatusUpdatedAtOnApplicationCreation(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/api/job-listings", map[string]any{
		"company":        "Acme Corp",
		"jobDescription": "Some role. Salary: €40,000.",
	})
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	application := result["application"].(map[string]any)
	statusUpdatedAt, _ := application["statusUpdatedAt"].(string)
	if statusUpdatedAt == "" {
		t.Errorf("expected statusUpdatedAt to be set on Application creation, got %v", application["statusUpdatedAt"])
	}
}

func TestUpdateApplicationStatus_UpdatesStatusUpdatedAtOnEachTransition(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp1 := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "tailoring"})
	var app1 map[string]any
	if err := json.NewDecoder(resp1.Body).Decode(&app1); err != nil {
		t.Fatal(err)
	}
	resp1.Body.Close()
	firstUpdatedAt, _ := app1["statusUpdatedAt"].(string)
	if firstUpdatedAt == "" {
		t.Fatalf("expected statusUpdatedAt to be set after transition, got %v", app1["statusUpdatedAt"])
	}

	time.Sleep(2 * time.Millisecond)

	resp2 := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "sent"})
	var app2 map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&app2); err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	secondUpdatedAt, _ := app2["statusUpdatedAt"].(string)
	if secondUpdatedAt == "" || secondUpdatedAt == firstUpdatedAt {
		t.Errorf("expected statusUpdatedAt to change on a second transition, first=%q second=%q", firstUpdatedAt, secondUpdatedAt)
	}
}

func TestUpdateApplicationStatus_ReopenFromRejected_UpdatesStatusUpdatedAt(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	for _, status := range []string{"tailoring", "sent", "rejected"} {
		patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": status}).Body.Close()
	}

	time.Sleep(2 * time.Millisecond)

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "interviewing"})
	defer resp.Body.Close()
	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	statusUpdatedAt, _ := application["statusUpdatedAt"].(string)
	if statusUpdatedAt == "" {
		t.Errorf("expected statusUpdatedAt to be set on Reopen, got %v", application["statusUpdatedAt"])
	}
}

func TestListJobListings_IncludesIsStaleFalseForFreshApplication(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	saveJobListing(t, server.URL, "Acme Corp")

	resp, err := http.Get(server.URL + "/api/job-listings")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var results []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	application := results[0]["application"].(map[string]any)
	isStale, ok := application["isStale"].(bool)
	if !ok {
		t.Fatalf("expected isStale to be present in the list response, got %v", application["isStale"])
	}
	if isStale {
		t.Errorf("expected a freshly-saved Application (status saved) to not be stale")
	}
}

func TestUpdateApplicationMethod_ValidCorrection_WritesFileAndReturns200(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/method", map[string]any{
		"kind":  "portal",
		"value": "https://acme.example/apply",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	method := application["method"].(map[string]any)
	if method["kind"] != "portal" || method["value"] != "https://acme.example/apply" {
		t.Errorf("expected corrected method, got %v", method)
	}

	content, err := os.ReadFile(filepath.Join(dataDir, "applications", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("portal")) {
		t.Errorf("expected application file to record the corrected method, got:\n%s", content)
	}
}

func TestUpdateApplicationMethod_UnknownKind_Returns400(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/method", map[string]any{"kind": "carrier-pigeon"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUpdateApplicationMethod_UnknownApplication_Returns404(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp := patchJSON(t, server.URL+"/api/applications/does-not-exist/method", map[string]any{"kind": "portal"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRecordApplicationGeneration_AppendsHistoryAcrossRegenerates(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp1 := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":   "acme-corp",
		"cvPath": "output/acme-corp/cv.pdf",
	})
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp1.StatusCode)
	}

	resp2 := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":            "acme-corp",
		"cvPath":          "output/acme-corp/cv.pdf",
		"coverLetterPath": "output/acme-corp/cover-letter.pdf",
	})
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp2.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	generations, ok := application["generations"].([]any)
	if !ok || len(generations) != 2 {
		t.Fatalf("expected regenerating to append rather than overwrite (2 records), got %v", application["generations"])
	}
	latest := generations[1].(map[string]any)
	if latest["coverLetterPath"] != "output/acme-corp/cover-letter.pdf" {
		t.Errorf("expected latest record's coverLetterPath, got %v", latest)
	}
}

func TestRecordApplicationGeneration_PersistsEntryIDsFromRequest(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":     "acme-corp",
		"cvPath":   "output/acme-corp/cv.pdf",
		"entryIds": []string{"experience/example-client-a", "projects/emall"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	generations, ok := application["generations"].([]any)
	if !ok || len(generations) != 1 {
		t.Fatalf("expected 1 record, got %v", application["generations"])
	}
	record := generations[0].(map[string]any)
	entryIDs, ok := record["entryIds"].([]any)
	if !ok || len(entryIDs) != 2 || entryIDs[0] != "experience/example-client-a" || entryIDs[1] != "projects/emall" {
		t.Fatalf("expected entryIds to persist as recorded, got %v", record["entryIds"])
	}
}

func TestRecordApplicationGeneration_NoEntryIDs_OmitsField(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":   "acme-corp",
		"cvPath": "output/acme-corp/cv.pdf",
	})
	defer resp.Body.Close()

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	generations := application["generations"].([]any)
	record := generations[0].(map[string]any)
	if _, present := record["entryIds"]; present {
		t.Errorf("expected no entryIds field for a request that supplied none, got %v", record["entryIds"])
	}
}

func TestRecordApplicationGeneration_PersistsSourceSnippetIDs(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":             "acme-corp",
		"cvPath":           "output/acme-corp/cv.pdf",
		"coverLetterPath":  "output/acme-corp/cover-letter.pdf",
		"sourceSnippetIds": []string{"opener-1", "closer-2"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	generations := application["generations"].([]any)
	latest := generations[len(generations)-1].(map[string]any)
	ids, ok := latest["sourceSnippetIds"].([]any)
	if !ok || len(ids) != 2 || ids[0] != "opener-1" || ids[1] != "closer-2" {
		t.Fatalf("expected sourceSnippetIds [opener-1 closer-2] on the recorded Generation, got %v", latest["sourceSnippetIds"])
	}
}

func TestRecordApplicationGeneration_PersistsUsage(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":   "acme-corp",
		"cvPath": "output/acme-corp/cv.pdf",
		"usage": map[string]any{
			"inputTokens":      1000,
			"outputTokens":     200,
			"estimatedCostUsd": 0.012,
			"calls": []map[string]any{
				{"callType": "selection_rewrite", "inputTokens": 1000, "outputTokens": 200, "estimatedCostUsd": 0.012},
			},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	generations := application["generations"].([]any)
	record := generations[0].(map[string]any)
	usage, ok := record["usage"].(map[string]any)
	if !ok {
		t.Fatalf("expected the persisted record to include usage, got %v", record)
	}
	if usage["estimatedCostUsd"] != 0.012 {
		t.Errorf("usage.estimatedCostUsd = %v, want 0.012", usage["estimatedCostUsd"])
	}
}

func TestRecordApplicationGeneration_PersistsLanguage(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":     "acme-corp",
		"cvPath":   "output/acme-corp/cv.pdf",
		"language": "it",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	generations := application["generations"].([]any)
	latest := generations[len(generations)-1].(map[string]any)
	if latest["language"] != "it" {
		t.Errorf("expected the Generation record to persist the target language, got %v", latest)
	}
}

func TestRecordApplicationGeneration_WithGroundedness_PersistsIt(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{
		"slug":   "acme-corp",
		"cvPath": "output/acme-corp/cv.pdf",
		"groundedness": map[string]any{
			"bullets": []map[string]any{
				{
					"entryId":     "experience/example-client-a",
					"sourceIndex": 0,
					"flags": []map[string]any{
						{"sentence": "Fabricated claim.", "reason": "no-source-match"},
					},
				},
			},
		},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	generations := application["generations"].([]any)
	record := generations[0].(map[string]any)
	groundedness, ok := record["groundedness"].(map[string]any)
	if !ok {
		t.Fatalf("expected groundedness to persist on the record, got %v", record)
	}
	bullets := groundedness["bullets"].([]any)
	if len(bullets) != 1 {
		t.Fatalf("expected 1 flagged bullet to round-trip, got %v", groundedness["bullets"])
	}
}

func TestRecordApplicationGeneration_MissingSlug_Returns400(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/generations", map[string]any{"cvPath": "output/x/cv.pdf"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRecordApplicationGeneration_UnknownApplication_Returns404(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/api/applications/does-not-exist/generations", map[string]any{
		"slug":   "acme-corp",
		"cvPath": "output/acme-corp/cv.pdf",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAddApplicationContact_ValidPayload_WritesFileAndReturns201(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/contacts", map[string]any{
		"name":  "Jane Recruiter",
		"email": "jane@acme.example",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	contact := application["contacts"].([]any)[0].(map[string]any)
	if contact["name"] != "Jane Recruiter" || contact["email"] != "jane@acme.example" || contact["id"] == "" {
		t.Errorf("expected saved contact, got %v", contact)
	}

	content, err := os.ReadFile(filepath.Join(dataDir, "applications", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("jane@acme.example")) {
		t.Errorf("expected application file to record the contact, got:\n%s", content)
	}
}

func TestAddApplicationContact_MissingEmail_Returns400(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := postJSON(t, server.URL+"/api/applications/"+id+"/contacts", map[string]any{"name": "Jane Recruiter"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAddApplicationContact_UnknownApplication_Returns404(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp := postJSON(t, server.URL+"/api/applications/does-not-exist/contacts", map[string]any{
		"name":  "Jane Recruiter",
		"email": "jane@acme.example",
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetApplicationMailto_WithConfirmedContact_ReturnsURI(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	postJSON(t, server.URL+"/api/applications/"+id+"/contacts", map[string]any{
		"name":  "Jane Recruiter",
		"email": "jane@acme.example",
	}).Body.Close()

	resp, err := http.Get(server.URL + "/api/applications/" + id + "/mailto")
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
	uri, ok := result["uri"].(string)
	if !ok || !strings.HasPrefix(uri, "mailto:jane@acme.example?") {
		t.Errorf("expected a mailto URI for jane@acme.example, got %v", result["uri"])
	}
}

func TestGetApplicationMailto_NoConfirmedContact_Returns400(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp, err := http.Get(server.URL + "/api/applications/" + id + "/mailto")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGetApplicationMailto_UnknownApplication_Returns404(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/applications/does-not-exist/mailto")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetApplicationsStats_ReturnsAggregatedShape(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	saveJobListing(t, server.URL, "Acme Corp")

	resp, err := http.Get(server.URL + "/api/applications/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var stats map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats["total"].(float64) != 1 {
		t.Errorf("expected total 1, got %v", stats["total"])
	}
	if _, ok := stats["counts"].([]any); !ok {
		t.Errorf("expected counts array in response, got %v", stats["counts"])
	}
}

func TestGetApplicationsStats_NoApplications_Returns200WithZeroTotal(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/applications/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var stats map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats["total"].(float64) != 0 {
		t.Errorf("expected total 0, got %v", stats["total"])
	}
}

func TestApplicationContacts_AddEditDeleteAndPickForMailto(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()
	id := saveJobListing(t, server.URL, "Acme Corp")
	base := server.URL + "/api/applications/" + id

	contactsOf := func(resp *http.Response) []map[string]any {
		t.Helper()
		defer resp.Body.Close()
		var application struct {
			Contacts []map[string]any `json:"contacts"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
			t.Fatal(err)
		}
		return application.Contacts
	}
	mailto := func(query string) string {
		t.Helper()
		resp, err := http.Get(base + "/mailto" + query)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("mailto%s: expected 200, got %d", query, resp.StatusCode)
		}
		var out map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out["uri"]
	}

	contactsOf(postJSON(t, base+"/contacts", map[string]any{"name": "Boss", "email": "boss@acme.example", "role": "manager"}))
	contacts := contactsOf(postJSON(t, base+"/contacts", map[string]any{"name": "Rita", "email": "rita@acme.example", "role": "recruiter"}))
	if len(contacts) != 2 {
		t.Fatalf("expected 2 contacts, got %v", contacts)
	}
	bossID, ritaID := contacts[0]["id"].(string), contacts[1]["id"].(string)

	if uri := mailto(""); !strings.HasPrefix(uri, "mailto:rita@acme.example?") {
		t.Errorf("default should be the first recruiter, got %s", uri)
	}
	if uri := mailto("?contactId=" + bossID); !strings.HasPrefix(uri, "mailto:boss@acme.example?") {
		t.Errorf("chosen contact expected, got %s", uri)
	}
	resp, err := http.Get(base + "/mailto?contactId=nope")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown contact: expected 404, got %d", resp.StatusCode)
	}

	contacts = contactsOf(patchJSON(t, base+"/contacts/"+bossID, map[string]any{"name": "Boss", "email": "new-boss@acme.example", "role": "manager"}))
	if len(contacts) != 2 || contacts[0]["email"] != "new-boss@acme.example" || contacts[0]["id"] != bossID {
		t.Errorf("expected the first contact edited in place, got %v", contacts)
	}
	if resp := patchJSON(t, base+"/contacts/nope", map[string]any{"email": "x@y.example"}); resp.StatusCode != http.StatusNotFound {
		t.Errorf("edit unknown: expected 404, got %d", resp.StatusCode)
	}
	if resp := patchJSON(t, base+"/contacts/"+bossID, map[string]any{"email": "x@y.example", "role": "ceo"}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad role: expected 400, got %d", resp.StatusCode)
	}

	delResp := doJSON(t, http.MethodDelete, base+"/contacts/"+ritaID, nil, "")
	contacts = contactsOf(delResp)
	if len(contacts) != 1 || contacts[0]["id"] != bossID {
		t.Errorf("expected only the manager left, got %v", contacts)
	}
	if uri := mailto(""); !strings.HasPrefix(uri, "mailto:new-boss@acme.example?") {
		t.Errorf("with no recruiter the first contact is the default, got %s", uri)
	}
}
