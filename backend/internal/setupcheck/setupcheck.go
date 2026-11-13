// Package setupcheck answers "is this installation usable?" without calling
// Claude or the network: typst on PATH at the pinned version, a Claude key,
// Master Data present, template/ and output/ in place, the optional prose
// clean-up and LAN mode's token. It only reads; every outside dependency is
// injected through Env so the checks are testable. It never returns the value
// of a secret, only whether it is set.
package setupcheck

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Statuses of an Item.
const (
	StatusOK        = "ok"
	StatusMissing   = "missing"
	StatusNotNeeded = "notNeeded"
)

const troubleshooting = "See Troubleshooting in the docs (troubleshooting.md)."

// Item is one line of the report.
type Item struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Required bool   `json:"required"`
	Hint     string `json:"hint"`
}

// Report is the whole check. Ready is true when no required item is missing.
type Report struct {
	Ready bool   `json:"ready"`
	Items []Item `json:"items"`
}

// Env is everything the checks read from outside.
type Env struct {
	DataDir     string
	ProjectRoot string

	LookPath func(string) (string, error)
	// Run executes a command and returns its combined output.
	Run      func(name string, args ...string) (string, error)
	Stat     func(string) (os.FileInfo, error)
	ReadFile func(string) ([]byte, error)
	ReadDir  func(string) ([]os.DirEntry, error)
	Getenv   func(string) string
	// Writable reports whether files can be created in an existing dir.
	Writable func(dir string) bool
	// LANToken is whether LAN_AUTH_TOKEN is set (the value is never kept).
	LANTokenSet bool
}

// WithDefaults fills every nil dependency with the real one.
func (e Env) WithDefaults() Env {
	if e.LookPath == nil {
		e.LookPath = exec.LookPath
	}
	if e.Run == nil {
		e.Run = func(name string, args ...string) (string, error) {
			out, err := exec.Command(name, args...).CombinedOutput()
			return string(out), err
		}
	}
	if e.Stat == nil {
		e.Stat = os.Stat
	}
	if e.ReadFile == nil {
		e.ReadFile = os.ReadFile
	}
	if e.ReadDir == nil {
		e.ReadDir = os.ReadDir
	}
	if e.Getenv == nil {
		e.Getenv = os.Getenv
	}
	if e.Writable == nil {
		e.Writable = func(dir string) bool {
			f, err := os.CreateTemp(dir, ".setup-check-*")
			if err != nil {
				return false
			}
			name := f.Name()
			if err := f.Close(); err != nil {
				return false
			}
			return os.Remove(name) == nil
		}
	}
	return e
}

// Check runs every check.
func Check(e Env) Report {
	e = e.WithDefaults()
	items := []Item{
		checkTypst(e),
		checkClaudeKey(e),
		checkProfile(e),
		checkEntries(e),
		checkTemplate(e),
		checkOutput(e),
		checkPython(e),
		checkCleanSkill(e),
		checkLAN(e),
	}
	ready := true
	for _, it := range items {
		if it.Required && it.Status == StatusMissing {
			ready = false
		}
	}
	return Report{Ready: ready, Items: items}
}

func ok(id string, required bool, hint string) Item {
	return Item{ID: id, Status: StatusOK, Required: required, Hint: hint}
}
func missing(id string, required bool, hint string) Item {
	return Item{ID: id, Status: StatusMissing, Required: required, Hint: hint}
}
func notNeeded(id string, required bool, hint string) Item {
	return Item{ID: id, Status: StatusNotNeeded, Required: required, Hint: hint}
}

// pinnedTypstVersion reads TYPST_VERSION from typst-version.txt, "" if absent.
func pinnedTypstVersion(e Env) string {
	data, err := e.ReadFile(filepath.Join(e.ProjectRoot, "typst-version.txt"))
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		if v, found := strings.CutPrefix(strings.TrimSpace(sc.Text()), "TYPST_VERSION="); found {
			return strings.Trim(v, `"' `)
		}
	}
	return ""
}

func checkTypst(e Env) Item {
	const id = "typst"
	if _, err := e.LookPath("typst"); err != nil {
		return missing(id, true, "typst is not on PATH, so Render cannot run. Install it and put it on PATH. "+troubleshooting)
	}
	pinned := pinnedTypstVersion(e)
	out, err := e.Run("typst", "--version")
	if err != nil {
		return missing(id, true, "typst is on PATH but `typst --version` failed. "+troubleshooting)
	}
	fields := strings.Fields(out)
	got := ""
	if len(fields) >= 2 {
		got = fields[1]
	}
	if pinned != "" && got != pinned {
		return missing(id, true, "typst "+got+" is installed but typst-version.txt pins "+pinned+"; install that version so the CV stays on one page. "+troubleshooting)
	}
	return ok(id, true, "typst "+got+" is installed.")
}

