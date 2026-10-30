# Third-party notices

This project bundles or vendors the third-party work listed here. Each item
keeps its own licence; nothing here is covered by this project's AGPL-3.0
licence.

Dependencies resolved at build time (Go modules, npm packages) are not listed
individually — see `backend/go.mod`, `frontend/package.json` and
`extension/package.json`, and their lock files, for the authoritative set.

## Vendored source

### Turndown

- File: `extension/turndown.js`
- Upstream: https://github.com/mixmark-io/turndown
- Copyright (c) 2017 Dom Christie
- Licence: MIT (the full text is reproduced in the file's header)

Vendored rather than installed because the browser extension ships unbundled,
with no build step — content scripts load the file directly (see
`extension/manifest.json`).

### clean-user-facing-text skill (fetched, not committed)

- Pinned in: `plugins/sumisura/skills/tailor-cv/clean-skill.lock` (commit and
  per-file sha256), downloaded by `tailor-cv/scripts/clean-skill.sh`
- Upstream: https://github.com/guillaumemeyer/watermarks-remover
  (`skills/clean-user-facing-text`)
- Copyright (c) 2026 Guillaume Meyer and contributors
- Licence: MIT (the full text is downloaded with it, as `LICENSE`)

Fetched at run time from an exact commit and checked file by file against the
lock, so every Generation runs the same reviewed scripts. Update it by changing
the commit in the lock and regenerating the hashes.

## Fonts

### Instrument Serif

- Used for: the Sumisura wordmark (converted to outlines in `brand/`), display
  headings in the app and on the site
- Copyright: Copyright 2022 The Instrument Serif Project Authors
  (https://github.com/Instrument/instrument-serif)
- Licence: SIL Open Font License 1.1

### Inter

- Used for: UI and body text
- Copyright: Copyright (c) 2016-present Rasmus Andersson
- Licence: SIL Open Font License 1.1

The SIL Open Font License 1.1 is available at
https://openfontlicense.org. Outlines embedded in `brand/*.svg` are a permitted
use: the OFL allows embedding in documents, and no font file is redistributed
as a font.

## Typst templates

`template/cv.typ` and `template/cover-letter.typ` render with
[Typst](https://typst.app) (Apache-2.0), which is invoked as an external CLI
and is not bundled with this repository. The font used in rendered PDFs
(Liberation Sans, SIL OFL) comes from the container image, pinned per ADR-0012.
