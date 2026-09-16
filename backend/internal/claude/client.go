// Package claude is the real generation.Client implementation: it calls the
// Claude API directly, per ADR-0005. Every method forces a tool call so the
// response is structured, then decodes that tool call's input into the
// generation package's result types.
package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// Client wraps the Anthropic SDK client with the prompts and tool schemas
// the generation pipeline needs. It reads ANTHROPIC_API_KEY from the
// environment (via the SDK's default option), so constructing one never
// fails — only a call that actually reaches the API can, if the key is
// missing or invalid.
//
// Client also implements generation.UsageRecorder: every method appends
// the usage/cost of its own Claude API call(s) to usageMu-guarded
// usageCalls, drained via DrainUsage. This accumulator is per-Client, not
// per-request — correct for this app's single-user, sequential call
// pattern, but two overlapping Generate calls sharing the same *Client
// could interleave each other's usage into whichever drains first. Not a
// concern in practice for a localhost, no-auth personal tool (ADR
// context), but worth knowing if that assumption ever changes.
type Client struct {
	api    anthropic.Client
	models map[callSite]anthropic.Model

	usageMu    sync.Mutex
	usageCalls []generation.CallUsage
}

// New builds a Client using ANTHROPIC_API_KEY from the environment.
func New() *Client {
	return &Client{api: anthropic.NewClient(), models: resolveModels()}
}

// NewWithOptions builds a Client with explicit SDK request options (tests
// pointing at a fake server, etc). Like New, it takes each call site's
// model from defaultModels and the SUMISURA_MODEL_* environment (see
// resolveModels).
func NewWithOptions(opts ...option.RequestOption) *Client {
	return &Client{api: anthropic.NewClient(opts...), models: resolveModels()}
}

var _ generation.Client = (*Client)(nil)
var _ tracking.Client = (*Client)(nil)
var _ generation.UsageRecorder = (*Client)(nil)

// DrainUsage implements generation.UsageRecorder.
func (c *Client) DrainUsage() []generation.CallUsage {
	c.usageMu.Lock()
	defer c.usageMu.Unlock()
	drained := c.usageCalls
	c.usageCalls = nil
	return drained
}

// recordUsage best-effort appends one call's usage to the accumulator —
// usage capture must never fail or block the call it's recording (PRD
// story 9), so this cannot error.
func (c *Client) recordUsage(callType string, model anthropic.Model, usage anthropic.Usage, webSearchUses int64) {
	call := generation.CallUsage{
		CallType:         callType,
		Model:            string(model),
		InputTokens:      usage.InputTokens,
		OutputTokens:     usage.OutputTokens,
		CacheReadTokens:  usage.CacheReadInputTokens,
		CacheWriteTokens: usage.CacheCreationInputTokens,
		WebSearchUses:    int(webSearchUses),
		EstimatedCostUSD: estimateCost(model, usage, webSearchUses),
	}
	c.usageMu.Lock()
	c.usageCalls = append(c.usageCalls, call)
	c.usageMu.Unlock()
}

const selectAndRewriteSystemPrompt = `You are Tailoring a CV for one specific Job Description, following the "Tailor CV" process described below.

Selection: choose which Entries, and which of their bullets, are relevant to the Job Description, and in what order. The result must be trimmable enough to fit one page, so err on the side of cutting a marginal Entry or bullet rather than keeping everything.

Rewrite: adjust bullet phrasing to better match the Job Description's language and emphasis. Do not introduce facts, tools, or claims that aren't present in the source bullet — you may reword, never invent. For each bullet you keep, report its exact original text as "source" (verbatim, unmodified) and your reworded version as "rewritten". If you don't reword a bullet, "rewritten" must equal "source".

Call the select_and_rewrite tool with your result. Every "entryId" you return must be one of the candidate ids given to you. Every "sourceIndex" must be that bullet's position (0-based) in that Entry's bullet list, and "source" must match that bullet's text exactly.`

const selectAndRewriteToolName = "select_and_rewrite"

