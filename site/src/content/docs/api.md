---
title: API reference
description: Every route the backend serves, what it does, and which need the access token.
---

The web app, the browser extension, the `tailor-cv` skill and a phone's share
sheet all talk to the same HTTP API under `/api`. You only need this page to
script against it yourself, for example from an iOS Shortcut.

**Access.** With `LAN_AUTH_TOKEN` unset (the default, localhost only) nothing
is gated. Once it is set, every `/api` route needs the token, either in the
`X-Sumisura-Token` header or as the access cookie a browser gets from
`POST /api/auth/session`. The only exceptions are the two `/api/auth/*` paths
and the extension capture's CORS preflight. See
[LAN-reachable mode](./lan-mode.md).

**Shapes.** Request and response bodies are JSON. Their exact shapes are not
repeated here: the source of truth is `frontend/src/api/types.ts`, which the
contract fixtures under `frontend/src/api/contract/fixtures/` hold to the
backend (ADR-0035).

**Writes.** Routes that rewrite or delete a record accept an optional
`If-Match` version token and answer `409` when the record changed under you.
Without the header they write unconditionally.

The routes that call Claude can run for minutes. They carry a 10-minute
deadline.

## Health and access

| Route | What it does |
|---|---|
| `GET /api/healthz` | Liveness check: `{"status":"ok"}`. |
| `GET /api/auth/status` | Whether this installation needs a token, and whether this request has access. Ungated. |
| `POST /api/auth/session` | Exchanges the token for an HttpOnly access cookie. Ungated. |
| `DELETE /api/auth/session` | Clears the access cookie (signs this browser out). Ungated. |
| `GET /api/export` | Downloads your Job Listings, Applications and Pending Captures as a zip. Master Data is not included. |
| `GET /api/usage` | Claude API usage and estimated cost, per Generation and in total. |

## Master Data

| Route | What it does |
|---|---|
| `GET /api/master-data/profile` | The profile (`profile.yaml`). `404` until one exists. |
| `PUT /api/master-data/profile` | Replaces the profile. |
| `GET /api/master-data/entries` | Every Entry (experience and projects). |
| `POST /api/master-data/entries` | Creates an Entry. |
| `GET /api/master-data/entries/{id...}` | One Entry, by its path-like id (e.g. `experience/acme`). |
| `PUT /api/master-data/entries/{id...}` | Replaces an Entry. |
| `DELETE /api/master-data/entries/{id...}` | Deletes an Entry. |
| `GET /api/master-data/cover-letter-snippets` | Every Cover Letter Snippet. |
| `POST /api/master-data/cover-letter-snippets` | Creates a Snippet. |
| `GET /api/master-data/cover-letter-snippets/{id...}` | One Snippet. |
| `PUT /api/master-data/cover-letter-snippets/{id...}` | Replaces a Snippet. |
| `DELETE /api/master-data/cover-letter-snippets/{id...}` | Deletes a Snippet. |
| `GET /api/master-data/tag-lint` | Groups of Entry tags that look like spellings of the same thing. |

## Job Listings

| Route | What it does |
|---|---|
| `GET /api/job-listings` | Every Job Listing paired with its Application. Add `?archived=all` to include archived ones. |
| `POST /api/job-listings` | Saves a Job Listing from a pasted Job Description. Calls Claude (RAL Range, Application Method). |
| `POST /api/job-listings/from-extension` | The browser extension's capture. Calls Claude. |
| `OPTIONS /api/job-listings/from-extension` | CORS preflight for the extension capture. Ungated. |
| `GET /api/job-listings/{id}` | One Job Listing, with its full Job Description, and its Application. |
| `DELETE /api/job-listings/{id}` | Deletes a Job Listing and its Application. |
| `GET /api/job-listings/{id}/logo` | The Company Logo image, when one was downloaded. |
| `POST /api/job-listings/{id}/resolve` | Retries whichever of RAL Range and Application Method is still Unresolved. Calls Claude. |
| `POST /api/job-listings/{id}/suggest-contact` | Researches a Contact to suggest. Writes nothing. Calls Claude. |
| `POST /api/job-listings/{id}/check-freshness` | Rechecks whether the posting is still open. |
| `POST /api/job-listings/{id}/archive` | Archives the Job Listing. |
| `POST /api/job-listings/{id}/unarchive` | Brings it back from the archive. |

## Pending Captures (To complete)

| Route | What it does |
|---|---|
| `GET /api/pending-captures` | Every link shared from another device that is waiting for its Job Description. |
| `POST /api/pending-captures` | Saves a shared link (`url`, `text`, `title`). A public Greenhouse, Lever or Ashby link is saved straight as a Job Listing. |
| `DELETE /api/pending-captures/{id}` | Dismisses one. |
| `POST /api/pending-captures/{id}/hints` | Suggests Company and Job Title from a pasted Job Description (`jobDescription`), to pre-fill the completion form. Writes nothing. Calls Claude. |
| `POST /api/pending-captures/{id}/complete` | Turns it into a Job Listing from a confirmed Company and pasted Job Description. Calls Claude. |

## Applications

| Route | What it does |
|---|---|
| `GET /api/applications` | Every Application, grouped by Status. Takes the same `archived` parameter. |
| `GET /api/applications/stats` | The funnel and time-in-stage statistics. |
| `PATCH /api/applications/{id}/status` | Moves the Application to another Status. |
| `PATCH /api/applications/{id}/method` | Corrects the Application Method. |
| `PATCH /api/applications/{id}/contact` | Corrects the Contact. |
| `GET /api/applications/{id}/mailto` | A `mailto:` link drafting an email to the Contact. |
| `POST /api/applications/{id}/generations` | Records a rendered Generation in the Application's history. The skill uses this too. |
| `POST /api/applications/{id}/notes` | Adds a Note. |
| `PATCH /api/applications/{id}/notes/{noteId}` | Edits a Note. |
| `DELETE /api/applications/{id}/notes/{noteId}` | Deletes a Note. |

## Generations

| Route | What it does |
|---|---|
| `POST /api/generations` | Runs Selection, Rewrite and the Cover Letter draft for a Job Description (or Default Mode). Calls Claude. |
| `POST /api/generations/preview` | Selection only, to preview which Entries would be used. Calls Claude. |
| `POST /api/generations/render` | Renders the approved text to PDF with Typst and returns the page count and ATS Reports. Pass `jobDescription` for term coverage. |
| `GET /api/generations` | Every CV generated so far, and whether its files are still under `output/`. |
| `GET /api/generations/{slug}/ats-report` | The Generation's ATS Reports (CV and cover letter), from its record or from `output/`. `404` when none was kept. |
| `GET /api/generations/{slug}/{file}` | One of a Generation's files, e.g. `cv.pdf`. |
| `DELETE /api/generations/{slug}` | Deletes a Generation's files. |

## ATS job boards

| Route | What it does |
|---|---|
| `GET /api/ats/{provider}/{slug}/listings` | Open postings on a public Greenhouse, Lever or Ashby board. |
| `GET /api/ats/tracked-boards` | The boards you follow, each with a count of postings you haven't seen. |
| `POST /api/ats/tracked-boards` | Follows a board. |
| `DELETE /api/ats/tracked-boards/{id}` | Stops following a board. |
