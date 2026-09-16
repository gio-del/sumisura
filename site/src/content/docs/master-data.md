---
title: Writing your Master Data
description: The file shapes behind every tailored CV, and how to write Entries that select well.
---

Master Data is plain files in your checkout. Edit them in the web app or in your
editor; both write the same thing.

## `data/profile.yaml`

**This file is gitignored**, and so is every other file on this page. The repo
tracks stub copies under `data/examples/`; setup copies them into `data/` once
(`cp -r data/examples/. data/`) and from then on you edit untracked files — your
real name, employer and clients never enter git history
([ADR-0038](https://github.com/gio-del/sumisura/blob/main/docs/adr/0038-all-master-data-is-local-only.md)).
Backing `data/` up is therefore yours to arrange: a private repo or a synced
folder.

Contact details plus the **Static Sections** — education, publications,
certifications, awards, activities, languages. These are always included in
full, never selected or rewritten.

Certifications take an optional issuer, date and verification link:

```yaml
certifications:
  - title: "AWS Certified Cloud Practitioner"
    issuer: "Amazon Web Services"
    date: "2025-03"
    link: "https://example.com/verify/123"   # optional
```

```yaml
name: Jane Doe
location: Example City, Country
email: jane.doe@example.com
phone: "+1 555 0100"
linkedin: janedoe
github: janedoe
```

## `data/experience/*.md` and `data/projects/*.md`

One file per Entry: YAML frontmatter, then Markdown bullets.

```markdown
---
employer: Example Consulting S.p.A.
role: Data Engineer
client: Example Client A
location: Example City
start: "2024-10"
end: null            # null means "current"
flagship: true
tags:
  - AI Platform
  - MCP
---

- Built the ingestion service feeding the recommendation models, moving it
  from nightly batches to streaming.
- Led the migration of 40+ dashboards onto the new semantic layer.
```

### One file per client engagement

A consultancy role with three clients is **three Entries**, not one. Selection
works at Entry level, so splitting lets one client's work be surfaced for a job
where it is relevant while the others stay out.

### Tags matter

`tags` are the raw material for the Tech Stack section and a strong signal
during Selection. Keep spellings consistent — the app has a tag-lint view that
finds near-duplicates ("Postgres" vs "PostgreSQL").

### Bullets are printed literally

The files are Markdown, but the PDF template prints a bullet exactly as
written: `` `server.json` `` comes out with its backticks showing. Code spans
are unwrapped for you when the CV is assembled; asterisks and underscores are
left alone, because they may be real punctuation, and the quality check names
any that would be visible in the PDF.

### Write bullets you would defend

Rewrite may re-word a bullet, never invent one. A bullet with a real number in
it can be re-emphasised; a bullet with no number will never grow one. If a
result is worth claiming, put the evidence in Master Data.

## `data/cover-letter-snippets/*.md`

Optional. Reusable paragraphs — an opening, a closing, a "why this company"
pattern — that cover-letter drafting can draw on.

`lang` is the ISO 639-1 code the paragraph is written in, and it is optional:
a Snippet without one is usable in any language. When you apply in more than
one language, write each Snippet twice — one `lang: en`, one `lang: it` — and
drafting picks the one already in the target language. It falls back to an
unmarked Snippet, and only translates a Snippet from another language when
nothing else fits. That is the point of the field: a machine translation of
your own vetted wording reads like machine output, which is what a Snippet
library exists to avoid.

```markdown
---
kind: opening
lang: en
tags: [data-engineering]
---

I have spent the last four years making data pipelines boring: predictable,
observable, and cheap to run.
```

## What is not Master Data

`output/` is derived (safe to delete), and `data/jobs/` and
`data/applications/` are your tracking records — gitignored, because they are
about your job search rather than your history.