func selectAndRewriteTool() anthropic.ToolUnionParam {
	schema := anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"entries": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"entryId": map[string]any{"type": "string", "description": "Must match one of the candidate Entry ids."},
						"reason":  map[string]any{"type": "string", "description": "Why this Entry was selected, for Text Review."},
						"bullets": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"sourceIndex": map[string]any{"type": "integer"},
									"source":      map[string]any{"type": "string"},
									"rewritten":   map[string]any{"type": "string"},
								},
								"required": []string{"sourceIndex", "source", "rewritten"},
							},
						},
					},
					"required": []string{"entryId", "reason", "bullets"},
				},
			},
		},
		Required: []string{"entries"},
	}
	return anthropic.ToolUnionParamOfTool(schema, selectAndRewriteToolName)
}

// SelectAndRewrite asks Claude to select and rewrite Entries for req, via a
// forced call to the select_and_rewrite tool.
func (c *Client) SelectAndRewrite(ctx context.Context, req generation.SelectionRequest) (generation.SelectionResult, error) {
	candidates, err := json.Marshal(req.Candidates)
	if err != nil {
		return generation.SelectionResult{}, fmt.Errorf("marshaling candidates: %w", err)
	}

	userPrompt := fmt.Sprintf("Job Description:\n%s\n\nCandidate Entries (JSON):\n%s", req.JobDescription, candidates)

	message, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      c.modelFor(callSiteSelectionRewrite),
		MaxTokens:  8192,
		System:     []anthropic.TextBlockParam{{Text: selectAndRewriteSystemPrompt}},
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt))},
		Tools:      []anthropic.ToolUnionParam{selectAndRewriteTool()},
		ToolChoice: anthropic.ToolChoiceParamOfTool(selectAndRewriteToolName),
	})
	if err != nil {
		return generation.SelectionResult{}, fmt.Errorf("calling Claude API: %w", err)
	}
	c.recordUsage("selection_rewrite", message.Model, message.Usage, 0)

	for _, block := range message.Content {
		if block.Type != "tool_use" || block.Name != selectAndRewriteToolName {
			continue
		}
		var result generation.SelectionResult
		if err := json.Unmarshal(block.Input, &result); err != nil {
			return generation.SelectionResult{}, fmt.Errorf("decoding %s tool input: %w", selectAndRewriteToolName, err)
		}
		return result, nil
	}
	return generation.SelectionResult{}, fmt.Errorf("Claude response had no %s tool call", selectAndRewriteToolName)
}

const selectOnlySystemPrompt = `You are previewing Selection for one specific Job Description, with no Rewrite step. Choose which Entries, and which of their bullets, are relevant to the Job Description, and in what order, the same way Selection normally would (trimmable enough to fit one page, so err on the side of cutting a marginal Entry or bullet rather than keeping everything). Do not reword any bullet: report each kept bullet's exact original text as "source", verbatim.

Call the select_only tool with your result. Every "entryId" you return must be one of the candidate ids given to you. Every "sourceIndex" must be that bullet's position (0-based) in that Entry's bullet list, and "source" must match that bullet's text exactly.`

const selectOnlyToolName = "select_only"

func selectOnlyTool() anthropic.ToolUnionParam {
	schema := anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"entries": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"entryId": map[string]any{"type": "string", "description": "Must match one of the candidate Entry ids."},
						"reason":  map[string]any{"type": "string", "description": "Why this Entry was selected, for the preview."},
						"bullets": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"sourceIndex": map[string]any{"type": "integer"},
									"source":      map[string]any{"type": "string"},
								},
								"required": []string{"sourceIndex", "source"},
							},
						},
					},
					"required": []string{"entryId", "reason", "bullets"},
				},
			},
		},
		Required: []string{"entries"},
	}
	return anthropic.ToolUnionParamOfTool(schema, selectOnlyToolName)
}

