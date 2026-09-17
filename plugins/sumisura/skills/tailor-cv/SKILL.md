---
name: tailor-cv
description: Generate a tailored, one-page CV (PDF), plus a Cover Letter when a job is given, from this repo's Master Data, for a specific job (paste text or a URL), for a role already tracked as a Job Listing/Application in the web app (by company or job title), or, with no job given, a general-purpose Default Mode CV. Use when the user wants to apply for a job, update their CV, or asks to run/build/tailor a CV.
---

# Tailor CV

Read `CONTEXT.md` at the repo root first — it defines the vocabulary used below (Master Data, Entry, Client Engagement, Tailoring, Selection, Rewrite, Static Section, Generation, Tailored CV, Job Listing, Application, Status). Follow ADRs under `docs/adr/`.

## Inputs

Work out which of three modes this run is in, in this order:

1. **Targeting a tracked Application** — the user refers to a company/role they've already saved via the web app or the LinkedIn extension (e.g. "generate a CV for the Acme Corp application", "tailor for the Senior Backend Engineer role at Acme"). Go to "Targeting a tracked Application" below before starting the Pipeline.
2. **Job Description given directly** — pasted text, or a URL (fetch it).
3. **Default Mode** — no input at all.

If it's genuinely unclear whether the user means a tracked Application or is just naming a company as part of pasted/URL text, ask rather than guessing.

## Targeting a tracked Application

This mode needs the backend running (`docker-compose up` — `http://127.0.0.1:8080`); the other two modes don't. Make every backend call through `./plugins/sumisura/skills/tailor-cv/scripts/api.sh METHOD PATH [JSON_BODY]`, never a bare `curl`: when the installation runs with an access token (`LAN_AUTH_TOKEN`), the backend rejects any request without it with `401`, and the script adds it from the environment or `.env` on its own. Never print, echo or ask the user for the token. A `401` means the token in `.env` doesn't match the running backend's — say so and stop. If any backend call below fails for any reason (connection refused, 404, unexpected status), stop immediately with a clear message to the user — do not fall back to treating their company/title text as literal Job Description text, and do not proceed to Render believing a Generation is tracked when it isn't.

1. **Look up the Application.** Run:
   ```
   ./plugins/sumisura/skills/tailor-cv/scripts/api.sh GET '/api/job-listings?archived=all'
   ```
   This returns every tracked Job Listing paired 1:1 with its Application: `[{ "jobListing": {...}, "application": {...} }, ...]`. Keep `archived=all`: without it the endpoint leaves out Archived Job Listings, and the user can still mean one of those. A connection error or non-2xx here means the backend isn't running — stop and tell the user to run `docker-compose up`. Otherwise, match the user's text case-insensitively against each entry's `jobListing.company` and `jobListing.title`.
   - **No match**: stop and tell the user you couldn't find it — don't silently fall through to Default Mode or treat their text as a Job Description.
   - **One match**: proceed with it.
   - **More than one match**: list the candidates (company, title, `application.status`) and ask the user which one they mean before doing anything else.

   `jobListing` and `application` share the same `id` (they're 1:1) — keep it, it's needed in steps 4–5 below.

   An Application may carry `application.notes`: the user's own log of the process (what a recruiter said, a salary figure discussed). Notes are not Master Data and not the Job Description — never feed them into Selection, Rewrite or the Cover Letter, so nothing from a Note can appear on a Tailored CV.

### Optional actions on a matched Application

Once an Application is matched (step 1 above), any of the four actions below can be run before, after, or instead of the Pipeline (steps 2–5) — none of them require running Selection/Rewrite/Render, so e.g. "retry resolution for the Acme application" is a complete request on its own, with no CV generation involved. Each uses the same `<id>` from step 1 and inherits the same hard rule as the rest of this mode: a connection error, non-2xx response, or unexpected shape means stop and report — never continue as if it succeeded.

- **Retry resolution.** Offer this when checking an Application's status and either `jobListing.ral.source` or `application.method.kind` is `"unresolved"`. Bodyless — safe to call even if nothing is unresolved (it's a no-op):
  ```
  ./plugins/sumisura/skills/tailor-cv/scripts/api.sh POST /api/job-listings/<id>/resolve
  ```
  Returns the same `{"jobListing": {...}, "application": {...}}` shape as step 1 — re-read `jobListing.ral` and `application.method` off the response to see what changed. A field that still can't be resolved simply stays `"unresolved"`; that alone isn't an error.

