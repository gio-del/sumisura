# ADR-0041: A link shared from a phone is a Pending Capture, not an incomplete Job Listing

- Status: accepted
- Date: 2026-09-17
- Relates to: [ADR-0008](0008-job-listing-application-storage.md), [ADR-0034](0034-record-schema-version-and-one-shot-migration.md), [ADR-0040](0040-remote-access-through-a-private-network-only.md)

## Context

On a phone, job hunting happens inside the LinkedIn and Indeed apps. They expose no page to capture, only a Share action that hands out a link (issue #187). A Job Listing can't be built from that: `tracking.Save` requires a Company and a Job Description. Fetching the page server-side doesn't close the gap either, because LinkedIn and Indeed block unauthenticated fetches and scraping them raises terms-of-service questions.

A Job Listing also carries invariants the rest of the app relies on. Every one has an Application with a Status, counts toward the funnel and stage-time stats, takes part in duplicate detection, can seed a Generation, and gets RAL Range and Application Method inference from its Job Description.

## Decision

A shared link becomes a separate record, the **Pending Capture**, stored as one JSON file per record under `data/pending-captures/` (atomic writes, `schemaVersion` from birth). Its id is derived from the link's **Posting Key** (`backend/internal/postingkey`). Sharing the same posting twice therefore finds the existing file, and a share whose Posting Key matches an existing Job Listing's URL writes nothing and reports that listing instead.

It becomes a Job Listing only by being completed through the ordinary `tracking.Save`, with the user-confirmed Company and pasted Job Description. The Pending Capture is removed only after that save succeeds. Completing it from a desktop extension capture (#183) and resolving public ATS links automatically (#184) build on the same record and key.

## Alternatives considered

- **Relax Job Listing to allow a missing Job Description and Company.** Every consumer would need an "incomplete" branch: the stats funnel, duplicate scoring (company/title based), Generation's Job Description prefill, RAL and Method inference, freshness. An Application at Status Saved for a posting the user has only glanced at would also misstate the funnel.
- **A Status before Saved (e.g. "Captured") on the Application.** Same problem, pushed into the Status pipeline that CONTEXT.md defines precisely.
- **Server-side fetch of LinkedIn/Indeed pages to fill the Job Description at share time.** Unreliable (anti-bot walls) and a terms-of-service risk. Rejected for any purpose.

## Consequences

The inbox is its own page (`/inbox`, "To complete"), and nothing else in the app needs to know Pending Captures exist. Pending Captures are included in Export. The dedupe is by Posting Key only, so two genuinely different URLs for one posting (e.g. a LinkedIn repost with a new id) aren't recognised. That's acceptable, since duplicate detection still runs when the Job Listing is saved.

## Addendum (2026-09-19, issue #200)

A real share from the LinkedIn Android app carries only the link: no title and no text. The share screen therefore offers the completion form right away, so a Pending Capture can be finished on the phone. Company and Job Title are suggested by a Claude call (`capture_hints`) over the Job Description the user pasted, and only fill empty fields the user then confirms.

Fetching the posting server-side was re-examined at the same time. LinkedIn's public guest job page did return the full posting without a login, so "blocked" no longer holds. The decision stands anyway: automated retrieval is prohibited by LinkedIn's User Agreement (§8.2), and a feature built on it would ship in a published, soon-to-be-hosted product, where that risk multiplies.

## Addendum (2026-09-22, issue #206)

The Consequences above scope Posting Key dedupe to Pending Captures ("The dedupe is by Posting Key only…"). That is no longer the whole story: [ADR-0042](0042-one-job-listing-per-posting-key.md) makes the Posting Key the identity of a **Job Listing** too, refused in `tracking.Save` on every save path. The sentence about two genuinely different URLs for one posting not being recognised still holds, and now holds for Job Listings as well — but its consolation ("duplicate detection still runs when the Job Listing is saved") now means two checks: an exact Posting Key refusal, and the unchanged fuzzy `FindLikelyDuplicate` warning.

The line "a share whose Posting Key matches an existing Job Listing's URL writes nothing and reports that listing instead" was the first place a Job Listing's Posting Key was consulted. It is now the same rule the Job Listing itself is built on, rather than a special case of the inbox.
