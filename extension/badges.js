// Marks the rows of a search-results page you already track (issue #206,
// stories 47-51), so your pipeline is visible while you browse.
//
// This is the one part of the extension that writes into a third party's
// DOM rather than alongside it, so it is held to a strict rule: annotate
// when it can, and do nothing at all when it can't. A failed lookup, an
// answer in an unexpected shape, or a throw anywhere in here leaves the
// page exactly as it was and logs — it never blocks, never removes and
// never rewrites anything the board rendered.
//
// It reads each row's own href and sends those URLs; Posting Key
// computation stays in Go (ADR-0042), so the extension never holds a
// second, drifting copy of what counts as the same posting.

(function (root) {
  const BADGE_CLASS = "sumisura-row-badge";
  // The badge is one element appended next to the row's own link, styled
  // inline: a page-level stylesheet would be one more thing the board's
  // CSS could fight with, and there is no shadow root to hide in when the
  // badge has to sit inside the board's own layout.
  const BADGE_STYLE = [
    "display:inline-block",
    "margin-left:6px",
    "padding:1px 6px",
    "border-radius:999px",
    "background:#0a66c2",
    "color:#fff",
    "font:600 11px/1.6 -apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif",
    "vertical-align:middle",
    "white-space:nowrap",
  ].join(";");

  // Statuses that mean "this one is over" read in grey, so a rejected role
  // and a live one are different at a glance and not merely different
  // words (story 48).
  const CLOSED_STATUSES = ["rejected", "withdrawn"];
  const CLOSED_BACKGROUND = "#6b6b6b";

  const STATUS_TEXT = {
    saved: "Saved",
    tailoring: "Tailoring",
    sent: "Sent",
    interviewing: "Interviewing",
    rejected: "Rejected",
    offer: "Offer",
    withdrawn: "Withdrawn",
  };

  // maxPerRequest mirrors the backend's own cap on the batch lookup. A
  // results page holds a couple of dozen rows, so this only ever bites on
  // a page that has grown far past one screen.
  const MAX_PER_REQUEST = 200;

  // rowsToLookUp collects the distinct posting URLs on the page that have
  // not been asked about yet, capped at one request's worth. Exported so
  // the "which rows do we ask about" decision is testable on its own.
  function rowsToLookUp(doc, rowSelector, known, cap) {
    const urls = [];
    const seen = new Set();
    const byUrl = new Map();

    doc.querySelectorAll(rowSelector).forEach((anchor) => {
      const url = absoluteHref(anchor);
      if (!url || known.has(url)) return;
      if (!byUrl.has(url)) byUrl.set(url, []);
      byUrl.get(url).push(anchor);
      if (seen.has(url) || urls.length >= cap) return;
      seen.add(url);
      urls.push(url);
    });

    return { urls, byUrl };
  }

  // absoluteHref reads the row's link as the board rendered it. `href` on
  // an anchor is already resolved against the page, which is what makes a
  // relative row href usable without the extension reassembling URLs.
  function absoluteHref(anchor) {
    try {
      return anchor.href || anchor.getAttribute("href") || "";
    } catch {
      return "";
    }
  }

  // createBadger annotates one page. send is the message-passing function
  // (background.js does the network), injected so this can be driven
  // without the chrome.* runtime.
  function createBadger(options) {
    const doc = options.doc || document;
    const rowSelector = options.rowSelector;
    const send = options.send || defaultSend;
    const cap = options.cap || MAX_PER_REQUEST;

    // asked holds every URL this page session has already sent, so
    // scrolling adds rows without re-asking about the ones above them.
    const asked = new Set();
    let inFlight = false;

    function scan() {
      if (inFlight) return;
      let pending;
      try {
        pending = rowsToLookUp(doc, rowSelector, asked, cap);
      } catch (err) {
        console.error("[Sumisura] reading the results page threw, leaving it unannotated", err);
        return;
      }
      if (pending.urls.length === 0) return;

      pending.urls.forEach((url) => asked.add(url));
      inFlight = true;
      send({ type: "SUMISURA_LOOKUP", payload: { urls: pending.urls } }, (response) => {
        inFlight = false;
        try {
          apply(response, pending.byUrl);
        } catch (err) {
          console.error("[Sumisura] badging the results page threw, leaving it unannotated", err);
        }
      });
    }

    function apply(response, byUrl) {
      const results = response && response.ok && response.result && response.result.results;
      if (!Array.isArray(results)) {
        // A failure, or an answer in a shape this doesn't understand, is
        // simply no annotation (story 51).
        return;
      }
      results.forEach((result) => {
        if (!result || !result.tracked) return;
        (byUrl.get(result.url) || []).forEach((anchor) => badge(anchor, result.tracked.status));
      });
    }

    function badge(anchor, status) {
      const row = anchor.parentElement || anchor;
      if (row.querySelector("." + BADGE_CLASS)) return;

      const el = doc.createElement("span");
      el.className = BADGE_CLASS;
      el.textContent = "Sumisura · " + (STATUS_TEXT[status] || status || "tracked");
      el.setAttribute("style", BADGE_STYLE + (CLOSED_STATUSES.indexOf(status) !== -1 ? ";background:" + CLOSED_BACKGROUND : ""));
      anchor.insertAdjacentElement("afterend", el);
    }

    // start badges what is on screen and keeps up with rows the board
    // appends as the user scrolls (story 50). Nothing is on a timer: the
    // observer fires on the board's own rendering, and a scan with no new
    // rows sends nothing.
    function start() {
      scan();
      new MutationObserver(() => scan()).observe(doc.body, { childList: true, subtree: true });
    }

    return { start, scan, asked };
  }

  // defaultSend is the real message-passing path. A runtime error (the
  // extension reloaded under the page, say) is a silent no-annotation,
  // like every other failure here.
  function defaultSend(message, callback) {
    chrome.runtime.sendMessage(message, (response) => {
      callback(chrome.runtime.lastError ? { ok: false } : response);
    });
  }

  const SumisuraBadges = { createBadger, rowsToLookUp, BADGE_CLASS, MAX_PER_REQUEST };

  root.SumisuraBadges = SumisuraBadges;
  if (typeof module !== "undefined" && module.exports) {
    module.exports = SumisuraBadges;
  }
})(typeof window !== "undefined" ? window : globalThis);
