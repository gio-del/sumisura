package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// The prose clean-up of a Generation (issue #231, follow-up of #229): the
// upstream clean-user-facing-text scripts, pinned in
// plugins/sumisura/skills/tailor-cv/clean-skill.lock, run over the rewritten
// bullets, the About me and the Cover Letter before they reach Text Review.
// Only the deterministic parts run here: the Unicode pass is applied, the
// stylometry score is measured before and after and shown. The rewrite the
// skill does on a `high` tier is not part of the app.
//
// The scripts are not fetched at run time. The release image downloads and
// verifies them at build time (Dockerfile) and points CleanSkillDirEnv at
// them; from a checkout the variable is unset and the pass reports itself as
// unavailable. It is never a blocker: a Generation always completes.

const (
	// CleanSkillDirEnv names the directory holding the verified skill
	// (scripts/inspect_text.py, scripts/clean_text.py, ...).
	CleanSkillDirEnv = "SUMISURA_CLEAN_SKILL_DIR"
	// CleanPythonEnv names the Python 3.10+ interpreter; python3 by default.
	CleanPythonEnv = "SUMISURA_PYTHON"

	cleanupTimeout = 30 * time.Second
)

// Text kinds a CleanupText can describe.
const (
	CleanupBullets     = "bullets"
	CleanupAboutMe     = "aboutMe"
	CleanupCoverLetter = "coverLetter"
)

// CleanupText is the clean-up outcome for one text. Scores are nil when the
// upstream scorer reports none (too short a text); Tier is the density tier
// of the text as the Rewrite produced it, and CharsChanged how many
// characters the Unicode pass removed or replaced.
type CleanupText struct {
	Kind         string   `json:"kind"`
	ScoreBefore  *float64 `json:"scoreBefore"`
	ScoreAfter   *float64 `json:"scoreAfter"`
	Tier         string   `json:"tier"`
	CharsChanged int      `json:"charsChanged"`
}

// CleanupResult is the clean-up report of one Generation. Available false
// means the pass did not run (Reason says why) and the texts are as the
// Rewrite wrote them.
type CleanupResult struct {
	Available bool          `json:"available"`
	Reason    string        `json:"reason,omitempty"`
	Texts     []CleanupText `json:"texts"`
}

type textScore struct {
	Score *float64
	Tier  string
}

// cleaner runs the pinned scripts from skillDir with python.
type cleaner struct {
	python   string
	skillDir string
}

// cleanerFromEnv returns a cleaner, or the reason there is none.
func cleanerFromEnv() (*cleaner, string) {
	dir := strings.TrimSpace(os.Getenv(CleanSkillDirEnv))
	if dir == "" {
		return nil, CleanSkillDirEnv + " is not set"
	}
	for _, s := range []string{"inspect_text.py", "clean_text.py"} {
		if _, err := os.Stat(filepath.Join(dir, "scripts", s)); err != nil {
			return nil, "the clean-up scripts are missing from " + dir
		}
	}
	python := strings.TrimSpace(os.Getenv(CleanPythonEnv))
	if python == "" {
		python = "python3"
	}
	path, err := exec.LookPath(python)
	if err != nil {
		return nil, python + " is not installed"
	}
	return &cleaner{python: path, skillDir: dir}, ""
}

