# Sumisura

A self-hosted tool that turns one person's complete career history into a tailored, one-page CV (PDF) and cover letter, generated on demand for a specific job application. One person's history per installation — the tool is single-tenant by design; "the user" below always means the owner of that installation.

## Language

**Master Data**:
The complete, untailored superset of everything the user has ever done, plus the Static Sections. The single source of truth from which every generated CV is derived. Never itself shown to an employer as-is.
_Avoid_: config, profile, source data

**Entry**:
One unit of work experience or project, stored as its own file (YAML frontmatter for dates/Tags, Markdown body for bullets). The unit of Selection during Tailoring.
_Avoid_: item, record, job (ambiguous with Job Description)

**Client Engagement**:
An Entry representing the work done for one client while at a consultancy employer. Several Client Engagement Entries can share the same employer/title/date-range metadata but are independently selectable — Selection can surface one client's work prominently while omitting another's.
_Avoid_: sub-project, client project

**Tag**:
Metadata on an Entry (tools/technologies used) recorded in its frontmatter. Drives both Selection's relevance-matching and the derived Tech Stack line. Never independently maintained as a separate list.
_Avoid_: skill, keyword, label

**Tech Stack**:
The CV section listing technologies, derived entirely from the Tags of whichever Entries were Selected for a given Generation. Has no data of its own.
_Avoid_: skills list, skills section

**Static Section**:
Content (Awards, Activities, spoken Languages, Publications) that is always included in full on every generated CV. Never subject to Selection or Rewrite.
_Avoid_: extras, misc

**Job Description**:
The pasted text or fetched-URL content describing a role, held by a Job Listing, supplied as input to a Generation to guide Tailoring. Its absence triggers Default Mode.
_Avoid_: posting, job ad

**Job Listing**:
A persisted, tracked record of a role the user is considering: source (pasted, browser-extension capture, or ATS feed), company, its Job Title, an optional Company Logo, URL, saved date, its Job Description, and RAL Range. Distinct from Job Description itself, which is just the text/content field it holds.
_Avoid_: posting, job ad, listing

**Pending Capture**:
A link to a job posting saved from another device (typically a phone's share sheet) to be turned into a Job Listing later. It holds only the link, the posting it identifies (its Posting Key), the saved date and unconfirmed title/company hints taken from what was shared. It has no Job Description, no Application and no Status, and never counts toward the funnel/stats, duplicate detection or Generation. It leaves the To complete inbox either by being **completed** — the user supplies the Job Description and confirms Company, which saves an ordinary Job Listing for its link; Company and Job Title may be suggested from the pasted Job Description, never from fetching the posting — or by being dismissed. Sharing a posting that is already pending, or already a Job Listing, writes nothing, and a public ATS (Greenhouse, Lever, Ashby) posting never becomes one: it is resolved through its board's API and saved as a Job Listing at once, unless that lookup fails (ADR-0041).
_Avoid_: draft Job Listing, incomplete listing, bookmark

**Posting Key**:
The stable identity of one job posting across the URLs it can be reached by: LinkedIn's job id (from `/jobs/view/…` or a search pane's `currentJobId`), Indeed's `jk`, an ATS board plus job id, or otherwise the link reduced to host and path. Two links with the same Posting Key are the same posting.
_Avoid_: canonical URL, job id

**Archived** (of a Job Listing):
A property of the Job Listing, not a Status of its Application: set and cleared only by the user, never inferred from Status, freshness or age. An Archived Job Listing leaves the default Job Listings list and is shown only when the user asks for archived listings; nothing else changes. The Job Listing, its Application, Status history and Generation history all stay on disk exactly as they were, still count toward the funnel/stats view, and still take part in duplicate detection. Unarchiving brings it straight back. Distinct from deleting, which destroys the record and its history.
_Avoid_: deleted, hidden, closed

