# Job Listing and Application records carry a schema version, migrated by a one-shot command

Job Listing and Application records (ADR-0008) kept gaining fields, and every one was documented as "absent in records written before this field existed" — so an absent `sourceSnippetIds`, `entryIds`, `usage`, `language`, `statusUpdatedAt`, `statusHistory` or `freshnessStatus` could mean "none" or "predates the field", and each reader invented its own tolerance. Because `data/jobs/` and `data/applications/` are gitignored, no install's corpus has ever been seen and no history exists to reconstruct a record from (issue #100).

Each record now carries its own `schemaVersion` key: in a Job Listing's frontmatter, at the top level of an Application's YAML document, and on each Generation record inside an Application. An absent key is the zero value, `LegacySchemaVersion`; `CurrentSchemaVersion` is 1, defined once in `backend/internal/tracking/schema.go`. On a current record an absent field means genuinely none; on a legacy one it means unknowable. A version higher than the build knows is an error on read, never a silent legacy read. `Save` stamps new records, `RecordGeneration` stamps new Generations, and every other write path carries the version through unchanged — it neither drops nor advances it, so an edit made before migration never skips a backfill migration would have done.

Generations carry their own version because an Application outlives its Generations: a legacy Application brought to v1 keeps its old Generations, whose missing Snippet/Entry ids are still unknowable, and goes on to accumulate new ones whose missing ids mean none. A single per-Application version could not tell the two apart.

Legacy records become current through `backend/cmd/migrate-records`, a thin `main` over `tracking.MigrateRecords`. It is a dry run unless given `-write`, exits 3 when a dry run finds work pending, reads and checks every record before writing any (so an unparseable or newer record halts the run with nothing written), rewrites each record through a temporary file and a rename, edits the YAML as a node tree so unknown keys survive, and keeps a Job Listing's Markdown body byte for byte. It backfills only what is provably on disk: `freshnessStatus: not-yet-checked`, and for an Application still at Saved (no Status moves back to Saved, so it never transitioned) `statusUpdatedAt` and a single Saved `statusHistory` entry taken from the Job Listing's `savedAt`. It never synthesises history past Saved — `stats.go` computes time-in-stage from consecutive history entries and the stale nudges read `statusUpdatedAt`, so invented values would be confidently wrong — and never fills a legacy Generation's `sourceSnippetIds`, `entryIds`, `usage` or `language`; it names each of those in its report instead.

Readers stay tolerant of unversioned records, so the app keeps working before anyone runs the migration. The tolerances that remain are now named legacy-record rules rather than open-ended "might be missing": `parseJobListing` defaults `freshnessStatus` to `not-yet-checked` for legacy Job Listings only, and `IsStale` still treats an empty `statusUpdatedAt` as not stale, because a migrated Application that was past Saved legitimately has none. Changing what aggregate views show on the strength of `GenerationRecord.IsLegacy` (the funnel #36, Snippet usage #48, stale Entries #52) is follow-up work in those issues.

Considered and rejected:

- **A corpus manifest** (`data/schema-manifest.json`). ADR-0008 makes each record a self-describing file created and deleted independently; a manifest is a second source of truth that drifts from the files it describes.
- **Migrating lazily on read.** It turns every read into a write — `List` would rewrite the whole corpus on an ordinary page load, colliding with non-atomic writes (#88) and absent lost-update protection (#89) — and leaves no moment at which the corpus is known to be current.
- **Migration on server startup, or over HTTP.** Deliberately manual, and no corpus-rewriting endpoint on an API that can be LAN-reachable (#57).
- **A separate migration package.** It would need `tracking`'s unexported raw record structs and format helpers; the migration lives beside the format it understands.
- **Advancing a legacy record's version on any edit.** It would silently skip the Saved backfill for an Application transitioned before migration ran, and make migration status unobservable again.

Adding the next version: bump `CurrentSchemaVersion`, record what the new version guarantees in `schema.go`, teach `MigrateRecords` the step from the previous version (what it can backfill, what it must report as unknowable), and update this ADR — never add another "absent in older records" tolerance to a reader.

## Version 2 (2026-09-22, issue #206)

A Job Listing gained **location**, so the version went 1 → 2. On a v2 record an absent location means the source had none, or the user has not entered one; on a v1 or legacy record it is unknowable.

`MigrateRecords` stamps the version and backfills nothing: the board's own wording for where a role is does not appear in the Job Description reliably enough to re-derive, and a guess written into the record would read afterwards as a fact. It names `location` in its report instead, exactly as it already does for a legacy Generation's missing ids.

Applications gained no field, but they share `CurrentSchemaVersion` and are stamped forward too — a record left at v1 would go on claiming v1's guarantees about a corpus that is now at v2.
