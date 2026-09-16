package generation

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/gio-del/sumisura/backend/internal/masterdata"
)

// ErrInvalidSelection marks a Client response that Generate rejects because
// it can't be trusted: it names an Entry/bullet absent from Master Data, or
// misreports a bullet's source text. Either would let fabricated content
// slip past Text Review, which the no-invented-facts constraint (see
// CONTEXT.md's Rewrite entry, ADR-0002) does not allow.
var ErrInvalidSelection = errors.New("client returned a selection not traceable to Master Data")

// Generate runs Selection+Rewrite for one Generation: pasted/fetched Job
// Description text drives Tailoring via client; no Job Description at all
// runs Default Mode, which never calls client (see CONTEXT.md's Default
// Mode entry).
func Generate(ctx context.Context, dataDir string, client Client, req GenerateRequest) (GenerateResult, error) {
	entries, err := masterdata.ListEntries(dataDir)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("loading master data: %w", err)
	}

	jobDescription, err := ResolveJobDescription(ctx, req.JobDescription, req.JobDescriptionURL)
	if err != nil {
		return GenerateResult{}, err
	}

	if jobDescription == "" {
		return GenerateResult{Mode: ModeDefault, Selection: defaultModeSelection(entries), Language: DefaultLanguage}, nil
	}

	candidates := toCandidates(entries)

	selection, err := client.SelectAndRewrite(ctx, SelectionRequest{
		JobDescription:   jobDescription,
		Candidates:       candidates,
		LanguageOverride: req.LanguageOverride,
	})
	if err != nil {
		return GenerateResult{}, fmt.Errorf("selecting and rewriting: %w", err)
	}
	if err := validateSelection(selection, entries); err != nil {
		return GenerateResult{}, err
	}

	// The override wins even if the Client echoed something else back, so
	// a user-corrected language at Text Review is never second-guessed.
	language := selection.Language
	if req.LanguageOverride != "" {
		language = req.LanguageOverride
	}
	language = NormalizeLanguage(language)
	selection.Language = language

	snippets, err := masterdata.ListSnippets(dataDir)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("loading cover letter snippets: %w", err)
	}

	coverLetter, err := client.DraftCoverLetter(ctx, CoverLetterRequest{
		JobDescription: jobDescription,
		Candidates:     candidates,
		Snippets:       toCandidateSnippets(snippets),
		Language:       language,
	})
	if err != nil {
		return GenerateResult{}, fmt.Errorf("drafting cover letter: %w", err)
	}
	if err := validateCoverLetter(coverLetter, snippets); err != nil {
		return GenerateResult{}, err
	}

	ral, err := ResolveRAL(ctx, jobDescription, "", client)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("resolving RAL range: %w", err)
	}

	groundedness := computeGroundedness(selection, coverLetter, snippets)

	return GenerateResult{
		Mode:           ModeTailored,
		JobDescription: jobDescription,
		Selection:      selection,
		CoverLetter:    &coverLetter,
		RAL:            &ral,
		Usage:          aggregateUsage(DrainUsage(client)),
		Language:       language,
		Groundedness:   &groundedness,
	}, nil
}

// Preview runs Selection alone for req: no Rewrite, no Cover Letter
// drafting, no RAL Range estimation, and (unlike Generate) never persisted
// against an Application — a cheap sanity check of what Selection would
// pick before committing to a full Generation (see the "Dry-run Selection
// preview" PRD). Default Mode behaves exactly as in Generate: every Entry
// included, unmodified, no Client call.
func Preview(ctx context.Context, dataDir string, client Client, req GenerateRequest) (GenerateResult, error) {
	entries, err := masterdata.ListEntries(dataDir)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("loading master data: %w", err)
	}

	jobDescription, err := ResolveJobDescription(ctx, req.JobDescription, req.JobDescriptionURL)
	if err != nil {
		return GenerateResult{}, err
	}

	if jobDescription == "" {
		return GenerateResult{Mode: ModeDefault, Selection: defaultModeSelection(entries)}, nil
	}

	selection, err := client.SelectOnly(ctx, SelectionRequest{
		JobDescription: jobDescription,
		Candidates:     toCandidates(entries),
	})
	if err != nil {
		return GenerateResult{}, fmt.Errorf("selecting: %w", err)
	}
	if err := validateSelection(selection, entries); err != nil {
		return GenerateResult{}, err
	}

	return GenerateResult{Mode: ModeTailored, JobDescription: jobDescription, Selection: selection}, nil
}

