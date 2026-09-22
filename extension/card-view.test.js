const test = require("node:test");
const assert = require("node:assert/strict");
const { JSDOM } = require("jsdom");

const { createCard, outcomeFrom, pillLabel } = require("./card-view.js");

// The card's *traffic*: when it asks the backend about a posting, and when
// it doesn't (issue #206, ADR-0043, stories 29-32). What the card draws is
// card-model.js's job and is tested there; this covers the part that can
// only be got wrong against a real document.

function harness(options) {
  const dom = new JSDOM("<!doctype html><html><body></body></html>");
  const doc = dom.window.document;
  // The card ships its own SumisuraCard global in the browser; in Node it
  // has to be put where card-view.js looks for it.
  global.SumisuraCard = require("./card-model.js");

  const sent = [];
  const posting = { url: "https://www.linkedin.com/jobs/view/4012345678/", company: "Acme" };

  const card = createCard({
    doc,
    capture: () => ({ title: "Backend Engineer", company: posting.company, url: posting.url, description: "x".repeat(300) }),
    postingUrl: () => posting.url,
    validate: () => [],
    send: (message, callback) => {
      sent.push(message);
      if (message.type === "SUMISURA_LOOKUP") {
        const answer = (options && options.lookupAnswer) || {
          ok: true,
          serverUrl: "http://127.0.0.1:8080",
          result: { tracked: null, company: { listings: [] } },
        };
        callback(answer);
      } else {
        callback((options && options.saveAnswer) || { ok: true, serverUrl: "http://127.0.0.1:8080", saved: { jobListing: { id: "acme" } } });
      }
    },
    storage: { get: () => Promise.resolve({}), set: () => Promise.resolve() },
  });

  return { card, sent, posting, doc };
}

function lookups(sent) {
  return sent.filter((m) => m.type === "SUMISURA_LOOKUP");
}

test("opening a posting asks about it once", () => {
  const { card, sent } = harness();

  card.refresh(true);

  assert.equal(lookups(sent).length, 1);
  assert.deepEqual(lookups(sent)[0].payload, {
    url: "https://www.linkedin.com/jobs/view/4012345678/",
    company: "Acme",
  });
});

// Story 29: browsing must not turn into a stream of requests. The card
// re-reads the page on a timer to notice a split-pane change, so this is
// the check that matters.
test("re-reading the same posting sends nothing further", () => {
  const { card, sent } = harness();
  card.refresh(true);

  for (let i = 0; i < 20; i++) card.refresh(false);

  assert.equal(lookups(sent).length, 1);
});

// Story 30: the card must never show the previous posting's information.
test("clicking a different result in the split pane asks about the new posting", () => {
  const { card, sent, posting } = harness();
  card.refresh(true);

  posting.url = "https://www.linkedin.com/jobs/view/4099999999/";
  card.refresh(false);

  assert.equal(lookups(sent).length, 2);
  assert.equal(lookups(sent)[1].payload.url, "https://www.linkedin.com/jobs/view/4099999999/");
});

// Story 32: clicking back and forth between results is free.
test("going back to a posting already looked at asks nothing again", () => {
  const { card, sent, posting } = harness();
  card.refresh(true);
  posting.url = "https://www.linkedin.com/jobs/view/4099999999/";
  card.refresh(false);

  posting.url = "https://www.linkedin.com/jobs/view/4012345678/";
  card.refresh(false);

  assert.equal(lookups(sent).length, 2, "the third view was answered from the cache");
  assert.equal(card.state.lookup.state, "ready");
});

// Story 31: no stale information is actable on while the new answer is in
// flight. The harness answers synchronously, so this drives the ordering
// directly rather than through a timer.
test("changing posting clears the previous answer before the new one arrives", () => {
  const seen = [];
  const dom = new JSDOM("<!doctype html><html><body></body></html>");
  global.SumisuraCard = require("./card-model.js");
  const posting = { url: "https://www.linkedin.com/jobs/view/4012345678/" };
  let release = null;

  const card = createCard({
    doc: dom.window.document,
    capture: () => ({ title: "T", company: "Acme", url: posting.url, description: "x".repeat(300) }),
    postingUrl: () => posting.url,
    validate: () => [],
    send: (message, callback) => {
      if (message.type === "SUMISURA_LOOKUP") {
        release = () => callback({ ok: true, result: { tracked: { id: "acme", status: "saved", savedAt: "2026-09-20T00:00:00Z", allowedTransitions: [] }, company: { listings: [] } } });
        seen.push(card.state.lookup.state);
      }
    },
    storage: { get: () => Promise.resolve({}), set: () => Promise.resolve() },
  });

  card.refresh(true);
  release();
  assert.equal(card.state.lookup.state, "ready");

  posting.url = "https://www.linkedin.com/jobs/view/4099999999/";
  card.refresh(false);

  assert.deepEqual(seen, ["loading", "loading"], "each lookup leaves with the card already cleared to loading");
});

