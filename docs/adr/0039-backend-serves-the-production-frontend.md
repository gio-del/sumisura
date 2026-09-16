# ADR-0039: The backend serves the production frontend; releases ship one image

- Status: accepted
- Date: 2026-09-16
- Relates to: [ADR-0004](0004-standalone-web-app.md), [ADR-0009](0009-react-typescript-vite-frontend.md), [ADR-0012](0012-render-shells-out-to-typst-in-container.md)

## Context

"Self-hostable" meant cloning the repo and running what is, honestly, a
development environment:

- `docker-compose.yml` builds the backend from source on the user's machine.
- `frontend/Dockerfile` runs `npm install && npm run dev` — the **Vite dev
  server** — with the source tree bind-mounted over `/app`.

So running Sumisura required Node, Go's build step, a full checkout and a slow
first start, and what it exposed as "the app" was a dev server with a proxy in
front of the API. That is fine for the maintainer and a real barrier for
everyone else. It is also the wrong base for a hosted deployment later.

Two services also means two ports and a cross-origin boundary that exists only
because of how the app is built, not because anything needs it.

## Decision

**The Go backend serves the production frontend build as static files, and
releases publish one multi-arch image containing both.**

- `RouterConfig.StaticDir` (env `STATIC_DIR`) points at the built frontend.
  Empty — the default, and what the dev compose file uses — serves nothing at
  all, so development is untouched.
- Non-`/api` paths fall back to `index.html`, so client-side routes like
  `/generations` survive a reload or a shared link. `assets/` is fingerprinted
  by Vite and served `immutable`; `index.html` is served `no-cache`, because it
  names the current bundles.
- An unrouted `/api/` path answers a JSON `404`, registered whether or not
  static serving is on. Without it the SPA fallback would answer a mistyped API
  call with `200` and an HTML body — the hardest possible failure to diagnose
  from the client.
- `ghcr.io/gio-del/sumisura:<version>` and `:latest`, `linux/amd64` +
  `linux/arm64`, built on `release: published` beside the extension zip. The
  image carries the same pinned `typst` and `poppler-utils` as the dev image
  (ADR-0012, ADR-0016), so render fidelity cannot differ between them.
- `docker-compose.release.yml`: one service, `data/` and `output/` as volumes,
  `127.0.0.1` by default, `BIND_ADDR`/`LAN_AUTH_TOKEN` exactly as today.
- The development workflow is unchanged: `docker compose up` still builds from
  source and keeps Vite's hot reload.

### LAN mode's token gates `/api/`, not the app shell

`requireLANToken` previously wrapped every route, which was indistinguishable
from wrapping `/api/*` because `/api/*` was all there was. Now it is scoped
explicitly.

The frontend build is the same bytes for every install and holds none of the
user's data — the data arrives over `/api/*`, which still requires the token. A
browser also cannot attach a custom header to the document request that loads a
page, so gating the shell would make LAN mode unusable from the very device it
exists for, while protecting nothing.

## Consequences

- Self-hosting becomes: download one compose file, add `.env` and `data/`,
  `docker compose up`. No Node, no Go, no checkout.
- One port and no CORS, in the release path. The dev path keeps the Vite proxy.
- The image is bigger than a distroless Go binary would be, for the reason
  ADR-0012 already accepted: `typst` needs a real base image and fonts.
- arm64 is built under emulation in CI, because the image installs
  per-architecture typst and Debian packages and so has to run as its target
  architecture to assemble itself. That makes the release build slow; it runs
  once per release.
- CI builds the image and smoke-tests it (amd64 only) on every change: the
  container starts, `/` returns the app shell, a client-side route falls back
  to it, an unrouted API path answers 404, and a render of the stub data
  produces a one-page, ATS-parsable PDF. The thing self-hosters run is
  therefore exercised continuously, not first at release time.
