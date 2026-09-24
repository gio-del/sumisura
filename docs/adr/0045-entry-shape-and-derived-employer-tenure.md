# ADR-0045: An Entry has a shape, and Employer Tenure is derived from all of them

- Status: accepted
- Date: 2026-09-23
- Relates to: [ADR-0003](0003-master-data-per-entry-files.md), [ADR-0012](0012-render-shells-out-to-typst-in-container.md), [ADR-0028](0028-skill-runs-app-quality-checks-via-cvcheck-cli.md)

## Context

A CV rendered for a consultancy career was claiming the wrong employment history, and every check passed while it did.

Selecting three Quantyca Client Engagements — Arcese (Jul 2025 – Nov 2025), Autostrade (Oct 2024 – Jul 2025) and Chiesi (Jun 2026 – present) — produced this:

```
Junior Data Engineer – Quantyca S.p.A., Milan area        Jul 2025 – Nov 2025
Arcese
 • …
Autostrade per l'Italia
 • …
```

A reader sees five months of tenure at a junior title, ended ten months ago. The truth is Oct 2024 to the present, currently at Data Engineer. Page count, ATS-parsability, groundedness and language all reported clean, because each of them was right: the bullets traced to their sources, the text layer extracted, the page fit.

The cause was in the model, not the renderer. `Entry.start`/`end` was overloaded. The files hold the *engagement's* span, but `CONTEXT.md` said Client Engagements "share the same employer/title/date-range metadata", and `template/cv.typ` agreed with the glossary: it opened an employer block with whichever Entry sorted first and printed that Entry's role and dates as the whole employer's, then rendered every later engagement with no dates at all.

Underneath that sat a missing distinction. An **In-house Role** and a **Client Engagement** have genuinely different shapes:

- In-house, the employer *is* the job. One role, one span, one row. The template was written for this shape.
- At a consultancy, the employer tenure is the outer fact and each engagement is an inner span with its own role and dates, and two engagements may run concurrently.

Nothing in the model marked which shape an Entry had, so the renderer applied the in-house shape to consultancy data. And the fact a CV most needs to state about a consultancy employer — the tenure — existed nowhere: no term, no field, nothing to check against.

## Decision

**1. An Entry records its shape.** Experience Entries carry `kind: consultancy | in-house` in their frontmatter. When absent it is inferred — a `client` means `consultancy`, no `client` means `in-house` — so existing Master Data keeps working and no migration is forced.

Inference alone was the alternative, and it fails the case it most needs to get right: a Client Engagement whose client cannot be named (an NDA, a client who does not want to be listed) has no `client`, so inference would call it in-house and render it as a standalone job. Recording the shape also makes the invariants below checkable, because "this employer is a consultancy" stops being a guess about missing data.

**2. Employer Tenure is derived, from all of an employer's Entries.** Not from the Selected ones. Omitting a Client Engagement from a Generation narrows what the CV shows about the work; it must never narrow the tenure the CV claims. Two stints at one employer separated by a real gap are two Tenures — the span is never stretched across a gap to swallow it.

**3. The template renders the two facts separately.** The employer block header carries the employment fact (employer, location, Tenure). Each engagement beneath it carries its own role, client and span, so it is self-describing and a promotion reads correctly. An In-house Role, having no client, collapses back to a single row.

**4. Temporal claims are checked, at both levels.** A new check runs at the Visual Review check point, through the same `cvcheck` CLI the skill shells out to (ADR-0028):

- *Against Master Data*: every rendered span traces to an Entry; a rendered Employer Tenure equals the derived one; ongoing work is not rendered as ended.
- *Against the rendered text layer*: each Employer Tenure and each engagement span actually appears in the PDF.

The second level is what would have caught this bug. The assembled JSON was correct — it carried every engagement's true dates — and the template discarded them. A data-level check alone would have passed, exactly as the existing checks did. Anything a CV asserts about time has to be verified where the reader reads it.

Advisory, like every other check (ADR-0028): it reports, it never blocks a checkpoint.

## Consequences

- Every rendered CV changes shape for consultancy employers. This is the point; the previous shape was making a false claim.
- `kind` joins the Entry schema, the Master Data stubs under `data/examples/` and the Master Data docs page. Absent stays valid forever, since inference is the documented default and not a migration window. The web app's Entry editor offers it as a Shape field, defaulting to "Infer from client" so an Entry created there behaves exactly as one written by hand without it.
- The checks gain a class they did not have. Groundedness asked "does this sentence trace to a source bullet?"; nothing asked "does this date trace to anything?". Dates are claims too, and on a CV they are among the most consequential.
- An employer with concurrent engagements renders them as overlapping spans, which is what happened, rather than as a sequence.
