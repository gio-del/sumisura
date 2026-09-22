// The extension card's whole decision logic, as a pure function (issue
// #206). Given what the lookup found, what was captured off the page, and
// how the last save went, it returns what the card shows and which actions
// it offers. The DOM layer draws this and nothing else — so the card's
// behaviour is tested here (card-model.test.js) with no DOM and no
// chrome.*, the way validate-capture.js already is.
//
// Loaded as a bare-global content script ahead of content.js (see
// manifest.json), and required directly by the tests.
var SumisuraCard = (function () {
  // Resolution kinds, as POST /api/job-listings/from-extension names them.
  var SAVE_ANYWAY = "save-anyway";
  var REPLACE = "replace";
  var UNARCHIVE_EXISTING = "unarchive-existing";

  // Fields the card can edit. A capture whose only problems are these is
  // fixable without leaving the page; anything else (a description that
  // came out too short) needs the page itself to be in a better state.
  var EDITABLE_FIELD_PATTERNS = [/title/i, /company/i];

  var STATUS_LABELS = {
    saved: "Saved",
    tailoring: "Tailoring",
    sent: "Sent",
    interviewing: "Interviewing",
    rejected: "Rejected",
    offer: "Offer",
    withdrawn: "Withdrawn",
  };

  // MOVE_LABELS are the buttons' words: an imperative, not a state name,
  // because the user is doing the move rather than describing it.
  var MOVE_LABELS = {
    tailoring: "Start tailoring",
    sent: "Mark as sent",
    interviewing: "Interviewing",
    rejected: "Rejected",
    offer: "Offer",
    withdrawn: "Withdraw",
  };

  function statusLabel(status) {
    return STATUS_LABELS[status] || status || "";
  }

  function jobListingLink(serverUrl, id) {
    return String(serverUrl || "").replace(/\/+$/, "") + "/jobs/" + encodeURIComponent(id);
  }

  // shortDate keeps the card compact. An unparseable date degrades to the
  // raw string rather than to "Invalid Date".
  function shortDate(value) {
    var parsed = new Date(value);
    if (!value || isNaN(parsed.getTime())) return String(value || "");
    return parsed.toLocaleDateString();
  }

  // sibling turns one listing from the lookup (or from the company-decision
  // refusal, which carries the same shape) into a row, each carrying the
  // replace action for itself so the DOM layer never builds a resolution.
  function sibling(listing, serverUrl) {
    return {
      id: listing.id,
      title: listing.title || "(no job title)",
      savedAt: listing.savedAt,
      savedAtLabel: shortDate(listing.savedAt),
      status: listing.status,
      statusLabel: statusLabel(listing.status),
      link: jobListingLink(serverUrl, listing.id),
      replaceAction: {
        id: "replace:" + listing.id,
        label: "Save and archive this one",
        resolution: { kind: REPLACE, jobListingId: listing.id },
      },
    };
  }

  function siblingsOf(listings, serverUrl) {
    return (listings || []).map(function (listing) {
      return sibling(listing, serverUrl);
    });
  }

  function unarchiveAction(id) {
    return {
      id: "unarchive",
      label: "Bring it back from the archive",
      resolution: { kind: UNARCHIVE_EXISTING, jobListingId: id },
    };
  }

  // statusMoves turns allowedTransitions into buttons. The list comes
  // straight from the backend's own state machine, so the card can never
  // offer a move the app forbids — and the backend re-checks every one
  // anyway (stories 44, 45).
  function statusMoves(tracked) {
    return ((tracked && tracked.allowedTransitions) || []).map(function (status) {
      return { status: status, label: MOVE_LABELS[status] || statusLabel(status) };
    });
  }

  // fixableHere reports whether every validation problem is one of the two
  // fields the card lets the user edit.
  function fixableHere(problems) {
    return (problems || []).every(function (problem) {
      return EDITABLE_FIELD_PATTERNS.some(function (pattern) {
        return pattern.test(problem);
      });
    });
  }

  function editableFields(capture) {
    return {
      editable: true,
      title: (capture && capture.title) || "",
      company: (capture && capture.company) || "",
    };
  }

  function emptyView(state, headline) {
    return {
      state: state,
      headline: headline,
      detail: "",
      problems: [],
      siblings: [],
      actions: [],
      statusMoves: [],
      fields: { editable: false, title: "", company: "" },
      link: null,
      busy: false,
      fixableHere: true,
    };
  }

  // cardModel({ serverUrl, capture, validation, lookup, outcome }) -> view.
  //
  // The branches are ordered by what the user most recently did: an
  // outcome they just caused outranks the lookup that was already on
  // screen, and a capture that cannot be sent outranks anything the
  // backend said about a posting.
  function cardModel(input) {
    input = input || {};
    var serverUrl = input.serverUrl || "";
    var capture = input.capture || null;
    var validation = input.validation || [];
    var lookup = input.lookup || { state: "idle" };
    var outcome = input.outcome || { state: "idle" };

    if (outcome.state === "saving") {
      var saving = emptyView("saving", "Saving to Sumisura…");
      saving.busy = true;
      return saving;
    }

    if (outcome.state === "saved") {
      return savedView(outcome, input, serverUrl);
    }

    if (outcome.state === "refused") {
      return refusedView(outcome, serverUrl, capture);
    }

    if (outcome.state === "error") {
      var failed = emptyView("error", outcome.error || "Saving failed.");
      failed.actions = [saveAction(lookup)];
      failed.fields = editableFields(capture);
      return failed;
    }

    if (!capture) {
      return emptyView("no-posting", "Open a job posting to save it.");
    }

    if (validation.length > 0) {
      var invalid = emptyView("invalid", "This capture doesn't look like a job posting.");
      invalid.problems = validation.slice();
      invalid.fields = editableFields(capture);
      invalid.fixableHere = fixableHere(validation);
      invalid.detail = invalid.fixableHere
        ? "Correct the fields below and save."
        : "Reading the posting itself went wrong — open the posting's own page and try again.";
      invalid.actions = [saveAction(lookup)];
      return invalid;
    }

    if (lookup.state === "loading") {
      return emptyView("loading", "Checking Sumisura…");
    }

    // A lookup that failed is not an answer. Saying "not saved yet" here
    // would be a guess dressed as a fact (story 37), and the capture must
    // still be possible (story 38).
    if (lookup.state === "failed") {
      var unreachable = emptyView("unreachable", "Couldn't reach Sumisura, so this posting's status is unknown.");
      unreachable.detail = lookup.error || "";
      unreachable.actions = [saveAction(lookup)];
      unreachable.fields = editableFields(capture);
      return unreachable;
    }

    if (lookup.state !== "ready") {
      return emptyView("no-posting", "Open a job posting to save it.");
    }

    var result = lookup.result || {};
    if (result.tracked) {
      return trackedView(result.tracked, serverUrl);
    }
    return untrackedView(result, serverUrl, capture);
  }

  // saveAction is the plain save. It carries save-anyway only once the
  // card has actually shown the siblings — otherwise the backend's
  // same-company question is the thing that surfaces them (story 24).
  function saveAction(lookup) {
    var listings = lookup && lookup.result && lookup.result.company && lookup.result.company.listings;
    if (listings && listings.length > 0) {
      return { id: "save-anyway", label: "Save anyway", resolution: { kind: SAVE_ANYWAY } };
    }
    return { id: "save", label: "Save to Sumisura", resolution: null };
  }

  function untrackedView(result, serverUrl, capture) {
    var listings = (result.company && result.company.listings) || [];
    var view = emptyView("untracked", "Not saved yet.");
    view.siblings = siblingsOf(listings, serverUrl);
    view.fields = editableFields(capture);

    if (listings.length === 0) {
      view.actions = [{ id: "save", label: "Save to Sumisura", resolution: null }];
      return view;
    }

    // The siblings are on screen, so the user has already been asked.
    // Saving carries the decision on the first request rather than
    // bouncing off the backend's question (see the issue's Further Notes).
    view.detail =
      listings.length === 1
        ? "You already track one other role at this company."
        : "You already track " + listings.length + " other roles at this company.";
    view.actions = [{ id: "save-anyway", label: "Save anyway", resolution: { kind: SAVE_ANYWAY } }];
    return view;
  }

  function trackedView(tracked, serverUrl) {
    var view = emptyView("tracked", "Already saved to Sumisura.");
    view.detail =
      statusLabel(tracked.status) +
      " · saved " +
      shortDate(tracked.savedAt) +
      (tracked.archived ? " · archived" : "");
    view.link = { href: jobListingLink(serverUrl, tracked.id), label: "Open in Sumisura" };
    view.statusMoves = statusMoves(tracked);
    if (tracked.archived) {
      view.actions = [unarchiveAction(tracked.id)];
    }
    return view;
  }

  function refusedView(outcome, serverUrl, capture) {
    var conflict = outcome.conflict || {};

    if (conflict.reason === "company-has-listings") {
      var listings = (conflict.company && conflict.company.listings) || [];
      var decide = emptyView("needs-decision", conflict.message || "You already track other roles at this company.");
      decide.siblings = siblingsOf(listings, serverUrl);
      decide.actions = [
        { id: "save-anyway", label: "Save anyway", resolution: { kind: SAVE_ANYWAY } },
        { id: "cancel", label: "Cancel", resolution: null },
      ];
      decide.fields = editableFields(capture);
      return decide;
    }

    if (conflict.reason === "duplicate-posting") {
      var existing = conflict.existing || {};
      var dup = emptyView("refused-duplicate", conflict.message || "This posting is already saved as a Job Listing.");
      if (existing.id) {
        dup.detail = (existing.title || "(no job title)") + " · saved " + shortDate(existing.savedAt);
        dup.link = { href: jobListingLink(serverUrl, existing.id), label: "Open the existing Job Listing" };
        if (existing.archived) {
          dup.actions = [unarchiveAction(existing.id)];
        }
      }
      return dup;
    }

    var other = emptyView("refused", conflict.message || outcome.error || "Sumisura refused this save.");
    other.fields = editableFields(capture);
    return other;
  }

  function savedView(outcome, input, serverUrl) {
    var saved = outcome.saved || {};
    var listing = saved.jobListing || {};
    var view = emptyView("saved", "Saved to Sumisura.");
    if (listing.id) {
      view.link = { href: jobListingLink(serverUrl, listing.id), label: "Open in Sumisura" };
    }

    if (saved.archiveFailed) {
      // Never let a half-done consolidation read as a finished one
      // (story 20).
      view.detail = "Saved — but Sumisura couldn't archive the role you replaced. Archive it yourself in the app.";
      return view;
    }
    if (saved.archivedJobListingId) {
      view.detail = "Saved, and archived " + describeArchived(saved.archivedJobListingId, input) + ".";
      return view;
    }
    if (saved.completedPendingCaptureId) {
      view.detail = "It also completed the link you shared from your phone.";
    }
    return view;
  }

  // describeArchived names the replaced role by its Job Title when the
  // card still holds the lookup that listed it, and by id otherwise —
  // better a bare id than a vague "one of them".
  function describeArchived(id, input) {
    var listings =
      (input.lookup && input.lookup.result && input.lookup.result.company && input.lookup.result.company.listings) || [];
    for (var i = 0; i < listings.length; i++) {
      if (listings[i].id === id && listings[i].title) return listings[i].title;
    }
    return id;
  }

  return {
    cardModel: cardModel,
    statusLabel: statusLabel,
    jobListingLink: jobListingLink,
    SAVE_ANYWAY: SAVE_ANYWAY,
    REPLACE: REPLACE,
    UNARCHIVE_EXISTING: UNARCHIVE_EXISTING,
  };
})();

if (typeof module !== "undefined" && module.exports) {
  module.exports = SumisuraCard;
}
