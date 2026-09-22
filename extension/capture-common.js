// Shared, board-agnostic pieces used by every board's content script:
// mounting the injected card, and generic DOM-reading helpers
// (Turndown-based HTML-to-Markdown, "first non-empty text from a selector
// list", "longest element matching a selector"). Board-specific field
// extraction (which selectors to try, how to build the canonical URL, etc.)
// stays in each board's own content script — this module only holds what
// genuinely doesn't vary by board.
//
// The card's own behaviour lives in card-model.js (what to show) and
// card-view.js (how to draw it); this file only hands the card the board's
// functions.

(function (root) {
  function firstNonEmptyText(doc, selectors) {
    for (const selector of selectors) {
      const el = doc.querySelector(selector);
      const value = el && el.textContent.trim();
      if (value) return value;
    }
    return "";
  }

  function longestElement(doc, selector) {
    let longestEl = null;
    let longestLen = 0;
    for (const el of doc.querySelectorAll(selector)) {
      const len = el.textContent.trim().length;
      if (len > longestLen) {
        longestLen = len;
        longestEl = el;
      }
    }
    return longestEl;
  }

  function htmlToMarkdown(html) {
    // turndown.js is loaded ahead of this module in every content-script
    // entry (see manifest.json), defining the global TurndownService.
    return new TurndownService().turndown(html).trim();
  }

  function descriptionMarkdown(doc, selector, stripSelectors) {
    const el = longestElement(doc, selector);
    if (!el) return "";
    // Job boards commonly nest stray interactive controls ("see more"
    // toggles, apply buttons) inside the description container itself, so
    // their labels leak into the captured text if read as-is. Clone first
    // and strip them out — degrades to the unmodified description (rather
    // than failing capture entirely) if the clone/strip step ever throws.
    let html = el.innerHTML;
    try {
      const clone = el.cloneNode(true);
      (stripSelectors || ["button"]).forEach((sel) => {
        clone.querySelectorAll(sel).forEach((node) => node.remove());
      });
      html = clone.innerHTML;
    } catch (err) {
      console.error("[Sumisura] description clean-up threw, using unmodified description", err);
    }
    return htmlToMarkdown(html);
  }

  // initCaptureUI mounts the card (card-view.js) for one board. The button
  // and its five-second status line it replaced had nowhere to put what the
  // card now shows before the click — tracked state, the company's other
  // roles, correctable fields (issue #206).
  //
  // Each board passes its own capture and postingUrl: postingUrl must be
  // cheap, because it is what the card re-reads to notice the posting
  // changed, while capture runs Turndown over the whole description.
  function initCaptureUI(options) {
    console.log("[Sumisura] content script loaded", root.location.href);
    SumisuraCardView.createCard(options).start();
  }

  const SumisuraCommon = {
    firstNonEmptyText,
    longestElement,
    descriptionMarkdown,
    initCaptureUI,
  };

  root.SumisuraCommon = SumisuraCommon;
  if (typeof module !== "undefined" && module.exports) {
    module.exports = SumisuraCommon;
  }
})(typeof window !== "undefined" ? window : globalThis);
