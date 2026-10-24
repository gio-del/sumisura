package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gio-del/sumisura/backend/internal/generation"
)

// cvcheckBin is the path to the cvcheck binary built once in TestMain. The
// tests exercise the real binary — flags, stdout, exit status — because
// that boundary is what SKILL.md documents and the skill depends on.
var cvcheckBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "cvcheck-test-")
	if err != nil {
		panic(err)
	}
	cvcheckBin = filepath.Join(dir, "cvcheck")
	build := exec.Command("go", "build", "-o", cvcheckBin, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		os.RemoveAll(dir)
		panic("building cvcheck: " + err.Error())
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type runResult struct {
	stdout, stderr string
	exitCode       int
}

// runCLI runs the built binary in dir with args. env, when non-nil,
// replaces the process environment (used to hide pdftotext from PATH).
func runCLI(t *testing.T, dir string, env []string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(cvcheckBin, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("running cvcheck: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return runResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: code}
}

const fixtureEntry = `---
employer: Acme Corp
kind: consultancy
client: Example Client
role: Senior Engineer
start: "2022"
end: null
tags:
  - Go
---

- Migrated the legacy billing service to a microservices architecture, reducing incident response time by 60 percent.
- Led a cross-functional team of 5 engineers.
`

const (
	sourceBullet0 = "Migrated the legacy billing service to a microservices architecture, reducing incident response time by 60 percent."
	sourceBullet1 = "Led a cross-functional team of 5 engineers."
)

// writeFixtureProject lays out a minimal repo-root-shaped directory (data/
// with one Entry, output/<slug>/) and returns its path, so the CLI can be
// run from it with the same relative paths SKILL.md uses.
func writeFixtureProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "data", "experience", "acme.md"), fixtureEntry)
	if err := os.MkdirAll(filepath.Join(root, "output", "acme-corp"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSelection(t *testing.T, root string, selection generation.SelectionResult) string {
	t.Helper()
	encoded, err := json.MarshalIndent(selection, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join("output", "acme-corp", "selection.json")
	mustWrite(t, filepath.Join(root, rel), string(encoded))
	return rel
}

func groundedSelection() generation.SelectionResult {
	return generation.SelectionResult{
		Language: "en",
		Entries: []generation.SelectedEntry{{
			EntryID: "experience/acme",
			Reason:  "Go backend work matches the role.",
			Bullets: []generation.SelectedBullet{
				{SourceIndex: 0, Source: sourceBullet0, Rewritten: "Moved the aging billing service onto a microservices architecture, cutting incident response time 60 percent."},
				{SourceIndex: 1, Source: sourceBullet1, Rewritten: sourceBullet1},
			},
		}},
	}
}

func fabricatedSelection() generation.SelectionResult {
	s := groundedSelection()
	s.Entries[0].Bullets[0].Rewritten = "Migrated the legacy billing service to a microservices architecture, reducing incident response time by 90 percent."
	return s
}

func TestGroundedness_AllGrounded_ExitsZeroWithBriefSummary(t *testing.T) {
	root := writeFixtureProject(t)
	sel := writeSelection(t, root, groundedSelection())

	res := runCLI(t, root, nil, "groundedness", "--selection", sel)

	if res.exitCode != 0 {
		t.Fatalf("exit = %d, want 0; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "2 rewritten bullets") || !strings.Contains(strings.ToLower(res.stdout), "no flags") {
		t.Errorf("expected a brief all-clear naming the bullet count, got %q", res.stdout)
	}
}

func TestGroundedness_FabricatedNumber_ExitsOneAndNamesBulletAndReason(t *testing.T) {
	root := writeFixtureProject(t)
	sel := writeSelection(t, root, fabricatedSelection())

	res := runCLI(t, root, nil, "groundedness", "--selection", sel)

	if res.exitCode != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	for _, want := range []string{"experience/acme", "bullet 0", string(generation.ReasonNumericMismatch), "by 90 percent"} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("stdout missing %q: %q", want, res.stdout)
		}
	}
	if strings.Contains(res.stdout, "bullet 1") {
		t.Errorf("expected only bullet 0 to be reported, got %q", res.stdout)
	}
}

func TestGroundedness_JSONMode_RoundTripsIntoAPIResultType(t *testing.T) {
	root := writeFixtureProject(t)
	selection := fabricatedSelection()
	sel := writeSelection(t, root, selection)

	res := runCLI(t, root, nil, "groundedness", "--selection", sel, "--data-dir", "data", "--json")

	if res.exitCode != 1 {
		t.Fatalf("exit = %d, want 1; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
	}
	// Strict decode into the exact type the API's GenerateResult and the
	// record-Generation request carry: any extra/renamed field would make
	// a skill-recorded Generation render differently from an app one.
	dec := json.NewDecoder(strings.NewReader(res.stdout))
	dec.DisallowUnknownFields()
	var got generation.GroundednessResult
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("decoding --json output into GroundednessResult: %v\n%s", err, res.stdout)
	}
	want, err := generation.CheckSelectionGroundedness(filepath.Join(root, "data"), selection)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CLI JSON = %+v, want the facade's result %+v", got, want)
	}
}

func TestGroundedness_JSONMode_CleanRunIsEmptyObject(t *testing.T) {
	root := writeFixtureProject(t)
	sel := writeSelection(t, root, groundedSelection())

	res := runCLI(t, root, nil, "groundedness", "--selection", sel, "--json")

	if res.exitCode != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", res.exitCode, res.stderr)
	}
	if strings.TrimSpace(res.stdout) != "{}" {
		t.Errorf("expected {} for a clean run, got %q", res.stdout)
	}
}

func TestGroundedness_ReportsNormalizedTargetLanguage(t *testing.T) {
	tests := []struct {
		detected string
		want     string
	}{
		{detected: "it", want: "Target language: it"},
		{detected: "fr", want: "Target language: en"},
		{detected: "", want: "Target language: en"},
	}
	for _, tt := range tests {
		t.Run(tt.detected, func(t *testing.T) {
			root := writeFixtureProject(t)
			selection := groundedSelection()
			selection.Language = tt.detected
			sel := writeSelection(t, root, selection)

			res := runCLI(t, root, nil, "groundedness", "--selection", sel)

			if !strings.Contains(res.stdout, tt.want) {
				t.Errorf("stdout missing %q: %q", tt.want, res.stdout)
			}
		})
	}
}

func TestGroundedness_CouldNotRun_ExitsTwo(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string) []string
	}{
		{
			name: "missing selection file",
			setup: func(t *testing.T, root string) []string {
				return []string{"groundedness", "--selection", "output/acme-corp/selection.json"}
			},
		},
		{
			name: "malformed selection file",
			setup: func(t *testing.T, root string) []string {
				mustWrite(t, filepath.Join(root, "output", "acme-corp", "selection.json"), `{"entries": [`)
				return []string{"groundedness", "--selection", "output/acme-corp/selection.json"}
			},
		},
		{
			name: "misnamed field in selection file",
			setup: func(t *testing.T, root string) []string {
				// "rewrittenText" instead of "rewritten" would otherwise
				// decode to an empty rewrite and silently pass the check.
				mustWrite(t, filepath.Join(root, "output", "acme-corp", "selection.json"),
					`{"entries":[{"entryId":"experience/acme","reason":"","bullets":[{"sourceIndex":1,"source":"`+sourceBullet1+`","rewrittenText":"Led 50 engineers."}]}]}`)
				return []string{"groundedness", "--selection", "output/acme-corp/selection.json"}
			},
		},
		{
			name: "source bullet not verbatim from master data",
			setup: func(t *testing.T, root string) []string {
				s := groundedSelection()
				s.Entries[0].Bullets[1].Source = "Led a cross-functional team of 50 engineers."
				return []string{"groundedness", "--selection", writeSelection(t, root, s)}
			},
		},
		{
			name: "missing --selection flag",
			setup: func(t *testing.T, root string) []string {
				return []string{"groundedness"}
			},
		},
		{
			name: "unknown subcommand",
			setup: func(t *testing.T, root string) []string {
				return []string{"bogus"}
			},
		},
		{
			name: "no subcommand",
			setup: func(t *testing.T, root string) []string {
				return nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeFixtureProject(t)
			args := tt.setup(t, root)

			res := runCLI(t, root, nil, args...)

			if res.exitCode != 2 {
				t.Fatalf("exit = %d, want 2; stdout=%q stderr=%q", res.exitCode, res.stdout, res.stderr)
			}
			if !strings.Contains(res.stderr, "unavailable") && !strings.Contains(res.stderr, "usage") {
				t.Errorf("expected stderr to say the check is unavailable (or print usage), got %q", res.stderr)
			}
		})
	}
}
