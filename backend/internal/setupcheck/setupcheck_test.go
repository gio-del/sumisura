package setupcheck

import (
	"errors"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func fakeEnv(files map[string]string, env map[string]string) Env {
	fsys := fstest.MapFS{}
	for p, c := range files {
		fsys[p] = &fstest.MapFile{Data: []byte(c)}
	}
	return Env{
		DataDir:     "data",
		ProjectRoot: ".",
		LookPath:    func(string) (string, error) { return "/bin/x", nil },
		Run: func(name string, args ...string) (string, error) {
			if name == "typst" {
				return "typst 0.15.1 (abc)\n", nil
			}
			return "True\n", nil
		},
		Stat:     func(p string) (os.FileInfo, error) { return fsys.Stat(strings.TrimPrefix(p, "./")) },
		ReadFile: func(p string) ([]byte, error) { return fsys.ReadFile(strings.TrimPrefix(p, "./")) },
		ReadDir:  func(p string) ([]os.DirEntry, error) { return fsys.ReadDir(strings.TrimPrefix(p, "./")) },
		Getenv:   func(k string) string { return env[k] },
		Writable: func(string) bool { return true },
	}
}

var goodFiles = map[string]string{
	"typst-version.txt":         "TYPST_VERSION=0.15.1\n",
	"data/profile.yaml":         "name: A\n",
	"data/experience/a.md":      "x",
	"template/cv.typ":           "x",
	"template/cover-letter.typ": "x",
	"output/.keep":              "",
}

func find(r Report, id string) Item {
	for _, it := range r.Items {
		if it.ID == id {
			return it
		}
	}
	return Item{}
}

func TestAllGood(t *testing.T) {
	r := Check(fakeEnv(goodFiles, map[string]string{"ANTHROPIC_API_KEY": "sk-secret"}))
	if !r.Ready {
		t.Fatalf("not ready: %+v", r)
	}
	for _, id := range []string{"typst", "claudeKey", "profile", "entries", "template", "output"} {
		if it := find(r, id); it.Status != StatusOK {
			t.Errorf("%s = %+v", id, it)
		}
	}
	for _, it := range r.Items {
		if strings.Contains(it.Hint, "sk-secret") {
			t.Errorf("hint leaks the key: %q", it.Hint)
		}
	}
}

func TestTypstMissingAndWrongVersion(t *testing.T) {
	e := fakeEnv(goodFiles, map[string]string{"ANTHROPIC_API_KEY": "k"})
	e.LookPath = func(string) (string, error) { return "", errors.New("nope") }
	r := Check(e)
	it := find(r, "typst")
	if it.Status != StatusMissing || !strings.Contains(it.Hint, "troubleshooting.md") || r.Ready {
		t.Fatalf("%+v ready=%v", it, r.Ready)
	}

	e = fakeEnv(goodFiles, map[string]string{"ANTHROPIC_API_KEY": "k"})
	e.Run = func(string, ...string) (string, error) { return "typst 0.14.0 (abc)", nil }
	if it := find(Check(e), "typst"); it.Status != StatusMissing || !strings.Contains(it.Hint, "0.15.1") {
		t.Fatalf("%+v", it)
	}
}

func TestClaudeKey(t *testing.T) {
	if it := find(Check(fakeEnv(goodFiles, nil)), "claudeKey"); it.Status != StatusMissing {
		t.Fatalf("%+v", it)
	}
	if it := find(Check(fakeEnv(goodFiles, map[string]string{"SUMISURA_CLAUDE_BACKEND": "claude-code"})), "claudeKey"); it.Status != StatusNotNeeded {
		t.Fatalf("%+v", it)
	}
}

func TestDataItems(t *testing.T) {
	files := map[string]string{"template/cv.typ": "x"}
	r := Check(fakeEnv(files, nil))
	for _, id := range []string{"profile", "entries", "template"} {
		if find(r, id).Status != StatusMissing {
			t.Errorf("%s should be missing", id)
		}
	}
	e := fakeEnv(goodFiles, nil)
	e.Writable = func(string) bool { return false }
	if find(Check(e), "output").Status != StatusMissing {
		t.Error("output should be missing")
	}
}

func TestOptionalItemsDoNotBlock(t *testing.T) {
	e := fakeEnv(goodFiles, map[string]string{"ANTHROPIC_API_KEY": "k"})
	e.Run = func(name string, args ...string) (string, error) {
		if name == "typst" {
			return "typst 0.15.1", nil
		}
		return "False", nil
	}
	r := Check(e)
	if find(r, "python").Status != StatusMissing || find(r, "cleanSkill").Status != StatusMissing {
		t.Fatalf("%+v", r)
	}
	if !r.Ready {
		t.Fatal("optional items must not make the install not ready")
	}
	e.Getenv = func(k string) string {
		return map[string]string{"ANTHROPIC_API_KEY": "k", "SUMISURA_CLEAN_SKILL_DIR": "skill"}[k]
	}
	e.Stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	if find(Check(e), "cleanSkill").Status != StatusMissing {
		t.Error("scripts absent should be missing")
	}
}

func TestLAN(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "k", "BIND_ADDR": "0.0.0.0"}
	r := Check(fakeEnv(goodFiles, env))
	if find(r, "lan").Status != StatusMissing || r.Ready {
		t.Fatalf("%+v", r)
	}
	e := fakeEnv(goodFiles, env)
	e.LANTokenSet = true
	if find(Check(e), "lan").Status != StatusOK {
		t.Error("token set should be ok")
	}
	if find(Check(fakeEnv(goodFiles, map[string]string{"BIND_ADDR": "127.0.0.1"})), "lan").Status != StatusNotNeeded {
		t.Error("local bind needs no token")
	}
}
