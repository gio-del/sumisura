const test = require("node:test");
const assert = require("node:assert");

const { cardModel, SAVE_ANYWAY, REPLACE, UNARCHIVE_EXISTING } = require("./card-model.js");

// The card's whole decision logic lives here, so these tests are the card's
// real coverage (issue #206). No DOM, no chrome.*: the rendering layer only
// draws what this returns.

const SERVER = "http://127.0.0.1:8080";
const CAPTURE = {
  title: "Backend Engineer",
  company: "Acme",
  url: "https://www.linkedin.com/jobs/view/4012345678/",
  description: "x".repeat(300),
};

function model(overrides) {
  return cardModel(Object.assign({ serverUrl: SERVER, capture: CAPTURE, lookup: { state: "ready", result: { tracked: null, company: { listings: [] } } } }, overrides));
}

// actionKinds is what the card offers, in order.
function actionKinds(view) {
  return view.actions.map((a) => a.id);
}

test("while the lookup is in flight, the card says so and offers nothing to act on", () => {
  const view = model({ lookup: { state: "loading" } });

  assert.equal(view.state, "loading");
  assert.match(view.headline, /checking/i);
  assert.deepEqual(actionKinds(view), []);
  assert.equal(view.link, null);
});

test("an untracked posting at a company with nothing else offers a plain save", () => {
  const view = model();

  assert.equal(view.state, "untracked");
  assert.match(view.headline, /not saved/i);
  assert.deepEqual(view.siblings, []);
  assert.deepEqual(actionKinds(view), ["save"]);
  // Story 41: a bad extraction is something to fix, not something that defeats you.
  assert.equal(view.fields.editable, true);
  assert.equal(view.fields.title, "Backend Engineer");
  assert.equal(view.fields.company, "Acme");
});

test("an untracked posting at a company I already track lists those roles before I click", () => {
  const view = model({
    lookup: {
      state: "ready",
      result: {
        tracked: null,
        company: {
          listings: [
            { id: "acme-2", title: "Platform Engineer", savedAt: "2026-09-18T11:02:10Z", status: "tailoring" },
            { id: "acme-3", title: "Staff Engineer", savedAt: "2026-09-10T09:00:00Z", status: "sent" },
          ],
        },
      },
    },
  });

  assert.equal(view.state, "untracked");
  assert.equal(view.siblings.length, 2);
  assert.equal(view.siblings[0].title, "Platform Engineer");
  assert.match(view.siblings[0].statusLabel, /tailoring/i);
  assert.equal(view.siblings[0].link, SERVER + "/jobs/acme-2");

  // Story 16: adding alongside. Story 17: replacing one, in one action.
  const kinds = actionKinds(view);
  assert.ok(kinds.includes("save-anyway"), kinds);
  assert.equal(view.siblings[0].replaceAction.resolution.kind, REPLACE);
  assert.equal(view.siblings[0].replaceAction.resolution.jobListingId, "acme-2");
});

test("having been shown the siblings, saving carries the decision on the first request", () => {
  const view = model({
    lookup: { state: "ready", result: { tracked: null, company: { listings: [{ id: "acme-2", title: "Platform Engineer", savedAt: "2026-09-18T11:02:10Z", status: "sent" }] } } },
  });

  const save = view.actions.find((a) => a.id === "save-anyway");
  assert.equal(save.resolution.kind, SAVE_ANYWAY, "the card already asked, so the save must not ask again");
});

test("a tracked posting shows its Status and links into the app instead of offering a save", () => {
  const view = model({
    lookup: {
      state: "ready",
      result: {
        tracked: { id: "acme", title: "Backend Engineer", savedAt: "2026-09-20T09:12:44Z", status: "saved", archived: false, allowedTransitions: ["tailoring", "withdrawn"] },
        company: { listings: [] },
      },
    },
  });

  assert.equal(view.state, "tracked");
  assert.match(view.headline, /already saved/i);
  assert.match(view.detail, /saved/i);
  // Story 40: a way into the record, not just a label on it.
  assert.equal(view.link.href, SERVER + "/jobs/acme");
  assert.ok(!actionKinds(view).includes("save"), "a tracked posting must not offer a save");
});

test("the Status moves offered are exactly the ones the backend called legal", () => {
  function movesFor(allowedTransitions, status) {
    const view = model({
      lookup: { state: "ready", result: { tracked: { id: "acme", title: "T", savedAt: "2026-09-20T09:12:44Z", status, archived: false, allowedTransitions }, company: { listings: [] } } },
    });
    return view.statusMoves.map((m) => m.status);
  }

  assert.deepEqual(movesFor(["tailoring", "withdrawn"], "saved"), ["tailoring", "withdrawn"]);
  assert.deepEqual(movesFor(["interviewing", "rejected", "withdrawn"], "sent"), ["interviewing", "rejected", "withdrawn"]);
  // Story 44: a terminal Status offers nothing rather than something illegal.
  assert.deepEqual(movesFor([], "offer"), []);
  assert.deepEqual(movesFor(undefined, "offer"), []);
});

