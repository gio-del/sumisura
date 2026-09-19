package docsdrift

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// repoRoot is the checkout this test runs in, from backend/internal/docsdrift.
const repoRoot = "../../.."

const (
	apiPage        = "site/src/content/docs/api.md"
	configPage     = "site/src/content/docs/configuration.md"
	tailoringPage  = "site/src/content/docs/tailoring.md"
	upgradingPage  = "site/src/content/docs/upgrading.md"
	routerSource   = "backend/internal/api/router.go"
	modelsSource   = "backend/internal/claude/models.go"
	cvcheckSource  = "backend/cmd/cvcheck/main.go"
	migrateSource  = "backend/cmd/migrate-records/main.go"
	envExampleFile = ".env.example"
)

// exemptEnvVars are variables read or forwarded somewhere that a
// self-hoster never sets, each with why. Switches read only from tests,
// like UPDATE_CONTRACT_FIXTURES, are never collected, so need no entry.
var exemptEnvVars = map[string]string{}

// exemptRoutes are patterns router.go registers that are not routes anyone
// calls, each with why.
var exemptRoutes = map[string]string{
	"/api/": "the catch-all that answers 404 for an unrouted /api path; not a route to call",
}

// TestDocsMatchCode is the drift check itself: every route, environment
// variable and shipped-CLI flag the code has is on its docs page, and
// every one a page documents still exists.
func TestDocsMatchCode(t *testing.T) {
	surfaces := []Surface{
		{
			Kind:       "route",
			Page:       apiPage,
			Items:      routes(t),
			Documented: InlineCode,
			Mentioned:  RouteMentions,
			Exempt:     exemptRoutes,
		},
		{
			Kind:       "environment variable",
			Page:       configPage,
			Items:      envVars(t),
			Documented: InlineCode,
			Mentioned:  EnvVarMentions,
			Exempt:     exemptEnvVars,
		},
		{
			Kind:       "cvcheck flag",
			Page:       tailoringPage,
			Items:      flags(t, cvcheckSource),
			Documented: Flag,
		},
		{
			Kind:       "migrate-records flag",
			Page:       upgradingPage,
			Items:      flags(t, migrateSource),
			Documented: Flag,
		},
	}
	for _, s := range surfaces {
		if len(s.Items) == 0 {
			t.Fatalf("found no %ss in the code: the collector no longer matches the source", s.Kind)
		}
		for _, problem := range Check(s, read(t, s.Page)) {
			t.Error(problem)
		}
	}
}

func TestCheck(t *testing.T) {
	surface := func(items []string, exempt map[string]string) Surface {
		return Surface{
			Kind: "route", Page: "api.md", Items: items,
			Documented: InlineCode, Mentioned: RouteMentions, Exempt: exempt,
		}
	}
	tests := []struct {
		name    string
		surface Surface
		page    string
		want    []string
	}{
		{
			name:    "everything documented",
			surface: surface([]string{"GET /api/a"}, nil),
			page:    "| `GET /api/a` | reads a |",
		},
		{
			name:    "a new route without docs",
			surface: surface([]string{"GET /api/a", "POST /api/b"}, nil),
			page:    "`GET /api/a`",
			want:    []string{`route "POST /api/b" is not documented: add it to api.md, or exempt it with a reason`},
		},
		{
			name:    "a documented route the code dropped",
			surface: surface([]string{"GET /api/a"}, nil),
			page:    "`GET /api/a` and `DELETE /api/gone`",
			want:    []string{`api.md documents route "DELETE /api/gone", which the code no longer has: remove it`},
		},
		{
			name:    "a reasoned exemption",
			surface: surface([]string{"GET /api/a", "GET /api/internal"}, map[string]string{"GET /api/internal": "test only"}),
			page:    "`GET /api/a`",
		},
		{
			name:    "an exemption without a reason",
			surface: surface([]string{"GET /api/a"}, map[string]string{"GET /api/a": ""}),
			page:    "`GET /api/a`",
			want:    []string{`route "GET /api/a" is exempt without a reason: say why it stays out of api.md`},
		},
		{
			name:    "an exemption for something removed",
			surface: surface(nil, map[string]string{"GET /api/gone": "was internal"}),
			want:    []string{`route "GET /api/gone" is exempt but the code no longer has it: drop the exemption`},
		},
		{
			name:    "a mention outside inline code does not count",
			surface: surface([]string{"GET /api/a"}, nil),
			page:    "call GET /api/a",
			want:    []string{`route "GET /api/a" is not documented: add it to api.md, or exempt it with a reason`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Check(tt.surface, tt.page); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Check() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFlag(t *testing.T) {
	page := "run `cvcheck pdf --pdf x --data-dir data` or `-write`"
	for name, want := range map[string]bool{"pdf": true, "data-dir": true, "write": true, "data": false, "json": false} {
		if got := Flag(page, name); got != want {
			t.Errorf("Flag(%q) = %v, want %v", name, got, want)
		}
	}
}

// routes are the patterns router.go registers, the same way the contract
// test finds them.
func routes(t *testing.T) []string {
	return matches(t, regexp.MustCompile(`mux\.HandleFunc\("([^"]+)"`), read(t, routerSource))
}

// envVars are every variable the backend reads by literal name, every
// per-call-site model override the claude package derives, every variable
// a compose file forwards and every key .env.example offers.
func envVars(t *testing.T) []string {
	set := map[string]bool{}
	add := func(names ...string) {
		for _, n := range names {
			set[n] = true
		}
	}
	err := filepath.WalkDir(filepath.Join(repoRoot, "backend"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		add(matches(t, regexp.MustCompile(`os\.(?:Getenv|LookupEnv)\("([A-Z0-9_]+)"\)`), string(content))...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	models := read(t, modelsSource)
	for _, site := range matches(t, regexp.MustCompile(`callSite\s*=\s*"([a-z_]+)"`), models) {
		add("SUMISURA_MODEL_" + strings.ToUpper(site))
	}
	add(matches(t, regexp.MustCompile(`defaultModelEnvVar\s*=\s*"([A-Z_]+)"`), models)...)

	composeFiles, err := filepath.Glob(filepath.Join(repoRoot, "docker-compose*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range composeFiles {
		add(matches(t, regexp.MustCompile(`\$\{([A-Z0-9_]+)`), readPath(t, f))...)
	}
	add(matches(t, regexp.MustCompile(`(?m)^#?\s*([A-Z][A-Z0-9_]+)=`), read(t, envExampleFile))...)

	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// flags are the flags a shipped command defines on its flag.FlagSet.
func flags(t *testing.T, source string) []string {
	return matches(t, regexp.MustCompile(`\b(?:fs|flags)\.(?:String|Bool|Int|Duration)\("([^"]+)"`), read(t, source))
}

func matches(t *testing.T, re *regexp.Regexp, text string) []string {
	t.Helper()
	seen := map[string]bool{}
	var names []string
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
		}
	}
	return names
}

func read(t *testing.T, rel string) string {
	t.Helper()
	return readPath(t, filepath.Join(repoRoot, rel))
}

func readPath(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