type selectOnlyResult struct {
	Entries []struct {
		EntryID string `json:"entryId"`
		Reason  string `json:"reason"`
		Bullets []struct {
			SourceIndex int    `json:"sourceIndex"`
			Source      string `json:"source"`
		} `json:"bullets"`
	} `json:"entries"`
}

// SelectOnly asks Claude to select, without rewriting, Entries for req, via
// a forced call to the select_only tool. This is a dedicated, smaller
// Claude call than SelectAndRewrite — not a filtered view of its response —
// per the "Dry-run Selection preview" PRD: discarding SelectAndRewrite's
// Rewritten field would still incur Rewrite's full cost and latency.
// Rewritten mirrors Source in the result since no rewrite occurred, so
// callers can reuse SelectionResult's existing shape.
func (c *Client) SelectOnly(ctx context.Context, req generation.SelectionRequest) (generation.SelectionResult, error) {
	candidates, err := json.Marshal(req.Candidates)
	if err != nil {
		return generation.SelectionResult{}, fmt.Errorf("marshaling candidates: %w", err)
	}

	userPrompt := fmt.Sprintf("Job Description:\n%s\n\nCandidate Entries (JSON):\n%s", req.JobDescription, candidates)

	message, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      c.modelFor(callSiteSelectionPreview),
		MaxTokens:  4096,
		System:     []anthropic.TextBlockParam{{Text: selectOnlySystemPrompt}},
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt))},
		Tools:      []anthropic.ToolUnionParam{selectOnlyTool()},
		ToolChoice: anthropic.ToolChoiceParamOfTool(selectOnlyToolName),
	})
	if err != nil {
		return generation.SelectionResult{}, fmt.Errorf("calling Claude API: %w", err)
	}
	c.recordUsage("selection_preview", message.Model, message.Usage, 0)

	for _, block := range message.Content {
		if block.Type != "tool_use" || block.Name != selectOnlyToolName {
			continue
		}
		var raw selectOnlyResult
		if err := json.Unmarshal(block.Input, &raw); err != nil {
			return generation.SelectionResult{}, fmt.Errorf("decoding %s tool input: %w", selectOnlyToolName, err)
		}
		result := generation.SelectionResult{Entries: make([]generation.SelectedEntry, len(raw.Entries))}
		for i, e := range raw.Entries {
			bullets := make([]generation.SelectedBullet, len(e.Bullets))
			for j, b := range e.Bullets {
				bullets[j] = generation.SelectedBullet{SourceIndex: b.SourceIndex, Source: b.Source, Rewritten: b.Source}
			}
			result.Entries[i] = generation.SelectedEntry{EntryID: e.EntryID, Reason: e.Reason, Bullets: bullets}
		}
		return result, nil
	}
	return generation.SelectionResult{}, fmt.Errorf("Claude response had no %s tool call", selectOnlyToolName)
}

const draftCoverLetterSystemPrompt = `You draft a Cover Letter for one specific Job Description.

Write it in the target language given below, whatever language the Job Description or the Snippets are in.

If any Cover Letter Snippets are given, select and lightly adapt among them for this Job Description; list every Snippet id you drew from in "sourceSnippetIds". If none are given, or none fit, write fresh prose grounded only in the candidate Entries and the Job Description — leave "sourceSnippetIds" empty in that case.

A Snippet may carry a "lang". Prefer a Snippet whose lang is the target language, then one with no lang at all. Translate a Snippet written in another language only when nothing else fits: it is the user's own vetted wording, and a translation of it reads like machine output, which is the whole thing a Snippet library exists to avoid.

Same constraint as Rewrite: do not invent facts, tools, employers, or claims absent from the candidate Entries or the Snippets you cite.

Call the draft_cover_letter tool with your result.`

const draftCoverLetterToolName = "draft_cover_letter"

func draftCoverLetterTool() anthropic.ToolUnionParam {
	schema := anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"body": map[string]any{"type": "string", "description": "The full Cover Letter prose."},
			"sourceSnippetIds": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Ids of any Cover Letter Snippets selected/adapted from. Empty if freshly generated.",
			},
		},
		Required: []string{"body"},
	}
	return anthropic.ToolUnionParamOfTool(schema, draftCoverLetterToolName)
}

