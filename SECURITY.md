# Security policy

## Reporting a vulnerability

**Please do not open a public issue for a security problem.**

Use GitHub's private vulnerability reporting instead: go to the repository's
**Security** tab → **Report a vulnerability**. That opens a private advisory
visible only to the maintainer.

Include what you did, what happened, and what you expected — a proof of concept
helps. You can expect a first response within a week. This is a
single-maintainer project with no paid support and no bug bounty, but genuine
reports are taken seriously and credited in the advisory unless you prefer
otherwise.

## Supported versions

Only the latest release is supported. Before 1.0 there are no backports: fixes
land in the next release.

## What is in scope

The backend, frontend, browser extension, the `tailor-cv` skill, and this
repository's workflows.

## What is not a vulnerability

Two documented design decisions regularly look like findings and are not:

- **The app is localhost-only with no authentication by default**
  ([ADR-0004](docs/adr/0004-standalone-web-app.md)). Anyone with access to your
  machine's `127.0.0.1:8080` can read and write your data. That is the intended
  model for a personal, local tool.
- **LAN-reachable mode is an opt-in exception with stated trade-offs**: a single
  shared static token (sent in an `X-Sumisura-Token` header, or exchanged once
  per browser for an HttpOnly cookie derived from it), no TLS, one trusted
  network assumed. Enabling it and then finding the token travels in plaintext,
  or that the token is not a session system, is documented behaviour rather than
  a vulnerability.
- **Remote access is supported only through a private overlay network** such
  as Tailscale, with the app still bound to `127.0.0.1`
  ([ADR-0040](docs/adr/0040-remote-access-through-a-private-network-only.md)).
  An install exposed directly to the internet (port forwarding, a public reverse
  proxy, `tailscale funnel`) is outside the supported model.

Reports that *are* in scope include: a way to reach the API without the token
when LAN mode is on, path traversal out of `data/`/`output/`, anything letting a
web page a user visits reach the local API (CORS/CSRF), the browser extension
sending captured content anywhere but the configured backend, a workflow that
leaks a secret, and prompt-injection in a Job Description that causes the app to
do something outside generating text (exfiltrate files, call other endpoints).

## Your data and your API key

Sumisura runs on your machine, stores Master Data and records as files under
`data/`, and calls the Claude API with **your** key. Job Descriptions and the
Master Data selected for a Generation are sent to Anthropic as part of that
call; nothing is sent anywhere else. There is no telemetry and no phone-home.
