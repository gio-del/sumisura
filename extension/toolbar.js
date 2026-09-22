// What the extension's toolbar icon says about the posting in the active
// tab (issue #206, story 52), and the name of the capture shortcut
// (stories 53-54).
//
// Pure: the background service worker applies these to chrome.action, and
// the tests exercise them directly. Kept out of settings.js, which is
// about the connection, and out of card-model.js, which the service worker
// does not load.

(function (root) {
  // SAVE_COMMAND is the command id. The manifest declares it and
  // background.js listens for it; naming it once keeps a typo from
  // becoming a shortcut that silently does nothing.
  const SAVE_COMMAND = "save-posting";

  const TRACKED_COLOR = "#0a66c2";
  const ARCHIVED_COLOR = "#6b6b6b";

  const STATUS_TEXT = {
    saved: "Saved",
    tailoring: "Tailoring",
    sent: "Sent",
    interviewing: "Interviewing",
    rejected: "Rejected",
    offer: "Offer",
    withdrawn: "Withdrawn",
  };

  // toolbarBadge turns the lookup's tracked state into what chrome.action
  // should show. An empty text clears the badge: the icon claims something
  // only when there is something to claim, so "not tracked" and "we don't
  // know" both leave it bare and differ in the tooltip instead.
  function toolbarBadge(tracked, options) {
    if (options && options.unreachable) {
      return { text: "", color: TRACKED_COLOR, title: "Couldn't reach Sumisura, so this posting's status is unknown." };
    }
    if (!tracked) {
      return { text: "", color: TRACKED_COLOR, title: "Not saved to Sumisura yet." };
    }
    const status = STATUS_TEXT[tracked.status] || tracked.status || "tracked";
    if (tracked.archived) {
      return { text: "✓", color: ARCHIVED_COLOR, title: "In Sumisura, archived (" + status + ")." };
    }
    return { text: "✓", color: TRACKED_COLOR, title: "In Sumisura — " + status + "." };
  }

  const SumisuraToolbar = { toolbarBadge, SAVE_COMMAND };

  root.SumisuraToolbar = SumisuraToolbar;
  if (typeof module !== "undefined" && module.exports) {
    module.exports = SumisuraToolbar;
  }
})(typeof self !== "undefined" ? self : globalThis);
