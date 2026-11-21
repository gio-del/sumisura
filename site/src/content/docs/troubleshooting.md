---
title: Troubleshooting
description: The failures people actually hit, and what they mean.
---

The app's **Installation** page (`/setup`, also linked from a notice on the Today page when something required is missing) checks all of the items below at once and says what to do about each.

## `typst: command not found`

Rendering shells out to the Typst CLI. Install it and make sure it is on your
`PATH`. The repo pins a version in `typst-version.txt`; the skill warns before
rendering if your local version differs, because a different version can change
pagination — and the CV has a hard one-page limit.

## The PDF is two pages

Selection chose too much, or your bullets are long. Re-run and ask for fewer
Entries, or tighten the source bullets. The page-count check runs before Visual
Review precisely so you see this before you send it anywhere.

## "ATS-parsability: unavailable"

That check needs `pdftotext` (poppler-utils). Without it the check reports
itself unavailable rather than failing — install poppler if you want it.

## Every Claude call fails

Check `ANTHROPIC_API_KEY` in `.env`, then restart the backend — the key is read
at startup. A 401 from the API means the key; a 429 means rate limits; an error
naming a model usually means an override in `.env` pointing at a model id that
does not exist.

## Port already in use

Something else is on `8080` or `5173`. Stop it, or change the ports in
`docker-compose.yml`.

## The extension button does nothing

Sumisura has to be running, since the extension posts to
`http://localhost:8080`. If it is running, the posting may be behind a login
wall or in a layout the extractor does not recognise — paste the description
into the app instead.

## A save "lost" my job description

Saving does best-effort research (salary range, application method) before
writing. Stopping the backend mid-save aborts that, though the shutdown drains
in-flight requests for 15 seconds first. If you stopped it during a save, paste
it again.

## Generation takes minutes

That is normal: Selection, Rewrite, cover letter and research are several model
calls. There is deliberately no total-response timeout, because a cap tight
enough to be useful would break the pipeline. Claude-calling routes do carry a
10-minute deadline.

## A warning on the Master Data page names a file

An Entry file under `data/experience/` or `data/projects/` could not be read, usually broken frontmatter (an unclosed quote, a missing `---`). The app leaves it out of the list, Generation and checks, shows the file and the reason, and never changes it. Open the named file, fix the frontmatter, and reload.

## Something else

Search the [issues](https://github.com/gio-del/sumisura/issues), then open one
with your version, how you run it, and the steps. Redact your CV content —
please do not paste your career history into a public issue.
