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
| `GET /api/job-listings` | Every Job Listing paired with its Application. Add `?archived=all` to include archived ones, or `?location=` to filter by where the role is. |
| `POST /api/job-listings` | Saves a Job Listing from a pasted Job Description. Calls Claude (RAL Range, Application Method). |
| `POST /api/job-listings/from-extension` | The browser extension's capture. Calls Claude. |
| `OPTIONS /api/job-listings/from-extension` | CORS preflight for the extension capture. Ungated. |
| `POST /api/job-listings/capture-lookup` | Whether a posting is tracked, and what else you track at that company. Read-only, no Claude call. |
| `OPTIONS /api/job-listings/capture-lookup` | CORS preflight for the lookup. Ungated. |
| `GET /api/job-listings/{id}` | One Job Listing, with its full Job Description, and its Application. |
| `PATCH /api/job-listings/{id}` | Corrects the Job Title, Company, Location or RAL Range. Honours `If-Match`. |
| `DELETE /api/job-listings/{id}` | Deletes a Job Listing and its Application. |
| `GET /api/job-listings/{id}/logo` | The Company Logo image, when one was downloaded. |
| `POST /api/job-listings/{id}/resolve` | Retries whichever of RAL Range and Application Method is still Unresolved. Calls Claude. |
| `POST /api/job-listings/{id}/suggest-contact` | Researches a Contact to suggest. Writes nothing. Calls Claude. |
| `POST /api/job-listings/{id}/check-freshness` | Rechecks whether the posting is still open. |
| `POST /api/job-listings/{id}/archive` | Archives the Job Listing. |
| `POST /api/job-listings/{id}/unarchive` | Brings it back from the archive. |

### One Job Listing per posting

Every route that saves a Job Listing — the manual form, the ATS browse save, the extension capture and Pending Capture completion — refuses a posting you already track. The refusal is `409` with this body:

```json
{
  "reason": "duplicate-posting",
  "message": "This posting is already saved as a Job Listing.",
  "existing": {
    "id": "acme",
    "title": "Backend Engineer",
    "company": "Acme",
    "savedAt": "2026-09-20T09:12:44.102Z",
    "archived": false
  }
}
```

Two URLs are the same posting when they share a Posting Key, so LinkedIn's search-pane URL and the posting's own `/jobs/view/` page count as one, as do Indeed's `jk` and `vjk` and a Greenhouse, Lever or Ashby posting and its apply page. A listing with no URL, or one whose URL is not an absolute `http(s)` link, has no Posting Key and is never refused.

An archived Job Listing still holds its Posting Key, so re-saving its posting is refused too. `existing.archived` says so, which is what lets a client offer to bring it back instead; the message itself is the same either way.

This is separate from `duplicateWarning`, the fuzzy "this looks like a role you already have" hint that still comes back on a successful save and never blocks anything.

### Looking a posting up before saving it

`POST /api/job-listings/capture-lookup` is what the browser extension asks about the posting you have open, before you click anything. It writes nothing and makes no Claude call. It takes one of two forms.

**Single** — the posting you are looking at, plus what else you track at that company:

```json
{ "url": "https://www.linkedin.com/jobs/view/4012345678/", "company": "Acme" }
```

```json
{
  "tracked": {
    "id": "acme",
    "title": "Backend Engineer",
    "savedAt": "2026-09-20T09:12:44.102Z",
    "status": "saved",
    "archived": false,
    "allowedTransitions": ["tailoring", "withdrawn"]
  },
  "company": {
    "listings": [
      { "id": "acme-2", "title": "Platform Engineer", "savedAt": "2026-09-18T11:02:10.551Z", "status": "tailoring" }
    ]
  }
}
```

`tracked` is `null` when the posting is not tracked. `allowedTransitions` is the Status state machine's own answer for the current Status, so a client offers only legal moves — every move is still re-validated on the way in. `company.listings` excludes archived Job Listings and the tracked listing itself, and matches company names through the same normalisation the duplicate score uses, so "Acme Inc.", "ACME, Inc" and "Acme S.p.A." are one company.

