# ADR-0049: Master Data import proposes drafts and never writes on its own

- Status: accepted
- Date: 2026-10-31
- Issue: #257

## Context

Without Master Data of your own no Generation is useful, and writing `data/experience/*.md` by hand is the longest step of setup. A Claude call can read a pasted CV, but Master Data is what every Tailored CV is grounded on (ADR-0037/0038): a fabricated bullet there would be reproduced as fact.

## Decision

- A new call site, `import_master_data` (ADR-0025), on Sonnet 5: the output is judgment the user reviews, like Selection. It is priced already; override with `SUMISURA_MODEL_IMPORT_MASTER_DATA`.
- `POST /api/master-data/import/preview` returns drafts and writes nothing. Each proposed bullet is checked against the pasted text (a whitespace-insensitive substring, else the same groundedness check Generation uses) and the ones not found are flagged `notInText` rather than dropped, so the user decides.
- `POST /api/master-data/import/entries` saves one draft at a time, through `atomicfile`, under the id the preview proposed, and answers 409 when that id exists. The preview proposes a free id (`-2`, `-3`, …), so a clash only happens if a file appeared in between. It is a separate route from `POST /api/master-data/entries`, which picks a fresh slug and would silently create a duplicate.
- Claude is reached through an optional `ProposeEntries` method on the client, not through `tracking.Client`; a provider without it answers 501.

## Consequences

- The check is lexical: a bullet that paraphrases the text can pass. The page still shows every draft for approval, which is the real control.
- Only Entries are imported; the Static Sections stay in `profile.yaml`.
