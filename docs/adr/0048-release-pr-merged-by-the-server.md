# ADR-0048: The Release PR is merged by a job on the maintainer's server

- Status: accepted
- Date: 2026-10-25
- Relates to: [ADR-0039](0039-backend-serves-the-production-frontend.md)

## Context

Releases used to ship only when a human merged the release-please Release PR. The maintainer also runs the app on their own server, which makes that server a good judge of whether the code is healthy.

## Decision

A scheduled job on the maintainer's server merges the Release PR, by fixed rules, when all of these hold:

- the changelog has at least one `feat`, `fix` or `perf` entry;
- the bump is not major, not the move to 1.0, and nothing is marked breaking (the maintainer always merges those);
- CI on the Release PR is green;
- at least 7 days since the previous release, unless the release is fix-only;
- the code has run on the server for at least 3 days without failures and with no new `bug` issue since.

The merge is announced at least 24 hours ahead. The maintainer can postpone it (the job then waits 7 days or until the changelog changes) and can merge or close the Release PR by hand at any time.

## Consequences

- Releases follow a steady cadence without manual work, after a soak period on real use.
- The workflow, its merge method and its permissions are unchanged; only the documentation records who merges.
