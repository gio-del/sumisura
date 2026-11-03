# ADR-0050: The web app's Generation runs the prose clean-up, Unicode pass only

- Status: accepted
- Date: 2026-11-03
- Issue: #231

## Context

The `tailor-cv` skill cleans the rewritten bullets, the About me and the Cover Letter with the upstream `clean-user-facing-text` skill before Text Review (#229). The web app's Generation skipped that pass.

## Decision

- The backend runs the same pinned scripts (`clean-skill.lock`) as `python3 -I`, on the directory named by `SUMISURA_CLEAN_SKILL_DIR`, with the interpreter overridable by `SUMISURA_PYTHON`.
- The release image downloads and verifies the files at build time with `clean-skill.sh` and the same lock, and installs Python 3. Nothing is fetched at run time.
- Only the deterministic parts run: the Unicode pass (without space normalization) and the stylometry score before and after. The skill's rewrite on a `high` tier is not part of the app and would be a separate decision, since it is an extra Claude call.
- It is not blocking. If the variable is unset, Python is missing or a script fails, the texts stay as the Rewrite wrote them and the result says the clean-up is not available.
- The outcome is `GenerateResult.cleanup`: per text (bullets, About me, Cover Letter) the score before and after, the density tier and the characters changed. Text Review shows it as one compact block. It runs before the groundedness checks, which therefore see the cleaned text.

## Consequences

- A checkout run (`go run ./cmd/server`) shows "not available" unless the variable points at a downloaded copy (`scripts/clean-skill.sh` prints one).
- Default Mode and the Selection preview run no Rewrite and carry no `cleanup`.
