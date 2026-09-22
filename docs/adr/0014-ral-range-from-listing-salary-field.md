# RAL Range can be Stated from a listing's own salary field, not just the Job Description text

`ParseStatedRAL` now also runs against an optional salary-field text captured separately from the Job Description — LinkedIn shows a computed salary-insight badge next to the job title that's often absent from the description prose entirely, so relying on Job Description text alone under-detects Stated RAL for listings that only expose it as a separate field. When the description and the listing field both state a figure and their ranges don't overlap at all, `RALRange` reports a new `conflict` source with both figures labeled, rather than picking one silently; overlapping or matching figures resolve to the Job-Description-stated value.

## Considered Options

- Fold the captured salary text into the Job Description string sent to the backend (no schema change) — rejected because it repeats the "fold captured signals into the description blob" pattern this repo's own Job Title work (PR #20) just moved away from, and stretches Job Description's definition ("the pasted text or fetched-URL content describing a role") to include something that was never part of that text.
- Always prefer one source over the other when they disagree — rejected because RAL Range has no manual-correction UI (unlike Application Method), so a silently wrong pick can't be fixed by the user afterward; surfacing both figures lets them judge which is right for that listing.

## Consequences

- `RALRange` gains a `conflict` source and two optional labeled sub-figures (populated only in that state); every consumer of `RALRange` (FE `RALBadge`, YAML persistence, generation output) needs to handle the new case.
- The dedicated salary-field text is only ever populated by the browser-extension capture path today — manual-paste and ATS-board intake have no equivalent structured field to read from, so RAL resolution for those paths is unchanged.
- The Go parser's numeric shorthand (`numberToken`) needs a companion fix to handle fractional-K notation ("63,2K" / "63.2K" meaning 63,200) — LinkedIn's own salary badge uses this format, and the existing regex silently mis-parses it as 63 rather than failing loudly.

## Addendum (2026-09-22, issue #206): a `manual` source

The Considered Options above reject "always prefer one source when they disagree" partly **because RAL Range had no manual-correction UI, unlike Application Method, so a silently wrong pick couldn't be fixed afterwards**. That premise no longer holds: `RALSource` now has a sixth value, `manual`, and `PATCH /api/job-listings/{id}` lets the user type a figure in — one learned in a conversation with a recruiter, say, which no amount of reading the posting would ever recover.

The decision stands anyway, for the reason that outlives the premise: a `conflict` is two sources genuinely disagreeing, and picking a winner would be the tool asserting something neither source said. What changes is the remedy. A conflict the user can settle is now settled by them entering the figure, which replaces the conflict with a `manual` range rather than by the code guessing.

`manual` is the most reliable source on the record and the only one no inference produces:

- `ParseStatedRAL` never yields it, and no Claude call can. The API stamps it server-side whatever a client sends, so a client cannot pass a figure off as stated by the posting.
- `tracking.Resolve` leaves it alone exactly as it leaves a `stated` one — it only ever re-resolves an `unresolved` range — so a retry can never overwrite the best figure on the record.
- Entering one clears any `descriptionStated`/`listingStated` detail: a figure the user vouches for is one range, not an unsettled disagreement between two others.
- `RALBadge` labels it "Entered by you" in a colour of its own, so the user is never shown their own figure as though Claude had estimated it — the same rule the rest of this ADR is built on.
