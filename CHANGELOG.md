# Changelog

## [0.3.0](https://github.com/gio-del/sumisura/compare/v0.2.0...v0.3.0) (2026-09-22)


### Features

* finish a shared job on the phone, with Company and Title suggested from the pasted description ([#202](https://github.com/gio-del/sumisura/issues/202)) ([bb34172](https://github.com/gio-del/sumisura/commit/bb34172230fe97c0e912c3a5bf9f8648f4e33158))
* keep an ATS Report on every Generation, with a view of what an ATS sees ([#203](https://github.com/gio-del/sumisura/issues/203)) ([2831645](https://github.com/gio-del/sumisura/commit/2831645bd70e48178501f1a9d0de276e1a154acc))

## [0.2.0](https://github.com/gio-del/sumisura/compare/v0.1.0...v0.2.0) (2026-09-17)


### Features

* add a Certifications Static Section to the profile ([#172](https://github.com/gio-del/sumisura/issues/172)) ([b6e7f4e](https://github.com/gio-del/sumisura/commit/b6e7f4e2ad8a89bee603509aca19926e2ebba260))
* cap and round-robin the derived Tech Stack line ([#177](https://github.com/gio-del/sumisura/issues/177)) ([aafd794](https://github.com/gio-del/sumisura/commit/aafd794d2c637d8c7f8c2a6bb3cd25aa6905ebc3))
* complete a Pending Capture when its posting is captured on the desktop ([#191](https://github.com/gio-del/sumisura/issues/191)) ([3da3d85](https://github.com/gio-del/sumisura/commit/3da3d850dccc0aab4a05a1b7930a038aaa79d87d))
* delete a Generation's files from Generated CVs ([#175](https://github.com/gio-del/sumisura/issues/175)) ([3ac2e2d](https://github.com/gio-del/sumisura/commit/3ac2e2d3b478d153d711f22fd2ddd6355d36173b))
* enter the access token once per device in the web app ([#188](https://github.com/gio-del/sumisura/issues/188)) ([2a58ee4](https://github.com/gio-del/sumisura/commit/2a58ee44746cfe2654d45374516dd67aaaa88e73))
* give Cover Letter Snippets a language ([#176](https://github.com/gio-del/sumisura/issues/176)) ([a110786](https://github.com/gio-del/sumisura/commit/a1107862b3bbfdf96dced874d1983fd83118d74b))
* install Sumisura on Android and share jobs to it ([#193](https://github.com/gio-del/sumisura/issues/193)) ([3d421bf](https://github.com/gio-del/sumisura/commit/3d421bf73692ef5c31a55d816c93b5ea13a80907))
* keep the user's profile.yaml out of git ([#163](https://github.com/gio-del/sumisura/issues/163)) ([0bc63f0](https://github.com/gio-del/sumisura/commit/0bc63f0e33b8da6dd5c5f932e05abaf4e4cc0dc3))
* list every CV generated so far ([#174](https://github.com/gio-del/sumisura/issues/174)) ([09a3ef2](https://github.com/gio-del/sumisura/commit/09a3ef26bcf1088c3b3605f8c5bb81639e3c2b08))
* publish a prebuilt image with the backend serving the frontend ([#179](https://github.com/gio-del/sumisura/issues/179)) ([b160417](https://github.com/gio-del/sumisura/commit/b160417a1db743c95921cba5973e404b033e843a))
* save job links shared from another device to complete later ([#190](https://github.com/gio-del/sumisura/issues/190)) ([454b832](https://github.com/gio-del/sumisura/commit/454b832b7c65cb7200aa04a66cd269ba34bb55bd))
* save shared Greenhouse, Lever and Ashby links straight as Job Listings ([#192](https://github.com/gio-del/sumisura/issues/192)) ([fe73ed5](https://github.com/gio-del/sumisura/commit/fe73ed53833ee4447e946e07349b548b174416f9))
* stop Markdown markup reaching the PDF verbatim ([#178](https://github.com/gio-del/sumisura/issues/178)) ([64cd342](https://github.com/gio-del/sumisura/commit/64cd34224f80d54d069108b76ecc1eb829887c7e))


### Bug fixes

* scrub real employer and client names from tests and fixtures ([#171](https://github.com/gio-del/sumisura/issues/171)) ([4ead8b9](https://github.com/gio-del/sumisura/commit/4ead8b90c280f8dafb9d3cd668da6859b3ef296c))
* send the access token from the browser extension and the tailor-cv skill ([#196](https://github.com/gio-del/sumisura/issues/196)) ([e909f44](https://github.com/gio-del/sumisura/commit/e909f444edd96b243310de650c485ffd5d2f0616))

## 0.1.0 (2026-09-15)

The first public release of **Sumisura** — self-hosted CV tailoring that keeps
every rewritten bullet grounded in what you actually did.

The project was previously called CV Reporter and was never released. This
release is the point at which it becomes something another person can install:
it has a licence, a name, documentation and a version.

### What you get

* **Tailoring.** `/sumisura:tailor-cv` walks Selection → Rewrite → Text Review
  (you approve) → Render → Visual Review (you approve), and produces a one-page
  CV and a matching cover letter for one specific job.
* **A groundedness check** that compares every rewritten bullet against the
  Entry it came from, before you are asked to approve anything — plus page
  count, ATS-parsability and language checks on the rendered PDF.
* **Application tracking**: Job Listings and Applications with status, notes,
  contacts, salary ranges and the Generations produced for each, fed by paste,
  by Greenhouse/Lever/Ashby boards, or by the browser extension on LinkedIn and
  Indeed.
* **Documentation** at https://gio-del.github.io/sumisura/ — quickstart, Master
  Data, tailoring, configuration, LAN mode, upgrading, troubleshooting.
* **AGPL-3.0**, with the name and logo kept as trademarks, and an explicit
  statement that the CVs and cover letters you generate are yours.

### ⚠ BREAKING CHANGES

* **rename to Sumisura** ([#145](https://github.com/gio-del/sumisura/pull/145)).
  Only affects an installation that predates this release: environment
  variables moved from `CV_REPORTER_MODEL_*` to `SUMISURA_MODEL_*`, the LAN
  header is now `X-Sumisura-Token`, and the skill is installed as
  `sumisura@sumisura-local` and invoked as `/sumisura:tailor-cv`. No
  compatibility fallbacks — see
  [ADR-0036](https://github.com/gio-del/sumisura/blob/main/docs/adr/0036-renamed-to-sumisura.md).

### Features

* add the landing page and docs site (Astro + Starlight) ([#147](https://github.com/gio-del/sumisura/issues/147)) ([953fe52](https://github.com/gio-del/sumisura/commit/953fe52e071dbb39b98d4d1e83a85631c93b1557))
* license under AGPL-3.0 and add the community health files ([#144](https://github.com/gio-del/sumisura/issues/144)) ([f638231](https://github.com/gio-del/sumisura/commit/f638231d83881bb2f79597e5f234fba535c42899)), closes [#135](https://github.com/gio-del/sumisura/issues/135)
* redesign the brand identity for Sumisura ([#143](https://github.com/gio-del/sumisura/issues/143)) ([5a0fb9d](https://github.com/gio-del/sumisura/commit/5a0fb9dd5d690cb6a0df422294597cce2bc370db)), closes [#134](https://github.com/gio-del/sumisura/issues/134)
* rename to Sumisura ([#145](https://github.com/gio-del/sumisura/issues/145)) ([bffdd59](https://github.com/gio-del/sumisura/commit/bffdd593f28d61cb22867f914cc91f5a6e8fb8bf)), closes [#136](https://github.com/gio-del/sumisura/issues/136)

### Install

See the [quickstart](https://gio-del.github.io/sumisura/quickstart). You need
Docker, an Anthropic API key, Claude Code, and the `typst` CLI. The browser
extension is attached to this release as a zip; load it unpacked.
