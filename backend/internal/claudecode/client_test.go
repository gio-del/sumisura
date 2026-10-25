package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/generation"
)

// fakeClaude writes a shell script standing in for the claude binary: it
// records its arguments and stdin, then prints output.
func fakeClaude(t *testing.T, output string, exit int) (bin, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "claude")
	argsFile = filepath.Join(dir, "args")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + argsFile + "\n" +
		"printf 'KEY=%s\\n' \"$ANTHROPIC_API_KEY\" >> " + argsFile + "\n" +
		"cat > " + argsFile + ".stdin\n" +
		"cat <<'JSON'\n" + output + "\nJSON\n" +
		"exit " + string(rune('0'+exit)) + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, argsFile
}

func TestSelectAndRewrite_DecodesStructuredOutputAndRecordsUsage(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-should-not-leak")
	bin, argsFile := fakeClaude(t, `{"subtype":"success","is_error":false,"result":"","total_cost_usd":0.12,
"usage":{"input_tokens":10,"output_tokens":20,"cache_read_input_tokens":3},
"structured_output":{"entries":[{"entryId":"a","reason":"r","bullets":[{"sourceIndex":0,"source":"s","rewritten":"w"}]}]}}`, 0)
	c := &Client{Bin: bin}
	c.fallback = nil

	got, err := c.SelectAndRewrite(context.Background(), generation.SelectionRequest{JobDescription: "JD-TEXT"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Bullets[0].Rewritten != "w" {
		t.Fatalf("unexpected result: %+v", got)
	}

	args, _ := os.ReadFile(argsFile)
	for _, want := range []string{"-p", "--json-schema", "--no-session-persistence", "--strict-mcp-config", "--tools", "KEY=\n"} {
		if !strings.Contains(string(args)+"\n", want) {
			t.Errorf("args missing %q:\n%s", want, args)
		}
	}
	if strings.Contains(string(args), "--bare") {
		t.Error("must not use --bare: it ignores the OAuth token")
	}
	stdin, _ := os.ReadFile(argsFile + ".stdin")
	if !strings.Contains(string(stdin), "JD-TEXT") {
		t.Errorf("job description not sent on stdin: %s", stdin)
	}

	c.usageMu.Lock()
	usage := c.usageCalls
	c.usageMu.Unlock()
	if len(usage) != 1 || usage[0].CallType != "selection_rewrite" || usage[0].InputTokens != 10 ||
		usage[0].CacheReadTokens != 3 || usage[0].EstimatedCostUSD != 0.12 {
		t.Errorf("unexpected usage: %+v", usage)
	}
}

func TestSelectOnly_MirrorsSourceIntoRewritten(t *testing.T) {
	bin, _ := fakeClaude(t, `{"subtype":"success","is_error":false,"structured_output":{"entries":[{"entryId":"a","reason":"r","bullets":[{"sourceIndex":1,"source":"orig"}]}]}}`, 0)
	c := &Client{Bin: bin}
	got, err := c.SelectOnly(context.Background(), generation.SelectionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if b := got.Entries[0].Bullets[0]; b.Rewritten != "orig" || b.SourceIndex != 1 {
		t.Errorf("unexpected bullet: %+v", b)
	}
}

func TestRun_SurfacesErrorResultFromStdout(t *testing.T) {
	bin, _ := fakeClaude(t, `{"subtype":"success","is_error":true,"result":"Invalid API key · Please run /login"}`, 1)
	c := &Client{Bin: bin}
	_, err := c.DraftCoverLetter(context.Background(), generation.CoverLetterRequest{})
	if err == nil || !strings.Contains(err.Error(), "Please run /login") {
		t.Fatalf("want the CLI's own message, got %v", err)
	}
}

func TestRun_MissingBinaryIsExplained(t *testing.T) {
	c := &Client{Bin: "/nonexistent/claude"}
	_, err := c.DraftCoverLetter(context.Background(), generation.CoverLetterRequest{})
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("got %v", err)
	}
}

func TestRun_MissingStructuredOutputIsAnError(t *testing.T) {
	bin, _ := fakeClaude(t, `{"subtype":"error_max_structured_output_retries","is_error":false}`, 0)
	c := &Client{Bin: bin}
	if _, err := c.DraftCoverLetter(context.Background(), generation.CoverLetterRequest{}); err == nil {
		t.Fatal("want an error")
	}
}

func TestSubprocessEnv_DropsAPICredentialsKeepsOAuthToken(t *testing.T) {
	got := subprocessEnv([]string{"A=1", "ANTHROPIC_API_KEY=x", "ANTHROPIC_AUTH_TOKEN=y", "CLAUDE_CODE_OAUTH_TOKEN=z"})
	if strings.Join(got, " ") != "A=1 CLAUDE_CODE_OAUTH_TOKEN=z" {
		t.Errorf("got %v", got)
	}
}