**Job Title**:
The name of the role a Job Listing is for (e.g. "Senior Backend Engineer"), captured from the source (browser-extension DOM capture, an ATS board's own listing, or entered manually) and shown alongside Company wherever a Job Listing is displayed. Distinct from Entry's `role` field, which is the user's own job title at a past employer/client — never the other way around.
_Avoid_: role (ambiguous with Entry's role), position, job

**Company Logo**:
An optional image for a Job Listing, downloaded server-side from the source's logo URL at save time and stored alongside that Job Listing's record. Only ever populated from a browser-extension capture (LinkedIn or Indeed) today — ATS feeds and manual entry have no logo source, so a Job Listing from either simply has none.
_Avoid_: image, photo, icon

**RAL Range**:
The gross annual salary (Reddito Annuo Lordo) range for a Job Listing, with a source: Stated (found directly from the source — either in the Job Description text, or a separate structured salary field the source exposes on the listing itself, e.g. LinkedIn's own salary-insight badge, distinct from and not part of the Job Description), Estimated (Claude web-researched it for that role/company/location — a guess, not a fact), Conflict (the Job Description text and the listing's own salary field both state a figure and the two ranges don't overlap at all — both shown, neither picked automatically), or N/A (couldn't find anything). Always shown in the FE, source labeled.
_Avoid_: salary, salary range, pay

**Application**:
The tracked record of one attempt to apply to a Job Listing (exactly one Application per Job Listing), created the moment it's saved: its Status, Application Method, Contact (if applicable), its Notes, and a history of every Generation run for it — the most recent Tailored CV and Cover Letter being what you'd actually send.
_Avoid_: submission

**Note**:
A timestamped, Markdown observation the user records against an Application as its process unfolds — what a recruiter said, a deadline agreed, a salary figure discussed. Ordered newest-first and never derived from anything else: the tool writes Status history on its own, but a Note is only ever authored by the user. Its body is correctable (an edited Note is marked as edited) and it can be deleted, but its timestamp is fixed at the moment it was written. Adding one never changes the Application's Status or staleness, and a Note never feeds Generation. Deliberately not a reminder: nothing about a Note ever fires.
_Avoid_: comment, log entry, memo, activity

**Status** (of an Application):
Where an Application stands: Saved → Tailoring → Sent → Interviewing → Rejected/Offer. Rejected is not fully terminal: it can be moved back to Interviewing via Reopen, for when a rejection turns out to be premature (e.g. a recruiter reaches back out). Both moving to Rejected and Reopening from it require explicit user confirmation, since each reverses the other. Offer remains fully terminal. Withdrawn is a separate Status, additively reachable from Saved, Tailoring, Sent, or Interviewing, recording the user ending the process on their own initiative — distinct from Rejected, which means the employer ended it. Withdrawn follows the same terminal-but-reopenable shape as Rejected: its only outbound move is back to Interviewing via Reopen, with the same explicit-confirmation requirement in both directions.
_Avoid_: state, stage

**Application Method**:
How a Job Listing says to apply, inferred by Claude from its Job Description (portal link, email, LinkedIn Easy Apply, other) and correctable by the user. Determines which guidance the FE surfaces — e.g. a Contact/email draft only for the email method.
_Avoid_: apply type

**Contact**:
The recruiter/hiring-manager name and email for an email-method Application. Entered manually by the user or suggested by Claude via web search and confirmed by the user — never trusted unconfirmed.
_Avoid_: recruiter

**Schema Version** (of a Job Listing, Application or Generation record):
Which generation of the persisted record format a record was written in, stamped on the record itself. What it buys is the meaning of an absent field: on a current record, absent means genuinely none (a Generation with no Snippet ids used no Cover Letter Snippet); on a **legacy** record — one written before Schema Versions existed — absent means unknowable. Brought current only by the one-shot record migration, which backfills what is provably on disk and leaves the rest legacy-empty, never guessed. Not the web app's read-time version token (README's "Lost-update protection"), which identifies one record file's exact bytes, changes on every write and is never stored.
_Avoid_: format version, revision, vintage

**Cover Letter**:
A Generation output alongside the Tailored CV. Selected/adapted from a user-authored library of Cover Letter Snippets in Master Data when one exists; otherwise freshly generated prose grounded in Master Data and the Job Description, under the same no-invented-facts constraint as Rewrite. Reviewed at Text Review like the CV.
_Avoid_: motivation letter

**Cover Letter Snippet**:
Optional Master Data: one reusable cover-letter paragraph (e.g. an opening, a why-this-company, a closing), stored one-per-file like an Entry (YAML frontmatter with a kind and Tags, Markdown body). Selected/lightly rewritten during Cover Letter generation the same way Entries are selected for a CV; if none exist, Cover Letter generation falls back to fresh prose.
_Avoid_: template, boilerplate

**Generation**:
The end-to-end pipeline that turns Master Data plus an optional Job Description into a Tailored CV and Cover Letter: Selection, Rewrite, Text Review, Render, Visual Review.
_Avoid_: pipeline, build, run

**Tailoring**:
The part of a Generation driven by a Job Description: Selection followed by Rewrite, constrained so the result fits one page.
_Avoid_: customization

**Selection**:
The Tailoring step that chooses which Entries, and which of their bullets, to include and in what order, based on relevance to the Job Description.
_Avoid_: filtering, curation

**Rewrite**:
The Tailoring step that adjusts an Entry's bullet phrasing to better match a Job Description's language. Must not introduce facts absent from Master Data.
_Avoid_: rephrasing, editing

**Default Mode**:
A Generation run without a Job Description. Selection falls back to the most recent/representative Entries; Rewrite is skipped since there is no Job Description to tailor phrasing toward, and no Cover Letter is produced, since there is nothing to ground one in.
_Avoid_: generic CV, general CV (that's the output; this is the mode that produces it)

**Text Review**:
The first Human-in-the-Loop checkpoint: the user approves or corrects the Tailoring output (Selection + Rewrite) as text, before Render — and the Cover Letter prose, when there is one, which can be corrected or rejected on its own without discarding the approved CV content. Alongside it, an automated groundedness check scores every rewritten bullet against its source bullet and flags likely-invented content — the same check whether the Generation came from the web app or the `tailor-cv` skill (ADR-0028). A signal shown here, not a second checkpoint; the human decision stays Text Review's alone.
_Avoid_: draft review

**Render**:
The Typst compilation step that turns approved Tailoring output into a PDF.
_Avoid_: compile, build

**Visual Review**:
The second Human-in-the-Loop checkpoint: the user checks the rendered PDF for layout issues (overflow, bad page breaks) after Text Review is approved. Alongside it, an automated ATS-parsability check (extracted PDF text vs. the rendered source data) and page count are shown as non-blocking warnings — attached by Render in the web app, run by the `tailor-cv` skill right after its compile, the same check either way (ADR-0028). A second signal shown here, not a third checkpoint; the human decision stays Visual Review's alone.
_Avoid_: final check, PDF review

**Tailored CV**:
The final one-page PDF produced by a Generation, for a specific Job Description or from Default Mode. A derived artifact, not versioned in the repo.
_Avoid_: resume, output, generated CV
