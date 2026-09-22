// Runs only on indeed.com job-view pages the user is already viewing, and
// only reads the page's own DOM — the same capture-not-scrape posture
// content.js already established for LinkedIn (see ADR-0007). It acts only
// when the injected button is clicked; the network call itself happens in
// background.js.
//
// Selectors below are based on Indeed's commonly-documented job-view
// markup (data-testid attributes and the long-standing #jobDescriptionText
// container id), not verified against a live Indeed page from this
// environment (no browser access here) — re-confirm against a real page
// before relying on this in production, per the PRD's own caveat. Each
// extraction function takes an explicit Document so it can be exercised in
// tests against a static HTML fixture instead.

const SumisuraCommon =
  typeof module !== "undefined" && module.exports ? require("./capture-common.js") : window.SumisuraCommon;

const DESCRIPTION_SELECTOR = "#jobDescriptionText";

function titleFromPage(doc) {
  // The job-view header's <h1> carries this data-testid most consistently;
  // document.title ("<Title> - <Company> - <City> | Indeed.com") is the
  // fallback when that markup isn't present.
  const heading = SumisuraCommon.firstNonEmptyText(doc, [
    '[data-testid="jobsearch-JobInfoHeader-title"]',
    "h1.jobsearch-JobInfoHeader-title",
    "h1",
  ]);
  if (heading) return heading;
  const [jobTitle] = doc.title.split(" - ").map((part) => part.trim());
  return jobTitle || "";
}

function companyName(doc) {
  return SumisuraCommon.firstNonEmptyText(doc, [
    '[data-testid="inlineHeader-companyName"]',
    '[data-company-name="true"]',
    ".jobsearch-CompanyInfoContainer a",
  ]);
}

function locationText(doc) {
  return SumisuraCommon.firstNonEmptyText(doc, [
    '[data-testid="inlineHeader-companyLocation"]',
    '[data-testid="job-location"]',
  ]);
}

function captureUrl(doc) {
  // Indeed identifies a posting by a "jk" query param on /viewjob; strip
  // every other query param (tracking/source/campaign IDs) so the saved
  // URL reliably reopens the same posting later (story 5), mirroring how
  // content.js strips LinkedIn's currentJobId noise.
  const jobKey = new URLSearchParams(doc.location.search).get("jk");
  const origin = doc.location.origin || "https://www.indeed.com";
  if (jobKey) {
    return `${origin}/viewjob?jk=${jobKey}`;
  }
  return doc.location.href.split("?")[0];
}

function logoUrl(doc) {
  const img = doc.querySelector(
    'img.jobsearch-CompanyAvatar-image, [data-testid="jobsearch-CompanyAvatar"] img, img[alt*="logo" i]'
  );
  if (!img) return "";
  return img.src || img.getAttribute("data-src") || "";
}

function description(doc) {
  return SumisuraCommon.descriptionMarkdown(doc, DESCRIPTION_SELECTOR, ["button"]);
}

// salaryText looks for Indeed's own salary line first (e.g. "$120,000 -
// $150,000 a year"), then falls back to scanning short page elements for
// currency-plus-number-shaped text, the same best-effort approach
// content.js uses for LinkedIn's salary badge. Any failure or no match
// simply yields no new signal — description-text RAL resolution is
// unaffected either way.
function salaryText(doc) {
  try {
    const dedicated = SumisuraCommon.firstNonEmptyText(doc, [
      "#salaryInfoAndJobType",
      '[data-testid="attribute_snippet_testid"]',
    ]);
    if (dedicated) return dedicated;

    const descriptionEl = doc.querySelector(DESCRIPTION_SELECTOR);
    const salaryPatternRe = /[€$£]\s*\d[\d.,]*\s*k?|\d[\d.,]*\s*k\b[^\d]{0,10}\/\s*(yr|hour|year)/i;
    for (const el of doc.querySelectorAll("body *")) {
      if (descriptionEl && descriptionEl.contains(el)) continue;
      const text = el.textContent.trim();
      if (!text || text.length >= 60) continue;
      if (salaryPatternRe.test(text)) return text;
    }
  } catch (err) {
    console.error("[Sumisura] salaryText threw", err);
  }
  return "";
}

function captureJobPosting(doc) {
  doc = doc || document;
  return {
    title: titleFromPage(doc),
    company: companyName(doc),
    location: locationText(doc),
    url: captureUrl(doc),
    description: description(doc),
    logoUrl: logoUrl(doc),
    listingSalaryText: salaryText(doc),
  };
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = { captureJobPosting, titleFromPage, companyName, locationText, captureUrl, logoUrl, description, salaryText };
} else {

  // validateCapture (extension/validate-capture.js, loaded as a content
  // script ahead of this one — see manifest.json) applies issue #58's
  // per-field sanity checks instead of capture-common.js's bare emptiness
  // fallback, so a plausible-but-wrong Indeed capture (a stale nav element
  // read as the company, a truncated Job Description) is refused with a
  // specific reason rather than silently saved.
  SumisuraCommon.initCaptureUI({
    capture: (doc) => captureJobPosting(doc || document),
    postingUrl: (doc) => captureUrl(doc || document),
    validate: validateCapture,
  });
}