- **Suggest a Contact.** Offer this when the user asks about a contact for the matched Application and `application.contact` is absent. Bodyless:
  ```
  ./plugins/sumisura/skills/tailor-cv/scripts/api.sh POST /api/job-listings/<id>/suggest-contact
  ```
  Returns a Contact suggestion to show the user — `{"name": "...", "email": "..."}`. This is research only; it is never written to the Application. Only apply it if the user explicitly accepts, via the Contact-correction call below.

- **Correct the Contact.** Use this to apply an accepted suggestion, or a manual correction, to the Application's Contact. Ask the user for `name` and `email` if you don't already have them (e.g. from a suggestion just shown) — never fabricate either field. `email` is required (a `400` response means it was missing or blank):
  ```
  ./plugins/sumisura/skills/tailor-cv/scripts/api.sh PATCH /api/applications/<id>/contact '{"name": "<name>", "email": "<email>"}'
  ```
  Returns the updated Application (`Content-Type: application/json`, same `application` shape as step 1).

- **Correct the Method.** Use this to override `application.method` by hand. Ask the user which `kind` they mean — one of `"portal"`, `"email"`, `"easy_apply"`, `"other"` (`"unresolved"` is a system-set sentinel, not something a user can pick — a `400` response means an unknown `kind` was sent) — and, if applicable, `value` (the detected application URL or email address; leave it empty for `"other"` or when nothing applies):
  ```
  ./plugins/sumisura/skills/tailor-cv/scripts/api.sh PATCH /api/applications/<id>/method '{"kind": "<kind>", "value": "<value>"}'
  ```
  Returns the updated Application.

2. **Use its Job Description.** The list in step 1 carries only a summary of each Job Listing — `jobListing.hasJobDescription` says whether there is one, not what it says — so fetch the matched record in full:
   ```
   ./plugins/sumisura/skills/tailor-cv/scripts/api.sh GET /api/job-listings/<id>
   ```
   It returns the same `{"jobListing": {...}, "application": {...}}` shape, and its `jobListing.jobDescription` is this run's Job Description — feed it into Selection/Rewrite in the Pipeline below exactly as pasted/URL text would be. For the label behind the Pipeline's `<slug>` (step 5), default to a kebab-case slug of the company name (e.g. `acme-corp`) unless the user prefers another — step 5 still appends the timestamp to it.

3. Run the Pipeline below in full (Load Master Data → Selection → Rewrite and Cover Letter draft → Text Review → Assemble → Render → Visual Review) — nothing about it changes for this mode. Come back here once Visual Review is approved.

4. **Record the Generation.** POST the rendered result to the same endpoint the web app's own Generate button uses, so it appears in the Application's history there (`<id>` is the id from step 1, `<lang>` the target language written into `data.json`). The groundedness result is attached the same way the app attaches it, by re-running the check over the approved `selection.json` in JSON mode, in the same command:
   ```
   groundedness="$(./plugins/sumisura/skills/tailor-cv/scripts/quality-check.sh groundedness --selection output/<slug>/selection.json --json)"
   ./plugins/sumisura/skills/tailor-cv/scripts/api.sh POST /api/applications/<id>/generations \
     "{\"slug\": \"<slug>\", \"cvPath\": \"output/<slug>/cv.pdf\", \"coverLetterPath\": \"output/<slug>/cover-letter.pdf\", \"sourceSnippetIds\": [<ids>], \"language\": \"<lang>\", \"groundedness\": ${groundedness:-null}}"
   ```
   Use the full timestamped `<slug>` from step 5 (the directory actually written), not the bare label.
   The check's `--json` output is exactly the `groundedness` shape the endpoint accepts (`{}` when nothing was flagged), so the Generation displays in the web app like an app-produced one. If the check couldn't run it prints nothing and `null` is sent — never an invented result. `coverLetterPath` and `sourceSnippetIds` record the Cover Letter approved at Text Review and Visual Review: `<ids>` is the Snippet ids it drew from as quoted JSON strings (`\"opening\", \"closing\"` inside the double-quoted JSON body string), already checked in step 3 to resolve to loaded Snippet files, or nothing (`[]`) for fresh prose — so Snippet usage tracking counts skill runs the same as app runs. If no Cover Letter was produced (declined at Text Review), remove both fields from the body rather than sending empty values; an absent `sourceSnippetIds` already means "no usage signal". Omit `usage`, deliberately: it records the backend's own Claude API calls, and a skill run makes none, so there's no honest figure to send. A connection error or non-2xx response means the Generation was **not** recorded — stop and tell the user; don't report the run as complete.

