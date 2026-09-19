package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gio-del/sumisura/backend/internal/generation"
)

// capturedRequest is the part of a Messages API request body the
// per-call-site model tests assert on.
type capturedRequest struct {
	Model      string `json:"model"`
	ToolChoice *struct {
		Name string `json:"name"`
	} `json:"tool_choice"`
}

// recordingAnthropicServer is fakeAnthropicServer's request-capturing
// variant: it decodes every /v1/messages request body, in order, and
// answers each with a response shaped for that request — a tool_use block
// for the forced tool when tool_choice names one, plain text otherwise (the
// research half of a two-call method) — echoing the requested model back
// as the model that ran.
func recordingAnthropicServer(t *testing.T) (*httptest.Server, func() []capturedRequest) {
	t.Helper()
	var mu sync.Mutex
	var captured []capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req capturedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		captured = append(captured, req)
		mu.Unlock()

		content := `{"type": "text", "text": "research notes"}`
		stopReason := "end_turn"
		if req.ToolChoice != nil && req.ToolChoice.Name != "" {
			content = fmt.Sprintf(`{"type": "tool_use", "id": "toolu_1", "name": %q, "input": {}}`, req.ToolChoice.Name)
			stopReason = "tool_use"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"id": "msg_1", "type": "message", "role": "assistant",
			"model": %q, "stop_reason": %q, "content": [%s],
			"usage": {"input_tokens": 100, "output_tokens": 10}
		}`, req.Model, stopReason, content)
	}))
	t.Cleanup(server.Close)
	return server, func() []capturedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]capturedRequest(nil), captured...)
	}
}

// callSiteInvocation drives one Client method and names, in request order,
// the call site each of its requests belongs to — two for the
// research+extraction methods (ADR-0011).
type callSiteInvocation struct {
	method string
	sites  []callSite
	call   func(context.Context, *Client) error
}

var callSiteInvocations = []callSiteInvocation{
	{"SelectAndRewrite", []callSite{callSiteSelectionRewrite}, func(ctx context.Context, c *Client) error {
		_, err := c.SelectAndRewrite(ctx, generation.SelectionRequest{JobDescription: "a role"})
		return err
	}},
	{"SelectOnly", []callSite{callSiteSelectionPreview}, func(ctx context.Context, c *Client) error {
		_, err := c.SelectOnly(ctx, generation.SelectionRequest{JobDescription: "a role"})
		return err
	}},
	{"DraftCoverLetter", []callSite{callSiteCoverLetter}, func(ctx context.Context, c *Client) error {
		_, err := c.DraftCoverLetter(ctx, generation.CoverLetterRequest{JobDescription: "a role"})
		return err
	}},
	{"EstimateRAL", []callSite{callSiteRALResearch, callSiteRALExtraction}, func(ctx context.Context, c *Client) error {
		_, err := c.EstimateRAL(ctx, "a role")
		return err
	}},
	{"InferApplicationMethod", []callSite{callSiteApplicationMethodInference}, func(ctx context.Context, c *Client) error {
		_, err := c.InferApplicationMethod(ctx, "a role")
		return err
	}},
	{"SuggestContact", []callSite{callSiteContactResearch, callSiteContactExtraction}, func(ctx context.Context, c *Client) error {
		_, err := c.SuggestContact(ctx, "Acme", "a role")
		return err
	}},
	{"SuggestCaptureHints", []callSite{callSiteCaptureHints}, func(ctx context.Context, c *Client) error {
		_, err := c.SuggestCaptureHints(ctx, "a role")
		return err
	}},
}

// sentModels invokes inv against a fresh recording server with a Client
// built by NewWithOptions, returning the model each request sent, in order.
func sentModels(t *testing.T, inv callSiteInvocation) []string {
	t.Helper()
	server, captured := recordingAnthropicServer(t)
	c := NewWithOptions(option.WithBaseURL(server.URL), option.WithAPIKey("test-key"))
	if err := inv.call(context.Background(), c); err != nil {
		t.Fatalf("%s() error = %v", inv.method, err)
	}
	var models []string
	for _, req := range captured() {
		models = append(models, req.Model)
	}
	return models
}

// documentedDefaults is the per-call-site default table from issue #101,
// written out independently of defaultModels so the test checks the wire
// against the documented policy rather than against the map itself.
var documentedDefaults = map[callSite]anthropic.Model{
	callSiteSelectionRewrite:           anthropic.ModelClaudeSonnet5,
	callSiteSelectionPreview:           anthropic.ModelClaudeSonnet5,
	callSiteCoverLetter:                anthropic.ModelClaudeSonnet5,
	callSiteRALResearch:                anthropic.ModelClaudeSonnet5,
	callSiteRALExtraction:              anthropic.ModelClaudeHaiku4_5,
	callSiteApplicationMethodInference: anthropic.ModelClaudeHaiku4_5,
	callSiteContactResearch:            anthropic.ModelClaudeSonnet5,
	callSiteContactExtraction:          anthropic.ModelClaudeHaiku4_5,
	callSiteCaptureHints:               anthropic.ModelClaudeHaiku4_5,
}

func TestEveryCallSite_SendsItsDocumentedDefaultModel(t *testing.T) {
	clearModelEnv(t)
	covered := map[callSite]bool{}
	for _, inv := range callSiteInvocations {
		var want []string
		for _, site := range inv.sites {
			want = append(want, string(documentedDefaults[site]))
			covered[site] = true
		}
		t.Run(inv.method, func(t *testing.T) {
			if got := sentModels(t, inv); !slices.Equal(got, want) {
				t.Errorf("%s sent models %v, want %v", inv.method, got, want)
			}
		})
	}
	if len(documentedDefaults) != len(defaultModels) {
		t.Errorf("documentedDefaults has %d call sites, defaultModels has %d", len(documentedDefaults), len(defaultModels))
	}
	for site := range defaultModels {
		if !covered[site] {
			t.Errorf("call site %q has a default model but no entry in callSiteInvocations", site)
		}
	}
}

// Every model a call site can run on by default must be priced, or that
// call's cost silently estimates to $0 (issue #39). Opus 5 is the
// documented override target, so it is held to the same bar; the dated
// snapshot ids the SDK exports for those aliases must resolve too, since
// the API may report one of those as the model that ran.
func TestPricingTable_CoversEveryReachableModel(t *testing.T) {
	reachable := []anthropic.Model{anthropic.ModelClaudeOpus5, anthropic.ModelClaudeHaiku4_5_20251001}
	for _, model := range defaultModels {
		reachable = append(reachable, model)
	}
	for _, model := range reachable {
		if _, ok := lookupPricing(model); !ok {
			t.Errorf("model %q is reachable but has no pricingTable entry", model)
		}
	}
}

// clearModelEnv blanks every model override variable for t's duration, so
// a SUMISURA_MODEL_* exported in the developer's shell can't leak into
// a test (an empty value means unset).
func clearModelEnv(t *testing.T) {
	t.Helper()
	t.Setenv(defaultModelEnvVar, "")
	for site := range defaultModels {
		t.Setenv(modelEnvVar(site), "")
	}
}

func invocation(t *testing.T, method string) callSiteInvocation {
	t.Helper()
	for _, inv := range callSiteInvocations {
		if inv.method == method {
			return inv
		}
	}
	t.Fatalf("no callSiteInvocation for %s", method)
	return callSiteInvocation{}
}

func TestModelEnvVar_Names(t *testing.T) {
	if got, want := modelEnvVar(callSiteSelectionRewrite), "SUMISURA_MODEL_SELECTION_REWRITE"; got != want {
		t.Errorf("modelEnvVar(selection_rewrite) = %q, want %q", got, want)
	}
	if got, want := defaultModelEnvVar, "SUMISURA_MODEL_DEFAULT"; got != want {
		t.Errorf("defaultModelEnvVar = %q, want %q", got, want)
	}
}

func TestPerCallSiteOverride_ChangesOnlyThatCallSite(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("SUMISURA_MODEL_RAL_EXTRACTION", "claude-opus-5")

	if got, want := sentModels(t, invocation(t, "EstimateRAL")), []string{"claude-sonnet-5", "claude-opus-5"}; !slices.Equal(got, want) {
		t.Errorf("EstimateRAL sent %v, want %v", got, want)
	}
	if got, want := sentModels(t, invocation(t, "SuggestContact")), []string{"claude-sonnet-5", "claude-haiku-4-5"}; !slices.Equal(got, want) {
		t.Errorf("SuggestContact sent %v, want %v (neighbouring extraction call site must be untouched)", got, want)
	}
}

// The blanket override moving every call site is also what catches a call
// site that bypasses the registry and hardcodes a model.
func TestBlanketOverride_MovesEveryCallSite(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("SUMISURA_MODEL_DEFAULT", "claude-opus-5")

	for _, inv := range callSiteInvocations {
		t.Run(inv.method, func(t *testing.T) {
			for i, got := range sentModels(t, inv) {
				if got != "claude-opus-5" {
					t.Errorf("%s request %d sent %q, want claude-opus-5", inv.method, i, got)
				}
			}
		})
	}
}

func TestPerCallSiteOverride_WinsOverBlanketOverride(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("SUMISURA_MODEL_DEFAULT", "claude-haiku-4-5")
	t.Setenv("SUMISURA_MODEL_SELECTION_REWRITE", "claude-opus-5")

	if got, want := sentModels(t, invocation(t, "SelectAndRewrite")), []string{"claude-opus-5"}; !slices.Equal(got, want) {
		t.Errorf("SelectAndRewrite sent %v, want %v", got, want)
	}
	if got, want := sentModels(t, invocation(t, "DraftCoverLetter")), []string{"claude-haiku-4-5"}; !slices.Equal(got, want) {
		t.Errorf("DraftCoverLetter sent %v, want %v", got, want)
	}
}

func TestEmptyOverride_FallsBackToBuiltInDefault(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("SUMISURA_MODEL_DEFAULT", "   ")
	t.Setenv("SUMISURA_MODEL_APPLICATION_METHOD_INFERENCE", "")

	if got, want := sentModels(t, invocation(t, "InferApplicationMethod")), []string{"claude-haiku-4-5"}; !slices.Equal(got, want) {
		t.Errorf("InferApplicationMethod sent %v, want %v", got, want)
	}
}

func TestUnpricedOverride_ConstructsClientAndWarns(t *testing.T) {
	clearModelEnv(t)
	t.Setenv("SUMISURA_MODEL_COVER_LETTER", "claude-unpriced-override-test")
	var buf bytes.Buffer
	prevOut := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prevOut) })

	if c := New(); c == nil {
		t.Fatal("New() = nil, want a Client even with an unpriced override")
	}

	if !strings.Contains(buf.String(), "claude-unpriced-override-test") {
		t.Errorf("log output = %q, want a warning naming the unpriced model", buf.String())
	}
}
