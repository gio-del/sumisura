// Package generation implements the Generation pipeline (Selection,
// Rewrite, Cover Letter drafting, RAL Range lookup, Render) described in
// CONTEXT.md and ADR-0005: the app calls the Claude API directly rather
// than delegating to the tailor-cv skill.
package generation

// CandidateEntry is the subset of a Master Data Entry that Selection
// reasons over: identity plus its bullets. Keeping this separate from
// masterdata.Entry stops the full Entry shape (including fields Selection
// never needs) from leaking into prompts and Client responses.
type CandidateEntry struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Employer string   `json:"employer,omitempty"`
	Client   string   `json:"client,omitempty"`
	Role     string   `json:"role,omitempty"`
	Name     string   `json:"name,omitempty"`
	Start    string   `json:"start"`
	Flagship bool     `json:"flagship,omitempty"`
	Tags     []string `json:"tags"`
	Bullets  []string `json:"bullets"`
}

// SelectionRequest is what the generation service asks a Client to choose
// and rewrite Entries from.
type SelectionRequest struct {
	JobDescription string
	Candidates     []CandidateEntry

	// LanguageOverride, when non-empty, tells the Client to write the
	// rewritten bullets in this language instead of detecting one from
	// JobDescription — set when the user corrects a wrong detection at
	// Text Review (issue #41's PRD, story 4).
	LanguageOverride string
}

// SelectedBullet is one bullet Selection chose for a Generation. Source is
// kept verbatim from Master Data (so Text Review can diff Rewritten against
// it, per story 4) alongside SourceIndex, the bullet's position in the
// source Entry's Bullets slice.
type SelectedBullet struct {
	SourceIndex int    `json:"sourceIndex"`
	Source      string `json:"source"`
	Rewritten   string `json:"rewritten"`
}

// SelectedEntry is one Entry Selection chose, with the bullets it kept (in
// the order they should render) and why it was chosen.
type SelectedEntry struct {
	EntryID string           `json:"entryId"`
	Reason  string           `json:"reason"`
	Bullets []SelectedBullet `json:"bullets"`
}

// SelectionResult is a Client's Selection+Rewrite output for one Generation.
// Language is the Client's detected (or, with LanguageOverride set,
// echoed-back) ISO 639-1 language code — Generate normalizes it via
// NormalizeLanguage before it reaches GenerateResult.
type SelectionResult struct {
	Entries  []SelectedEntry `json:"entries"`
	Language string          `json:"language,omitempty"`
}

// CandidateSnippet is a Cover Letter Snippet made available to the Client
// for drafting a Cover Letter, per CONTEXT.md's Cover Letter Snippet entry.
type CandidateSnippet struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Lang is the Snippet's own language (ISO 639-1), empty when unmarked.
	// The Client uses it to prefer a Snippet already written in the target
	// language over translating one (issue #168).
	Lang string   `json:"lang,omitempty"`
	Tags []string `json:"tags"`
	Body string   `json:"body"`
}

// CoverLetterRequest is what the generation service asks a Client to draft
// a Cover Letter from: the Job Description plus grounding material (Master
// Data Entries and any Cover Letter Snippets).
type CoverLetterRequest struct {
	JobDescription string
	Candidates     []CandidateEntry
	Snippets       []CandidateSnippet

	// Language is the target language (already resolved/normalized by
	// Generate) to draft the Cover Letter in.
	Language string
}

// CoverLetterResult is a Client's Cover Letter draft. SourceSnippetIDs
// names which Cover Letter Snippets (if any) it selected/adapted from — nil
// or empty means it was freshly generated prose, per CONTEXT.md's Cover
// Letter entry.
type CoverLetterResult struct {
	Body             string   `json:"body"`
	SourceSnippetIDs []string `json:"sourceSnippetIds,omitempty"`
}

// GenerateRequest is the input to Generate: a pasted Job Description, a URL
// to fetch one from, or neither (Default Mode).
type GenerateRequest struct {
	JobDescription    string `json:"jobDescription"`
	JobDescriptionURL string `json:"jobDescriptionUrl"`

	// LanguageOverride, when set, forces the target language instead of
	// letting the Client detect one — used to re-run Selection+Rewrite
	// and the Cover Letter after the user corrects a wrong detection at
	// Text Review (issue #41's PRD, story 4).
	LanguageOverride string `json:"languageOverride,omitempty"`
}

