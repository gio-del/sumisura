package api

import (
	"log"
	"net/http"
	"time"

	"github.com/gio-del/sumisura/backend/internal/atsboard"
	"github.com/gio-del/sumisura/backend/internal/claude"
	"github.com/gio-del/sumisura/backend/internal/tracking"
)

// RouterConfig holds everything NewRouter needs to build the app's API
// handler. Only DataDir is required; the zero value of every other field
// selects the default documented on it, so a call site names the
// dependencies it actually controls and stays silent about the rest.
type RouterConfig struct {
	// DataDir is the root directory containing profile.yaml, experience/,
	// projects/ and cover-letter-snippets/ — the same Master Data files the
	// tailor-cv skill reads and writes, alongside the tracked Job Listings
	// and Applications. Required — it is the one field with no sensible
	// default, so NewRouter panics on an empty DataDir rather than
	// silently rooting the app at the process working directory and
	// serving an empty Master Data set that would read as data loss.
	DataDir string

	// ProjectRoot is the directory holding template/, output/ and data/,
	// which Render passes to typst as its single --root (ADR-0012).
	// Empty means ".", the process working directory.
	ProjectRoot string

	// Addr is the TCP address NewServer listens on; it does nothing for a
	// NewRouter-only call site. Empty means DefaultAddr
	// ("0.0.0.0:8080") — which is not the LAN-reachable opt-in, since
	// docker-compose.yml's port mapping is what decides reachability
	// (issue #57, ADR-0004).
	Addr string

	// GenerationClient backs Generation, RAL estimation and the other
	// Claude-API-facing routes (tracking.Client embeds generation.Client).
	// Nil means a real claude.New() client, which calls the Claude API
	// directly (ADR-0005) using ANTHROPIC_API_KEY from the environment.
	// Tests inject a fake here.
	GenerationClient tracking.Client

	// ATSHTTPDoer performs the outbound calls to public ATS job boards
	// (Greenhouse/Lever/Ashby) made by job-board aggregation and the
	// freshness recheck (ADR-0007). Nil means http.DefaultClient. Tests
	// that must not reach a live board inject a fake here.
	ATSHTTPDoer atsboard.HTTPDoer

	// ClaudeRouteTimeout bounds a single request on the Claude-calling
	// routes (Generation, Job Listing save, resolve, suggest-contact) via
	// a request context deadline — the bound that stands in for the
	// http.Server WriteTimeout NewServer deliberately leaves off. Zero
	// means DefaultClaudeRouteTimeout (10 minutes). Tests inject a short
	// value here; the other routes are never wrapped.
	ClaudeRouteTimeout time.Duration

	// LANAuthToken opts the handler into LAN-reachable mode's auth gate
	// (issue #57, see lan_auth.go): once non-empty, every /api/* route
	// requires it via the X-Sumisura-Token header or the access cookie a
	// browser gets from POST /api/auth/session (issue #181). Empty — the default
	// — leaves every route unwrapped with no check wired in at all,
	// preserving ADR-0004's localhost-only, no-auth default exactly.
	LANAuthToken string

	// StaticDir is the directory holding the production frontend build
	// (index.html plus hashed assets/) for the backend to serve alongside
	// the API — one image, one port, no CORS (ADR-0039). Empty — the
	// default, and what the dev compose file uses — serves no static files
	// at all, leaving the Vite dev server in charge of the frontend.
	StaticDir string
}

