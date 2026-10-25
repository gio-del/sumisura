// Package claudecode is a second generation.Client that runs the same
// prompts through headless Claude Code (`claude -p`) on the self-hoster's own
// Claude subscription instead of the pay-per-token API (ADR-0047). It is
// selected with SUMISURA_CLAUDE_BACKEND=claude-code; internal/claude stays
// the default.
//
// Only the Selection, Selection-preview and Cover Letter call sites run
// through Claude Code so far. Every other call (RAL, contact research,
// capture hints, method inference) goes to the Fallback client unchanged.
package claudecode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/gio-del/sumisura/backend/internal/claude"
	"github.com/gio-del/sumisura/backend/internal/generation"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// Fallback serves the call sites this package does not run yet.
type Fallback interface {
	tracking.Client
	generation.UsageRecorder
}

// Client runs Selection and Cover Letter calls through `claude -p`.
type Client struct {
	// Bin is the claude executable; empty means "claude" from PATH.
	Bin      string
	fallback Fallback

	usageMu    sync.Mutex
	usageCalls []generation.CallUsage
}

var _ tracking.Client = (*Client)(nil)
var _ generation.UsageRecorder = (*Client)(nil)

// New builds a Client delegating the call sites it does not cover to fallback.
func New(fallback Fallback) *Client { return &Client{fallback: fallback} }

