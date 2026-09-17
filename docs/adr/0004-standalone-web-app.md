# Standalone local web app, superseding the no-runtime stance

Adding a FE for browsing/tracking Job Listings and Applications, plus a browser extension to feed it, requires a real running app — not something a Claude Code conversation can drive. We're building a standalone local web app (backend + FE, run via docker-compose, localhost-only, no auth) that owns Job Listings, Applications, and Master Data. This directly reverses ADR-0001's "no Node.js/JS runtime" stance: that decision was correct for a conversational, skill-only tool, but a FE and browser extension need one. ADR-0001 is superseded by this decision.

## Update: opt-in LAN-reachable exception (issue #57)

"Localhost-only, no auth" remains the default. Issue #57 adds an explicit, opt-in LAN-reachable mode (a `lan` docker-compose profile plus `BIND_ADDR`/`LAN_AUTH_TOKEN` env vars) so the app can be reached from other devices on the same network, gated behind a shared-secret token check. This is a deliberate, opt-in exception to this ADR's default posture for the case where the user chooses it — not a reversal of the default case, which is unchanged.

## Update: remote access through a private network only (issue #180)

Reaching the app from outside the home network is supported only through a private overlay network with the app still bound to loopback — see ADR-0040. Public exposure (port forwarding, a public reverse proxy, `tailscale funnel`) stays unsupported.