// NewRouter builds the HTTP handler for the app's API from cfg. See
// RouterConfig for what each field controls and what its zero value
// defaults to.
func NewRouter(cfg RouterConfig) http.Handler {
	dataDir := cfg.DataDir
	if dataDir == "" {
		panic("api.NewRouter: RouterConfig.DataDir is required")
	}
	projectRoot := cfg.ProjectRoot
	if projectRoot == "" {
		projectRoot = "."
	}
	generationClient := cfg.GenerationClient
	if generationClient == nil {
		generationClient = claude.New()
	}
	atsHTTPDoer := cfg.ATSHTTPDoer
	if atsHTTPDoer == nil {
		atsHTTPDoer = http.DefaultClient
	}
	lanAuthToken := cfg.LANAuthToken
	// claudeRoute marks the routes that call the Claude API, the only ones
	// that can legitimately run for minutes and so the only ones that need
	// a bound of their own. Everything else — Master Data browsing,
	// file-serving, the tracking reads and writes — stays unwrapped.
	claudeRoute := func(h http.HandlerFunc) http.HandlerFunc {
		return withRequestDeadline(cfg.ClaudeRouteTimeout, h)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", healthHandler)
	mux.HandleFunc("GET /api/auth/status", authStatusHandler(lanAuthToken))
	mux.HandleFunc("POST /api/auth/session", createAuthSessionHandler(lanAuthToken))
	mux.HandleFunc("DELETE /api/auth/session", deleteAuthSessionHandler)
	mux.HandleFunc("GET /api/export", exportDataHandler(dataDir))
	mux.HandleFunc("GET /api/master-data/entries", listEntriesHandler(dataDir, projectRoot))
	mux.HandleFunc("POST /api/master-data/entries", createEntryHandler(dataDir))
	mux.HandleFunc("GET /api/master-data/entries/{id...}", getEntryHandler(dataDir, projectRoot))
	mux.HandleFunc("PUT /api/master-data/entries/{id...}", putEntryHandler(dataDir))
	mux.HandleFunc("DELETE /api/master-data/entries/{id...}", deleteEntryHandler(dataDir))
	mux.HandleFunc("GET /api/master-data/profile", getProfileHandler(dataDir))
	mux.HandleFunc("PUT /api/master-data/profile", putProfileHandler(dataDir))
	mux.HandleFunc("GET /api/master-data/tag-lint", tagLintHandler(dataDir))
	mux.HandleFunc("GET /api/master-data/cover-letter-snippets", listSnippetsHandler(dataDir))
	mux.HandleFunc("POST /api/master-data/cover-letter-snippets", createSnippetHandler(dataDir))
	mux.HandleFunc("GET /api/master-data/cover-letter-snippets/{id...}", getSnippetHandler(dataDir))
	mux.HandleFunc("PUT /api/master-data/cover-letter-snippets/{id...}", putSnippetHandler(dataDir))
	mux.HandleFunc("DELETE /api/master-data/cover-letter-snippets/{id...}", deleteSnippetHandler(dataDir))
	mux.HandleFunc("GET /api/job-listings", listJobListingsHandler(dataDir, projectRoot))
	mux.HandleFunc("POST /api/job-listings", claudeRoute(createJobListingHandler(dataDir, generationClient, atsHTTPDoer)))
	mux.HandleFunc("POST /api/job-listings/from-extension", claudeRoute(captureJobListingFromExtensionHandler(dataDir, generationClient, atsHTTPDoer)))
	mux.HandleFunc("OPTIONS /api/job-listings/from-extension", captureJobListingCORSPreflightHandler)
	mux.HandleFunc("POST /api/job-listings/capture-lookup", captureLookupHandler(dataDir))
	mux.HandleFunc("OPTIONS /api/job-listings/capture-lookup", captureLookupCORSPreflightHandler)
	mux.HandleFunc("GET /api/job-listings/{id}", getJobListingHandler(dataDir, projectRoot))
	mux.HandleFunc("DELETE /api/job-listings/{id}", deleteJobListingHandler(dataDir))
	mux.HandleFunc("GET /api/job-listings/{id}/logo", getJobListingLogoHandler(dataDir))
	mux.HandleFunc("POST /api/job-listings/{id}/suggest-contact", claudeRoute(suggestContactHandler(dataDir, generationClient)))
	mux.HandleFunc("POST /api/job-listings/{id}/resolve", claudeRoute(resolveJobListingHandler(dataDir, generationClient)))
	mux.HandleFunc("POST /api/job-listings/{id}/check-freshness", checkFreshnessHandler(dataDir, atsHTTPDoer))
	mux.HandleFunc("POST /api/job-listings/{id}/archive", setJobListingArchivedHandler(dataDir, true))
	mux.HandleFunc("POST /api/job-listings/{id}/unarchive", setJobListingArchivedHandler(dataDir, false))
	mux.HandleFunc("GET /api/pending-captures", listPendingCapturesHandler(dataDir))
	mux.HandleFunc("POST /api/pending-captures", claudeRoute(addPendingCaptureHandler(dataDir, generationClient, atsHTTPDoer)))
	mux.HandleFunc("DELETE /api/pending-captures/{id}", deletePendingCaptureHandler(dataDir))
	mux.HandleFunc("POST /api/pending-captures/{id}/complete", claudeRoute(completePendingCaptureHandler(dataDir, generationClient, atsHTTPDoer)))
	mux.HandleFunc("POST /api/pending-captures/{id}/hints", claudeRoute(suggestPendingCaptureHintsHandler(dataDir, generationClient)))
	mux.HandleFunc("GET /api/applications", listApplicationsHandler(dataDir))
	mux.HandleFunc("GET /api/applications/stats", getApplicationsStatsHandler(dataDir))
	mux.HandleFunc("PATCH /api/applications/{id}/status", updateApplicationStatusHandler(dataDir))
	mux.HandleFunc("OPTIONS /api/applications/{id}/status", corsPreflight("PATCH"))
	mux.HandleFunc("PATCH /api/applications/{id}/method", updateApplicationMethodHandler(dataDir))
	mux.HandleFunc("PATCH /api/applications/{id}/contact", updateApplicationContactHandler(dataDir))
	mux.HandleFunc("GET /api/applications/{id}/mailto", getApplicationMailtoHandler(dataDir))
	mux.HandleFunc("POST /api/applications/{id}/generations", recordApplicationGenerationHandler(dataDir))
	mux.HandleFunc("POST /api/applications/{id}/notes", addApplicationNoteHandler(dataDir))
	mux.HandleFunc("PATCH /api/applications/{id}/notes/{noteId}", editApplicationNoteHandler(dataDir))
	mux.HandleFunc("DELETE /api/applications/{id}/notes/{noteId}", deleteApplicationNoteHandler(dataDir))
	mux.HandleFunc("POST /api/generations", claudeRoute(createGenerationHandler(dataDir, generationClient)))
	mux.HandleFunc("POST /api/generations/preview", claudeRoute(previewGenerationHandler(dataDir, generationClient)))
	mux.HandleFunc("POST /api/generations/render", renderGenerationHandler(dataDir, projectRoot))
	mux.HandleFunc("GET /api/generations", listGenerationsHandler(dataDir, projectRoot))
	mux.HandleFunc("GET /api/generations/{slug}/ats-report", getATSReportsHandler(dataDir, projectRoot))
	mux.HandleFunc("GET /api/generations/{slug}/{file}", getGenerationFileHandler(projectRoot))
	mux.HandleFunc("DELETE /api/generations/{slug}", deleteGenerationHandler(projectRoot))
	mux.HandleFunc("GET /api/ats/{provider}/{slug}/listings", listAtsListingsHandler(dataDir, atsHTTPDoer))
	mux.HandleFunc("GET /api/ats/tracked-boards", listTrackedBoardsHandler(dataDir, atsHTTPDoer))
	mux.HandleFunc("POST /api/ats/tracked-boards", createTrackedBoardHandler(dataDir))
	mux.HandleFunc("DELETE /api/ats/tracked-boards/{id}", deleteTrackedBoardHandler(dataDir))
	mux.HandleFunc("GET /api/usage", getUsageHandler(dataDir))

	// An unrouted /api/ path must answer 404, not fall through to the SPA
	// shell below. Registered even without StaticDir so the two modes give
	// the same answer to a mistyped API call.
	mux.HandleFunc("/api/", apiNotFoundHandler)

	var handler http.Handler = mux
	if cfg.StaticDir != "" {
		mux.Handle("/", staticHandler(cfg.StaticDir))
	}
	if lanAuthToken == "" {
		return handler
	}
	return requireLANToken(lanAuthToken, handler)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Printf("api: writing health response body: %v", err)
	}
}
