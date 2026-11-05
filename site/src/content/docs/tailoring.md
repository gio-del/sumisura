---
title: Tailoring a CV
description: Running the tailor-cv skill, the two review checkpoints, and the quality checks that run before each one.
---

The tailoring pipeline is a Claude Code skill, not a build script — the
interesting parts are conversations, and two of them are approvals.

```
/sumisura:tailor-cv
```

## Three ways to start

| Input | What happens |
|---|---|
| A job description (pasted text or a URL) | Selection and Rewrite target that job; a cover letter is drafted too |
| A tracked Application ("the Acme backend role") | Same, using the Job Description already saved, and the Generation is recorded back against that Application |
| Nothing (**Default Mode**) | A general-purpose CV from your strongest material — no cover letter |

## The pipeline

1. **Selection** — which Entries belong on this one page.
2. **Rewrite** — bullets re-worded toward the job's language, your About me
   (`summary`) adapted the same way, and a row of at most ten of each Entry's
   own tags picked for the job, and the headline and each employer's role chosen
   from the options your profile lists. Plus a **cover
   letter** draft when there is a job, drawn from your Cover Letter Snippets if
   you have any.
3. **Prose clean-up** — the rewritten bullets and the cover letter go through the
   `clean-user-facing-text` skill (an upstream skill pinned by commit and hash,
   downloaded on first use): a style score, a deterministic pass
   that removes invisible Unicode and typographic artefacts, and a rewrite only
   when the score is high, under the same no-new-facts rule. You see the scores
   and the cleaned text at Text Review. Needs Python 3.10+ (standard library
   only); without it the skill stops and asks you to install it.

   The web app runs the same scripts on a Generation, but only the
   deterministic part: the Unicode pass and the before and after scores, shown
   in a "Text clean-up" block at Text Review. It never rewrites, and it never
   blocks: when Python or the scripts are missing (a checkout rather than the
   release image, which ships both) the block says the clean-up is not
   available and the Generation goes on with the text as written. See
   [Configuration](./configuration.md) for the two variables.
4. **Groundedness check** — every rewritten bullet compared against its source,
   and the cover letter checked for names, tools, figures and years that appear
   in none of the Entries the Selection kept, the Snippets it drew from or your
   profile. A flag names the term (`unsupported-term`). It is advisory and
   never blocks approval. The company you apply to comes from the job posting,
   not from your Master Data, so its name is flagged too; ignore that one.
5. **Text Review** — *you approve*. CV and cover letter are approved
   independently. Nothing renders before this. Under the cover letter each
   paragraph has a **Save as snippet** action: it opens the Cover Letter Snippet
   form filled with that paragraph, with `kind` and tags for you to set. Nothing
   is written until you press Create, and Cancel writes nothing. A new snippet
   never overwrites an existing one (a repeated `kind` gets a numbered id).
6. **Render** — Typst produces the PDFs.
7. **PDF checks** — page count, the **ATS Report** (see below), and the
   document language.
8. **Visual Review** — *you approve* the rendered result.

The prose clean-up is the one step that changes text, and it does so before
Text Review, where you see the result. Checks are advisory by design: exit `0` clean, `1` flagged, `2` could not run.
They inform a checkpoint where you are already looking; they never block or
silently edit.

## The ATS Report

Applicant tracking systems read a PDF's text layer, not its layout. Every
Generation keeps an **ATS Report** of what that layer says, for the CV and the
cover letter:

- **The extracted text**, top to bottom, as a screener reads it.
- **Field by field:** your name, contact details (email, phone, LinkedIn,
  GitHub), each section header, each employer and project. Each one is
  *found*, *missing* or *out of order*. Anything missing or out of order turns
  the report into a warning.
- **Job Description terms:** which of the job's terms that are also tags in
  your Master Data made it onto this CV. Only your own tags count, so the
  report never nudges you to claim a skill you don't have. This part is for
  information and never raises a warning. It is left out in Default Mode.

The report is saved with the Generation, so you can reopen it right before you
send the CV, even after `output/` is cleaned up. Open it from **Open the ATS
Report** at Visual Review, from an Application's Generation history, or from
**Generated CVs**. A Generation made before reports were kept says so instead
of showing a pass.

## Where the files go

Every run gets its own directory, so a later run never overwrites an earlier
one:

```
output/<label>-<UTC yyyymmdd-hhmmss>/
├── cv.pdf
├── cover-letter.pdf
├── data.json          # what the template rendered
├── ats-report.json    # the ATS Report
└── selection.json     # Entries chosen + every source→rewrite pair
```

`selection.json` is what lets a groundedness verdict be re-derived later.
`output/` is gitignored and safe to delete; Application records survive and show
a "no longer on disk" note.

Every regeneration adds a directory, so `output/` only grows. The **Space used
by output/** link on the Generated CVs page shows its total size and each
Generation's size, date and Application, and proposes the superseded ones: not
the latest Generation of their Application, or belonging to an Application
Rejected or Withdrawn more than a configurable number of days ago (90 by
default). Nothing is deleted until you select rows and confirm; a recorded
Generation keeps its record.

## Running a check by hand

```sh
./plugins/sumisura/skills/tailor-cv/scripts/quality-check.sh \
  groundedness --selection output/<slug>/selection.json

./plugins/sumisura/skills/tailor-cv/scripts/quality-check.sh \
  pdf --pdf output/<slug>/cv.pdf --data output/<slug>/data.json
```

`groundedness` also checks a cover letter draft when you pass
`--cover-letter <file>` (plain text) and `--snippet-ids id,id` (the Snippets it
drew from; leave it out for fresh prose).

Both wrap `backend/cmd/cvcheck`, the same Go code the web app calls. Add
`--json` for machine-readable output. `groundedness` reads Master Data from
`data/` unless you pass `--data-dir <dir>`. No running backend is needed.

`pdf` takes a few more options for the full ATS Report:

- `--job-description <file>` adds Job Description term coverage. The tags come
  from `data/`, or from `--data-dir <dir>`.
- `--cover-letter output/<slug>/cover-letter.pdf --cover-letter-data
  output/<slug>/cover-letter-data.json` checks the cover letter too.
- `--report-out output/<slug>/ats-report.json` writes the report file the
  skill later attaches to the Generation record.

## Rendering manually

```sh
typst compile --root . template/cv.typ output/<slug>/cv.pdf \
  --input data=output/<slug>/data.json
```

The templates are pure presentation: they read one JSON file and lay it out.
Changing your CV's look means editing `template/cv.typ`, not the pipeline.