test("a tracked posting that is archived offers to bring it back", () => {
  const view = model({
    lookup: {
      state: "ready",
      result: { tracked: { id: "acme", title: "Backend Engineer", savedAt: "2026-09-20T09:12:44Z", status: "saved", archived: true, allowedTransitions: ["tailoring", "withdrawn"] }, company: { listings: [] } },
    },
  });

  assert.equal(view.state, "tracked");
  assert.match(view.detail, /archiv/i);
  const unarchive = view.actions.find((a) => a.id === "unarchive");
  assert.equal(unarchive.resolution.kind, UNARCHIVE_EXISTING);
  assert.equal(unarchive.resolution.jobListingId, "acme");
});

// Story 37: don't mistake a connection problem for "not tracked".
// Story 38: a lookup problem never costs a capture.
test("a failed lookup says so plainly and still lets me save", () => {
  const view = model({ lookup: { state: "failed", error: "Could not reach Sumisura." } });

  assert.equal(view.state, "unreachable");
  assert.match(view.headline, /couldn't|could not/i);
  assert.ok(!/not saved|not tracked/i.test(view.headline), "an unreachable backend must not read as 'not tracked'");
  assert.ok(actionKinds(view).includes("save"), "a lookup failure must not block a capture");
});

// Story 42: fixing a bad capture means editing a word, not retyping everything.
test("a capture that fails validation says what is wrong and pre-fills what it found", () => {
  const view = model({
    capture: { title: "", company: "LinkedIn", url: CAPTURE.url, description: "too short" },
    validation: ["missing title", 'company name looks wrong ("LinkedIn")', "description looks too short to be a real job posting"],
  });

  assert.equal(view.state, "invalid");
  assert.match(view.headline, /capture/i);
  assert.equal(view.problems.length, 3);
  assert.equal(view.fields.editable, true);
  assert.equal(view.fields.title, "");
  assert.equal(view.fields.company, "LinkedIn", "the field is pre-filled with what was found, wrong as it is");
  assert.ok(actionKinds(view).includes("save"), "fixing the fields and saving must stay possible");
});

// A description problem is the one thing the card cannot fix, since the
// card edits Title and Company only.
test("a capture whose description failed says the fields won't fix it", () => {
  const view = model({
    capture: { title: "Backend Engineer", company: "Acme", url: CAPTURE.url, description: "too short" },
    validation: ["description looks too short to be a real job posting"],
  });

  assert.equal(view.state, "invalid");
  assert.equal(view.fixableHere, false);
});

test("a capture whose only problems are Title and Company is fixable in the card", () => {
  const view = model({
    capture: { title: "", company: "Jobs", url: CAPTURE.url, description: "x".repeat(300) },
    validation: ["missing title", 'company name looks wrong ("Jobs")'],
  });

  assert.equal(view.fixableHere, true);
});

test("a save refused as a duplicate names the record that already holds the posting", () => {
  const view = model({
    outcome: {
      state: "refused",
      conflict: { reason: "duplicate-posting", message: "This posting is already saved as a Job Listing.", existing: { id: "acme", title: "Backend Engineer", company: "Acme", savedAt: "2026-09-14T08:30:00Z", archived: false } },
    },
  });

  assert.equal(view.state, "refused-duplicate");
  assert.match(view.headline, /already saved/i);
  assert.equal(view.link.href, SERVER + "/jobs/acme");
  assert.ok(!view.actions.some((a) => a.id === "unarchive"), "a live match has nothing to bring back");
});

test("a duplicate refusal naming an archived record offers to bring it back", () => {
  const view = model({
    outcome: {
      state: "refused",
      conflict: { reason: "duplicate-posting", message: "This posting is already saved as a Job Listing.", existing: { id: "acme", title: "Backend Engineer", company: "Acme", savedAt: "2026-09-14T08:30:00Z", archived: true } },
    },
  });

  const unarchive = view.actions.find((a) => a.id === "unarchive");
  assert.equal(unarchive.resolution.jobListingId, "acme");
});

// Story 24's backstop, seen from the card: the lookup failed or state moved
// underneath, so the decision arrives as a refusal instead.
test("a save refused for a decision shows the siblings and the same choices", () => {
  const view = model({
    lookup: { state: "failed" },
    outcome: {
      state: "refused",
      conflict: { reason: "company-has-listings", message: "You already track other roles at this company.", company: { listings: [{ id: "acme-2", title: "Platform Engineer", savedAt: "2026-09-18T11:02:10Z", status: "sent" }] } },
    },
  });

  assert.equal(view.state, "needs-decision");
  assert.equal(view.siblings.length, 1);
  assert.ok(actionKinds(view).includes("save-anyway"));
  assert.equal(view.siblings[0].replaceAction.resolution.kind, REPLACE);
  // Story 25: "actually, no" is a supported answer.
  assert.ok(actionKinds(view).includes("cancel"));
});

// Story 39: adding a Note or starting Tailoring is one click away.
test("a saved capture links straight to the record it created", () => {
  const view = model({
    outcome: { state: "saved", saved: { jobListing: { id: "acme", title: "Backend Engineer" }, application: { status: "saved" } } },
  });

  assert.equal(view.state, "saved");
  assert.equal(view.link.href, SERVER + "/jobs/acme");
  assert.match(view.headline, /saved/i);
});

test("a replace that saved but failed to archive says so rather than claiming success", () => {
  const view = model({
    outcome: {
      state: "saved",
      saved: { jobListing: { id: "acme", title: "Backend Engineer" }, application: { status: "saved" }, archivedJobListingId: "", archiveFailed: true },
    },
  });

  assert.equal(view.state, "saved");
  assert.match(view.detail, /could not|couldn't/i);
  assert.match(view.detail, /archiv/i);
});

test("a replace that archived what it replaced says which role that was", () => {
  const view = model({
    lookup: { state: "ready", result: { tracked: null, company: { listings: [{ id: "acme-2", title: "Platform Engineer", savedAt: "2026-09-18T11:02:10Z", status: "sent" }] } } },
    outcome: {
      state: "saved",
      saved: { jobListing: { id: "acme", title: "Backend Engineer" }, application: { status: "saved" }, archivedJobListingId: "acme-2" },
    },
  });

  assert.match(view.detail, /archiv/i);
  assert.match(view.detail, /Platform Engineer|acme-2/);
});

test("a save that failed outright shows the error and lets me try again", () => {
  const view = model({ outcome: { state: "error", error: "Could not reach Sumisura." } });

  assert.equal(view.state, "error");
  assert.match(view.headline, /could not reach/i);
  assert.ok(actionKinds(view).includes("save"));
});

test("while a save is in flight nothing is offered twice", () => {
  const view = model({ outcome: { state: "saving" } });

  assert.equal(view.state, "saving");
  assert.equal(view.busy, true);
  assert.deepEqual(actionKinds(view), []);
});

// The card is rendered on pages that are not postings at all; with no
// capture to act on it must not offer to save one.
test("with nothing captured on the page there is nothing to save", () => {
  const view = model({ capture: null, lookup: { state: "idle" } });

  assert.equal(view.state, "no-posting");
  assert.deepEqual(actionKinds(view), []);
});

test("a save that also completed a link shared from a phone says so", () => {
  const view = model({
    outcome: {
      state: "saved",
      saved: { jobListing: { id: "acme" }, application: { status: "saved" }, completedPendingCaptureId: "linkedin-abc123" },
    },
  });

  assert.match(view.detail, /shared from your phone/i);
});

// Acting from the card (issue #206, stories 43-46).
test("each Status move carries what the request needs and nothing the card invents", () => {
  const view = model({
    lookup: {
      state: "ready",
      result: {
        tracked: { id: "acme", title: "Backend Engineer", savedAt: "2026-09-20T09:12:44Z", status: "sent", archived: false, allowedTransitions: ["interviewing", "rejected", "withdrawn"] },
        company: { listings: [] },
      },
    },
  });

  assert.deepEqual(
    view.statusMoves.map((m) => m.status),
    ["interviewing", "rejected", "withdrawn"],
  );
  assert.equal(view.statusMoves[0].applicationId, "acme");
  // The label is an imperative, since the user is doing the move.
  assert.equal(typeof view.statusMoves[0].label, "string");
  assert.ok(view.statusMoves[0].label.length > 0);
});

test("a Status move in flight shows as busy without offering the move twice", () => {
  const view = model({
    lookup: { state: "ready", result: { tracked: { id: "acme", title: "T", savedAt: "2026-09-20T09:12:44Z", status: "saved", archived: false, allowedTransitions: ["tailoring"] }, company: { listings: [] } } },
    outcome: { state: "moving", status: "tailoring" },
  });

  assert.equal(view.busy, true);
  assert.deepEqual(view.statusMoves, []);
});

// Story 46: the change has visibly taken.
test("after a Status move the card reports the Status it moved to", () => {
  const view = model({
    lookup: { state: "ready", result: { tracked: { id: "acme", title: "T", savedAt: "2026-09-20T09:12:44Z", status: "saved", archived: false, allowedTransitions: ["tailoring"] }, company: { listings: [] } } },
    outcome: { state: "moved", status: "tailoring" },
  });

  assert.equal(view.state, "tracked");
  assert.match(view.detail, /tailoring/i);
});

// Story 45 from the card's side: the backend refused, so the card says so
// rather than pretending the move took.
test("a refused Status move says so and leaves the old Status on screen", () => {
  const view = model({
    lookup: { state: "ready", result: { tracked: { id: "acme", title: "T", savedAt: "2026-09-20T09:12:44Z", status: "saved", archived: false, allowedTransitions: ["tailoring"] }, company: { listings: [] } } },
    outcome: { state: "move-failed", error: "cannot move from \"saved\" to \"sent\"" },
  });

  assert.equal(view.state, "tracked");
  assert.match(view.detail, /saved/i);
  assert.match(view.problems.join(" "), /cannot move/i);
  assert.deepEqual(view.statusMoves.map((m) => m.status), ["tailoring"]);
});
