// Shared, board-agnostic pieces used by every board's content script:
// capture-button/status UI injection, click -> background.js message-passing,
// and generic DOM-reading helpers (Turndown-based HTML-to-Markdown, "first
// non-empty text from a selector list", "longest element matching a
// selector"). Board-specific field extraction (which selectors to try, how
// to build the canonical URL, etc.) stays in each board's own content
// script — this module only holds what genuinely doesn't vary by board.

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

  function showStatus(statusEl, ok, message) {
    statusEl.textContent = message;
    statusEl.className = "sumisura-capture-status " + (ok ? "sumisura-capture-status--ok" : "sumisura-capture-status--error");
    statusEl.hidden = false;
    root.clearTimeout(showStatus._timer);
    showStatus._timer = root.setTimeout(() => {
      statusEl.hidden = true;
    }, 5000);
  }

  // successMessage is the status text for a saved capture. When the backend
  // reports the capture completed a link shared earlier from a phone
  // (issue #183), say so, so the two halves visibly join up.
  function successMessage(response) {
    if (response && response.completedPendingCapture) {
      return "Saved to Sumisura — and completed the link you shared from your phone.";
    }
    return "Saved to Sumisura.";
  }

  function onCaptureClick(captureJobPosting, button, statusEl, validate) {
    console.log("[Sumisura] button clicked");
    let payload;
    try {
      payload = captureJobPosting();
    } catch (err) {
      console.error("[Sumisura] captureJobPosting threw", err);
      showStatus(statusEl, false, "Capture failed: " + err.message);
      return;
    }
    console.log("[Sumisura] captured payload", payload);

    // validate (extension/validate-capture.js, when loaded ahead of a given
    // board's content script — see manifest.json) replaces the bare
    // emptiness check below with per-field sanity checks (issue #58), so a
    // plausible-but-wrong capture (stale nav element, truncated
    // description) is caught instead of silently saved. Boards that don't
    // load it yet fall back to the original bare check.
    if (validate) {
      const problems = validate(payload);
      if (problems.length > 0) {
        showStatus(statusEl, false, "Capture looks wrong: " + problems.join(", "));
        return;
      }
    } else if (!payload.company || !payload.description) {
      showStatus(statusEl, false, "Couldn't find a job posting on this page — open a specific listing and try again.");
      return;
    }

    button.disabled = true;
    button.textContent = "Saving…";

    console.log("[Sumisura] sending message to background");
    chrome.runtime.sendMessage({ type: "SUMISURA_CAPTURE", payload }, (response) => {
      console.log("[Sumisura] got response", response, "lastError:", chrome.runtime.lastError);
      button.disabled = false;
      button.textContent = "Save to Sumisura";

      if (chrome.runtime.lastError) {
        showStatus(statusEl, false, chrome.runtime.lastError.message);
        return;
      }
      if (response && response.ok) {
        showStatus(statusEl, true, successMessage(response));
      } else {
        showStatus(statusEl, false, (response && response.error) || "Failed to save.");
      }
    });
  }

  function ensureUI(doc, captureJobPosting, validate) {
    if (doc.getElementById("sumisura-capture-btn")) return;

    const button = doc.createElement("button");
    button.id = "sumisura-capture-btn";
    button.type = "button";
    button.className = "sumisura-capture-btn";
    button.textContent = "Save to Sumisura";

    const statusEl = doc.createElement("div");
    statusEl.id = "sumisura-capture-status";
    statusEl.className = "sumisura-capture-status";
    statusEl.hidden = true;

    button.addEventListener("click", () => onCaptureClick(captureJobPosting, button, statusEl, validate));

    doc.body.appendChild(button);
    doc.body.appendChild(statusEl);
  }

  function initCaptureUI(captureJobPosting, validate) {
    console.log("[Sumisura] content script loaded", root.location.href);
    ensureUI(document, captureJobPosting, validate);
    // Job boards are typically single-page apps; guard against our injected
    // elements being removed by their own re-renders on client-side
    // navigation between postings.
    new MutationObserver(() => ensureUI(document, captureJobPosting, validate)).observe(document.body, { childList: true, subtree: false });
  }

  const SumisuraCommon = {
    firstNonEmptyText,
    longestElement,
    descriptionMarkdown,
    showStatus,
    successMessage,
    onCaptureClick,
    ensureUI,
    initCaptureUI,
  };

  root.SumisuraCommon = SumisuraCommon;
  if (typeof module !== "undefined" && module.exports) {
    module.exports = SumisuraCommon;
  }
})(typeof window !== "undefined" ? window : globalThis);