// A lookup that lands after the user has moved on must not overwrite the
// posting now on screen.
test("a late answer for a posting the user has left is not drawn", () => {
  const dom = new JSDOM("<!doctype html><html><body></body></html>");
  global.SumisuraCard = require("./card-model.js");
  const posting = { url: "https://www.linkedin.com/jobs/view/4012345678/" };
  const pending = [];

  const card = createCard({
    doc: dom.window.document,
    capture: () => ({ title: "T", company: "Acme", url: posting.url, description: "x".repeat(300) }),
    postingUrl: () => posting.url,
    validate: () => [],
    send: (message, callback) => {
      if (message.type === "SUMISURA_LOOKUP") pending.push({ url: message.payload.url, callback });
    },
    storage: { get: () => Promise.resolve({}), set: () => Promise.resolve() },
  });

  card.refresh(true);
  posting.url = "https://www.linkedin.com/jobs/view/4099999999/";
  card.refresh(false);

  // The first posting's answer arrives now, after the move.
  pending[0].callback({ ok: true, result: { tracked: { id: "stale", status: "saved", savedAt: "2026-09-20T00:00:00Z", allowedTransitions: [] }, company: { listings: [] } } });

  assert.equal(card.state.lookup.state, "loading", "the card is still waiting for the posting actually on screen");
});

// Story 38: a lookup problem never costs a capture, and the failure is
// remembered so a broken backend isn't retried on every timer tick.
test("a failed lookup is recorded as failed, once", () => {
  const { card, sent } = harness({ lookupAnswer: { ok: false, error: "Could not reach Sumisura." } });

  card.refresh(true);
  card.refresh(false);
  card.refresh(false);

  assert.equal(lookups(sent).length, 1);
  assert.equal(card.state.lookup.state, "failed");
});

// A save changes what is tracked, so the cached answer for that posting is
// now a lie and must not be served to the next look.
test("saving drops the cached answer for that posting", () => {
  const { card, sent, posting } = harness();
  card.refresh(true);
  assert.equal(card.lookups.size, 1);

  card.save(null);
  assert.equal(card.state.outcome.state, "saved");
  assert.equal(card.lookups.size, 0);

  // Coming back to the same posting now asks again rather than repeating
  // the answer from before the save.
  posting.url = "https://www.linkedin.com/jobs/view/4099999999/";
  card.refresh(false);
  posting.url = "https://www.linkedin.com/jobs/view/4012345678/";
  card.refresh(false);
  assert.equal(lookups(sent).length, 3);
});

// A save carries the decision the card already collected, so the backend's
// same-company question is a backstop rather than a second round trip.
test("saving sends the resolution the card was given", () => {
  const { card, sent } = harness();
  card.refresh(true);

  card.save({ kind: "replace", jobListingId: "acme-2" });

  const save = sent.filter((m) => m.type === "SUMISURA_CAPTURE").pop();
  assert.deepEqual(save.payload.resolution, { kind: "replace", jobListingId: "acme-2" });
});

test("the card mounts into a shadow root so the board's CSS can't reach it", () => {
  const { card, doc } = harness();
  card.mount();
  card.refresh(true);
  card.render();

  const host = doc.getElementById("sumisura-card");
  assert.ok(host, "the card mounts a host element");
  assert.ok(host.shadowRoot, "the card's content lives in a shadow root");
  assert.equal(host.children.length, 0, "nothing leaks into the page's own tree");
});

test("outcomeFrom keeps a refusal the card can answer apart from an error it can't", () => {
  const conflict = { reason: "duplicate-posting", message: "Already saved." };

  assert.deepEqual(outcomeFrom({ ok: false, refused: true, conflict }), { state: "refused", conflict });
  assert.deepEqual(outcomeFrom({ ok: false, error: "boom" }), { state: "error", error: "boom" });
  assert.equal(outcomeFrom({ ok: true, saved: { jobListing: { id: "acme" } } }).state, "saved");
  assert.equal(outcomeFrom(null).state, "error");
});

// Story 35: collapsing is remembered, so it isn't done again on every page.
test("the collapsed pill still carries the answer the card gave", () => {
  assert.match(pillLabel({ state: "tracked" }), /in sumisura/i);
  assert.match(pillLabel({ state: "saved" }), /saved/i);
  assert.match(pillLabel({ state: "untracked" }), /sumisura/i);
});

// Moving a Status from the card (issue #206, stories 43-46).
test("a Status move sends the Application and the target Status, and nothing else", () => {
  const { card, sent } = harness({ saveAnswer: { ok: true, application: { status: "tailoring" } } });
  card.refresh(true);

  card.moveStatus({ applicationId: "acme", status: "tailoring" });

  const move = sent.filter((m) => m.type === "SUMISURA_STATUS").pop();
  assert.deepEqual(move.payload, { applicationId: "acme", status: "tailoring" });
});

test("a Status move drops the cached answer, which no longer reflects the record", () => {
  const { card } = harness({ saveAnswer: { ok: true, application: { status: "tailoring" } } });
  card.refresh(true);
  assert.equal(card.lookups.size, 1);

  card.moveStatus({ applicationId: "acme", status: "tailoring" });

  assert.equal(card.lookups.size, 0);
  assert.equal(card.state.outcome.state, "moved");
  assert.equal(card.state.outcome.status, "tailoring");
});

test("a refused Status move is recorded as refused, not as having taken", () => {
  const { card } = harness({ saveAnswer: { ok: false, error: 'cannot move from "saved" to "sent"' } });
  card.refresh(true);

  card.moveStatus({ applicationId: "acme", status: "sent" });

  assert.equal(card.state.outcome.state, "move-failed");
  assert.match(card.state.outcome.error, /cannot move/);
});
