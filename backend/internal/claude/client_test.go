package claude

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gio-del/sumisura/backend/internal/generation"
)

// fakeAnthropicServer returns an httptest.Server that answers every
// /v1/messages call with body (a canned Messages API response), regardless
// of the request — enough to test what Client does with a response's
// usage, without needing a real API key or network access.
func fakeAnthropicServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

const selectAndRewriteResponse = `{
	"id": "msg_1",
	"type": "message",
	"role": "assistant",
	"model": "claude-sonnet-5",
	"stop_reason": "tool_use",
	"content": [
		{
			"type": "tool_use",
			"id": "toolu_1",
			"name": "select_and_rewrite",
			"input": {"entries": []}
		}
	],
	"usage": {
		"input_tokens": 1000,
		"output_tokens": 200,
		"cache_creation_input_tokens": 0,
		"cache_read_input_tokens": 0
	}
}`

func TestSelectAndRewrite_RecordsUsage(t *testing.T) {
	clearModelEnv(t)
	server := fakeAnthropicServer(t, selectAndRewriteResponse)
	c := NewWithOptions(option.WithBaseURL(server.URL), option.WithAPIKey("test-key"))

	_, err := c.SelectAndRewrite(context.Background(), generation.SelectionRequest{JobDescription: "a role"})
	if err != nil {
		t.Fatalf("SelectAndRewrite() error = %v", err)
	}

	calls := c.DrainUsage()
	if len(calls) != 1 {
		t.Fatalf("DrainUsage() = %d calls, want 1", len(calls))
	}
	got := calls[0]
	if got.CallType != "selection_rewrite" {
		t.Errorf("CallType = %q, want %q", got.CallType, "selection_rewrite")
	}
	if got.InputTokens != 1000 || got.OutputTokens != 200 {
		t.Errorf("tokens = %d/%d, want 1000/200", got.InputTokens, got.OutputTokens)
	}
	if got.EstimatedCostUSD <= 0 {
		t.Errorf("EstimatedCostUSD = %v, want > 0", got.EstimatedCostUSD)
	}

	// Draining again must not resurrect the same call.
	if again := c.DrainUsage(); len(again) != 0 {
		t.Errorf("second DrainUsage() = %+v, want empty", again)
	}
}

// The Selection preview is a real, billed Claude call, so its usage must
// be recorded like every sibling method's.
func TestSelectOnly_RecordsSelectionPreviewUsage(t *testing.T) {
	clearModelEnv(t)
	server, _ := recordingAnthropicServer(t)
	c := NewWithOptions(option.WithBaseURL(server.URL), option.WithAPIKey("test-key"))

	if _, err := c.SelectOnly(context.Background(), generation.SelectionRequest{JobDescription: "a role"}); err != nil {
		t.Fatalf("SelectOnly() error = %v", err)
	}

	calls := c.DrainUsage()
	if len(calls) != 1 {
		t.Fatalf("DrainUsage() = %d calls, want 1", len(calls))
	}
	got := calls[0]
	if got.CallType != "selection_preview" {
		t.Errorf("CallType = %q, want %q", got.CallType, "selection_preview")
	}
	if got.Model != "claude-sonnet-5" {
		t.Errorf("Model = %q, want %q", got.Model, "claude-sonnet-5")
	}
	if got.EstimatedCostUSD <= 0 {
		t.Errorf("EstimatedCostUSD = %v, want > 0", got.EstimatedCostUSD)
	}
}

func TestDrainUsage_NothingRecordedYet_ReturnsEmpty(t *testing.T) {
	c := New()
	if got := c.DrainUsage(); len(got) != 0 {
		t.Errorf("DrainUsage() on a fresh Client = %+v, want empty", got)
	}
}

// promptRecordingServer answers like fakeAnthropicServer but keeps the full
// request bodies, so a test can assert on what was actually sent.
func promptRecordingServer(t *testing.T, toolName string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"id": "msg_1", "type": "message", "role": "assistant",
			"model": "claude-sonnet-5", "stop_reason": "tool_use",
			"content": [{"type": "tool_use", "id": "toolu_1", "name": %q, "input": {"body": "Dear ..."}}],
			"usage": {"input_tokens": 10, "output_tokens": 5}
		}`, toolName)
	}))
	t.Cleanup(server.Close)
	return server, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), bodies...)
	}
}

// The target language is resolved by the generation service and carried on
// CoverLetterRequest, but it used to be dropped on the floor here: the
// prompt never mentioned it, so the letter came out in whatever language
// the model inferred from the Job Description, ignoring an explicit
// override (issue #168).
func TestDraftCoverLetter_SendsTheTargetLanguage(t *testing.T) {
	clearModelEnv(t)
	server, sent := promptRecordingServer(t, "draft_cover_letter")
	c := NewWithOptions(option.WithBaseURL(server.URL), option.WithAPIKey("test-key"))

	if _, err := c.DraftCoverLetter(context.Background(), generation.CoverLetterRequest{
		JobDescription: "Un annuncio in italiano.",
		Language:       "it",
	}); err != nil {
		t.Fatalf("DraftCoverLetter() error = %v", err)
	}

	bodies := sent()
	if len(bodies) != 1 {
		t.Fatalf("expected 1 request, got %d", len(bodies))
	}
	if !strings.Contains(bodies[0], "Target language (ISO 639-1): it") {
		t.Errorf("the request must state the resolved target language, got:\n%s", bodies[0])
	}
}

// A Snippet's own language reaches the model, so it can prefer one already
// written in the target language over translating the user's prose.
func TestDraftCoverLetter_SendsEachSnippetsLanguage(t *testing.T) {
	clearModelEnv(t)
	server, sent := promptRecordingServer(t, "draft_cover_letter")
	c := NewWithOptions(option.WithBaseURL(server.URL), option.WithAPIKey("test-key"))

	if _, err := c.DraftCoverLetter(context.Background(), generation.CoverLetterRequest{
		JobDescription: "Un annuncio in italiano.",
		Language:       "it",
		Snippets: []generation.CandidateSnippet{
			{ID: "opening-it", Kind: "opening", Lang: "it", Body: "Vi scrivo perché…"},
			{ID: "opening-en", Kind: "opening", Lang: "en", Body: "I am writing because…"},
		},
	}); err != nil {
		t.Fatalf("DraftCoverLetter() error = %v", err)
	}

	body := sent()[0]
	for _, want := range []string{`\"lang\":\"it\"`, `\"lang\":\"en\"`} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %s in the request, got:\n%s", want, body)
		}
	}
}
