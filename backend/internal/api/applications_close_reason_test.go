package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/api"
)

func decodeCloseReasonApp(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var application map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		t.Fatal(err)
	}
	return application
}

func moveTo(t *testing.T, base, id string, extra map[string]any, statuses ...string) {
	t.Helper()
	for _, status := range statuses {
		payload := map[string]any{"status": status}
		for k, v := range extra {
			payload[k] = v
		}
		resp := patchJSON(t, base+"/api/applications/"+id+"/status", payload)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("moving to %q: expected 200, got %d", status, resp.StatusCode)
		}
	}
}

func TestUpdateApplicationStatus_CloseReason_StoredWithRejectedAndWithdrawn(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	moveTo(t, server.URL, id, nil, "tailoring", "sent")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "rejected", "closeReason": "no-response"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := decodeCloseReasonApp(t, resp)["closeReason"]; got != "no-response" {
		t.Errorf("expected closeReason no-response, got %v", got)
	}

	// It survives a read from disk.
	list, err := http.Get(server.URL + "/api/applications")
	if err != nil {
		t.Fatal(err)
	}
	defer list.Body.Close()
	var items []struct {
		Application map[string]any `json:"application"`
	}
	if err := json.NewDecoder(list.Body).Decode(&items); err == nil {
		for _, item := range items {
			if item.Application["id"] == id && item.Application["closeReason"] != "no-response" {
				t.Errorf("expected the persisted closeReason, got %v", item.Application["closeReason"])
			}
		}
	}
}

func TestUpdateApplicationStatus_CloseReason_WithNonTerminalStatus_Returns400(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "tailoring", "closeReason": "other"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	// The refused move changed nothing.
	moveTo(t, server.URL, id, nil, "tailoring")
}

func TestUpdateApplicationStatus_CloseReason_UnknownValue_Returns400(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	moveTo(t, server.URL, id, nil, "withdrawn")
	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/close-reason", map[string]any{"closeReason": "bored"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUpdateApplicationStatus_ReopeningClearsCloseReason(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	moveTo(t, server.URL, id, nil, "tailoring", "sent")
	moveTo(t, server.URL, id, map[string]any{"closeReason": "rejected-after-screening"}, "rejected")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "interviewing"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got, present := decodeCloseReasonApp(t, resp)["closeReason"]; present {
		t.Errorf("expected closeReason cleared, got %v", got)
	}
}

func TestUpdateApplicationCloseReason_SetsAfterTheStatusChange(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	moveTo(t, server.URL, id, nil, "withdrawn")

	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/close-reason", map[string]any{"closeReason": "salary-mismatch"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := decodeCloseReasonApp(t, resp)["closeReason"]; got != "salary-mismatch" {
		t.Errorf("expected salary-mismatch, got %v", got)
	}

	resp = patchJSON(t, server.URL+"/api/applications/"+id+"/close-reason", map[string]any{"closeReason": ""})
	if got, present := decodeCloseReasonApp(t, resp)["closeReason"]; present {
		t.Errorf("expected closeReason cleared, got %v", got)
	}
}

func TestUpdateApplicationCloseReason_OnOpenApplication_Returns400(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	resp := patchJSON(t, server.URL+"/api/applications/"+id+"/close-reason", map[string]any{"closeReason": "other"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUpdateApplicationCloseReason_WithStaleVersion_Returns409(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	moveTo(t, server.URL, id, nil, "withdrawn")
	_, version := listingVersions(t, server.URL, id)
	other := patchJSON(t, server.URL+"/api/applications/"+id+"/close-reason", map[string]any{"closeReason": "position-closed"}) // the record changes under the stale holder
	other.Body.Close()

	resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/close-reason", map[string]any{"closeReason": "other"}, version)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestUpdateApplicationStatus_CloseReason_WithStaleVersion_Returns409(t *testing.T) {
	dataDir := seedDataDir(t)
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DataDir: dataDir, GenerationClient: &fakeGenerationClient{}}))
	defer server.Close()

	id := saveJobListing(t, server.URL, "Acme Corp")
	_, version := listingVersions(t, server.URL, id)
	moveTo(t, server.URL, id, nil, "withdrawn")

	resp := doJSON(t, http.MethodPatch, server.URL+"/api/applications/"+id+"/status", map[string]any{"status": "rejected", "closeReason": "other"}, version)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}
