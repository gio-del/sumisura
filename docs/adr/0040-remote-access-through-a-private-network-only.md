# ADR-0040: Remote access goes through a private overlay network, never public exposure

- Status: accepted
- Date: 2026-09-17
- Relates to: [ADR-0004](0004-standalone-web-app.md), [ADR-0039](0039-backend-serves-the-production-frontend.md)

## Context

The owner's typical setup is an always-on home computer, with the phone used away from home. The phone-capture work in #187 needs both reach and HTTPS: a PWA can only be installed, and can only receive shares, in a secure context. LAN mode (ADR-0004's #57 exception) gives neither. It is same-network only and plain HTTP.

## Decision

The supported answer is a private overlay network in front of an app that stays bound to `127.0.0.1`. The recommended setup is Tailscale with `tailscale serve --bg --https=443 http://127.0.0.1:8080`. That gives a `*.ts.net` HTTPS name with a managed certificate, reachability from the owner's own devices anywhere, no router changes, and nothing listening on the LAN or the internet. The access token (`LAN_AUTH_TOKEN`) is still recommended on top, because a tailnet can include devices the owner didn't intend to give access to. The docs page is `site/src/content/docs/remote-access.md`.

## Alternatives considered

- **Port forwarding plus dynamic DNS, or a public reverse proxy with Let's Encrypt.** This puts a single-shared-secret app, with no rate limiting and no accounts, on the public internet, next to the owner's whole career history and the ability to spend their Anthropic key. Hardening it for that (real authentication, lockout, audit) is the hosted product's problem, not a self-hosting option.
- **`tailscale funnel` or Cloudflare Tunnel.** Same exposure as above. Free TLS doesn't make it private. Cloudflare Access in front would fix that, but adds a second identity system for a single-user tool.
- **Terminating TLS inside the app.** Certificate issuance and renewal for a name only reachable privately is exactly what the overlay network already solves, and self-signed certificates break PWA installation.
- **Bundling Tailscale as a compose sidecar.** Feasible, but it couples the image to one vendor and needs an auth key in `.env`. Host installation is one command and keeps working for other containers. Revisit if host installation proves to be real friction.

## Consequences

No code depends on Tailscale. The backend only trusts `X-Forwarded-Proto: https`, which `tailscale serve` sets, to mark the access cookie `Secure`. Headscale or plain WireGuard fit the same rule. Public exposure stays explicitly unsupported in `SECURITY.md`, so a report that "an internet-exposed install can be brute-forced" is out of scope.