// DraftCoverLetter asks Claude to draft a Cover Letter for req, via a
// forced call to the draft_cover_letter tool.
func (c *Client) DraftCoverLetter(ctx context.Context, req generation.CoverLetterRequest) (generation.CoverLetterResult, error) {
	candidates, err := json.Marshal(req.Candidates)
	if err != nil {
		return generation.CoverLetterResult{}, fmt.Errorf("marshaling candidates: %w", err)
	}
	snippets, err := json.Marshal(req.Snippets)
	if err != nil {
		return generation.CoverLetterResult{}, fmt.Errorf("marshaling snippets: %w", err)
	}

	userPrompt := fmt.Sprintf(
		"Target language (ISO 639-1): %s\n\nJob Description:\n%s\n\nCandidate Entries (JSON):\n%s\n\nCover Letter Snippets (JSON):\n%s",
		req.Language, req.JobDescription, candidates, snippets,
	)

	message, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      c.modelFor(callSiteCoverLetter),
		MaxTokens:  4096,
		System:     []anthropic.TextBlockParam{{Text: draftCoverLetterSystemPrompt}},
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt))},
		Tools:      []anthropic.ToolUnionParam{draftCoverLetterTool()},
		ToolChoice: anthropic.ToolChoiceParamOfTool(draftCoverLetterToolName),
	})
	if err != nil {
		return generation.CoverLetterResult{}, fmt.Errorf("calling Claude API: %w", err)
	}
	c.recordUsage("cover_letter", message.Model, message.Usage, 0)

	for _, block := range message.Content {
		if block.Type != "tool_use" || block.Name != draftCoverLetterToolName {
			continue
		}
		var result generation.CoverLetterResult
		if err := json.Unmarshal(block.Input, &result); err != nil {
			return generation.CoverLetterResult{}, fmt.Errorf("decoding %s tool input: %w", draftCoverLetterToolName, err)
		}
		return result, nil
	}
	return generation.CoverLetterResult{}, fmt.Errorf("Claude response had no %s tool call", draftCoverLetterToolName)
}

const extractRALToolName = "extract_ral_range"

func extractRALTool() anthropic.ToolUnionParam {
	schema := anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"found":    map[string]any{"type": "boolean", "description": "Whether a confident RAL figure was found in the research notes."},
			"min":      map[string]any{"type": "integer", "description": "Minimum of the range, in whole currency units. Required if found is true."},
			"max":      map[string]any{"type": "integer", "description": "Maximum of the range, in whole currency units. Required if found is true."},
			"currency": map[string]any{"type": "string", "description": "ISO currency code (e.g. EUR, USD). Required if found is true."},
		},
		Required: []string{"found"},
	}
	return anthropic.ToolUnionParamOfTool(schema, extractRALToolName)
}

type extractedRAL struct {
	Found    bool   `json:"found"`
	Min      int    `json:"min"`
	Max      int    `json:"max"`
	Currency string `json:"currency"`
}

