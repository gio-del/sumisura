# Sumisura — Job Capture

A browser extension that captures the LinkedIn or Indeed job posting you're currently viewing into your local Sumisura app as a Job Listing. See `docs/adr/0007-job-sourcing.md` for why this is scoped to reading a page you're already looking at, never scraping.

## How it works

- A "Save to Sumisura" button appears (bottom-right) on any `linkedin.com/jobs/*` or `*.indeed.com/viewjob*` page.
- Clicking it reads the Job Title, company, location, Company Logo, and Job Description (as Markdown) already rendered on the page — no request to LinkedIn/Indeed is made by the extension.
- That content is sent to your backend (`POST <address>/api/job-listings/from-extension`, address `http://127.0.0.1:8080` unless changed in the options page, with `X-Sumisura-Token` when an access token is set there — `settings.js`, `options.html`, issue #195), which saves it as a Job Listing the same way a manually-pasted one is saved (Company Logo downloaded, RAL Range looked up, Application Method inferred, Application created at Saved).
- The button shows a success/failure message after each attempt.
- Nothing happens automatically in the background — only an explicit click triggers a capture.
- `content.js` (LinkedIn) and `content-indeed.js` (Indeed) are board-specific: each has its own selectors and its own known fragility. They share only what's genuinely board-agnostic — the button/status UI, the message-passing to `background.js`, and Turndown-based HTML-to-Markdown conversion — via `capture-common.js`.

Requires the Sumisura backend running locally (`docker-compose up` from the repo root; see the root `README.md`).

## Loading it (unpacked, for local personal use — not published to any store)

**Chrome / Chromium-based:**

1. Open `chrome://extensions`.
2. Enable **Developer mode** (top-right toggle).
3. Click **Load unpacked** and select this `extension/` directory.
4. Visit any LinkedIn or Indeed job posting page — the "Save to Sumisura" button should appear.

**Firefox:**

1. Open `about:debugging#/runtime/this-firefox`.
2. Click **Load Temporary Add-on…** and select `extension/manifest.json` (the manifest file itself, not the folder).
3. Visit any LinkedIn or Indeed job posting page — the "Save to Sumisura" button should appear.

Note: Firefox unloads temporary add-ons when the browser restarts — you'll need to reload it each session. `manifest.json` declares both `background.service_worker` (Chrome) and `background.scripts` (Firefox) so the same extension works unmodified in both.

## Notes

- LinkedIn ships an atomic/hashed CSS build with no stable semantic class names, no `<h1>`, and no JSON-LD structured data on the job-view page. `content.js` instead reads: the job title from `document.title` (`"<Job Title> | <Company> | LinkedIn"`), the company from the first `a[href*="/company/"]` link, the Company Logo from that same link's `<img>` (its `src`, or `data-delayed-url` if LinkedIn hasn't lazy-loaded it yet), and the description as the longest `[data-testid="expandable-text-box"]` block on the page — its own "…altro"/"…more" toggle `<button>` stripped out first, so its label doesn't leak into the captured text — converted to Markdown by the vendored Turndown library (`turndown.js`) so paragraphs, line breaks, lists, bold/italic, and links survive. If capture starts failing, re-run the diagnostic snippet below in the page console and adjust `content.js` accordingly.
- LinkedIn's salary-insight pill (e.g. "63,2K € /yr - 70,8K € /yr") has no stable selector either — `content.js` scans short page elements outside the description for currency-plus-number-shaped text instead and sends the first match as `listingSalaryText`, a separate field from `description`. RAL Range resolution uses it alongside the Job Description text (ADR-0014); a missing or unparsed match changes nothing.
- The Company Logo is downloaded by the backend at save time and stored alongside the Job Listing record (ADR-0013) — a failed download never blocks the save, the Job Listing just ends up with no logo.
- The captured URL: on a direct `/jobs/view/<id>/` page, it's the stripped `window.location.href`. On the search-results split-pane view, clicking between postings only changes the `currentJobId` query param — `window.location` itself stays on the generic search page — so `content.js` reads `currentJobId` and builds `https://www.linkedin.com/jobs/view/<id>/` instead.
- The backend address and access token are set on the options page and kept in `chrome.storage.local`. A non-default address is granted through `optional_host_permissions` when saved; the two local defaults are regular `host_permissions`.
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

What is covered: LinkedIn extraction (`content.test.js` against `fixtures/linkedin-job-view.html` — Job Title, company, Company Logo including the lazy-load fallback, Job Description Markdown with the toggle stripped and the longest block chosen, canonical URL in both the split-pane and direct-page cases, and the salary badge scan found/description-excluded/absent), Indeed extraction (`content-indeed.test.js` against `fixtures/indeed-job-view.html`), the capture validator (`validate-capture.test.js`, pure payloads), and the manifest wiring (`manifest.test.js` — every bundle shipping a board capture script also ships `validate-capture.js`, ahead of it).

What is not: button injection, click handling, the mutation observer that re-injects after a single-page-app re-render, and message-passing to `background.js` all need the `chrome.*` runtime faked, so they stay untested and are exercised manually via "Loading it" above instead. And both fixtures are synthetic — a green suite says the extraction logic is correct against the DOM shape the scripts target, never that LinkedIn or Indeed still serve that shape (see `fixtures/README.md`). `extension/package.json`/`node_modules` exist solely for this test suite; the extension itself still ships as plain, unbundled scripts per `manifest.json`, no build step involved.

### If capture breaks again

Paste into the DevTools console on a LinkedIn job posting page to see what's actually there:

```js
(function(){const og=[...document.querySelectorAll('meta[property^="og:"], meta[name="description"]')].map(el=>({key:el.getAttribute('property')||el.getAttribute('name'),content:el.content}));const dataAttrs=[...document.querySelectorAll('[data-test-id], [data-testid], [data-view-name]')].map(el=>({tag:el.tagName,testId:el.getAttribute('data-test-id')||el.getAttribute('data-testid'),textLen:el.textContent.trim().length})).filter(el=>el.textLen>0);console.log('title:',document.title);console.log('og/meta:',og);console.log('data-attrs:',dataAttrs);})();
```
