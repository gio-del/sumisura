package claude

import (
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// callSite identifies one kind of Claude API request this package makes,
// for choosing the model that serves it (ADR-0025). It is deliberately
// finer-grained than the CallUsage.CallType strings recorded for usage:
// EstimateRAL and SuggestContact each make two different requests
// (research, then extraction — ADR-0011) recorded under one CallType, and
// those halves need independent models. CallType strings are persisted on
// every GenerationRecord, so they stay unchanged; model choice gets its
// own key instead.
type callSite string

const (
	callSiteSelectionRewrite           callSite = "selection_rewrite"
	callSiteSelectionPreview           callSite = "selection_preview"
	callSiteCoverLetter                callSite = "cover_letter"
	callSiteRALResearch                callSite = "ral_research"
	callSiteRALExtraction              callSite = "ral_extraction"
	callSiteApplicationMethodInference callSite = "application_method_inference"
	callSiteContactResearch            callSite = "contact_research"
	callSiteContactExtraction          callSite = "contact_extraction"
	callSiteCaptureHints               callSite = "capture_hints"
)

// defaultModels is the single place a call site's model is decided
// (ADR-0025). Calls whose output is judgment the user reviews — Selection,
// Rewrite, Cover Letter drafting — and the two web-research calls stay on
// Sonnet 5. Calls whose output is mechanically derived from a previous
// call's notes (the extraction halves of ADR-0011's two-call pattern) or is
// one click from user correction (Application Method inference, the
// Company/Job Title hints for completing a Pending Capture) run on Haiku 4.5.
//
// Adding a Claude call means adding its call site here — and, if it names a
// model not yet in pricingTable, pricing that model too
// (TestPricingTable_CoversEveryReachableModel).
var defaultModels = map[callSite]anthropic.Model{
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

// defaultModelEnvVar overrides the model for every call site at once.
const defaultModelEnvVar = "SUMISURA_MODEL_DEFAULT"

// modelEnvVar is the variable overriding site's model alone, e.g.
// SUMISURA_MODEL_SELECTION_REWRITE.
func modelEnvVar(site callSite) string {
	return "SUMISURA_MODEL_" + strings.ToUpper(string(site))
}

// resolveModels builds a Client's per-call-site model lookup from
// defaultModels and the environment. Precedence per call site: its own
// SUMISURA_MODEL_<CALL_SITE>, then SUMISURA_MODEL_DEFAULT, then the
// built-in default; an empty (or whitespace-only) value counts as unset.
//
// An override value is passed to the API as-is, never validated against a
// local allowlist: a typo or a model newer than this code should not stop
// a localhost tool from starting, and the API's own error names the
// problem better. A resolved model with no pricing entry logs a warning
// here, at startup, rather than only when its first call is costed.
func resolveModels() map[callSite]anthropic.Model {
	blanket := strings.TrimSpace(os.Getenv(defaultModelEnvVar))
	models := make(map[callSite]anthropic.Model, len(defaultModels))
	for site, model := range defaultModels {
		if blanket != "" {
			model = anthropic.Model(blanket)
		}
		if override := strings.TrimSpace(os.Getenv(modelEnvVar(site))); override != "" {
			model = anthropic.Model(override)
		}
		if _, ok := lookupPricing(model); !ok {
			warnUnpricedOnce(model)
		}
		models[site] = model
	}
	return models
}

// modelFor returns the model that serves site's requests.
func (c *Client) modelFor(site callSite) anthropic.Model {
	return c.models[site]
}
