const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { JSDOM } = require("jsdom");

// Same two globals content-indeed.test.js sets up, for the same reasons:
// capture-common.js reads a bare `TurndownService` identifier (a global in
// the browser, where turndown.js declares it with a top-level `var`), and
// turndown's own HTML-string parser falls back to a bare `document` when no
// `window`/`DOMParser` exists — a throwaway document is enough there, it
// only parses the markup string being converted, never the fixture under
// test.
global.TurndownService = require("./turndown.js");
global.document = new JSDOM("<!doctype html><html><body></body></html>").window.document;

const { captureJobPosting, locationText } = require("./content.js");

const SPLIT_PANE_URL =
  "https://www.linkedin.com/jobs/search-results/?currentJobId=4321567890&keywords=ml%20engineer&refId=xyz";

// Every variant below is a mutation of this one fixture (the Indeed suite
// already establishes that convention) rather than another committed file.
function loadFixtureDocument(url) {
  const html = fs.readFileSync(path.join(__dirname, "fixtures", "linkedin-job-view.html"), "utf8");
  const dom = new JSDOM(html, { url: url || SPLIT_PANE_URL });
  return dom.window.document;
}

test("captureJobPosting reads the Job Title out of the document title's first segment", () => {
  const doc = loadFixtureDocument();

  assert.equal(captureJobPosting(doc).title, "Senior Machine Learning Engineer");

  // Pinned against the document title specifically, not against whatever
  // heading-ish element the page happens to render: LinkedIn's job view has
  // no <h1> and no stable semantic class, so "<Job Title> | <Company> |
  // LinkedIn" is the signal (issue #17).
  doc.title = "  Staff Data Engineer | Acme Rockets | LinkedIn";
  assert.equal(captureJobPosting(doc).title, "Staff Data Engineer");
});

test("captureJobPosting reads the company name from the company link", () => {
  const payload = captureJobPosting(loadFixtureDocument());

  assert.equal(payload.company, "Acme Rockets");
});

test("captureJobPosting reads the Company Logo URL from the company link's image", () => {
  const payload = captureJobPosting(loadFixtureDocument());

  assert.equal(payload.logoUrl, "https://media.example.invalid/logos/acme-rockets.png");
});

test("captureJobPosting falls back to the lazy-load attribute when the logo src is still empty", () => {
  const doc = loadFixtureDocument();
  // LinkedIn ships the <img> with data-delayed-url populated and src empty
  // until it lazy-loads; the fixture carries both, so removing src is enough
  // to reproduce the pre-load state.
  doc.querySelector('a[href*="/company/"] img').removeAttribute("src");

  const payload = captureJobPosting(doc);

  assert.equal(payload.logoUrl, "https://media.example.invalid/logos/acme-rockets-delayed.png");
});

test("captureJobPosting converts the Job Description to Markdown, preserving formatting", () => {
  const payload = captureJobPosting(loadFixtureDocument());

  assert.match(payload.description, /\*\*Senior Machine Learning Engineer\*\*/);
  assert.match(payload.description, /Design, train and ship ranking models in production/);
  assert.match(payload.description, /_measurable_/);
});

test("captureJobPosting strips the description's own expand toggle from the Job Description", () => {
  const payload = captureJobPosting(loadFixtureDocument());

  // Issue #25: the "…see more"/"…altro" button label leaked into the
  // captured text when the container was read as-is.
  assert.doesNotMatch(payload.description, /see more/);
});

test("captureJobPosting picks the longest expandable block, not the about-the-company blurb", () => {
  const payload = captureJobPosting(loadFixtureDocument());

  assert.doesNotMatch(payload.description, /orbital delivery infrastructure/);
});

test("captureJobPosting rebuilds the canonical URL from the split-pane currentJobId", () => {
  const payload = captureJobPosting(loadFixtureDocument());

  // Issue #17: clicking between postings in the search-results split pane
  // only changes currentJobId — window.location stays on the generic search
  // page — so the stripped href would save the wrong URL.
  assert.equal(payload.url, "https://www.linkedin.com/jobs/view/4321567890/");
});

test("captureJobPosting leaves a direct job-view page's stripped URL unchanged", () => {
  const doc = loadFixtureDocument("https://www.linkedin.com/jobs/view/4321567890/?refId=abc&trackingId=def");

  assert.equal(captureJobPosting(doc).url, "https://www.linkedin.com/jobs/view/4321567890/");
});

test("captureJobPosting finds the listing's salary badge for RAL Range resolution", () => {
  const payload = captureJobPosting(loadFixtureDocument());

  assert.equal(payload.listingSalaryText, "63,2K € /yr - 70,8K € /yr");
});

test("captureJobPosting ignores salary-shaped text inside the Job Description", () => {
  const doc = loadFixtureDocument();
  // With the badge gone, the only currency figure left on the page is the
  // "€65.000 gross per year" quoted in the description prose — that is not
  // the listing's own structured salary field and must not be reported as
  // one (ADR-0014 keeps listingSalaryText separate from description text).
  doc.querySelectorAll("li").forEach((el) => {
    if (el.textContent.includes("63,2K")) el.remove();
  });

  assert.equal(captureJobPosting(doc).listingSalaryText, "");
});

test("captureJobPosting yields an empty salary rather than throwing when no badge is present", () => {
  const dom = new JSDOM(
    '<!doctype html><html><head><title>Role | Co | LinkedIn</title></head><body><div data-testid="expandable-text-box"><p>No figures anywhere on this page.</p></div></body></html>',
    { url: "https://www.linkedin.com/jobs/view/1/" }
  );

  assert.equal(captureJobPosting(dom.window.document).listingSalaryText, "");
});

test("captureJobPosting yields empty company/description (capture-failure signal) on a page with no job posting markup", () => {
  const dom = new JSDOM("<!doctype html><html><head><title>LinkedIn</title></head><body><p>Jobs</p></body></html>", {
    url: "https://www.linkedin.com/jobs/",
  });

  const payload = captureJobPosting(dom.window.document);

  assert.equal(payload.company, "");
  assert.equal(payload.description, "");
});

// Location (issue #206, story 55). LinkedIn renders it as the first
// segment of the metadata row under the company link — "Milan, Lombardy,
// Italy · 2 weeks ago · 47 applicants". Best-effort like the salary badge:
// no match is an ordinary empty location, never a failed capture.
test("locationText reads the location out of the metadata row's first segment", () => {
  assert.equal(locationText(loadFixtureDocument()), "Milan, Lombardy, Italy");
});

test("locationText never returns the job title or the company name", () => {
  const location = locationText(loadFixtureDocument());
  assert.notEqual(location, "Senior Machine Learning Engineer");
  assert.notEqual(location, "Acme Rockets");
});

test("locationText yields nothing when the metadata row isn't there", () => {
  const doc = loadFixtureDocument();
  doc.querySelectorAll("ul").forEach((ul) => ul.remove());

  assert.equal(locationText(doc), "");
});

test("locationText yields nothing rather than a posting age when that is all the row holds", () => {
  const doc = loadFixtureDocument();
  const row = doc.querySelector("main ul");
  row.innerHTML = "<li>2 weeks ago &middot; 47 applicants</li>";

  assert.equal(locationText(doc), "");
});

test("captureJobPosting carries the location it found", () => {
  assert.equal(captureJobPosting(loadFixtureDocument()).location, "Milan, Lombardy, Italy");
});