// GenerateMode distinguishes a Tailoring run from Default Mode (see
// CONTEXT.md).
type GenerateMode string

const (
	ModeDefault  GenerateMode = "default"
	ModeTailored GenerateMode = "tailored"
)

// GenerateResult is the Text-Review-ready output of a Generation's
// Selection+Rewrite (and, in Tailored Mode, Cover Letter drafting) step.
// CoverLetter is nil in Default Mode: there's no Job Description to ground
// fresh prose in, mirroring Rewrite being skipped (see CONTEXT.md's Default
// Mode entry). Groundedness is nil in Default Mode too, for the same
// reason: nothing was rewritten to check (see checkGroundedness).
type GenerateResult struct {
	Mode           GenerateMode        `json:"mode"`
	JobDescription string              `json:"jobDescription,omitempty"`
	Selection      SelectionResult     `json:"selection"`
	CoverLetter    *CoverLetterResult  `json:"coverLetter,omitempty"`
	RAL            *RALRange           `json:"ral,omitempty"`
	Groundedness   *GroundednessResult `json:"groundedness,omitempty"`

	// Usage is the Claude API usage/cost this Generate call caused, drained
	// from client if it implements UsageRecorder (PRD story 3: per-Generation
	// visibility). Zero-value (no Calls) in Default Mode, which never calls
	// client.
	Usage GenerationUsage `json:"usage"`

	// Language is the final, normalized target language (NormalizeLanguage
	// applied) the CV/Cover Letter were written in — DefaultLanguage in
	// Default Mode, since there's no Job Description to detect one from.
	// Editable at Text Review (see issue #41's PRD).
	Language string `json:"language"`
}

// GroundednessReason explains why checkGroundedness flagged a sentence.
type GroundednessReason string

const (
	// ReasonNoSourceMatch marks a sentence that shares too little overlap
	// with its source text to plausibly trace back to it at all.
	ReasonNoSourceMatch GroundednessReason = "no-source-match"
	// ReasonNumericMismatch marks a sentence that otherwise traces back to
	// its source, but states a number, date, or proper noun the source
	// doesn't — weighted more heavily than general phrasing drift (PRD
	// story 6), since a fabricated specific is the more concerning case.
	ReasonNumericMismatch GroundednessReason = "numeric-mismatch"
)

// GroundednessFlag is one sentence from Rewrite or Cover Letter output whose
// overlap with its source(s) fell below checkGroundedness's threshold — a
// hint surfaced at Text Review, never a Generation-blocking error (ADR-0002
// keeps the human review as the actual gate; see ADR-0010, which names this
// exact gap for Rewrite).
type GroundednessFlag struct {
	Sentence string             `json:"sentence" yaml:"sentence"`
	Reason   GroundednessReason `json:"reason" yaml:"reason"`
}

// BulletGroundedness is a SelectedBullet whose Rewritten text has at least
// one GroundednessFlag.
type BulletGroundedness struct {
	EntryID     string             `json:"entryId" yaml:"entryId"`
	SourceIndex int                `json:"sourceIndex" yaml:"sourceIndex"`
	Flags       []GroundednessFlag `json:"flags" yaml:"flags"`
}

// GroundednessResult is checkGroundedness's verdict across one Generation's
// Rewrite and Cover Letter output. Attached to GenerateResult so Text
// Review can render it, and persisted on the GenerationRecord it's
// eventually recorded against (tracking package) so it's visible after the
// fact, not only in the live Text Review session (PRD story 9). Bullets and
// CoverLetter are only populated for sentences that were actually flagged —
// an empty GroundednessResult means the check ran and found nothing.
type GroundednessResult struct {
	Bullets     []BulletGroundedness `json:"bullets,omitempty" yaml:"bullets,omitempty"`
	CoverLetter []GroundednessFlag   `json:"coverLetter,omitempty" yaml:"coverLetter,omitempty"`
}
