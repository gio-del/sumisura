# ADR-0043: The extension asks about the posting you have open, before you click

- Status: accepted
- Date: 2026-09-22
- Relates to: [ADR-0004](0004-standalone-web-app.md), [ADR-0040](0040-remote-access-through-a-private-network-only.md), [ADR-0042](0042-one-job-listing-per-posting-key.md)

## Context

The extension's stated posture was **no background polling, no unsolicited requests**: it talked to the backend exactly once, when the user clicked "Save to Sumisura". That made it trivially easy to reason about — a request meant a deliberate capture — and it is why the extension needs no host-level trust beyond the boards it injects into.

It also made the button a one-way door. Everything the user might want to know before clicking — is this posting already saved, what Status is it at, am I already chasing two other roles at this company — was invisible until after the record existed (issue #206). The only duplicate signal was a fuzzy score reported *after* the save, and [ADR-0042](0042-one-job-listing-per-posting-key.md) has since turned exact re-saves into a refusal, which is better but still after the click.

Showing any of that means the extension must ask the backend something before the user acts. That is a real change to the posture, not a detail.

## Decision

The stance is **revised, not quietly bent**. It becomes:

> No polling, and no request except about a posting you have open.

Concretely:

- The extension calls `POST /api/job-listings/capture-lookup`, a **read-only** route that writes nothing and makes no Claude call, when the user opens a posting — at most **once per posting per page session**, with results cached in memory. Clicking back and forth between search results is free.
- On a search-results page it calls the same route's **batch form once for the visible rows**, not once per row, and rows that appear on scroll are picked up by the existing MutationObserver.
- It still **never polls**, never runs on a timer, and never sends anything about a page the user is not looking at. Closing the tab ends all traffic.
- A failed lookup degrades to "unknown", never to a broken page or a blocked capture: the card still offers Save, and results-page badging silently does nothing.

The request carries the posting's URL and the Company read off the page — the same two things a capture would send moments later, and nothing more.

## Why this is acceptable here, and would not be elsewhere

The backend is the user's own, on their own machine (ADR-0004) or their own private network (ADR-0040). The lookup tells that backend which posting its owner is looking at. That is the same fact a capture tells it, arriving a few seconds earlier and for postings the user ends up not saving.

There is no third party in the path, no analytics, and nothing leaves the user's machine or private network. Had the backend been a hosted multi-tenant service, "which postings did you browse" would be a materially different thing to send, and this decision would not follow.

## Alternatives considered

- **Keep the strict posture and show nothing before the click.** This is the status quo, and it is the problem: the user learns they already track a posting only by creating a second record, or now, by being refused.
- **Mirror the corpus into the extension and answer locally.** No requests at all, but it means a background sync (exactly the polling the posture forbids), a second copy of the records living in extension storage outside `data/`, and a stale-cache class of bug where the card confidently contradicts the app.
- **Look up only on hover or on an explicit "check" button.** Preserves the letter of the posture but not its point: the information has to be there *before* the user decides, and a check-then-decide two-step is the round trip the card exists to remove.
- **One request per search-result row.** Simple, and turns a results page into dozens of requests. The batch form exists precisely so a page costs one.
- **Compute the Posting Key in the extension and ask by key.** It would move ADR-0042's identity function into JavaScript and let the two drift. The extension sends URLs; Go computes keys.

## Consequences

The extension now talks to the backend on page open, so "Sumisura isn't reachable" becomes a state the card must show honestly — distinct from "not tracked", which it would otherwise be mistaken for. A lookup failure never costs a capture.

Because the lookup is cross-origin from a `moz-extension://` or `chrome-extension://` origin, it carries CORS headers and answers an `OPTIONS` preflight without a token, exactly as the capture route has since issue #195. The POST itself stays gated in LAN mode.

The card can now show Status and offer Status moves, which means the extension writes as well as reads. Those writes go through the existing `PATCH /api/applications/{id}/status`, whose validation stays authoritative: the card offers only the moves the backend's own state machine reports, and the backend re-checks every one, so a stale card can never put the pipeline into a state the app forbids.
