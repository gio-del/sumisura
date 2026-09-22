<p align="center">
  <img src="brand/logo-lockup.svg" alt="Sumisura" width="320">
</p>

<p align="center">
  Resumes tailored to every job, never made up. Sumisura turns your complete career history into a one-page CV and a cover letter, written for one specific job and checked against what you actually did.
</p>

<p align="center">
  <img alt="Go" src="https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white">
  <img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-3178C6?logo=typescript&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB">
  <img alt="Typst" src="https://img.shields.io/badge/Typst-239DAD?logo=typst&logoColor=white">
  <img alt="License: AGPL-3.0" src="https://img.shields.io/badge/license-AGPL--3.0-1B2A4A">
</p>

<p align="center">
  <a href="https://gio-del.github.io/sumisura/quickstart"><strong>Docs</strong></a> &middot;
  <a href="https://gio-del.github.io/sumisura/">Website</a> &middot;
  <a href="https://github.com/gio-del/sumisura/releases">Releases</a>
</p>

<p align="center">
  <img src="docs/screenshots/job-listings.png" alt="The Job Listings view: saved jobs with their salary range, status and a Generate CV action" width="90%">
</p>
<p align="center">
  <img src="docs/screenshots/applications.png" alt="The Applications view, grouped by status from Saved through Interviewing" width="90%">
</p>

*Su misura* is Italian for "made to measure". Sumisura is not a generic resume builder and not a template gallery: you keep your whole career history as Master Data in this repo, and each run produces a *Tailored CV* for one *Job Description* — selected from real Entries, rewritten under a groundedness check, and approved by you before anything is rendered. The AI never invents experience you don't have.

It is **self-hosted**: it runs on your machine, your data stays in files you own, and it calls the Claude API with your own key. Bring your own career history — `data/examples/` ships stub content (Jane Doe) to copy across and edit, and your copies are gitignored.

See `CONTEXT.md` for the domain vocabulary (Master Data, Entry, Client Engagement, Selection, Rewrite, Tailored CV, Job Listing, Application, ...) and `docs/adr/` for why the architecture looks like this.

## What it does

**1. Tailors.** A Claude Code skill (`/sumisura:tailor-cv`) walks Selection →
Rewrite → **Text Review (you approve)** → Render → **Visual Review (you
approve)**, and produces a one-page CV plus a cover letter for one specific job.
Rendering is [Typst](https://typst.app); there is no Node/JS in this part.

**2. Refuses to make things up.** Rewriting may change emphasis and wording,
never facts. A groundedness check compares every rewritten bullet against the
Entry it came from *before* you are asked to approve anything, and the rendered
PDF is checked for page count, ATS-parsability and language.

**3. Tracks where you applied.** A local web app (Go + React, `docker compose`)
for browsing and editing your Master Data, and for Job Listings and
Applications: status, notes, contacts, salary ranges, and every Generation you
produced. Jobs come in by paste, from Greenhouse/Lever/Ashby boards, or from the
browser extension while you read a posting on LinkedIn or Indeed.

## Quickstart

```sh
git clone https://github.com/gio-del/sumisura.git && cd sumisura
cp .env.example .env            # set ANTHROPIC_API_KEY
cp -r data/examples/. data/     # stub Master Data to edit; your copies stay untracked
docker compose up               # dev: app on 127.0.0.1:5173, API on :8080
claude plugin marketplace add . && claude plugin install sumisura@sumisura-local
```

Prefer not to build anything? Run the published image instead — one container,
backend and frontend together on `127.0.0.1:8080`, no Node or Go needed
(ADR-0039):

```sh
curl -O https://raw.githubusercontent.com/gio-del/sumisura/main/docker-compose.release.yml
docker compose -f docker-compose.release.yml up
```

Then replace the copied stub Master Data in `data/` with your own, and run
`/sumisura:tailor-cv` in Claude Code. You need Docker, an Anthropic API key,
Claude Code, and the `typst` CLI.

**Full documentation: [https://gio-del.github.io/sumisura/quickstart](https://gio-del.github.io/sumisura/quickstart)** — Master Data,
tailoring, tracking, configuration, LAN mode, remote access, upgrading and troubleshooting.

## Repo layout

| Path | What's in it |
|---|---|
| `data/` | Your Master Data: `profile.yaml`, `experience/*.md`, `projects/*.md`, `cover-letter-snippets/*.md` — all **gitignored**, copied once from the tracked stubs in `data/examples/` (ADR-0038). |
| `template/` | `cv.typ` and `cover-letter.typ` — pure presentation, each reads one assembled JSON file. |
| `output/` | Gitignored. One directory per Generation: PDFs, the assembled data, and `selection.json`. Safe to delete. |
| `backend/` | Go API over `data/`, plus `cmd/cvcheck` (the quality checks, offline) and `cmd/migrate-records`. |
| `frontend/` | React + TypeScript + Vite app. In a release image it is a static build the backend serves (ADR-0039). |
| `Dockerfile`, `docker-compose.release.yml` | The published image (backend + built frontend, one port) and the compose file that runs it. |
| `docker-compose.yml`, `docker-compose.preview.yml` | For working on Sumisura: the dev pair with hot reload, and a release-shaped build from your checkout on its own port (see `CONTRIBUTING.md`). |
| `extension/` | Browser extension that captures LinkedIn/Indeed postings. |
| `plugins/sumisura/` | The `tailor-cv` skill, served by this repo's own plugin marketplace (ADR-0015). |
| `site/` | The landing page and docs (Astro + Starlight). |
| `brand/` | Logo, palette and type. |
| `docs/adr/` | Architecture decision records. |

Adding a job or project means adding a Markdown file under `data/`, not writing
code.

## Releases

Tagged releases carry one version for the whole product — backend, frontend,
extension and plugin — because they are only guaranteed to work together at the
same commit. See [Releases](https://github.com/gio-del/sumisura/releases) for
the changelog and the extension zip, and
[`CHANGELOG.md`](CHANGELOG.md) for the same history in the repo.

Before 1.0, a breaking change bumps the minor version, and "breaking" means a
self-hoster has to act — a renamed environment variable or header, a record
migration, a removed route, a plugin reinstall. Those are called out in the
release notes.

## Licence

Sumisura is licensed under the [GNU AGPL-3.0](LICENSE).

**Your output is yours.** The CVs, cover letters and application records you
produce with it are your own work, not derivative works of this software: the
AGPL places no obligation on them, and neither the maintainer nor this project
claims any right over your Master Data or anything rendered from it.

The **name and logo** are trademarks and are not covered by the AGPL — see
[`TRADEMARKS.md`](TRADEMARKS.md). Fork freely; give your fork its own name.

Third-party code and fonts keep their own licences, listed in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the dev loop, the test and lint
commands, the API contract fixtures, and how architecture decisions get
recorded. Bug reports and questions are welcome; by contributing you agree to
the [Code of Conduct](CODE_OF_CONDUCT.md).

## Further reading

- [The docs](https://gio-del.github.io/sumisura/quickstart) — everything a self-hoster needs
- [`CONTEXT.md`](CONTEXT.md) — domain vocabulary (ubiquitous language)
- [`docs/adr/`](docs/adr/) — architecture decision records
- [`extension/README.md`](extension/README.md) — how the LinkedIn/Indeed capture extension works and how to load it
- [`brand/palette.md`](brand/palette.md) — the color palette behind the logo, applied across the frontend's shadcn/ui theme
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — dev loop, tests, lint, contract fixtures
- [`SECURITY.md`](SECURITY.md) — how to report a vulnerability
