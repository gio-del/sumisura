<!--
  Title format (checked by CI): <type>[!]: <lowercase subject>
  e.g. feat: add an Applications view grouped by Status
       fix!: reject the pre-rename LAN header
  Types: feat fix perf refactor docs test build ci chore revert
  Only feat/fix/perf and breaking changes reach the changelog.
-->

## Summary

<!-- What does this PR change, and why? Reference the PRD/issue it implements. -->

## Checklist

- [ ] Branched from `main` (not committing directly to `main`)
- [ ] Title is a Conventional Commit; `!` if a self-hoster must act (renamed env var/header, record migration, removed route, plugin reinstall)
- [ ] New architectural decisions recorded in `docs/adr/` (see `CONTEXT.md`)
- [ ] `CONTEXT.md` updated if this PR introduces or renames domain vocabulary
- [ ] Docs updated for anything a self-hoster or integrator sees: `site/` for setup, configuration and the API reference; `CONTRIBUTING.md` for the dev loop; `CONTEXT.md`/ADRs for vocabulary and decisions. CI checks routes, env vars and CLI flags (`backend/internal/docsdrift`); prose, screenshots and behaviour descriptions are on you

## Test plan

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass in `backend/`
- [ ] `npx tsc -b` and `npx oxlint .` pass in `frontend/`
- [ ] Manually exercised the change (dev server / real app), or noted why that wasn't possible

<!-- Add specifics: which flows you clicked through, which live APIs you hit, etc. -->
