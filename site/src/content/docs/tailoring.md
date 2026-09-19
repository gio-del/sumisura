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
2. **Rewrite** — bullets re-worded toward the job's language. Plus a **cover
   letter** draft when there is a job, drawn from your Cover Letter Snippets if
   you have any.
3. **Groundedness check** — every rewritten bullet compared against its source.
4. **Text Review** — *you approve*. CV and cover letter are approved
   independently. Nothing renders before this.
5. **Render** — Typst produces the PDFs.
6. **PDF checks** — page count, the **ATS Report** (see below), and the
   document language.
7. **Visual Review** — *you approve* the rendered result.

Checks are advisory by design: exit `0` clean, `1` flagged, `2` could not run.
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

## Running a check by hand

```sh
./plugins/sumisura/skills/tailor-cv/scripts/quality-check.sh \
  groundedness --selection output/<slug>/selection.json

./plugins/sumisura/skills/tailor-cv/scripts/quality-check.sh \
  pdf --pdf output/<slug>/cv.pdf --data output/<slug>/data.json
```

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