5. **Offer the Status move.** If `application.status` (from step 1) was `"saved"`, ask the user whether to move it to `"tailoring"`. If they say yes:
   ```
   ./plugins/sumisura/skills/tailor-cv/scripts/api.sh PATCH /api/applications/<id>/status '{"status": "tailoring"}'
   ```
   If `application.status` was already past `"saved"` (`tailoring`, `sent`, `interviewing`, `rejected`, `offer`, `withdrawn`), skip this ask entirely — don't touch Status.

## Pipeline

1. **Load Master Data.** Read `data/profile.yaml` (contact info + Static Sections: education, publications, certifications, awards, activities, languages — always included verbatim, never selected or rewritten) and every Entry file under `data/experience/*.md` and `data/projects/*.md` (YAML frontmatter + Markdown bullets). Unless this is Default Mode, also read every Cover Letter Snippet under `data/cover-letter-snippets/*.md`: YAML frontmatter with a required `kind` (e.g. `opening`, `why-this-company`, `closing`), an optional `lang` (ISO 639-1, the language the paragraph is written in) and optional `tags`, then a Markdown paragraph body; one Snippet per file, and its id is the file name without `.md` (`data/cover-letter-snippets/opening.md` → `opening`). Read them from disk, never from `GET /api/master-data/cover-letter-snippets` — the pasted/URL mode must keep working with no backend running. A missing `data/cover-letter-snippets/` directory, or one with no `.md` files, is an empty library, not an error: carry on, and the Cover Letter is drafted as fresh prose (step 3).

2. **Selection.** Choose which Entries, and which of their bullets, are relevant to the Job Description, and in what order. In Default Mode, favor the most recent and most representative Entries (e.g. the `flagship: true` Entry) instead of matching a Job Description. The result must be trimmable enough to satisfy the one-page constraint in step 4 — err on the side of cutting a marginal Entry or bullet rather than keeping everything.