// ResolveRAL implements the PRD's RAL Range lookup, now against two
// sources: the Job Description text and, for a LinkedIn capture, the
// listing's own dedicated salary-badge text (listingSalaryText — empty for
// every other intake path, which behaves exactly as before). If only one
// source states a figure, that's Stated exactly as before, regardless of
// which source it came from. If both state figures, overlapping/matching
// ranges resolve to the Job-Description-stated figure (the listing figure
// discarded, no conflict surfaced); non-overlapping ranges report
// RALSourceConflict with both figures labeled, rather than picking one
// silently (ADR-0014). Only when neither source states anything does this
// ask the Client to research one (via web search), same as before.
// Exported so the tracking package (Job Listing's RAL Range, see
// CONTEXT.md) can reuse the same lookup Generate uses, rather than
// duplicating it.
func ResolveRAL(ctx context.Context, jobDescription string, listingSalaryText string, client Client) (RALRange, error) {
	descriptionRAL, descriptionOK := ParseStatedRAL(jobDescription)
	listingRAL, listingOK := ParseStatedRAL(listingSalaryText)

	switch {
	case descriptionOK && listingOK:
		if ralRangesOverlap(descriptionRAL, listingRAL) {
			return descriptionRAL, nil
		}
		return RALRange{
			Source:            RALSourceConflict,
			DescriptionStated: ralFigureFrom(descriptionRAL),
			ListingStated:     ralFigureFrom(listingRAL),
		}, nil
	case descriptionOK:
		return descriptionRAL, nil
	case listingOK:
		return listingRAL, nil
	}

	ral, err := client.EstimateRAL(ctx, jobDescription)
	if err != nil {
		return RALRange{}, err
	}
	if ral.Source != RALSourceEstimated && ral.Source != RALSourceNA {
		// Defensive: only ParseStatedRAL may report RALSourceStated.
		ral.Source = RALSourceEstimated
	}
	return ral, nil
}

// ralRangesOverlap reports whether a and b's [Min,Max] ranges share any
// point at all — touching counts as overlapping (e.g. 40000-50000 and
// 50000-60000).
func ralRangesOverlap(a, b RALRange) bool {
	return *a.Min <= *b.Max && *b.Min <= *a.Max
}

func ralFigureFrom(ral RALRange) *RALFigure {
	return &RALFigure{Min: *ral.Min, Max: *ral.Max, Currency: ral.Currency}
}

// ResolveJobDescription returns the Job Description text to use: text
// verbatim if set, the text extracted from url if that's set instead, or ""
// (Default Mode, for a Generation) if neither is. Exported so the tracking
// package (saving a Job Listing from pasted text or a URL, story 1) can
// reuse the same resolution Generate uses.
func ResolveJobDescription(ctx context.Context, text, url string) (string, error) {
	if text != "" {
		return text, nil
	}
	if url == "" {
		return "", nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building request for job description URL: %w", err)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("fetching job description URL: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // idiomatic response-body drain: the read is already done and there's nothing to do about a close failure
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching job description URL: got status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading job description URL body: %w", err)
	}
	return htmlToText(string(body)), nil
}

var (
	scriptOrStyleRe = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	tagRe           = regexp.MustCompile(`(?s)<[^>]+>`)
	whitespaceRe    = regexp.MustCompile(`[ \t]+`)
	blankLinesRe    = regexp.MustCompile(`\n{3,}`)
)

