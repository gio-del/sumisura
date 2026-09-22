# ADR-0042: A Job Listing's identity is its Posting Key, and Save refuses a second one

- Status: accepted
- Date: 2026-09-22
- Relates to: [ADR-0008](0008-job-listing-application-storage.md), [ADR-0041](0041-pending-capture-is-not-a-job-listing.md), [ADR-0044](0044-job-listing-fields-are-correctable-except-identity-and-derivation.md)

## Context

Nothing enforced one record per posting. `tracking.Save` had no identity check at all, so the same posting captured from LinkedIn's search pane and later from its own `/jobs/view/` page became two Job Listings — each with its own Application, Status, Status history, Notes and Generations (issue #206).

The only signal was `FindLikelyDuplicate`, a fuzzy title+company similarity score run *after* the record was already on disk, reported as a non-blocking `duplicateWarning`. It answers a genuinely useful but different question — "a similar role, possibly spelled differently" — and by the time it speaks, the second record exists and the user's pipeline already double-counts the role.

`internal/postingkey` (ADR-0041) already computes a stable identity for a posting across the URLs it can be reached by, but that identity was only ever used to deduplicate Pending Captures.

## Decision

**A Job Listing's identity is its Posting Key.** `tracking.Save` refuses to create a second Job Listing for a Posting Key an existing one already holds, returning `ErrDuplicate` (a `DuplicatePostingError` carrying the existing record). The API maps it to `409` with `reason: "duplicate-posting"` and the existing listing's id, title, company, saved date and archived flag.

The refusal lives in `Save`, not in a handler, so every caller inherits it: the extension capture, the manual save form, the ATS browse save, Pending Capture completion, and anything added later. **There is no force/override flag** — no caller wants one, and one would make the invariant a suggestion.

It runs before the Job Description is resolved and before any Claude call, so a refused save costs neither a fetch nor a token and writes nothing.

Three deliberate boundaries:

- **A posting whose URL yields no Posting Key is never refused.** Absent, or not an absolute `http(s)` URL, means identity cannot be computed — which is not a reason to block a paste-only Job Listing.
- **Archived Job Listings participate.** They hold their Posting Key like any other record, so archiving is never a back door to two records for one posting. The refusal reads identically either way; only the `archived` flag differs, so a client can additionally offer to bring the record back.
- **`FindLikelyDuplicate` stays exactly as it is**, on every route that returns it today. The two checks are not redundant: one is an exact identity refusal, the other a fuzzy non-blocking hint about a different thing.

Nothing rewrites existing records. `cmd/migrate-records` reports Posting Keys held by more than one Job Listing and never merges or archives anything to resolve one.

## Alternatives considered

- **Keep warning, don't refuse.** This is what the fuzzy warning already does, and it is exactly what fails: by the time the warning appears the duplicate exists, and merging two Status histories afterwards is a decision no tool should make on the user's behalf.
- **Refuse in the handlers rather than in `Save`.** Every new save path would have to remember to opt in, which is how the invariant would quietly stop holding. A property of the record belongs where the record is written.
- **Deduplicate on write by deriving the Job Listing's id from the Posting Key** (as Pending Captures do). It would make the invariant structural, but Job Listing ids are company slugs that appear in URLs, file names and the `output/` directory names of every Generation; changing that scheme would break links and rename files across an existing corpus for no user-visible gain.
- **A `force` flag for the case where two postings really are distinct.** The only way two genuinely different postings share a Posting Key is a provider reusing an id, which has not been observed. Adding the escape hatch first would guarantee clients grow to depend on it.
- **Merge the two records automatically when a duplicate arrives.** It would have to pick which Status history, which Notes and which Generations survive. That is the user's call, and archiving (reversible) covers the real need without destroying anything.

## Consequences

One posting is now exactly one Job Listing with one Application and one history, whichever door the save came in through. A client that previously assumed every save returns `201` must handle `409` — the FE save paths report the refusal and link to the existing record.

A corpus written before this rule may hold duplicates; they keep working, and `migrate-records`' dry run names them so the user can clean up by hand.

Two genuinely different URLs for one posting (a LinkedIn repost under a new id, say) are still not recognised as the same posting — the same limitation ADR-0041 records for Pending Captures. The fuzzy warning remains the only signal there, which is one reason it stays.

## Addendum: what `migrate-records` does about an existing duplicate

`cmd/migrate-records` reports every Posting Key held by more than one Job Listing — each record's file, Job Title, saved date, Application Status and whether it is archived, oldest saved first — and counts their presence as pending work in its existing exit-3 sense.

`-write` never resolves one. Merging would mean choosing which Status history, which Notes and which Generations survive, which is exactly the decision [the Decision above refuses to make automatically](#decision). A `-write` run therefore completes normally and keeps reporting them until the user resolves them by hand.
