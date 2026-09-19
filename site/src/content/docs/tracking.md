---
title: Tracking applications
description: Job Listings, Applications, statuses, notes and generations — the part of Sumisura that is not about writing.
---

Tailoring a CV for a job you never record is how job searches get lost. Every
job you save becomes a **Job Listing** with an **Application** attached.

## Saving a job

- **Paste** a URL or the description text into the web app.
- **Pull from an ATS board** — Greenhouse, Lever and Ashby expose public job
  boards; track a board and browse its listings in the app.
- **Capture what you are reading** with the [browser extension](./extension.md)
  on LinkedIn or Indeed.
- **Save a link from your phone** for later. It waits in **To complete** until
  you add its description (see below).

On save, Sumisura does some best-effort work: it resolves a salary range where
the posting or public sources allow, infers how one applies (ATS form, email,
referral), and downloads the company logo. Any of these can come back
`unresolved` without blocking the save, and you can retry later.

## To complete: links saved from another device

A job spotted in the LinkedIn or Indeed app on your phone doesn't come with its
description, only a link. Sumisura keeps such a link as a **Pending Capture** in
the **To complete** inbox rather than as a half-empty Job Listing:

- a public **Greenhouse, Lever or Ashby** link skips the inbox entirely:
  Sumisura reads the posting from the board's public API and saves the Job
  Listing straight away, the same way *Browse ATS Boards* does. If that lookup
  fails (board gone, posting closed), the link waits in To complete as usual;
- it has no Application or status and doesn't count in your statistics;
- sharing the same posting twice keeps one entry, and sharing a posting you
  already track tells you so instead of saving it again;
- capturing the posting with the [browser extension](./extension.md), or saving
  a Job Listing for the same link by hand, completes it automatically;
- **Complete** asks for the pasted description, the company and the job title,
  then saves an ordinary Job Listing. Company and title are prefilled when the
  share mentioned them, and otherwise suggested from the description you paste
  (only empty fields are filled). The same form appears on the share screen
  right after sharing, so a job can be finished on the phone
  ([details](./phone-capture.md#finishing-a-job-on-the-phone));
- **Dismiss** drops a link you've lost interest in.

Share entry points send the link to `POST /api/pending-captures` as
`{ "url": …, "text": …, "title": … }`. Only one of `url` or `text` needs to
contain the link. The response says what happened (`outcome`: `pending`,
`already-pending`, `already-tracked`, or `job-listing` for an ATS link saved
straight away) along with a ready-made `message`.

## The Application

Each Application carries:

- a **status** through a defined pipeline: saved → tailoring → sent →
  interviewing → offer, plus rejected and withdrawn
- **timestamped notes** you can edit and delete
- a **contact**, which can be suggested for you and corrected by hand
- every **Generation** produced for the job, with the model used and its cost
- a **freshness** check: is the posting still live?

The Applications view groups everything by status, and floats stale
applications — nothing has moved for two weeks — to the top of their group.

## Generated CVs

**Generated CVs** lists every CV this installation has produced, newest first.
It reads two places at once:

- Generations **recorded** against an Application, which carry the company,
  the job title, the language and the groundedness verdict
- directories in `output/` that **no record mentions** — a Default Mode run,
  or one driven by the `tailor-cv` skill. These are marked *Not tracked*, and
  their date is read from the directory name's timestamp.

A Generation whose files have since been deleted stays on the list without
working links: `output/` is derived, safe to clear out, and the record is the
thing worth keeping.

**Delete files** clears one Generation's `output/<slug>/` directory — the PDFs
and the assembled JSON beside them. It never touches the record: a Generation
tracked against an Application keeps its entry, with the cost, language and
groundedness verdict you may need months later, and its row simply reports the
files as missing. An untracked Generation has no record to keep, so its row
goes away with the files. The page asks once before deleting, and nothing is
recoverable afterwards — the files are removed, not moved to a trash folder.

## Archiving

Archive a Job Listing to get it out of the default view without deleting
anything. Archived listings still count in your statistics and can be brought
back.

## Your records are yours

`data/jobs/`, `data/applications/` and `data/pending-captures/` are gitignored
flat files: they are about your job search, not your career history. Use
**Export** in the app to download a zip of all three — that is the backup path, since git is deliberately not carrying
them.
