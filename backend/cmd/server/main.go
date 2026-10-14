package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gio-del/sumisura/backend/internal/api"
	"github.com/gio-del/sumisura/backend/internal/claude"
	"github.com/gio-del/sumisura/backend/internal/version"
)

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	projectRoot := os.Getenv("PROJECT_ROOT")
	if projectRoot == "" {
		projectRoot = "."
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	// LISTEN_HOST is the address the process binds. Inside a container it
	// stays 0.0.0.0, since Docker's port mapping (BIND_ADDR in the compose
	// files) is what decides who can reach it. Running the binary directly on
	// a host behind a reverse proxy, set 127.0.0.1 so only the proxy can.
	listenHost := os.Getenv("LISTEN_HOST")
	if listenHost == "" {
		listenHost = "0.0.0.0"
	}
	// LAN_AUTH_TOKEN being set is what makes LAN-reachable mode "active" from
	// the backend's point of view — the bind-address switch itself
	// (BIND_ADDR) lives entirely in docker-compose.yml's port mapping, which
	// the process inside the container can't observe (see issue #57).
	lanAuthToken := os.Getenv("LAN_AUTH_TOKEN")
	// LAN_AUTH_USER + LAN_AUTH_PASSWORD let a browser sign in with a
	// username and password (kept in a password manager) instead of the
	// token; the token still works for the extension and the skill.
	lanAuthUser := os.Getenv("LAN_AUTH_USER")
	lanAuthPassword := os.Getenv("LAN_AUTH_PASSWORD")
	// STATIC_DIR is set by the release image, which ships the production
	// frontend build beside the binary and serves both from one port
	// (ADR-0039). Unset — the development default — leaves the frontend to
	// the Vite dev server, exactly as before.
	staticDir := os.Getenv("STATIC_DIR")

	srv := api.NewServer(api.RouterConfig{
		Addr:             net.JoinHostPort(listenHost, port),
		DataDir:          dataDir,
		ProjectRoot:      projectRoot,
		GenerationClient: claude.New(),
		ATSHTTPDoer:      http.DefaultClient,
		LANAuthToken:     lanAuthToken,
		LANAuthUser:      lanAuthUser,
		LANAuthPassword:  lanAuthPassword,
		StaticDir:        staticDir,
	})

	if lanAuthToken != "" {
		log.Printf("LAN-reachable mode: binding %s, auth required", srv.Addr)
		if (lanAuthUser == "") != (lanAuthPassword == "") {
			log.Printf("LAN_AUTH_USER and LAN_AUTH_PASSWORD must be set together: password sign-in is off")
		}
	}
	log.Printf("sumisura %s backend listening on %s (data dir: %s, project root: %s)", version.Version, srv.Addr, dataDir, projectRoot)
	// SIGINT (Ctrl-C in a local `go run`) and SIGTERM (`docker compose
	// down`, a restart to pick up an .env change) both reach here: the
	// Dockerfile's exec-form ENTRYPOINT makes the server PID 1, so the
	// signal arrives directly with no shell in between. Run turns either
	// one into a bounded drain instead of instant death.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := api.Run(ctx, srv, api.DefaultDrainTimeout); err != nil {
		log.Fatal(err)
	}
}