**Batch** — tracked state only, for badging a results page in one request:

```json
{ "urls": ["https://www.linkedin.com/jobs/view/4012345678/", "https://www.linkedin.com/jobs/view/4099999999/"] }
```

```json
{
  "results": [
    { "url": "https://www.linkedin.com/jobs/view/4012345678/", "tracked": { "id": "acme", "status": "saved" } },
    { "url": "https://www.linkedin.com/jobs/view/4099999999/", "tracked": null }
  ]
}
```

At most 200 URLs per request; more is a `400`. A URL with no recognisable Posting Key is simply untracked.

Like the capture route, the lookup carries CORS headers and answers an `OPTIONS` preflight without a token, because a browser strips custom headers from a preflight. The POST itself is gated in LAN mode like any other `/api` request.

### Correcting a Job Listing

`PATCH /api/job-listings/{id}` takes any subset of `title`, `company`, `location` and `ral`. A field you don't send is left alone; sending `title` or `location` as `""` clears it, and an empty `company` is a `400`, since a Job Listing without one isn't a Job Listing.

```json
{ "title": "Staff Backend Engineer", "location": "Remote (EU)", "ral": { "min": 55000, "max": 65000, "currency": "EUR" } }
```

A `ral` you send is recorded with `"source": "manual"` — the figure is yours, and you're never shown it as though Claude had estimated it. The source is stamped by the backend whatever you send, so a client can't claim a figure was stated by the posting, and `POST /api/job-listings/{id}/resolve` leaves a `manual` range alone exactly as it leaves a `stated` one: a retry can't overwrite the most reliable figure on the record.

**`url` and `jobDescription` are refused with a `400`**, rather than ignored. The URL is the record's identity, which one-Job-Listing-per-posting rests on; the Job Description is what the RAL Range, the Application Method and every recorded Generation were derived from.

Like every other record-writing route, it honours an optional `If-Match` carrying the Job Listing's version token and answers `409` when the record changed on disk since you read it.

### Deciding about a company you already track

`POST /api/job-listings/from-extension` takes an optional `resolution`, the client's answer to "you already track other roles at this company".

If a capture arrives with **no** `resolution` and the company has non-archived Job Listings, nothing is written, no Claude call is made, and the answer is `409`:

```json
{
  "reason": "company-has-listings",
  "message": "You already track other roles at this company.",
  "company": {
    "listings": [
      { "id": "acme-2", "title": "Platform Engineer", "savedAt": "2026-09-18T11:02:10.551Z", "status": "tailoring" }
    ]
  }
}
```

This is a question, not a refusal — the client answers it:

| `resolution` | What happens |
|---|---|
| `{"kind": "save-anyway"}` | Saves the role alongside the existing ones. Answers the company question only: the one-Job-Listing-per-posting refusal still applies and is not skippable. |
| `{"kind": "replace", "jobListingId": "acme-2"}` | Saves the new role, then archives the named one. `201` carries `archivedJobListingId`. |
| `{"kind": "unarchive-existing", "jobListingId": "acme"}` | Brings that Job Listing back from the archive and saves nothing. Answers `200` with the listing. |

A `replace` target is checked before anything is written — it must exist, not already be archived, and belong to the company being saved. Otherwise the answer is `409` with `reason: "replace-target-unavailable"` and nothing is written, so a stale choice fails cleanly instead of half-applying. Replace **archives, never deletes**: the replaced Application's Status history, Notes and Generations all survive, and you can unarchive it.

If the save succeeds but the archive does not, the `201` carries `"archiveFailed": true` rather than swallowing it — you are never left believing you consolidated something you didn't.

This gate is on the extension route only. `POST /api/job-listings` and the ATS browse save keep today's post-save `duplicateWarning` and never ask. That asymmetry is deliberate and temporary; the two are expected to converge.

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
| `PATCH /api/applications/{id}/status` | Moves the Application to another Status. Carries CORS headers, for the browser extension's card. |
| `OPTIONS /api/applications/{id}/status` | CORS preflight for that move. Ungated. |
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