// EstimateRAL researches a RAL Range via Claude's web search tool, then
// extracts a structured result from the research notes with a second,
// forced-tool-call request (server-executed tools like web search can't be
// combined with a forced custom tool choice in the same request).
func (c *Client) EstimateRAL(ctx context.Context, jobDescription string) (generation.RALRange, error) {
	researchPrompt := "Research the likely gross annual salary range for the role, company, and location described in this Job Description, using web search. Summarize what you found, including a minimum and maximum figure and currency if you found a credible source, or say plainly that you couldn't find one.\n\nJob Description:\n" + jobDescription

	research, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.modelFor(callSiteRALResearch),
		MaxTokens: 2048,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(researchPrompt))},
		Tools: []anthropic.ToolUnionParam{
			{OfWebSearchTool20260209: &anthropic.WebSearchTool20260209Param{MaxUses: param.NewOpt(int64(5))}},
		},
	})
	if err != nil {
		return generation.RALRange{}, fmt.Errorf("researching RAL range: %w", err)
	}
	c.recordUsage("ral_estimation", research.Model, research.Usage, research.Usage.ServerToolUse.WebSearchRequests)

	var notes strings.Builder
	for _, block := range research.Content {
		if block.Type == "text" {
			notes.WriteString(block.Text)
		}
	}

	extraction, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      c.modelFor(callSiteRALExtraction),
		MaxTokens:  1024,
		System:     []anthropic.TextBlockParam{{Text: "Extract a structured RAL range from these research notes by calling the extract_ral_range tool."}},
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(notes.String()))},
		Tools:      []anthropic.ToolUnionParam{extractRALTool()},
		ToolChoice: anthropic.ToolChoiceParamOfTool(extractRALToolName),
	})
	if err != nil {
		return generation.RALRange{}, fmt.Errorf("extracting RAL range: %w", err)
	}
	c.recordUsage("ral_estimation", extraction.Model, extraction.Usage, 0)

	for _, block := range extraction.Content {
		if block.Type != "tool_use" || block.Name != extractRALToolName {
			continue
		}
		var result extractedRAL
		if err := json.Unmarshal(block.Input, &result); err != nil {
			return generation.RALRange{}, fmt.Errorf("decoding %s tool input: %w", extractRALToolName, err)
		}
		if !result.Found {
			return generation.RALRange{Source: generation.RALSourceNA}, nil
		}
		return generation.RALRange{
			Min:      &result.Min,
			Max:      &result.Max,
			Currency: result.Currency,
			Source:   generation.RALSourceEstimated,
		}, nil
	}
	return generation.RALRange{}, fmt.Errorf("Claude response had no %s tool call", extractRALToolName)
}

const inferApplicationMethodSystemPrompt = `Classify how a candidate is expected to apply for the role described in this Job Description.

kind must be exactly one of:
- "portal": applying through a company/ATS website (Greenhouse, Lever, Workday, a "Apply Now" link, ...).
- "email": the Job Description asks candidates to email a CV/application to a specific address.
- "easy_apply": explicitly a LinkedIn Easy Apply posting.
- "other": none of the above, or genuinely unclear.

value is the detected application URL (for portal) or email address (for email), copied verbatim from the Job Description if present. Leave value empty for easy_apply and other, or when no concrete URL/address is stated.

Call the infer_application_method tool with your result.`

const inferApplicationMethodToolName = "infer_application_method"

func inferApplicationMethodTool() anthropic.ToolUnionParam {
	schema := anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"kind": map[string]any{
				"type":        "string",
				"enum":        []string{"portal", "email", "easy_apply", "other"},
				"description": "How the candidate is expected to apply.",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "The detected application URL or email address, verbatim from the Job Description. Empty if none, or kind is easy_apply/other.",
			},
		},
		Required: []string{"kind"},
	}
	return anthropic.ToolUnionParamOfTool(schema, inferApplicationMethodToolName)
}

type inferredApplicationMethod struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// InferApplicationMethod asks Claude to classify how a candidate should
// apply for jobDescription's role, via a forced call to the
// infer_application_method tool (story 5).
func (c *Client) InferApplicationMethod(ctx context.Context, jobDescription string) (tracking.ApplicationMethod, error) {
	message, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      c.modelFor(callSiteApplicationMethodInference),
		MaxTokens:  512,
		System:     []anthropic.TextBlockParam{{Text: inferApplicationMethodSystemPrompt}},
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("Job Description:\n" + jobDescription))},
		Tools:      []anthropic.ToolUnionParam{inferApplicationMethodTool()},
		ToolChoice: anthropic.ToolChoiceParamOfTool(inferApplicationMethodToolName),
	})
	if err != nil {
		return tracking.ApplicationMethod{}, fmt.Errorf("calling Claude API: %w", err)
	}
	c.recordUsage("application_method_inference", message.Model, message.Usage, 0)

	for _, block := range message.Content {
		if block.Type != "tool_use" || block.Name != inferApplicationMethodToolName {
			continue
		}
		var result inferredApplicationMethod
		if err := json.Unmarshal(block.Input, &result); err != nil {
			return tracking.ApplicationMethod{}, fmt.Errorf("decoding %s tool input: %w", inferApplicationMethodToolName, err)
		}
		return tracking.ApplicationMethod{Kind: tracking.ApplicationMethodKind(result.Kind), Value: result.Value}, nil
	}
	return tracking.ApplicationMethod{}, fmt.Errorf("Claude response had no %s tool call", inferApplicationMethodToolName)
}