// htmlToText strips an HTML document down to its visible text. Good enough
// for extracting a Job Description from a job-posting page, not a general
// HTML renderer.
func htmlToText(html string) string {
	text := scriptOrStyleRe.ReplaceAllString(html, "\n")
	text = tagRe.ReplaceAllString(text, "\n")
	text = whitespaceRe.ReplaceAllString(text, " ")
	text = blankLinesRe.ReplaceAllString(text, "\n\n")
	lines := strings.Split(text, "\n")
	var kept []string
	for _, l := range lines {
		if t := strings.TrimSpace(l); t != "" {
			kept = append(kept, t)
		}
	}
	return strings.Join(kept, "\n")
}

func toCandidates(entries []masterdata.Entry) []CandidateEntry {
	candidates := make([]CandidateEntry, len(entries))
	for i, e := range entries {
		candidates[i] = CandidateEntry{
			ID:       e.ID,
			Type:     e.Type,
			Employer: e.Employer,
			Client:   e.Client,
			Role:     e.Role,
			Name:     e.Name,
			Start:    e.Start,
			Flagship: e.Flagship,
			Tags:     e.Tags,
			Bullets:  e.Bullets,
		}
	}
	return candidates
}

func toCandidateSnippets(snippets []masterdata.Snippet) []CandidateSnippet {
	candidates := make([]CandidateSnippet, len(snippets))
	for i, s := range snippets {
		candidates[i] = CandidateSnippet{ID: s.ID, Kind: s.Kind, Lang: s.Lang, Tags: s.Tags, Body: s.Body}
	}
	return candidates
}

// validateCoverLetter rejects a Client response that cites a Cover Letter
// Snippet not present in Master Data, the same traceability backstop
// validateSelection applies to Selection (see ADR-0010).
func validateCoverLetter(result CoverLetterResult, snippets []masterdata.Snippet) error {
	known := make(map[string]bool, len(snippets))
	for _, s := range snippets {
		known[s.ID] = true
	}
	for _, id := range result.SourceSnippetIDs {
		if !known[id] {
			return fmt.Errorf("%w: unknown cover letter snippet id %q", ErrInvalidSelection, id)
		}
	}
	return nil
}

// defaultModeSelection includes every Entry, unmodified (Rewrite is
// skipped, per CONTEXT.md's Default Mode entry), Flagship Entries first
// then by most recent Start date, so the FE and Render see the most
// representative work first without the user needing to reorder it.
func defaultModeSelection(entries []masterdata.Entry) SelectionResult {
	sorted := make([]masterdata.Entry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Flagship != sorted[j].Flagship {
			return sorted[i].Flagship
		}
		return sorted[i].Start > sorted[j].Start
	})

	result := SelectionResult{Entries: make([]SelectedEntry, len(sorted))}
	for i, e := range sorted {
		bullets := make([]SelectedBullet, len(e.Bullets))
		for j, b := range e.Bullets {
			bullets[j] = SelectedBullet{SourceIndex: j, Source: b, Rewritten: b}
		}
		result.Entries[i] = SelectedEntry{EntryID: e.ID, Reason: "Default Mode: included in full.", Bullets: bullets}
	}
	return result
}

// validateSelection rejects a Client response that isn't traceable to
// Master Data: an unknown EntryID, an out-of-range SourceIndex, or a
// reported Source that doesn't match what's actually on disk for that
// bullet. Rewritten text is exempt — Rewrite is allowed to reword it.
func validateSelection(selection SelectionResult, entries []masterdata.Entry) error {
	byID := make(map[string]masterdata.Entry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}

	for _, se := range selection.Entries {
		entry, ok := byID[se.EntryID]
		if !ok {
			return fmt.Errorf("%w: unknown entry id %q", ErrInvalidSelection, se.EntryID)
		}
		for _, b := range se.Bullets {
			if b.SourceIndex < 0 || b.SourceIndex >= len(entry.Bullets) {
				return fmt.Errorf("%w: entry %q has no bullet at index %d", ErrInvalidSelection, se.EntryID, b.SourceIndex)
			}
			if entry.Bullets[b.SourceIndex] != b.Source {
				return fmt.Errorf("%w: entry %q bullet %d source text does not match master data", ErrInvalidSelection, se.EntryID, b.SourceIndex)
			}
		}
	}
	return nil
}