3. **Rewrite.** Adjust bullet phrasing to better match the Job Description's language and emphasis. Do not introduce facts, tools, or claims that aren't present in the source Entry — Rewrite may reword, not invent.

   **Bullets are plain text, not Markdown.** `template/cv.typ` prints what it is given literally, so a bullet carrying `` `server.json` `` or `**bold**` renders with the backticks and asterisks visible. Master Data files *are* Markdown, so a source bullet may well contain them: drop a code span's backticks when you carry the bullet through (the app's Render does the same), and leave asterisks and underscores to the author — they may be real punctuation. The rendered-CV check in step 6 lists anything that survived (issue #170).

   **House style (applies to rewritten bullets and to the Cover Letter).** The output has to read like the user wrote it on a good day, not like a model produced it. A reader who suspects the letter was generated stops reading it, so this is a quality bar, not a preference.

   Never write: "I am writing to express my keen interest", "I am thrilled/excited to apply", "passionate about", "leverage", "cutting-edge", "seamlessly", "robust", "delve", "spearhead", "in today's fast-paced world", "your innovative approach to", "I believe my skills align". Avoid the rule-of-three list ("scalable, reliable, and maintainable"), sentences that all run to the same length, and paragraphs that open with a participle.

   Do write: short declarative sentences with a concrete fact, number or system name in each one. Plain verbs — built, cut, migrated, shipped, owned. At most one sentence about the employer, and it has to name something specific about them; skip it entirely rather than pad it. Prefer the phrasing already in Master Data over a smoother synonym: the user's own words are the point.

   **Target language.** Detect the language the Job Description is written in, as an ISO 639-1 code (`it`, `en`, `fr`, …), and write the rewritten bullets in it. The tool supports `en` and `it`; any other language falls back to `en` — the same rule the web app applies (`generation.NormalizeLanguage`), and the groundedness check in step 4 prints the resolved code, which is authoritative. An unsupported language never stops the run. In Default Mode there's no Job Description, so the target language is `en` without asking.

   **Draft the Cover Letter.** Skip this entirely in Default Mode — a Cover Letter is grounded in the Job Description, and Default Mode has none, so don't draft one or offer one (the app's Default Mode Generation has no Cover Letter either). Otherwise draft one now, following the same contract the web app gives its own drafting call (`draft_cover_letter` in `backend/internal/claude/client.go`), so the two paths produce the same kind of letter:
   - **Snippets first.** If step 1 loaded any Cover Letter Snippets, select and lightly adapt among them for this Job Description — reuse the user's own vetted phrasing rather than re-inventing it — and keep a list of every Snippet id you drew from (`sourceSnippetIds`).
   - **Fresh prose otherwise.** If the library is empty, or no Snippet fits this Job Description, write fresh prose instead, and `sourceSnippetIds` is empty. An empty library is the normal case, not a degraded one.
   - **Same Selection as the CV.** Ground the letter in the Entries this run's Selection kept (and their rewritten bullets) and the Job Description, so the CV and the Cover Letter tell one consistent story.
   - **No invented facts** — the same constraint as Rewrite: never state a fact, tool, employer, number or claim that isn't in those Entries or in a Snippet you cite. The letter may reword and connect what Master Data contains, never add to it.
   - **Same language as the CV:** write it in the target language resolved above (`it` CV, `it` letter).
   - **Pick a Snippet already in that language.** Prefer a Snippet whose `lang` is the target language, then one with no `lang` at all. Translate a Snippet written in another language only when nothing else fits, and say so at Text Review — it is the user's own vetted wording, and a translation of it reads like machine output, which is exactly what a Snippet library exists to prevent. A library holding both `lang: en` and `lang: it` versions of the same `kind` is the normal bilingual setup, not a duplicate.
   - **Citations must resolve.** Before presenting it, check every id in `sourceSnippetIds` against the Snippet files loaded in step 1. An id that doesn't match a loaded file is a bug in the draft, not something to show or record — drop the citation, or redraft, until every id resolves. (The app rejects such a draft the same way, in `validateCoverLetter`.)

   The draft is plain text: paragraphs separated by a blank line, no Markdown or Typst markup (the template prints the body literally, line breaks included, so `*`/`#` would show up as-is). The template adds only the contact header above it — any greeting or sign-off belongs in the prose. Keep the draft and its `sourceSnippetIds` for Text Review (step 4).

