package claude

import (
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/gio-del/sumisura/backend/internal/generation"
)

// Spec is one call site's prompt, output schema and model, exported so the
// claudecode provider (ADR-0047) runs exactly the same prompts as this
// package's API client — only the transport differs.
type Spec struct {
	// CallType is the generation.CallUsage.CallType the call is recorded under.
	CallType string
	Model    string
	System   string
	// Schema is the JSON Schema of the structured result.
	Schema json.RawMessage
}

func specFor(site callSite, callType, system string, tool anthropic.ToolUnionParam) Spec {
	schema, err := json.Marshal(tool.OfTool.InputSchema)
	if err != nil {
		panic(fmt.Sprintf("claude: marshaling %s schema: %v", callType, err))
	}
	return Spec{CallType: callType, Model: string(resolveModels()[site]), System: system, Schema: schema}
}

// SelectAndRewriteSpec describes the Selection + Rewrite call.
func SelectAndRewriteSpec() Spec {
	return specFor(callSiteSelectionRewrite, "selection_rewrite", selectAndRewriteSystemPrompt, selectAndRewriteTool())
}

// SelectOnlySpec describes the Selection-preview call.
func SelectOnlySpec() Spec {
	return specFor(callSiteSelectionPreview, "selection_preview", selectOnlySystemPrompt, selectOnlyTool())
}

// CoverLetterSpec describes the Cover Letter drafting call.
func CoverLetterSpec() Spec {
	return specFor(callSiteCoverLetter, "cover_letter", draftCoverLetterSystemPrompt, draftCoverLetterTool())
}

// SelectionUserPrompt is the user message of both Selection calls.
func SelectionUserPrompt(req generation.SelectionRequest) (string, error) {
	candidates, err := json.Marshal(req.Candidates)
	if err != nil {
		return "", fmt.Errorf("marshaling candidates: %w", err)
	}
	return selectionUserPrompt(req.JobDescription, req.Summary, candidates), nil
}

// CoverLetterUserPrompt is the user message of the Cover Letter call.
func CoverLetterUserPrompt(req generation.CoverLetterRequest) (string, error) {
	candidates, err := json.Marshal(req.Candidates)
	if err != nil {
		return "", fmt.Errorf("marshaling candidates: %w", err)
	}
	snippets, err := json.Marshal(req.Snippets)
	if err != nil {
		return "", fmt.Errorf("marshaling snippets: %w", err)
	}
	return coverLetterUserPrompt(req.Language, req.JobDescription, candidates, snippets), nil
}

func selectionUserPrompt(jobDescription, summary string, candidates []byte) string {
	prompt := fmt.Sprintf("Job Description:\n%s\n\nCandidate Entries (JSON):\n%s", jobDescription, candidates)
	if summary != "" {
		prompt += fmt.Sprintf("\n\nSummary (the About me to adapt):\n%s", summary)
	}
	return prompt
}

func coverLetterUserPrompt(language, jobDescription string, candidates, snippets []byte) string {
	return fmt.Sprintf(
		"Target language (ISO 639-1): %s\n\nJob Description:\n%s\n\nCandidate Entries (JSON):\n%s\n\nCover Letter Snippets (JSON):\n%s",
		language, jobDescription, candidates, snippets,
	)
}
