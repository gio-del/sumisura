// Runs only on linkedin.com/jobs/* pages the user is already viewing, and
// only reads the page's own DOM — no requests to LinkedIn are made by this
// script (story 2). It acts only when the injected button is clicked
// (story 6); the network call itself happens in background.js.
//
// Each extraction function takes an explicit Document (rather than reading
// the ambient one) so it can be exercised in tests against a static HTML
// fixture — the same shape content-indeed.js already uses. In the browser
// document.location is window.location, so reading location off the passed
// document is the same value the ambient-global version read.

const SumisuraCommon =
  typeof module !== "undefined" && module.exports ? require("./capture-common.js") : window.SumisuraCommon;

// LinkedIn's job-view page ships an atomic/hashed CSS build (class names
// like "b2cfd878") with no stable semantic classes, no <h1>, and no
// schema.org JSON-LD to read structured data from. These three signals
// held up under inspection instead:
//   - document.title follows "<Job Title> | <Company> | LinkedIn"
//   - the company name is the first `a[href*="/company/"]` link's text
//   - the job description is the longest `[data-testid="expandable-text-box"]`
//     block (a shorter one is typically an "about the company" blurb) —
//     it's already the full content regardless of visual line-clamping, so
//     there's no need to click the "see more" toggle first. Its innerHTML
//     (not textContent) is run through Turndown so paragraphs, line
//     breaks, lists, and bold/italic/links survive as Markdown.
// All three are only meaningful once the pane has finished rendering the
// posting the URL names — see paneMatchesPosting. In the split-pane view a
// click swaps the URL's currentJobId at once and the content follows up to
// ~1.5s later, and document.title in particular keeps naming the previous
// posting for that whole window.
//
// If LinkedIn changes any of this, capture will start failing — that's a
// known fragility of reading a third party's DOM (see extension/README.md):
// the fixture-based tests certify this logic, never that LinkedIn still
// serves this markup.

const DESCRIPTION_SELECTOR = '[data-testid="expandable-text-box"]';

function titleFromDocumentTitle(doc) {
  // "Senior Machine Learning Engineer | Prima | LinkedIn" -> the first part
  const [jobTitle] = doc.title.split("|").map((part) => part.trim());
  return jobTitle || "";
}

function companyName(doc) {
  return SumisuraCommon.firstNonEmptyText(doc, ['a[href*="/company/"]']);
}

function captureUrl(doc) {
  // The search-results split-pane view (.../jobs/search-results/?currentJobId=123...)
  // never changes window.location itself when you click between postings —
  // only the currentJobId query param does — so the stripped href is the
  // generic search page, not the posting. A direct /jobs/view/<id>/ page has
  // no currentJobId param, so the existing stripped-URL behavior still
  // applies there unchanged.
  const currentJobId = new URLSearchParams(doc.location.search).get("currentJobId");
  if (currentJobId) {
    return `https://www.linkedin.com/jobs/view/${currentJobId}/`;
  }
  return doc.location.href.split("?")[0];
}

// locationText reads the location out of the metadata row LinkedIn
// renders under the company link — "Milan, Lombardy, Italy · 2 weeks ago ·
// 47 applicants". There is no stable selector for it (hashed atomic
// classes, no data-testid), so this anchors on the company link, which
// content.js already relies on, and takes the first segment of the first
// list item beneath it.
//
// Best-effort exactly like salaryBadgeText: no match, or a row that holds
// only a posting age, simply means the Job Listing has no location, which
// is an ordinary empty value and never a failed capture.
const POSTING_AGE_RE = /^\d+\s+(second|minute|hour|day|week|month|year)s?\s+ago$/i;
const APPLICANT_COUNT_RE = /applicants?$/i;

function locationText(doc) {
  try {
    const link = doc.querySelector('a[href*="/company/"]');
    const container = link && link.closest ? link.closest("div") : null;
    const item = (container || doc).querySelector("ul li");
    if (!item) return "";

    // The row's segments are separated by middots; the location is the
    // first, when there is one at all.
    const [first] = item.textContent.split(/[·•]/);
    const location = (first || "").trim().replace(/\s+/g, " ");
    if (!location || location.length > 80) return "";
    if (POSTING_AGE_RE.test(location) || APPLICANT_COUNT_RE.test(location)) return "";
    return location;
  } catch (err) {
    console.error("[Sumisura] locationText threw", err);
  }
  return "";
}

function companyLogoUrl(doc) {
  // The company logo is an <img> inside the same a[href*="/company/"]
  // link the company name comes from. LinkedIn lazy-loads some images
  // via a data-delayed-url attribute before src is populated, so fall
  // back to that when src is still empty/placeholder.
  const link = doc.querySelector('a[href*="/company/"]');
  const img = link && link.querySelector("img");
  if (!img) return "";
  return img.src || img.getAttribute("data-delayed-url") || "";
}

function description(doc) {
  return SumisuraCommon.descriptionMarkdown(doc, DESCRIPTION_SELECTOR, ["button"]);
}