// cliResult is the part of `claude -p --output-format json` this package reads.
type cliResult struct {
	Subtype          string          `json:"subtype"`
	IsError          bool            `json:"is_error"`
	Result           string          `json:"result"`
	StructuredOutput json.RawMessage `json:"structured_output"`
	TotalCostUSD     float64         `json:"total_cost_usd"`
	Usage            struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

// subprocessEnv is the environment handed to claude: the host's own, minus
// the API credentials, since with ANTHROPIC_API_KEY set Claude Code would
// bill the API instead of the subscription (CLAUDE_CODE_OAUTH_TOKEN stays).
func subprocessEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		if strings.HasPrefix(kv, "ANTHROPIC_API_KEY=") || strings.HasPrefix(kv, "ANTHROPIC_AUTH_TOKEN=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// run executes one call and decodes its structured output into out. The
// context's deadline (ADR-0021) kills the subprocess.
func (c *Client) run(ctx context.Context, spec claude.Spec, userPrompt string, out any) error {
	bin := c.Bin
	if bin == "" {
		bin = "claude"
	}
	// Not --bare: it never reads the OAuth token. The isolation flags and
	// the empty working directory keep the host's Claude Code config
	// (CLAUDE.md, MCP servers, slash commands) out of a Generation.
	workDir, err := os.MkdirTemp("", "sumisura-claude-")
	if err != nil {
		return fmt.Errorf("creating working directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }() //nolint:errcheck // best-effort cleanup of an empty temp dir; nothing to do if it fails

	system := spec.System + "\n\nReturn your result as the structured output; there is no tool to call."
	cmd := exec.CommandContext(ctx, bin, "-p",
		"--output-format", "json",
		"--json-schema", string(spec.Schema),
		"--model", spec.Model,
		"--system-prompt", system,
		"--tools", "",
		"--no-session-persistence",
		"--strict-mcp-config",
		"--disable-slash-commands",
	)
	cmd.Dir = workDir
	cmd.Env = subprocessEnv(os.Environ())
	cmd.Stdin = strings.NewReader(userPrompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	runErr := cmd.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("running Claude Code: %w", ctx.Err())
	}
	var res cliResult
	decodeErr := json.Unmarshal(stdout.Bytes(), &res)
	// Auth and rate-limit failures are printed as the result on stdout, so
	// prefer that text over the bare exit status.
	if decodeErr == nil && (res.IsError || (runErr != nil && res.Result != "")) {
		return fmt.Errorf("Claude Code failed (%s): %s", res.Subtype, strings.TrimSpace(res.Result))
	}
	if runErr != nil {
		if errors.Is(runErr, exec.ErrNotFound) || errors.Is(runErr, os.ErrNotExist) {
			return fmt.Errorf("Claude Code is not installed (%q not found on PATH)", bin)
		}
		return fmt.Errorf("running Claude Code: %w: %s", runErr, strings.TrimSpace(stderr.String()))
	}
	if decodeErr != nil {
		return fmt.Errorf("decoding Claude Code output: %w", decodeErr)
	}
	c.recordUsage(spec, res)
	if len(res.StructuredOutput) == 0 {
		return fmt.Errorf("Claude Code returned no structured output for %s", spec.CallType)
	}
	if err := json.Unmarshal(res.StructuredOutput, out); err != nil {
		return fmt.Errorf("decoding %s output: %w", spec.CallType, err)
	}
	return nil
}

// recordUsage notes the call's tokens. On a subscription the cost is an
// API-equivalent estimate, not real spend.
func (c *Client) recordUsage(spec claude.Spec, res cliResult) {
	call := generation.CallUsage{
		CallType:         spec.CallType,
		Model:            spec.Model,
		InputTokens:      res.Usage.InputTokens,
		OutputTokens:     res.Usage.OutputTokens,
		CacheReadTokens:  res.Usage.CacheReadInputTokens,
		CacheWriteTokens: res.Usage.CacheCreationInputTokens,
		EstimatedCostUSD: res.TotalCostUSD,
	}
	c.usageMu.Lock()
	c.usageCalls = append(c.usageCalls, call)
	c.usageMu.Unlock()
}

// DrainUsage implements generation.UsageRecorder, merging in whatever the
// fallback client recorded.
func (c *Client) DrainUsage() []generation.CallUsage {
	c.usageMu.Lock()
	drained := c.usageCalls
	c.usageCalls = nil
	c.usageMu.Unlock()
	return append(drained, c.fallback.DrainUsage()...)
}

// SelectAndRewrite implements generation.Client.
func (c *Client) SelectAndRewrite(ctx context.Context, req generation.SelectionRequest) (generation.SelectionResult, error) {
	prompt, err := claude.SelectionUserPrompt(req)
	if err != nil {
		return generation.SelectionResult{}, err
	}
	var result generation.SelectionResult
	if err := c.run(ctx, claude.SelectAndRewriteSpec(), prompt, &result); err != nil {
		return generation.SelectionResult{}, err
	}
	return result, nil
}

// SelectOnly implements generation.Client; Rewritten mirrors Source.
func (c *Client) SelectOnly(ctx context.Context, req generation.SelectionRequest) (generation.SelectionResult, error) {
	prompt, err := claude.SelectionUserPrompt(req)
	if err != nil {
		return generation.SelectionResult{}, err
	}
	var result generation.SelectionResult
	if err := c.run(ctx, claude.SelectOnlySpec(), prompt, &result); err != nil {
		return generation.SelectionResult{}, err
	}
	for i := range result.Entries {
		for j := range result.Entries[i].Bullets {
			b := &result.Entries[i].Bullets[j]
			b.Rewritten = b.Source
		}
	}
	return result, nil
}

// DraftCoverLetter implements generation.Client.
func (c *Client) DraftCoverLetter(ctx context.Context, req generation.CoverLetterRequest) (generation.CoverLetterResult, error) {
	prompt, err := claude.CoverLetterUserPrompt(req)
	if err != nil {
		return generation.CoverLetterResult{}, err
	}
	var result generation.CoverLetterResult
	if err := c.run(ctx, claude.CoverLetterSpec(), prompt, &result); err != nil {
		return generation.CoverLetterResult{}, err
	}
	return result, nil
}

// EstimateRAL implements generation.Client via the fallback.
func (c *Client) EstimateRAL(ctx context.Context, jd string) (generation.RALRange, error) {
	return c.fallback.EstimateRAL(ctx, jd)
}

// InferApplicationMethod implements tracking.Client via the fallback.
func (c *Client) InferApplicationMethod(ctx context.Context, jd string) (tracking.ApplicationMethod, error) {
	return c.fallback.InferApplicationMethod(ctx, jd)
}

// SuggestContact implements tracking.Client via the fallback.
func (c *Client) SuggestContact(ctx context.Context, company, jd string) (tracking.Contact, error) {
	return c.fallback.SuggestContact(ctx, company, jd)
}

// SuggestCaptureHints implements tracking.Client via the fallback.
func (c *Client) SuggestCaptureHints(ctx context.Context, jd string) (tracking.CaptureHints, error) {
	return c.fallback.SuggestCaptureHints(ctx, jd)
}
