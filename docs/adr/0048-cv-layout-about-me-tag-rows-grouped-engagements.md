# ADR-0048: The CV leads with an About me, groups engagements as dated-by-employer projects, and replaces the Tech Stack with Tag Rows

- Status: accepted
- Date: 2026-10-26
- Amends: [ADR-0045](0045-entry-shape-and-derived-employer-tenure.md) (engagement spans on the rendered CV)
- Issue: #232

## Context

The owner hand-tuned a CV with a colleague's advice and the result became the model for every Tailored CV: an About me under the header; the consultancy employment as one block (one role, one tenure) with each Client Engagement below it as a project, named by a project title and an industry label instead of the client's name, with a short row of its own tags; no separate Tech Stack; the thesis and its publication under the degree; the remaining Static Sections on one closing line. Two engagements of the same kind of work (a motorway operator and a logistics company) were shown as one project.

## Decision

- `profile.yaml` gains optional `headline`, `summary`, `location_note` and `employers` (per Entry `employer` value: `display`, `role`), `education[].thesis` and `publications[].degree`. Entries gain optional `title`, `client_label` and `group`. Without them the CV still renders: the block falls back to the employer's name, the project line to role and client.
- The About me is `summary`, adapted by Rewrite under its own rules (no new facts) and scored by the groundedness check against the profile's text; Default Mode prints it as written. No `summary`, no About me: a Client that writes one anyway is ignored.
- Each selected Entry prints a Tag Row of at most ten of its own Tags, chosen by Selection; a tag the Entry does not have is dropped (app) or refused by the check (skill). The derived Tech Stack section is removed: it repeated every tag in one undifferentiated line.
- Client Engagements of one employer print under one block stating the Employer Tenure and, when set, the employer's display name and role. **The engagements print without their own dates.** This amends ADR-0045: its timeline check still compares every engagement's span with Master Data and still requires the block's tenure in the PDF text, but no longer requires each engagement's span there. An In-house Role keeps its own span.
- Engagements of one employer sharing a `group` print as one project (Engagement Group): the group is the title, Client Labels, Tag Rows and bullets are merged; the backend keeps them adjacent.
- Name and section headers are set in capitals, so the ATS-parsability check matches case-insensitively.

## Consequences

- The reader no longer sees how long each engagement lasted, only the tenure. That was the owner's explicit choice (shorter, and the colleague's advice), weighed against ADR-0045's concern that a reader attributes the block's dates to each engagement. The Master Data comparison keeps the dates honest at the source.
- Tag Rows are only as good as the Tags: renaming a tag in Master Data is how the CV's wording changes.
- A Generation recorded before this ADR re-renders with the new layout; its Selection carries no tags, so each Entry falls back to its first ten.