4. **Text Review (HITL, required).** Before presenting anything, persist the Selection + Rewrite result and run the groundedness check over it — the same check the web app runs over its own Rewrite output, via the same Go code (ADR-0028):

   1. **Write the Selection + Rewrite artifact** to `output/<slug>/selection.json` (choose `<slug>` now, following step 5's naming rule, and reuse it there). It's the `SelectionResult` shape the backend uses:
      ```json
      {
        "language": "it",
        "entries": [
          {
            "entryId": "experience/example-client-a",
            "reason": "Why Selection kept this Entry",
            "bullets": [
              { "sourceIndex": 0, "source": "<the Master Data bullet, copied verbatim>", "rewritten": "<the rewritten bullet>" }
            ]
          }
        ]
      }
      ```
      `entryId` is `experience/<file name>` or `projects/<file name>` without `.md`; `sourceIndex` is the bullet's 0-based position in that Entry file's bullet list; `source` must be the Master Data bullet character-for-character (the check refuses an artifact whose source text doesn't match, since scoring a rewrite against an edited source proves nothing); entries and bullets go in render order; `language` is the code detected in step 3. Write it in Default Mode too (with `rewritten` equal to `source` and `language` `"en"`) — it's this Generation's durable record of what was selected, so a groundedness verdict can be re-derived later instead of living only in this conversation.

   2. **Run the groundedness check** (skip it in Default Mode — Rewrite is skipped there, so there's nothing to check — and don't mention it):
      ```
      ./plugins/sumisura/skills/tailor-cv/scripts/quality-check.sh groundedness --selection output/<slug>/selection.json
      ```
      It's offline and needs no running backend (it builds `backend/cmd/cvcheck` from this checkout with Go). Exit `0`: every rewritten bullet traced back to its source — say so in one line. Exit `1`: some bullets were flagged — stdout lists each as `<entryId> bullet <sourceIndex> [<reason>: …]` plus the flagged sentence. Exit `2`: the check couldn't run; stderr says why. If the reason is the artifact itself (malformed JSON, an unknown field, a `source` that isn't verbatim), fix `selection.json` and re-run; if it's the tooling (no Go / no `cvcheck`, build failure), tell the user the groundedness check was unavailable and carry on. None of these exit codes stops the pipeline.

   Then present the Selection + Rewrite result to the user as text — what was kept, dropped, reordered, and reworded (a diff against the source bullets is more useful than just the final text). Fold the groundedness flags into that diff, next to the bullets they belong to, with the reason spelled out: `numeric-mismatch` means the rewrite states a number or date its source bullet doesn't (the fabricated-specific case — look hardest at these); `no-source-match` means the rewrite shares too little with its source to plausibly trace back to it (often a heavy paraphrase the user may be fine with — and expect more of these when rewriting into a language other than the Master Data's, exactly as in the app). Flags are signals for this checkpoint, not a second checkpoint: the user can approve a flagged bullet as-is. Also state the target language (the check's `Target language:` line) and that it can be corrected here; a correction replaces `language` in `selection.json` and means rewriting the bullets in the corrected language.

   In the same checkpoint (not a second one), present the Cover Letter draft from step 3, if there is one: the full prose as it will be rendered, followed by its provenance — which Snippet ids it drew from and each one's `kind` (e.g. "opening from `opening`, closing from `closing`"), or that it's fresh prose because the library is empty or nothing fit. The groundedness check above scores CV bullets only — it doesn't read the Cover Letter yet (the app scores its letter against the cited Snippets and the rewritten bullets) — so hold the letter to the no-invented-facts rule yourself before showing it, and don't describe it as checked. Show the CV as a diff and the Cover Letter as prose; they're reviewed together but approved independently:
   - The user can approve both, or correct either. A correction to the Cover Letter prose is carried into the render verbatim — don't re-polish approved wording.
   - The user can reject just the Cover Letter (ask for a redraft, or give direction). Redraft it under the same step-3 rules and present it again, keeping the approved CV content exactly as approved — rejecting the letter never re-runs Selection or Rewrite.
   - The user can decline a Cover Letter for this run. Then drop it: no Cover Letter artifacts are written, and the rest of the run proceeds exactly as a CV-only run.
   - If a CV correction drops an Entry, or changes the language, re-check the Cover Letter against the corrected Selection — a letter that now leans on a dropped Entry, or is in the old language, must be redrafted and shown again.

   If the user's corrections change any bullet or the language, update `selection.json` to match and re-run the check before rendering, so the artifact and its flags reflect what was actually approved. Wait for approval or corrections before rendering. Do not proceed to Render until the user explicitly approves the CV and has approved or declined the Cover Letter.

5. **Assemble the data file.** Merge the approved tailored content with the static parts of `data/profile.yaml` into a single JSON object matching the shape `template/cv.typ` expects (see the comment at the top of that file): `name`, `lang` (the target language approved at Text Review — the resolved `en`/`it` code, `en` in Default Mode; the template sets the document language from it), `location`, `email`, `phone`, `linkedin`, `github`, `education`, `experience` (grouped-by-employer array, each item has `employer`, `role`, `client`, `location`, `start`, `end`, `bullets`), `projects`, `tech_stack` (derived — see below), `publications`, `certifications`, `awards`, `activities`, `languages`. Write it to `output/<slug>/data.json`, where `<slug>` is this Generation's own, brand-new directory name — the same scheme the web app's Render uses, so a skill run and an app run can never overwrite each other's files (issue #105):
   **Deriving `tech_stack`.** Take the selected Entries' `tags`, deduplicated, **round-robin** — one tag from each Entry per pass, in Selection's order, each Entry's tags in the order they are written — and **stop at 30**. This is `generation.deriveTechStack` in prose; the app applies the same rule and cap (`TechStackMaxTags`), so a skill run and an app run produce the same line.

   Collecting every tag of every selected Entry does not survive a real career: four engagements and two projects produce around 67 tags over six lines, which on its own pushes the render to a second page, and a list containing everything says nothing about what the candidate is strong in (issue #169). Round-robin matters because it stops one heavily tagged Entry from spending the whole budget and hiding every later Entry's stack.

   - Start from a short kebab-case **label** (e.g. the company applied to, or `default`). The label alone is never the directory name.
   - Append the current UTC time as `-yyyymmdd-hhmmss`: `<slug>` = `<label>-<yyyymmdd-hhmmss>`, e.g. `acme-corp-20260911-143022` (`date -u +%Y%m%d-%H%M%S`).
   - Never write into a directory that already exists. If `output/<slug>/` exists, append `-2`, then `-3`, … until the name is free (e.g. `acme-corp-20260911-143022-2`), and create it with a plain `mkdir` (not `mkdir -p`, which silently succeeds on an existing directory).
   - Choose `<slug>` once per run: re-renders during this run's Visual Review (step 7) reuse it; the next run gets a new one.

   If a Cover Letter was approved at Text Review, also write `output/<slug>/cover-letter-data.json` in the same `<slug>` directory, with exactly the seven fields `template/cover-letter.typ` reads (see the comment at the top of that file) — the same object the app's Render writes:
   ```json
   {
     "name": "<profile.yaml name>",
     "location": "<profile.yaml location>",
     "email": "<profile.yaml email>",
     "phone": "<profile.yaml phone>",
     "linkedin": "<profile.yaml linkedin>",
     "github": "<profile.yaml github>",
     "body": "<the approved Cover Letter prose, verbatim; paragraphs separated by \n\n>"
   }
   ```
   The first six are copied verbatim from `data/profile.yaml`. Skip this file when there's no Cover Letter (Default Mode, or declined).

6. **Render.** This repo's render path is pinned to a specific `typst` version and the Liberation Sans font — recorded once in `typst-version.txt` at the repo root and shared with the container's own render path (ADR-0012), so the skill's host-side render and the container's stay in sync instead of silently drifting apart (issue #55). Before compiling, run the preflight check against that pin:
   ```
   ./plugins/sumisura/skills/tailor-cv/scripts/preflight-typst.sh
   ```
   It compares the host's `typst --version` and installed fonts (via `fc-list`, where available) against `typst-version.txt` and prints a warning naming expected vs. detected on any mismatch or missing font — non-blocking, so a warning doesn't stop the render, it's a hint to weigh before Visual Review. If `typst` isn't installed at all, the check is a no-op and the compile step below fails with the normal "typst not found" error. (The rendered PDF gets its own mechanical check at the start of Visual Review, step 7.) Then render:
   ```
   typst compile --root . template/cv.typ output/<slug>/cv.pdf --input data=output/<slug>/data.json
   ```
   If there's an approved Cover Letter, render it too — the preflight above covers the whole run, don't repeat it — with the invocation `template/cover-letter.typ`'s header comment documents:
   ```
   typst compile --root . template/cover-letter.typ output/<slug>/cover-letter.pdf --input data=output/<slug>/cover-letter-data.json
   ```
   and write the approved prose, verbatim and nothing else, to `output/<slug>/cover-letter.txt` (for pasting into an application portal or email body) — the same file the app's Render writes, so a skill-produced output directory looks like an app-produced one.

   Report a failure by document: if the CV compile fails, say the **CV** render failed (with typst's error) and fix `data.json`; if the Cover Letter compile fails, say the **Cover Letter** render failed and fix `cover-letter-data.json` — a missing or misspelled field shows up there as a typst error naming it. One document failing doesn't make the other's PDF wrong; don't go on to Visual Review until both approved documents have rendered.

7. **Visual Review (HITL, required).** First run the PDF check over the rendered file — the same page-count and ATS-parsability checks the web app's Render attaches to its own Visual Review, via the same Go code (ADR-0028), plus a check that `data.json` carries a supported `lang`:
   ```
   ./plugins/sumisura/skills/tailor-cv/scripts/quality-check.sh pdf --pdf output/<slug>/cv.pdf --data output/<slug>/data.json
   ```
   It prints three lines of signal. `Page count:` — anything but 1 is overflow to fix. `ATS-parsability:` — `ok`; `warning`, followed by which expected fields (name, section headers, employers, project names) were missing from the PDF's extracted text layer or came out in the wrong order, i.e. what an automated screener may not see; or `unavailable` (e.g. `pdftotext` isn't installed), which is a tooling gap and must be reported as "the check couldn't run", never as the PDF being unparsable. `Language:` — the resolved code, with a warning if `data.json`'s `lang` is missing or unsupported (fix `data.json` and re-render). Exit `0` = all clear, `1` = something flagged, `2` = couldn't run (a stderr note, or parsability unavailable with nothing else flagged) — none of them stops the pipeline.

   Then tell the user the PDF is ready at `output/<slug>/cv.pdf`, show those results as signals alongside it, and ask them to check it — overflow, awkward breaks, and anything the check flagged. The check doesn't add a checkpoint: the user's decision is still the gate, and they can approve despite a warning. If you re-render after a fix, re-run the check before asking again. If they report an issue, fix the data or trim Selection and re-render; don't guess silently.

   If a Cover Letter was rendered, name it in the same message — ready at `output/<slug>/cover-letter.pdf`, plain text at `output/<slug>/cover-letter.txt` — and ask the user to check its layout too. The one-page rule is a CV rule; the Cover Letter has no page-count rule, and the PDF check above is CV-specific (its expected fields are CV sections), so don't run it on the Cover Letter. A layout issue in the letter is fixed in `cover-letter-data.json` and re-rendered without touching the CV; a wording change the user asks for here goes back into the approved prose, `cover-letter-data.json` and `cover-letter.txt` together. Visual Review is approved when both documents are. If this run is targeting a tracked Application, once approved here go back to "Targeting a tracked Application" step 4 to record it.

## Notes

- `output/` is gitignored — it's derived, not Master Data. Never edit it as if it were a source of truth.
- `scripts/quality-check.sh` wraps the backend's `cvcheck` CLI (`backend/cmd/cvcheck`) so this skill runs the web app's automated checks through the app's own Go code rather than a prose re-description of them (ADR-0028). Every check is advisory: exit `0` = ran clean, `1` = ran and flagged something, `2` = couldn't run — and none of them is a reason to stop the pipeline.
- To add new Master Data (a new job, a new project), create a new file under `data/experience/` or `data/projects/` following the frontmatter shape of the existing files — this is normal manual editing, not something this skill automates.
- A Tailored-mode run produces a complete Generation — Tailored CV and Cover Letter — the same outputs as the web app's Generate button (CONTEXT.md's Generation and Cover Letter entries). ADR-0005 decides how the *app* runs Generation and keeps this skill as a parallel path; it doesn't narrow what the skill does, so the two stay aligned by mirroring the app's drafting contract (step 3).
- This skill never edits Master Data (including creating or editing Cover Letter Snippets) and never captures new Job Listings — those are data entry, done in the web app or the LinkedIn extension.
