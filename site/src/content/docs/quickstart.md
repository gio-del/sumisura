---
title: Quickstart
description: From a clone to your first tailored CV in about fifteen minutes.
---

Sumisura runs on your machine. Nothing is hosted, there is no account, and it
calls the Claude API with your own key.

## What you need

| Requirement | Why |
|---|---|
| Docker + Docker Compose | Runs the web app — one container from a published image |
| An Anthropic API key | Selection, Rewrite, cover letters and research calls (Claude API, pay per use) |
| [Claude Code](https://claude.ai/code) | Runs the `tailor-cv` skill, which drives the tailoring pipeline |
| [Typst](https://typst.app) on your `PATH` | Renders the PDF |
| `pdftotext` (poppler-utils) | Optional — the ATS-parsability check |

## 1. Get it

**From a published image (recommended).** No checkout, no Node, no Go — one
image with the backend and the frontend together, on one port
([ADR-0039](https://github.com/gio-del/sumisura/blob/main/docs/adr/0039-backend-serves-the-production-frontend.md)):

```sh
mkdir sumisura && cd sumisura
curl -O https://raw.githubusercontent.com/gio-del/sumisura/main/docker-compose.release.yml
curl -o .env https://raw.githubusercontent.com/gio-del/sumisura/main/.env.example
```

Pin a version in `.env` (`SUMISURA_VERSION=v0.4.0`) rather than following <!-- x-release-please-version -->
`latest`, so upgrades stay deliberate. You still need a checkout for the
`tailor-cv` skill, which runs in Claude Code — but not to run the app.

**From source**, if you want to change the code, or want the dev server's hot
reload:

<!-- x-release-please-start-version -->
```sh
git clone https://github.com/gio-del/sumisura.git
cd sumisura
git checkout v0.4.0   # or track main
```
<!-- x-release-please-end -->

## 2. Add your API key

```sh
cp .env.example .env
# then set ANTHROPIC_API_KEY=sk-ant-...

# from a checkout:
cp -r data/examples/. data/

# from a published image, fetch the stubs instead:
# curl -L https://github.com/gio-del/sumisura/archive/refs/heads/main.tar.gz \
#   | tar -xz --strip-components=2 '*/data/examples'
```

That copy seeds `data/` with stub Master Data to edit. Everything it writes —
`profile.yaml`, `experience/`, `projects/`, `cover-letter-snippets/` — is
**gitignored on purpose**: your name, your employer and your clients never go
near a commit ([ADR-0038](https://github.com/gio-del/sumisura/blob/main/docs/adr/0038-all-master-data-is-local-only.md)).
The tracked copies under `data/examples/` stay as examples.

Without a key the app still starts and you can browse and edit Master Data —
only the calls to Claude fail.

## 3. Start the app

```sh
docker compose up
```

From a published image, everything is on **one** port:

```sh
docker compose -f docker-compose.release.yml up
```

- App and API: `http://127.0.0.1:8080`

From a checkout, `docker compose up` runs the development pair instead:

- Backend: `http://127.0.0.1:8080`
- Frontend: `http://127.0.0.1:5173`

Both are bound to localhost with no authentication, on purpose. See
[LAN-reachable mode](./lan-mode.md) if you want to reach it from your phone.

## 4. Replace the stub data with your own

The copy in step 2 filled `data/` with stub content for a fictional "Jane Doe"
so the app has something to show on first run. Replace it with your own career
history — every file below is untracked, so edit freely:

- `data/profile.yaml` — contact details, education, publications, awards,
  languages
- `data/experience/*.md` — one file per job or client engagement
- `data/projects/*.md` — one file per project
- `data/cover-letter-snippets/*.md` — optional reusable cover-letter paragraphs

You can edit these in the web app, or in your editor — they are plain Markdown
with YAML frontmatter, and both paths write the same files. See
[Writing your Master Data](./master-data.md) for the shape of each file.

## 5. Install the tailoring skill

The pipeline is a Claude Code skill served from this repo's own plugin
marketplace:

```sh
claude plugin marketplace add .
claude plugin install sumisura@sumisura-local
```

## 6. Tailor your first CV

In Claude Code, from the repo:

```
/sumisura:tailor-cv
```

Give it a job description (pasted text or a URL), point it at an Application you
already track, or give it nothing at all for a general-purpose CV. It walks
Selection → Rewrite → **Text Review (you approve)** → Render → **Visual Review
(you approve)**, and writes everything into `output/<label>-<timestamp>/`.

That directory holds `cv.pdf`, `cover-letter.pdf`, the assembled `data.json`,
and `selection.json` — the record of which Entries were chosen and how each
bullet was rewritten.

## Next

- [How it works](./concepts.md) — the vocabulary, and where the guarantees are
- [Tracking applications](./tracking.md) — Job Listings, Applications, the extension
- [Configuration](./configuration.md) — models, costs, environment variables
