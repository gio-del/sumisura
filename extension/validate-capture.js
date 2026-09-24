// Pure, DOM-free validation for a capture payload (see PRD for issue #58) —
// runs on captureJobPosting()'s output in content.js before that payload is
// ever sent to background.js. Kept as its own content script, loaded before
// content.js (see manifest.json), so it has no chrome.*/DOM dependency and
// can be unit-tested directly (extension/validate-capture.test.js) against
// fixture payloads — mirroring how turndown.js is already loaded as an
// independent content script exposing a bare global (TurndownService).
var validateCapture = (function () {
  // The thresholds below are heuristics tuned against LinkedIn's current DOM
  // (see the selector notes at the top of content.js), not a guarantee of
  // correctness — they only raise the bar from "did we get literally
  // nothing" to "does this look plausible". Kept as named constants so a
  // future person tuning for DOM drift only has to touch these, not the
  // extraction logic.
  var MIN_COMPANY_LENGTH = 2;
  var MIN_DESCRIPTION_LENGTH = 200;
  var GENERIC_COMPANY_BLOCKLIST = ["linkedin", "jobs", "home"];

  // validateCapture(payload) -> string[] of human-readable problems, one
  // per failed check. An empty array means the capture is good enough to
  // send. location/logoUrl/listingSalaryText are intentionally not checked
  // — they're already best-effort/optional per their extraction functions'
  // comments.
  function validateCapture(payload) {
    var problems = [];

    // A pane still swapping postings fails every check below at once, and
    // three "missing X" lines read as a broken extension rather than as the
    // transient, self-healing state it is. Reported on its own, and first,
    // so the card can say what to do about it: wait and capture again.
    if (payload && payload.paneReady === false) {
      return ["the posting is still loading \u2014 capture again in a moment"];
    }

    var title = ((payload && payload.title) || "").trim();
    var company = ((payload && payload.company) || "").trim();
    var description = ((payload && payload.description) || "").trim();

    if (!title) {
      problems.push("missing title");
    }

    if (!company) {
      problems.push("missing company");
    } else if (company.length < MIN_COMPANY_LENGTH) {
      problems.push('company name looks wrong ("' + company + '")');
    } else if (GENERIC_COMPANY_BLOCKLIST.indexOf(company.toLowerCase()) !== -1) {
      problems.push('company name looks wrong ("' + company + '")');
    }

    if (!description) {
      problems.push("missing description");
    } else if (description.length < MIN_DESCRIPTION_LENGTH) {
      problems.push("description looks too short to be a real job posting");
    }

    return problems;
  }

  return validateCapture;
})();

// Node's test runner (extension/validate-capture.test.js) loads this file
// via require(), which has no `window`/browser globals — this branch is
// dead code in the extension itself (a real browser always defines
// `module` as undefined, never throwing on the typeof check) and only
// exists so the same file works unmodified in both environments.
if (typeof module !== "undefined" && module.exports) {
  module.exports = { validateCapture };
}
