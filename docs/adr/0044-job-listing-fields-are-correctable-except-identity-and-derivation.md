# ADR-0044: A Job Listing's captured fields are correctable; its identity and its Job Description are not

- Status: accepted
- Date: 2026-09-22
- Relates to: [ADR-0010](0010-generation-client-traceability-and-default-mode.md), [ADR-0014](0014-ral-range-from-listing-salary-field.md), [ADR-0042](0042-one-job-listing-per-posting-key.md)

## Context

Everything on a Job Listing was written once, at save time, and then frozen. A Job Title mangled by a DOM read, a company name captured as "ACME, Inc" on one posting and "Acme" on another (splitting one employer's roles across two groupings, and defeating the same-company warning), a badly-parsed location — each lived in the records forever (issue #206). The Application Method was the lone exception, correctable since PRD 3.

The obvious fix — "make the record editable" — is too broad. Two of its fields are not captured data at all: they are what other things are built on.

## Decision

**Correctable:** Job Title, Company, Location, RAL Range, through `PATCH /api/job-listings/{id}`. These are captured or inferred values. When capture gets one wrong, the record is wrong, and the user is the one who can see that.

**Fixed:** `url` and `jobDescription`. Both are **refused with a 400**, not silently ignored — a client that tries learns why rather than believing the edit applied.

The line falls there because those two are not values, they are load-bearing:

- **The URL is the record's identity.** [ADR-0042](0042-one-job-listing-per-posting-key.md) makes one Job Listing exist per Posting Key, and the Posting Key is computed from the URL. Editing it would move the record's identity underneath the invariant: a corpus with one Job Listing per posting could be turned into one with two, by an edit that looks like a typo fix.
- **The Job Description is what everything else was derived from.** The RAL Range, the Application Method and every recorded Generation were produced from that text. Editing it would quietly detach each of them from what they came from — a recorded Generation would go on claiming it was tailored to a Job Description it never saw, which is exactly the traceability [ADR-0010](0010-generation-client-traceability-and-default-mode.md) exists to keep honest.

Two supporting rules:

- **Company may be corrected, and correcting it re-evaluates nothing.** No stored value is derived from the company name; the same-company warning simply reads the new value the next time it runs. This is the point of fixing it: two spellings stop being two companies.
- **A hand-entered RAL Range is its own source** (`manual`), stamped server-side and never overwritten by re-resolution. See the addendum to [ADR-0014](0014-ral-range-from-listing-salary-field.md).

Corrections honour `If-Match` and answer 409 on mismatch, like every other record-writing route (issue #89).

## Alternatives considered

- **Make the whole record editable.** Simplest to build and the reason this ADR exists: it would silently break the duplicate invariant and Generation traceability, neither of which is visible from the edit form.
- **Allow editing the URL but recompute the Posting Key and re-check for duplicates.** Defensible, and a real feature — but it is a *move this record to another posting* operation, with its own questions (what happens to the Generations tailored to the old posting's text?). It is not a typo fix, and pretending it is would be the mistake.
- **Allow editing the Job Description and mark affected Generations stale.** The staleness machinery for Master Data Entries (#52) suggests the shape, but a Generation's Job Description is not stored on the Generation — it is the Job Listing's. Marking them stale would require copying the text onto every Generation, a much larger change than the correction it enables.
- **Silently ignore `url`/`jobDescription` in the request.** Less code, and it leaves a client believing an edit applied. A refusal that says why is worth the two branches.
- **Let the client set the RAL source.** It would let a hand-typed figure be labelled `stated`, which is the one thing the RAL Range's source exists to prevent.

## Consequences

A wrong capture is now something the user fixes in a few seconds, and correcting a company name retroactively merges that employer's roles in the same-company warning without touching any record but the one edited.

The two refusals are a real limit, not an oversight: a Job Listing saved against the wrong URL, or with a truncated Job Description, still has to be deleted and re-saved. That is the honest operation for it — a new record, with its own Generations — and the refusal message says as much.
