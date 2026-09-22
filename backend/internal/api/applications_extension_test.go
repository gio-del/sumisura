package api_test

import (
	"net/http"
	"testing"
)

// The card moves an Application's Status from the job board (issue #206,
// stories 43-46). That means the extension calls PATCH
// /api/applications/{id}/status cross-origin, so it needs the same CORS
// headers and preflight the capture and lookup routes carry — and the
// existing transition validation stays authoritative.

func statusPath(s lookupServer, id string) string {
	return s.url + "/api/applications/" + id + "/status"
}

func TestUpdateApplicationStatus_CarriesCORSHeadersAndAnswersPreflight(t *testing.T) {
	s := newLookupServer(t)
	id := s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")

	moved := patchJSON(t, statusPath(s, id), map[string]any{"status": "tailoring"})
	defer moved.Body.Close()
	if moved.StatusCode != http.StatusOK {
		t.Fatalf("expected the move to succeed, got %d", moved.StatusCode)
	}
	if got := moved.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected the response to carry CORS, got %q", got)
	}

	req, err := http.NewRequest(http.MethodOptions, statusPath(s, id), nil)
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

// Story 45: a stale card can't corrupt anything. The card offers only what
// the lookup called legal, but the backend is what decides.
func TestUpdateApplicationStatus_IllegalMove_IsRefusedWhateverTheClientOffered(t *testing.T) {
	s := newLookupServer(t)
	id := s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")

	// Saved -> Sent skips Tailoring, and is not in the state machine.
	refused := patchJSON(t, statusPath(s, id), map[string]any{"status": "sent"})
	defer refused.Body.Close()
	if refused.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected an illegal move refused with 400, got %d", refused.StatusCode)
	}

	body := s.application(t, id)
	if body["status"] != "saved" {
		t.Errorf("expected the Status untouched, got %v", body["status"])
	}
}

// Story 46: the card reflects the new Status straight away, which means
// the response has to carry it rather than only an acknowledgement.
func TestUpdateApplicationStatus_Response_CarriesTheNewStatusAndItsNextMoves(t *testing.T) {
	s := newLookupServer(t)
	id := s.save(t, "Acme", "Backend Engineer", "https://www.linkedin.com/jobs/view/4012345678/")

	moved := patchJSON(t, statusPath(s, id), map[string]any{"status": "tailoring"})
	defer moved.Body.Close()
	if got := decodeBody(t, moved)["status"]; got != "tailoring" {
		t.Errorf("expected the new Status on the response, got %v", got)
	}

	// The card re-reads the legal moves from the lookup, so they must
	// follow the Status it just set.
	body := s.lookup(t, map[string]any{"url": "https://www.linkedin.com/jobs/view/4012345678/", "company": "Acme"})
	tracked := body["tracked"].(map[string]any)
	if tracked["status"] != "tailoring" {
		t.Fatalf("expected the lookup to report the new Status, got %v", tracked["status"])
	}
	if got := stringsOf(t, tracked["allowedTransitions"]); !equalStringSlices(got, []string{"sent", "withdrawn"}) {
		t.Errorf("expected the moves legal from tailoring, got %v", got)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