// run executes one script with -I (no user site, no cwd on the import path)
// in a scratch directory and returns stdout and stderr.
func (c *cleaner) run(ctx context.Context, dir, script string, args ...string) ([]byte, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.python, append([]string{"-I", filepath.Join(c.skillDir, "scripts", script)}, args...)...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("%s: %w: %s", script, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func (c *cleaner) score(ctx context.Context, dir, file string) (textScore, error) {
	out, _, err := c.run(ctx, dir, "inspect_text.py", "--stylometry", "--json", file)
	if err != nil {
		return textScore{}, err
	}
	var parsed struct {
		Stylometry struct {
			Score       *float64 `json:"score"`
			DensityTier string   `json:"density_tier"`
		} `json:"stylometry"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return textScore{}, fmt.Errorf("inspect_text.py output: %w", err)
	}
	return textScore{Score: parsed.Stylometry.Score, Tier: parsed.Stylometry.DensityTier}, nil
}

// cleanText scores text, runs the Unicode pass and scores the result.
// Spaces are not normalized (the template and the quality checks decide
// about those), matching the tailor-cv skill.
func (c *cleaner) cleanText(ctx context.Context, kind, text string) (string, CleanupText, error) {
	dir, err := os.MkdirTemp("", "sumisura-clean-")
	if err != nil {
		return "", CleanupText{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	in, out := filepath.Join(dir, "in.txt"), filepath.Join(dir, "out.txt")
	if err := os.WriteFile(in, []byte(text), 0o600); err != nil {
		return "", CleanupText{}, err
	}
	before, err := c.score(ctx, dir, in)
	if err != nil {
		return "", CleanupText{}, err
	}
	_, stderr, err := c.run(ctx, dir, "clean_text.py", in, "-o", out, "--stats", "--no-normalize-spaces")
	if err != nil {
		return "", CleanupText{}, err
	}
	var stats struct {
		Removed  int `json:"removed_count"`
		Replaced int `json:"replaced_count"`
	}
	// --stats prints its JSON on stderr; tolerate other text around it.
	if i, j := bytes.IndexByte(stderr, '{'), bytes.LastIndexByte(stderr, '}'); i >= 0 && j > i {
		if err := json.Unmarshal(stderr[i:j+1], &stats); err != nil {
			return "", CleanupText{}, fmt.Errorf("clean_text.py stats: %w", err)
		}
	}
	cleaned, err := os.ReadFile(out)
	if err != nil {
		return "", CleanupText{}, err
	}
	if !utf8.Valid(cleaned) {
		return "", CleanupText{}, fmt.Errorf("clean_text.py wrote invalid UTF-8")
	}
	after, err := c.score(ctx, dir, out)
	if err != nil {
		return "", CleanupText{}, err
	}
	return string(cleaned), CleanupText{
		Kind:         kind,
		ScoreBefore:  before.Score,
		ScoreAfter:   after.Score,
		Tier:         before.Tier,
		CharsChanged: stats.Removed + stats.Replaced,
	}, nil
}

// cleanBullets cleans all rewritten bullets as one text, one per line, the
// way the skill does, so the score is over enough words to mean something.
// A bullet is left as it is when the pass changes its line structure.
func (c *cleaner) cleanBullets(ctx context.Context, selection *SelectionResult) (CleanupText, error) {
	var lines []string
	for _, e := range selection.Entries {
		for _, b := range e.Bullets {
			if strings.ContainsAny(b.Rewritten, "\r\n") {
				return CleanupText{}, fmt.Errorf("a rewritten bullet spans several lines")
			}
			lines = append(lines, b.Rewritten)
		}
	}
	if len(lines) == 0 {
		return CleanupText{}, fmt.Errorf("no bullets")
	}
	cleaned, res, err := c.cleanText(ctx, CleanupBullets, strings.Join(lines, "\n"))
	if err != nil {
		return CleanupText{}, err
	}
	out := strings.Split(cleaned, "\n")
	if len(out) != len(lines) {
		return CleanupText{}, fmt.Errorf("the Unicode pass changed the number of bullets")
	}
	i := 0
	for ei := range selection.Entries {
		for bi := range selection.Entries[ei].Bullets {
			selection.Entries[ei].Bullets[bi].Rewritten = out[i]
			i++
		}
	}
	return res, nil
}

// cleanupGeneration applies the clean-up to the Selection (bullets and About
// me) and the Cover Letter in place. It never fails the Generation: any
// problem turns into an unavailable result and leaves the text untouched.
func cleanupGeneration(ctx context.Context, selection *SelectionResult, coverLetter *CoverLetterResult) CleanupResult {
	c, reason := cleanerFromEnv()
	if c == nil {
		return CleanupResult{Reason: reason, Texts: []CleanupText{}}
	}

	// Work on copies so a failure half way leaves the originals as written.
	sel := *selection
	sel.Entries = make([]SelectedEntry, len(selection.Entries))
	for i, e := range selection.Entries {
		e.Bullets = append([]SelectedBullet(nil), e.Bullets...)
		sel.Entries[i] = e
	}
	letter := *coverLetter

	var texts []CleanupText
	fail := func(err error) CleanupResult {
		return CleanupResult{Reason: "the clean-up failed: " + err.Error(), Texts: []CleanupText{}}
	}
	if res, err := c.cleanBullets(ctx, &sel); err == nil {
		texts = append(texts, res)
	} else if err.Error() != "no bullets" {
		return fail(err)
	}
	if sel.Summary != "" {
		cleaned, res, err := c.cleanText(ctx, CleanupAboutMe, sel.Summary)
		if err != nil {
			return fail(err)
		}
		sel.Summary = cleaned
		texts = append(texts, res)
	}
	if letter.Body != "" {
		cleaned, res, err := c.cleanText(ctx, CleanupCoverLetter, letter.Body)
		if err != nil {
			return fail(err)
		}
		letter.Body = cleaned
		texts = append(texts, res)
	}

	*selection, *coverLetter = sel, letter
	return CleanupResult{Available: true, Texts: texts}
}