func checkClaudeKey(e Env) Item {
	const id = "claudeKey"
	if e.Getenv("SUMISURA_CLAUDE_BACKEND") == "claude-code" {
		return notNeeded(id, true, "The claude-code backend uses your own Claude subscription; no API key is needed.")
	}
	if strings.TrimSpace(e.Getenv("ANTHROPIC_API_KEY")) == "" {
		return missing(id, true, "ANTHROPIC_API_KEY is empty, so every Claude call fails. Set it in .env and restart. "+troubleshooting)
	}
	return ok(id, true, "ANTHROPIC_API_KEY is set.")
}

func checkProfile(e Env) Item {
	const id = "profile"
	if _, err := e.Stat(filepath.Join(e.DataDir, "profile.yaml")); err != nil {
		return missing(id, true, "data/profile.yaml is missing. Copy the stubs with `cp -r data/examples/. data/` and edit them (quickstart.md).")
	}
	return ok(id, true, "data/profile.yaml is present.")
}

func checkEntries(e Env) Item {
	const id = "entries"
	for _, dir := range []string{"experience", "projects"} {
		des, err := e.ReadDir(filepath.Join(e.DataDir, dir))
		if err != nil {
			continue
		}
		for _, de := range des {
			if !de.IsDir() && strings.HasSuffix(de.Name(), ".md") {
				return ok(id, true, "At least one Entry exists.")
			}
		}
	}
	return missing(id, true, "There is no Entry yet. Add a Markdown file under data/experience/ or data/projects/ (quickstart.md).")
}

func checkTemplate(e Env) Item {
	const id = "template"
	for _, f := range []string{"cv.typ", "cover-letter.typ"} {
		if _, err := e.Stat(filepath.Join(e.ProjectRoot, "template", f)); err != nil {
			return missing(id, true, "template/"+f+" is not found under the project root ("+e.ProjectRoot+"). Render needs template/ next to data/.")
		}
	}
	return ok(id, true, "template/ is in place.")
}

func checkOutput(e Env) Item {
	const id = "output"
	dir := filepath.Join(e.ProjectRoot, "output")
	if _, err := e.Stat(dir); err != nil {
		dir = e.ProjectRoot
	}
	if !e.Writable(dir) {
		return missing(id, true, dir+" is not writable, so Render cannot save PDFs. Fix its permissions or mount output/ writable.")
	}
	return ok(id, true, "output/ is writable.")
}

func pythonName(e Env) string {
	if p := strings.TrimSpace(e.Getenv("SUMISURA_PYTHON")); p != "" {
		return p
	}
	return "python3"
}

func checkPython(e Env) Item {
	const id = "python"
	name := pythonName(e)
	if _, err := e.LookPath(name); err != nil {
		return missing(id, false, "Optional: "+name+" is not installed; the prose clean-up needs Python 3.10+.")
	}
	out, err := e.Run(name, "-c", "import sys; print(sys.version_info >= (3, 10))")
	if err != nil || strings.TrimSpace(out) != "True" {
		return missing(id, false, "Optional: the prose clean-up needs Python 3.10 or newer.")
	}
	return ok(id, false, "Python 3.10+ is available.")
}

func checkCleanSkill(e Env) Item {
	const id = "cleanSkill"
	dir := strings.TrimSpace(e.Getenv("SUMISURA_CLEAN_SKILL_DIR"))
	if dir == "" {
		return missing(id, false, "Optional: SUMISURA_CLEAN_SKILL_DIR is not set, so Generations skip the prose clean-up. The release image sets it.")
	}
	for _, s := range []string{"inspect_text.py", "clean_text.py"} {
		if _, err := e.Stat(filepath.Join(dir, "scripts", s)); err != nil {
			return missing(id, false, "Optional: the clean-up scripts are missing from "+dir+".")
		}
	}
	return ok(id, false, "The prose clean-up skill is available.")
}

func isLocalAddr(a string) bool {
	switch strings.TrimSpace(a) {
	case "", "127.0.0.1", "localhost", "::1", "[::1]":
		return true
	}
	return strings.HasPrefix(strings.TrimSpace(a), "127.")
}

func checkLAN(e Env) Item {
	const id = "lan"
	if isLocalAddr(e.Getenv("BIND_ADDR")) {
		return notNeeded(id, true, "The app is reachable from this machine only.")
	}
	if !e.LANTokenSet {
		return missing(id, true, "BIND_ADDR exposes the app to the network but LAN_AUTH_TOKEN is empty. Set a token (lan-mode.md) or bind 127.0.0.1.")
	}
	return ok(id, true, "LAN mode has an access token.")
}