// salaryBadgeText looks for LinkedIn's own salary-insight pill near the
// job title (e.g. "63,2K € /yr - 70,8K € /yr") — often the only place a
// listing states RAL at all, absent from the description prose entirely.
// No stable selector exists for it (no data-testid/aria-label/role — pure
// hashed atomic CSS classes shared with every other pill in that row), so
// this scans short page elements for currency-plus-number-shaped text
// instead. Best-effort: any failure, or no match, simply yields no new
// signal — description-text RAL resolution is unaffected either way.
function salaryBadgeText(doc) {
  try {
    const descriptionEl = SumisuraCommon.longestElement(doc, DESCRIPTION_SELECTOR);
    const salaryPatternRe = /[€$£]\s*\d[\d.,]*\s*k?|\d[\d.,]*\s*k\b[^\d]{0,10}\/\s*yr/i;
    for (const el of doc.querySelectorAll("body *")) {
      if (descriptionEl && descriptionEl.contains(el)) continue;
      const text = el.textContent.trim();
      if (!text || text.length >= 40) continue;
      if (salaryPatternRe.test(text)) return text;
    }
  } catch (err) {
    console.error("[Sumisura] salaryBadgeText threw", err);
  }
  return "";
}

// postingId is the posting the *URL* currently names: the split pane's
// currentJobId, or the id in a /jobs/view/<id> path.
function postingId(doc) {
  const current = new URLSearchParams(doc.location.search).get("currentJobId");
  if (current) return current;
  const fromPath = doc.location.pathname.match(/\/jobs\/view\/(\d+)/);
  return fromPath ? fromPath[1] : "";
}

// paneMatchesPosting reports whether the rendered pane is the posting the
// URL names.
//
// Clicking a card in the split pane swaps currentJobId immediately and then
// takes up to ~1.5s to render that posting. Measured across that window:
// the URL already carries the new id, `document.title` still names the
// PREVIOUS posting, and the pane's own company and description nodes are
// absent entirely. A capture taken then is the worst kind — well-formed,
// correctly keyed to the new posting, and describing the old one.
//
// The pane's self-link is the signal that separates the two states: absent
// for the whole loading window, present exactly when the content lands. It
// beats waiting a fixed delay (a guess that is both too slow and not slow
// enough) and beats any hashed class name.
function paneMatchesPosting(doc) {
  const id = postingId(doc);
  // Nothing to check against — never block a capture on a page whose
  // posting we cannot identify.
  if (!id) return true;
  return Boolean(doc.querySelector('a[href*="/jobs/view/' + id + '"]'));
}

function captureJobPosting(doc) {
  doc = doc || document;
  const ready = paneMatchesPosting(doc);
  return {
    // document.title is a document-global fact used for a pane-local one,
    // so it is only trustworthy once the pane agrees with the URL. Left
    // empty otherwise rather than carrying the previous posting's title
    // into a record keyed to this one.
    title: ready ? titleFromDocumentTitle(doc) : "",
    company: companyName(doc),
    location: locationText(doc),
    url: captureUrl(doc),
    description: description(doc),
    logoUrl: companyLogoUrl(doc),
    listingSalaryText: salaryBadgeText(doc),
    // Read by validateCapture, which is DOM-free and cannot check this
    // itself.
    paneReady: ready,
  };
}

// hasResultRows reports whether this page is a list of postings, the only
// thing row badges make sense on.
//
// A standalone /jobs/view/<id> page is one posting, not a list — and it
// still contains several `/jobs/view/` links of its own (the "similar
// jobs" rail, and anchors sitting inside the posting's own metadata row).
// Badging every one of them appended Sumisura pills over LinkedIn's own
// salary/hybrid/full-time chips, which is a third party's layout broken by
// us: exactly what badges.js promises never to do. There is nothing to
// annotate on a single posting anyway — the capture UI above already
// reports that posting's state.
function hasResultRows(doc) {
  return !/^\/jobs\/view\//.test(doc.location.pathname);
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = {
    captureJobPosting,
    titleFromDocumentTitle,
    companyName,
    locationText,
    captureUrl,
    companyLogoUrl,
    postingId,
    paneMatchesPosting,
    description,
    hasResultRows,
    salaryBadgeText,
  };
} else {
  // validateCapture (extension/validate-capture.js, loaded as a content
  // script ahead of this one — see manifest.json) is the single place that
  // decides whether a capture is good enough to send: it replaces the old
  // bare-emptiness check with per-field sanity checks (see PRD for issue
  // #58), so a plausible-but-wrong capture (stale nav element, truncated
  // description) is caught instead of silently saved.
  SumisuraCommon.initCaptureUI({
    capture: (doc) => captureJobPosting(doc || document),
    // Cheap enough to re-read on a timer: it reads the URL only, while
    // captureJobPosting runs Turndown over the whole description.
    postingUrl: (doc) => captureUrl(doc || document),
    validate: validateCapture,
  });

  // Every search result row links to its own posting, so the rows are
  // found by that link rather than by any of LinkedIn's hashed class names
  // — the one selector here that isn't hostage to their CSS build (issue
  // #206, stories 47-51). Badging only ever adds a span beside a row's
  // link, and does nothing at all if anything goes wrong.
  //
  // Only on a page that actually lists postings: see hasResultRows.
  if (hasResultRows(document)) {
    SumisuraBadges.createBadger({
      doc: document,
      rowSelector: 'a[href*="/jobs/view/"]',
    }).start();
  }
}
