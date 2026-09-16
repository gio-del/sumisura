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

| Call site | Default | What it does |
|---|---|---|
| `selection_rewrite` | Sonnet 5 | Selection + Rewrite for a Generation |
| `selection_preview` | Sonnet 5 | Selection-only preview |
| `cover_letter` | Sonnet 5 | Cover letter drafting |
| `ral_research` | Sonnet 5 | Salary-range research |
| `ral_extraction` | Haiku 4.5 | Pulling the range out of those notes |
| `application_method_inference` | Haiku 4.5 | How one applies |
| `contact_research` | Sonnet 5 | Contact research |
| `contact_extraction` | Haiku 4.5 | Pulling the contact out of those notes |

Override one call site, or all of them:

```sh
SUMISURA_MODEL_SELECTION_REWRITE=claude-opus-5   # try Opus on Rewrite alone
SUMISURA_MODEL_DEFAULT=claude-haiku-4-5          # everything at once
```

A per-call-site variable beats `SUMISURA_MODEL_DEFAULT`, which beats the
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
| `DATA_DIR`, `PROJECT_ROOT` | Where records and templates live; the defaults suit a normal checkout. |
| `STATIC_DIR` | The built frontend the backend serves (ADR-0039). Set inside the release image; leave unset from a checkout, where Vite serves the frontend. |
| `SUMISURA_VERSION` | Which published image `docker-compose.release.yml` runs. Unset means `latest`; pin a tag for deliberate upgrades. |
