---
title: Configuration
description: Environment variables, the model used at each call site, and what a tailoring run costs.
---

Configuration lives in `.env` at the repo root (start from `.env.example`).
`docker-compose.yml` forwards these into the backend container.

## Required

| Variable | Meaning |
|---|---|
| `ANTHROPIC_API_KEY` | Your Claude API key. Without it the app runs but every call to Claude fails. |

## Which model runs where

Each kind of call runs on a model chosen for that kind of work. Calls whose
output you review, and the web-research calls, run on Sonnet; calls that only
read structured fields out of a previous call's notes run on Haiku.

| Call site | Default | What it does | Override |
|---|---|---|---|
| `selection_rewrite` | Sonnet 5 | Selection + Rewrite for a Generation | `SUMISURA_MODEL_SELECTION_REWRITE` |
| `selection_preview` | Sonnet 5 | Selection-only preview | `SUMISURA_MODEL_SELECTION_PREVIEW` |
| `cover_letter` | Sonnet 5 | Cover letter drafting | `SUMISURA_MODEL_COVER_LETTER` |
| `ral_research` | Sonnet 5 | Salary-range research | `SUMISURA_MODEL_RAL_RESEARCH` |
| `ral_extraction` | Haiku 4.5 | Pulling the range out of those notes | `SUMISURA_MODEL_RAL_EXTRACTION` |
| `application_method_inference` | Haiku 4.5 | How one applies | `SUMISURA_MODEL_APPLICATION_METHOD_INFERENCE` |
| `contact_research` | Sonnet 5 | Contact research | `SUMISURA_MODEL_CONTACT_RESEARCH` |
| `contact_extraction` | Haiku 4.5 | Pulling the contact out of those notes | `SUMISURA_MODEL_CONTACT_EXTRACTION` |
| `capture_hints` | Haiku 4.5 | Company and Job Title from a Job Description pasted on the phone | `SUMISURA_MODEL_CAPTURE_HINTS` |

Override one call site, or all of them:

```sh
SUMISURA_MODEL_SELECTION_REWRITE=claude-opus-5   # try Opus on Rewrite alone
SUMISURA_MODEL_DEFAULT=claude-haiku-4-5          # everything at once
```

`SUMISURA_MODEL_DEFAULT` sets every call site at once. A per-call-site variable beats `SUMISURA_MODEL_DEFAULT`, which beats the
built-in default. Values are passed to the API unchanged — an unknown model id
does not stop the backend starting, the affected call just fails with the API's
own error.

Restart the backend after changing `.env`.

## What it costs

The app tracks usage and estimated cost per Generation and in total — see the
usage view. Estimates come from a price table in the repo; a model missing from
it estimates at $0 and logs a warning. Your actual bill is whatever Anthropic
charges your key.

## Other variables

| Variable | Meaning |
|---|---|
| `BIND_ADDR` | `127.0.0.1` by default. See [LAN-reachable mode](./lan-mode.md). |
| `LAN_AUTH_TOKEN` | Shared token required in LAN mode. Unset means the check is off. |
| `PORT` | The port the backend listens on inside its container, `8080` by default. Change the published port in the compose file instead. |
| `DATA_DIR` | Where Master Data and records live. The default suits a normal checkout. |
| `PROJECT_ROOT` | The directory holding `template/`, `output/` and `data/` for rendering. The default suits a normal checkout. |
| `STATIC_DIR` | The built frontend the backend serves (ADR-0039). Set inside the release image; leave unset from a checkout, where Vite serves the frontend. |
| `SUMISURA_VERSION` | Which published image `docker-compose.release.yml` runs. Unset means `latest`; pin a tag for deliberate upgrades. |
