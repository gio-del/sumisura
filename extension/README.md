# Sumisura — Job Capture

A browser extension that captures the LinkedIn or Indeed job posting you're currently viewing into your local Sumisura app as a Job Listing. See `docs/adr/0007-job-sourcing.md` for why this is scoped to reading a page you're already looking at, never scraping.

## How it works

- A card appears (bottom-right) on any `linkedin.com/jobs/*` or `*.indeed.com/viewjob*` page, in a Shadow DOM so the board's own CSS can't reach it. Collapse it to a pill with the "–" button; that choice is remembered.
- **It tells you what it knows before you click.** On each posting you open it asks your backend once (`POST <address>/api/job-listings/capture-lookup`, read-only) whether that posting is already a Job Listing, and which other roles you already track at that company. Untracked postings offer Save; a tracked one shows its Application's Status and links into the app instead.
- Saving reads the Job Title, company, location, Company Logo, and Job Description (as Markdown) already rendered on the page — no request to LinkedIn/Indeed is made by the extension — and sends it to `POST <address>/api/job-listings/from-extension` (address `http://127.0.0.1:8080` unless changed in the options page, with `X-Sumisura-Token` when an access token is set there — `settings.js`, `options.html`, issue #195). The backend saves it as a Job Listing the same way a manually-pasted one is saved (Company Logo downloaded, RAL Range looked up, Application Method inferred, Application created at Saved).
- **Job Title and Company are editable in the card** before you save, so a bad extraction is something you fix rather than something that defeats you. A capture that fails validation pre-fills them with whatever it did find.
- **A posting you already track can't be saved twice.** The backend refuses it and names the record that holds it; if that record turned out to be archived, the card offers to bring it back.
- **You can move an Application's Status from the card** — only the moves your pipeline actually allows are offered, read from the backend's own state machine, and the backend re-checks every one, so a card left open on a stale posting can't put anything into a state the app forbids.
- **A company you already track asks first.** The card lists those roles with their Statuses, and offers either "Save anyway" or "Save and archive this one" against any of them. Replacing archives, never deletes, so the old Application's history survives.
- **On a LinkedIn search-results page, rows you already track are badged with their Status** — the whole visible page in one request, rows appearing as you scroll picked up too. If Sumisura is unreachable the page is simply left unannotated: badging never blocks, removes or rewrites anything the board rendered.
- **The toolbar icon marks a posting you already track**, so the signal survives collapsing the card. Its tooltip carries the Status.
- **`Alt+Shift+S` saves the posting you're on** (rebindable at `chrome://extensions/shortcuts`). It runs the card's own save, so it goes through the same capture, validation, duplicate and same-company checks — it's a faster path, not a way around them.
- Nothing happens on a timer, and nothing is sent about a page you aren't looking at: the extension talks to your backend only about a posting you have open, at most once per posting. See `docs/adr/0043-extension-looks-up-tracked-state-before-a-click.md` for why that posture was revised and what it still rules out.
- `content.js` (LinkedIn) and `content-indeed.js` (Indeed) are board-specific: each has its own selectors and its own known fragility. They share only what's genuinely board-agnostic — the card (`card-model.js` decides what it shows, `card-view.js` draws it), the message-passing to `background.js`, and Turndown-based HTML-to-Markdown conversion — via `capture-common.js`.

Requires the Sumisura backend running locally (`docker-compose up` from the repo root; see the root `README.md`).

## Loading it (unpacked, for local personal use — not published to any store)

**Chrome / Chromium-based:**

1. Open `chrome://extensions`.
2. Enable **Developer mode** (top-right toggle).
3. Click **Load unpacked** and select this `extension/` directory.
4. Visit any LinkedIn or Indeed job posting page — the Sumisura card should appear.

**Firefox:**

1. Open `about:debugging#/runtime/this-firefox`.
2. Click **Load Temporary Add-on…** and select `extension/manifest.json` (the manifest file itself, not the folder).
3. Visit any LinkedIn or Indeed job posting page — the Sumisura card should appear.

Note: Firefox unloads temporary add-ons when the browser restarts — you'll need to reload it each session. `manifest.json` declares both `background.service_worker` (Chrome) and `background.scripts` (Firefox) so the same extension works unmodified in both.

## Notes

- LinkedIn ships an atomic/hashed CSS build with no stable semantic class names, no `<h1>`, and no JSON-LD structured data on the job-view page. `content.js` instead reads: the job title from `document.title` (`"<Job Title> | <Company> | LinkedIn"`), the company from the first `a[href*="/company/"]` link, the Company Logo from that same link's `<img>` (its `src`, or `data-delayed-url` if LinkedIn hasn't lazy-loaded it yet), and the description as the longest `[data-testid="expandable-text-box"]` block on the page — its own "…altro"/"…more" toggle `<button>` stripped out first, so its label doesn't leak into the captured text — converted to Markdown by the vendored Turndown library (`turndown.js`) so paragraphs, line breaks, lists, bold/italic, and links survive. If capture starts failing, re-run the diagnostic snippet below in the page console and adjust `content.js` accordingly.
- LinkedIn's salary-insight pill (e.g. "63,2K € /yr - 70,8K € /yr") has no stable selector either — `content.js` scans short page elements outside the description for currency-plus-number-shaped text instead and sends the first match as `listingSalaryText`, a separate field from `description`. RAL Range resolution uses it alongside the Job Description text (ADR-0014); a missing or unparsed match changes nothing.
- The Company Logo is downloaded by the backend at save time and stored alongside the Job Listing record (ADR-0013) — a failed download never blocks the save, the Job Listing just ends up with no logo.
- The captured URL: on a direct `/jobs/view/<id>/` page, it's the stripped `window.location.href`. On the search-results split-pane view, clicking between postings only changes the `currentJobId` query param — `window.location` itself stays on the generic search page — so `content.js` reads `currentJobId` and builds `https://www.linkedin.com/jobs/view/<id>/` instead.
- The backend address and access token are set on the options page and kept in `chrome.storage.local`. A non-default address is granted through `optional_host_permissions` when saved; the two local defaults are regular `host_permissions`, alongside the two board pages the content scripts already run on — the shortcut relays through `chrome.tabs.sendMessage`, which needs host permission for the tab it messages.
- `turndown.js` is [Turndown](https://github.com/mixmark-io/turndown) vendored as a plain browser-global script (no npm/build step) and loaded as a `content_scripts` entry ahead of `content.js`/`content-indeed.js`, which use the `TurndownService` global it defines.
- `validate-capture.js` is a small, DOM-free content script (loaded ahead of the board script in *every* bundle — LinkedIn and Indeed alike, pinned by `manifest.test.js` so a board added later can't be left out) defining the `validateCapture` global: per-field sanity checks (non-empty title, plausible company, minimum-length description) run on the captured payload before it's ever sent to `background.js`, so a plausible-but-wrong capture (stale nav element, truncated description) surfaces as a specific error instead of silently saving a broken Job Listing. Being pure and DOM-free, it's unit-tested directly alongside the board extraction tests below.
- `content-indeed.js` reads the job title from `[data-testid="jobsearch-JobInfoHeader-title"]` (falling back to `document.title`), the company from `[data-testid="inlineHeader-companyName"]`, the description from `#jobDescriptionText`, the Company Logo from the header's `<img>`, and a salary line from `#salaryInfoAndJobType` when present — otherwise the same short-element currency-pattern scan LinkedIn's `listingSalaryText` uses. The canonical URL is rebuilt from the `jk` query param (`https://<host>/viewjob?jk=<id>`), dropping tracking params, the same way `content.js` rebuilds LinkedIn's `currentJobId`. **These selectors were written from Indeed's commonly-documented markup, not verified against a live page** (no browser access in the environment that wrote this) — re-confirm against a real Indeed job-view page before relying on this, and update `extension/fixtures/indeed-job-view.html` + `content-indeed.test.js` together if they've drifted.

## Tests

Each board's field-extraction logic (title/company/description/etc.) is written as a pure `(document) => payload` function, and `validate-capture.js`'s `validateCapture` is pure and DOM-free — all covered by fixture-based tests that run against a static HTML fixture in Node, without a live page or the `chrome.*` extension APIs:

```
cd extension
npm install
npm test
```

This suite is load-bearing, not optional local tooling: `.github/workflows/ci.yml` runs it as its own `extension` job (peer to `backend` and `frontend`) on every push to `main` and every pull request, installing from the committed `package-lock.json` so CI parses fixtures with the same jsdom version you do. A failing extension test fails the build.

What is covered: LinkedIn extraction (`content.test.js` against `fixtures/linkedin-job-view.html` — Job Title, company, Company Logo including the lazy-load fallback, Job Description Markdown with the toggle stripped and the longest block chosen, canonical URL in both the split-pane and direct-page cases, and the salary badge scan found/description-excluded/absent), Indeed extraction (`content-indeed.test.js` against `fixtures/indeed-job-view.html`), the capture validator (`validate-capture.test.js`, pure payloads), the card's decisions (`card-model.test.js` — every state the card can be in and what it offers in each, with no DOM and no `chrome.*`), the card's traffic (`card-view.test.js` — one lookup per posting, none on a re-read, a new one when the split pane changes posting, none when you come back to one already seen, and a late answer for a posting you've left never drawn), the toolbar badge (`toolbar.test.js`), search-results badging (`badges.test.js` — one request for the page, a tracked row badged with its Status and an untracked one left alone, rows appearing on scroll badged without re-asking about the settled ones, no row badged twice, and every failure shape leaving the page's HTML byte-identical), and the manifest wiring (`manifest.test.js` — every bundle shipping a board capture script also ships `validate-capture.js` ahead of it, both card scripts in order, the shortcut declared under the id `background.js` listens for, and host permission for every page a content script runs on).

What is not: how the card actually looks. `draw` builds the Shadow DOM from what `cardModel` returned, so the decisions are tested a layer down and the drawing is exercised manually via "Loading it" above. Click handling and message-passing to `background.js` need the `chrome.*` runtime faked; the card tests inject their own `send` instead of faking it. And both fixtures are synthetic — a green suite says the extraction logic is correct against the DOM shape the scripts target, never that LinkedIn or Indeed still serve that shape (see `fixtures/README.md`). `extension/package.json`/`node_modules` exist solely for this test suite; the extension itself still ships as plain, unbundled scripts per `manifest.json`, no build step involved.

### If capture breaks again

Paste into the DevTools console on a LinkedIn job posting page to see what's actually there:

```js
(function(){const og=[...document.querySelectorAll('meta[property^="og:"], meta[name="description"]')].map(el=>({key:el.getAttribute('property')||el.getAttribute('name'),content:el.content}));const dataAttrs=[...document.querySelectorAll('[data-test-id], [data-testid], [data-view-name]')].map(el=>({tag:el.tagName,testId:el.getAttribute('data-test-id')||el.getAttribute('data-testid'),textLen:el.textContent.trim().length})).filter(el=>el.textLen>0);console.log('title:',document.title);console.log('og/meta:',og);console.log('data-attrs:',dataAttrs);})();
```
