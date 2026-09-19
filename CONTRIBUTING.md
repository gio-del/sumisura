# Contributing

Thanks for taking a look. This is a small project with one maintainer, so the
process is light — but a few things are deliberate, and this file explains
which.

Read [`CONTEXT.md`](CONTEXT.md) first for the domain vocabulary (Master Data,
Entry, Selection, Rewrite, Tailored CV, Job Listing, Application, …); the rest
of this file assumes it. [`docs/adr/`](docs/adr/) records why the architecture
looks the way it does — check it before proposing to change something
structural.

## Before you start

- **Open an issue first** for anything beyond a small fix, so we agree on the
  shape before you write code.
- Branch from `main`; never commit to `main` directly.
- By contributing you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Contributor Licence Agreement

First-time contributors are asked to sign a CLA (the bot comments on your first
pull request). It keeps the project's licensing options open — including a
hosted version under different terms — while your contribution stays available
to everyone under the AGPL. If you would rather not sign, open an issue
describing the change and it can be implemented independently.

## Pull request titles

PR titles follow [Conventional Commits](https://www.conventionalcommits.org)
and **CI rejects a title that doesn't**
(`.github/workflows/pr-title.yml`). The reason is not style: the title becomes
the commit on `main`, and release-please reads those commits to build
`CHANGELOG.md` and pick the next version — an unparseable title is a change
missing from the changelog.

```
feat: add an Applications view grouped by Status
fix: keep timestamped Notes when an Application is archived
feat!: rename to Sumisura        # "!" marks a breaking change
build(deps): bump jsdom to 30.0.1
```

Types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `build`, `ci`,
`chore`, `revert`. Subjects start lowercase and don't end with a period. Only
`feat`, `fix`, `perf` and breaking changes (`!`, or a `BREAKING CHANGE:`
footer) appear in the changelog. PRs are squash-merged, so **the PR title
becomes the commit message on `main`** — write it for someone reading
`git log` a year from now.

## Releases

Releases are cut from an open **Release PR** that release-please keeps up to
date: it accumulates the changelog and bumps the version in `CHANGELOG.md`, the
extension manifest, both `package.json` files, `plugin.json` and
`marketplace.json`. Merging that PR tags `vX.Y.Z`, publishes the GitHub Release,
and attaches the extension zip.

One version covers the whole product — backend, frontend, extension and plugin
are only guaranteed to work together at the same commit. Before 1.0 a breaking
change bumps the minor version; **1.0.0 is reserved for the hosted launch**.

A change is breaking if a self-hoster has to act: a renamed environment
variable or header, a record schema change needing `migrate-records`, a removed
API route, or a plugin that must be reinstalled. Say so in the PR body as well
as the `!`.

## Dev loop

Prerequisites: Go, Node, Docker, the `typst` CLI, and `pdftotext` (poppler) for
the ATS-parsability check.

```
cp .env.example .env          # ANTHROPIC_API_KEY for anything that calls Claude
docker-compose up             # backend 127.0.0.1:8080, frontend 127.0.0.1:5173
```

Backend, from `backend/`:

```
go build ./... && go vet ./... && go test ./...
golangci-lint run ./...
```

Frontend, from `frontend/`:

```
npm run dev      # served by the frontend service above inside Docker
npm run build    # tsc -b + vite build
npm run lint
npm test         # npm run test:watch while working
```

Extension, from `extension/`: `npm test`.

## Tests

Backend tests are Go `testing`-package HTTP integration tests, run with `go test ./...` from `backend/`.

Backend lint: `golangci-lint run ./...` from `backend/`. It reads [`.golangci.yml`](.golangci.yml) at the repo root and is the same command CI runs, so a red lint build is reproducible locally. The enabled set is deliberately small — `gofmt`, `errcheck` (with `check-blank`, so `_ = someCall()` is flagged too), `ineffassign`, `unused`, `govet` — with the rationale for what's left out recorded in the config file itself. A genuinely intentional discard gets a `//nolint:errcheck` with a reason rather than a weaker gate. Install: `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2` (the version CI pins).


#### API contract fixtures

`frontend/src/api/types.ts` is hand-written, so a contract test keeps it honest (issue #99, ADR-0035). `backend/internal/api/contract_test.go` drives every JSON-returning route through its real handler and compares the response, normalized (sorted keys; fixed timestamps, version tokens and other clock-derived values), with a golden file in `frontend/src/api/contract/fixtures/`. `frontend/src/api/contract/contract.ts` then type-asserts each fixture against the type `types.ts` declares for that route, so `npm run build` (`tsc -b`) fails with the route and JSON path of any field that is sent but undeclared, declared required but not sent, of the wrong kind, or whose optionality disagrees with the backend (`.populated` fixtures must send every optional field, `.sparse` ones none).

A plain `go test ./...` only verifies, and fails when a response no longer matches its fixture. When a response shape changes on purpose, regenerate the fixtures, review the JSON diff, then update `types.ts` until the frontend build passes:

```
cd backend && UPDATE_CONTRACT_FIXTURES=1 go test ./internal/api -run TestContract
```

Adding a route means adding its fixture to `contractFixtures` and an assertion to `contract.ts` (or an exemption with a reason, for a route with no JSON body); `TestContractRoutesAllCovered` fails otherwise.


### Frontend dev loop

From `frontend/`: `npm run dev` (served by the `frontend` service above inside Docker), `npm run build`, `npm run lint`, `npm test` (`npm run test:watch` while working).

Frontend tests are Vitest + Testing Library, rendering the real page inside a real router and faking only HTTP with Mock Service Worker — the API client, its query-string building and its error handling all run for real, and an unhandled request fails the test. They need no backend, no `ANTHROPIC_API_KEY` and no `typst`. See [`docs/adr/0019-frontend-tests-fake-only-the-network.md`](docs/adr/0019-frontend-tests-fake-only-the-network.md) for why that seam, and `frontend/src/pages/GenerationPage.test.tsx` for the example to copy when adding more.


## Architecture decisions

A change that settles something structural — a new seam, a storage decision, a
rule about what the app may or may not do — gets an ADR in
[`docs/adr/`](docs/adr/), numbered in sequence, in the same pull request as the
code. Superseded ADRs are marked superseded, never rewritten.

## Keeping docs in sync

If a change alters something `README.md`, `CONTEXT.md`, `docs/adr/` or the
docs site (`site/src/content/docs/`) documents (a new or changed API route, a
new top-level directory, a new running-it step, a superseded decision, anything
a self-hoster configures or runs), update that documentation **in the same pull
request**, not in a follow-up.

`go test ./...` checks part of this: the docs drift test
(`backend/internal/docsdrift`) fails when an API route is missing from the
site's API reference, an environment variable from the Configuration page, or
a `cvcheck`/`migrate-records` flag from its page, and when a page documents one
the code no longer has.

## Checklist before you open a PR

- [ ] Branched from `main`
- [ ] `go build ./... && go vet ./... && go test ./...` pass in `backend/`
- [ ] `npm run build` and `npm run lint` pass in `frontend/`
- [ ] Contract fixtures regenerated if any response shape changed
- [ ] Docs updated for anything a self-hoster or integrator sees (site for setup, config and API; this file for the dev loop)
- [ ] New architectural decisions recorded in `docs/adr/`
- [ ] `CONTEXT.md` updated if domain vocabulary changed
- [ ] Manually exercised the change in the real app, or said why that wasn't possible
