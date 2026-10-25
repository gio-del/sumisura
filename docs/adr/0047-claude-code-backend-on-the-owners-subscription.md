# Generation can run on the owner's Claude subscription via headless Claude Code

Status: accepted (spike). An explicit exception to ADR-0005 ("the app calls the
Claude API directly"), not a reversal: the API client stays the default.

## Decision

`SUMISURA_CLAUDE_BACKEND=claude-code` swaps in `internal/claudecode`, a second
`generation.Client` / `tracking.Client` / `generation.UsageRecorder` that runs
each call as `claude -p --output-format json --json-schema <schema>` and reads
`structured_output`. Prompts, schemas and per-call-site models are the same
ones `internal/claude` uses (`claude.Spec`); only the transport differs.

- Isolation: system prompt replaced, `--tools ""`, `--no-session-persistence`,
  `--strict-mcp-config`, `--disable-slash-commands`, run from an empty temp
  directory. Never `--bare`: it ignores OAuth credentials.
- Auth: the owner's `CLAUDE_CODE_OAUTH_TOKEN` (from `claude setup-token`).
  `ANTHROPIC_API_KEY` is removed from the subprocess environment so the key
  is never billed instead. Sumisura runs no login flow and stores no claude.ai
  credentials.
- Usage: tokens come from the JSON result; `total_cost_usd` is an
  API-equivalent estimate on a subscription, not spend.
- Errors: failures Claude Code prints as the result on stdout are surfaced
  verbatim; the request deadline (ADR-0021) kills the subprocess.

## Policy boundary

Anthropic does not allow third-party apps to offer claude.ai login or route
other people's requests through Pro/Max credentials. This stays allowed only
while Sumisura is self-hosted, single-user, uses the owner's own token and the
official Claude Code binary. LAN mode does not change who is served. The docs
say "your own subscription, for your own use". Anthropic's subscription
billing for `claude -p` has changed several times in 2026, which is why the
API provider remains first-class.

## Scope of the spike

Covered: Selection + Rewrite, Selection preview, Cover Letter. The RAL,
contact research, capture-hint and method-inference calls still go to the API
client (they need `WebSearch` and the ADR-0011 two-call question settled).
Not done: the `claude` binary in the Docker image, and measuring latency,
quota, Pro-plan model availability and groundedness against a real account.
