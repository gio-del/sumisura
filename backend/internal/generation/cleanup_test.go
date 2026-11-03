package generation

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSkill writes stand-ins for the two upstream scripts: the scorer
// reports the word count as the score, the cleaner drops U+200B and prints
// the stats JSON on stderr, as the real ones do.
func fakeSkill(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	dir := t.TempDir()
	scripts := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	inspect := `import json, sys
text = open(sys.argv[-1], encoding="utf-8").read()
words = len(text.split())
print(json.dumps({"stylometry": {"score": float(words) if words >= 3 else None, "density_tier": "low" if words >= 3 else "uncalibrated"}}))
`
	clean := `import json, sys
args = sys.argv[1:]
src, out = args[0], args[args.index("-o") + 1]
text = open(src, encoding="utf-8").read()
cleaned = text.replace("​", "")
open(out, "w", encoding="utf-8").write(cleaned)
print(json.dumps({"removed_count": len(text) - len(cleaned), "replaced_count": 0}), file=sys.stderr)
`
	for name, body := range map[string]string{"inspect_text.py": inspect, "clean_text.py": clean} {
		if err := os.WriteFile(filepath.Join(scripts, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(CleanSkillDirEnv, dir)
}

func TestCleanupGenerationStripsInvisibleCharacters(t *testing.T) {
	fakeSkill(t)
	selection := SelectionResult{
		Summary: "Backend​ engineer with years of Go",
		Entries: []SelectedEntry{
			{EntryID: "a", Bullets: []SelectedBullet{{Rewritten: "Built Post​greSQL pipelines daily"}, {Rewritten: "Led a small team of four"}}},
			{EntryID: "b", Bullets: []SelectedBullet{{Rewritten: "Cut costs by a third"}}},
		},
	}
	letter := CoverLetterResult{Body: "Dear team,\nI would like to join you."}

	res := cleanupGeneration(context.Background(), &selection, &letter)

	if !res.Available || res.Reason != "" {
		t.Fatalf("want an available result, got %+v", res)
	}
	if got := selection.Entries[0].Bullets[0].Rewritten; got != "Built PostgreSQL pipelines daily" {
		t.Errorf("bullet not cleaned: %q", got)
	}
	if got := selection.Entries[1].Bullets[0].Rewritten; got != "Cut costs by a third" {
		t.Errorf("bullet order broke: %q", got)
	}
	if strings.Contains(selection.Summary, "​") {
		t.Errorf("summary not cleaned: %q", selection.Summary)
	}
	kinds := []string{}
	for _, tx := range res.Texts {
		kinds = append(kinds, tx.Kind)
	}
	if strings.Join(kinds, ",") != "bullets,aboutMe,coverLetter" {
		t.Fatalf("kinds = %v", kinds)
	}
	if res.Texts[0].CharsChanged != 1 || res.Texts[1].CharsChanged != 1 || res.Texts[2].CharsChanged != 0 {
		t.Errorf("chars changed = %+v", res.Texts)
	}
	if res.Texts[0].ScoreBefore == nil || res.Texts[0].Tier != "low" {
		t.Errorf("score/tier missing: %+v", res.Texts[0])
	}
}

func TestCleanupGenerationUnavailableLeavesTextAlone(t *testing.T) {
	t.Setenv(CleanSkillDirEnv, "")
	selection := SelectionResult{Entries: []SelectedEntry{{Bullets: []SelectedBullet{{Rewritten: "a​b"}}}}}
	letter := CoverLetterResult{Body: "x"}

	res := cleanupGeneration(context.Background(), &selection, &letter)

	if res.Available || res.Reason == "" || res.Texts == nil {
		t.Errorf("want unavailable with a reason and an empty list, got %+v", res)
	}
	if selection.Entries[0].Bullets[0].Rewritten != "a​b" {
		t.Error("text changed although the pass did not run")
	}
}

func TestCleanupGenerationMissingPythonIsUnavailable(t *testing.T) {
	fakeSkill(t)
	t.Setenv(CleanPythonEnv, "definitely-not-a-python")
	selection := SelectionResult{}
	res := cleanupGeneration(context.Background(), &selection, &CoverLetterResult{})
	if res.Available || !strings.Contains(res.Reason, "definitely-not-a-python") {
		t.Errorf("got %+v", res)
	}
}

func TestCleanupGenerationFailureKeepsOriginals(t *testing.T) {
	fakeSkill(t)
	selection := SelectionResult{Entries: []SelectedEntry{{Bullets: []SelectedBullet{{Rewritten: "one​ two three"}}}}, Summary: "s​"}
	letter := CoverLetterResult{Body: "ok"}
	// Break the cleaner after the bullets succeed: an unreadable scorer.
	os.WriteFile(filepath.Join(os.Getenv(CleanSkillDirEnv), "scripts", "inspect_text.py"), []byte("raise SystemExit(3)\n"), 0o644)

	res := cleanupGeneration(context.Background(), &selection, &letter)

	if res.Available || !strings.Contains(res.Reason, "failed") {
		t.Errorf("got %+v", res)
	}
	if selection.Entries[0].Bullets[0].Rewritten != "one​ two three" || selection.Summary != "s​" {
		t.Error("originals were modified by a failed pass")
	}
}