const extractContactToolName = "extract_contact"

func extractContactTool() anthropic.ToolUnionParam {
	schema := anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"found": map[string]any{"type": "boolean", "description": "Whether a plausible recruiter/hiring-manager contact was found."},
			"name":  map[string]any{"type": "string", "description": "The contact's name. Required if found is true."},
			"email": map[string]any{"type": "string", "description": "The contact's email address. Required if found is true."},
		},
		Required: []string{"found"},
	}
	return anthropic.ToolUnionParamOfTool(schema, extractContactToolName)
}

type extractedContact struct {
	Found bool   `json:"found"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// SuggestContact researches a plausible recruiter/hiring-manager Contact
// for company via Claude's web search tool, then extracts a structured
// result from the research notes with a second, forced-tool-call request
// (server-executed tools like web search can't be combined with a forced
// custom tool choice in the same request — the same two-call pattern as
// EstimateRAL). Story 7: this never persists anything, only researches.
func (c *Client) SuggestContact(ctx context.Context, company, jobDescription string) (tracking.Contact, error) {
	researchPrompt := fmt.Sprintf(
		"Research a plausible recruiter or hiring-manager contact (name and email) for applying to a role at %s, using web search. Ground this in the Job Description below if it names anyone directly. Summarize what you found, including a name and email if you found a credible one, or say plainly that you couldn't find a specific contact.\n\nJob Description:\n%s",
		company, jobDescription,
	)

	research, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.modelFor(callSiteContactResearch),
		MaxTokens: 2048,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(researchPrompt))},
		Tools: []anthropic.ToolUnionParam{
			{OfWebSearchTool20260209: &anthropic.WebSearchTool20260209Param{MaxUses: param.NewOpt(int64(5))}},
		},
	})
	if err != nil {
		return tracking.Contact{}, fmt.Errorf("researching contact: %w", err)
	}
	c.recordUsage("contact_suggestion", research.Model, research.Usage, research.Usage.ServerToolUse.WebSearchRequests)

	var notes strings.Builder
	for _, block := range research.Content {
		if block.Type == "text" {
			notes.WriteString(block.Text)
		}
	}

	extraction, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:      c.modelFor(callSiteContactExtraction),
		MaxTokens:  512,
		System:     []anthropic.TextBlockParam{{Text: "Extract a structured contact from these research notes by calling the extract_contact tool."}},
		Messages:   []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(notes.String()))},
		Tools:      []anthropic.ToolUnionParam{extractContactTool()},
		ToolChoice: anthropic.ToolChoiceParamOfTool(extractContactToolName),
	})
	if err != nil {
		return tracking.Contact{}, fmt.Errorf("extracting contact: %w", err)
	}
	c.recordUsage("contact_suggestion", extraction.Model, extraction.Usage, 0)

	for _, block := range extraction.Content {
		if block.Type != "tool_use" || block.Name != extractContactToolName {
			continue
		}
		var result extractedContact
		if err := json.Unmarshal(block.Input, &result); err != nil {
			return tracking.Contact{}, fmt.Errorf("decoding %s tool input: %w", extractContactToolName, err)
		}
		if !result.Found {
			return tracking.Contact{}, nil
		}
		return tracking.Contact{Name: result.Name, Email: result.Email}, nil
	}
	return tracking.Contact{}, fmt.Errorf("Claude response had no %s tool call", extractContactToolName)
}
