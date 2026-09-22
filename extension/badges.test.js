const test = require("node:test");
const assert = require("node:assert/strict");
const { JSDOM } = require("jsdom");

const { createBadger, rowsToLookUp, BADGE_CLASS } = require("./badges.js");

// Badging a search-results page with what you already track (issue #206,
// stories 47-51). This reads and writes a third party's DOM, so the rule
// it is held to is: annotate when it can, and do nothing at all when it
// can't — never break the page.

const ROW_SELECTOR = 'a[href*="/jobs/view/"]';

function page(hrefs) {
  const rows = hrefs.map((href) => `<li><a href="${href}">A role</a></li>`).join("");
  const dom = new JSDOM(`<!doctype html><html><body><ul>${rows}</ul></body></html>`, {
    url: "https://www.linkedin.com/jobs/search/",
  });
  return dom.window.document;
}

function badger(doc, answer) {
  const sent = [];
  const badgerInstance = createBadger({
    doc,
    rowSelector: ROW_SELECTOR,
    send: (message, callback) => {
      sent.push(message);
      callback(typeof answer === "function" ? answer(message) : answer);
    },
  });
  return { badger: badgerInstance, sent };
}

function badgeTexts(doc) {
  return Array.from(doc.querySelectorAll("." + BADGE_CLASS)).map((el) => el.textContent);
}

function trackedResults(message, tracked) {
  return {
    ok: true,
    result: {
      results: message.payload.urls.map((url) => ({ url, tracked: tracked[url] || null })),
    },
  };
}

test("rowsToLookUp takes each row's own href, once, skipping what is already known", () => {
  const doc = page([
    "https://www.linkedin.com/jobs/view/1111111111/",
    "https://www.linkedin.com/jobs/view/2222222222/",
    "https://www.linkedin.com/jobs/view/1111111111/",
  ]);

  const all = rowsToLookUp(doc, ROW_SELECTOR, new Set(), 200);
  assert.deepEqual(all.urls, [
    "https://www.linkedin.com/jobs/view/1111111111/",
    "https://www.linkedin.com/jobs/view/2222222222/",
  ]);

  const known = new Set(["https://www.linkedin.com/jobs/view/1111111111/"]);
  assert.deepEqual(rowsToLookUp(doc, ROW_SELECTOR, known, 200).urls, [
    "https://www.linkedin.com/jobs/view/2222222222/",
  ]);
});

test("rowsToLookUp never asks about more rows than one request allows", () => {
  const hrefs = [];
  for (let i = 0; i < 250; i++) hrefs.push(`https://www.linkedin.com/jobs/view/10000000${i}/`);

  assert.equal(rowsToLookUp(page(hrefs), ROW_SELECTOR, new Set(), 200).urls.length, 200);
});

// Story 49: one request for the page, not one per row.
test("the whole visible page is checked in one request", () => {
  const doc = page([
    "https://www.linkedin.com/jobs/view/1111111111/",
    "https://www.linkedin.com/jobs/view/2222222222/",
    "https://www.linkedin.com/jobs/view/3333333333/",
  ]);
  const { badger, sent } = badger_(doc);

  badger.scan();

  assert.equal(sent.length, 1);
  assert.equal(sent[0].type, "SUMISURA_LOOKUP");
  assert.equal(sent[0].payload.urls.length, 3);
});

// Stories 47, 48: tracked rows are marked, and the badge says which Status.
test("a tracked row is badged with its Status and an untracked one is left alone", () => {
  const doc = page(["https://www.linkedin.com/jobs/view/1111111111/", "https://www.linkedin.com/jobs/view/2222222222/"]);
  const { badger } = badger_(doc, (message) =>
    trackedResults(message, { "https://www.linkedin.com/jobs/view/1111111111/": { id: "acme", status: "rejected" } }),
  );

  badger.scan();

  const badges = badgeTexts(doc);
  assert.equal(badges.length, 1, "only the tracked row is badged");
  assert.match(badges[0], /rejected/i, "the badge says which Status, so 'saved' and 'rejected' differ at a glance");
});

test("each Status reads differently on the badge", () => {
  const doc = page(["https://www.linkedin.com/jobs/view/1111111111/", "https://www.linkedin.com/jobs/view/2222222222/"]);
  const { badger } = badger_(doc, (message) =>
    trackedResults(message, {
      "https://www.linkedin.com/jobs/view/1111111111/": { id: "a", status: "saved" },
      "https://www.linkedin.com/jobs/view/2222222222/": { id: "b", status: "interviewing" },
    }),
  );

  badger.scan();

  const badges = badgeTexts(doc);
  assert.equal(badges.length, 2);
  assert.notEqual(badges[0], badges[1]);
});

// Story 50: the annotation doesn't stop halfway down.
test("rows that appear on scroll are badged too, and settled rows are not re-asked", () => {
  const doc = page(["https://www.linkedin.com/jobs/view/1111111111/"]);
  const { badger, sent } = badger_(doc, (message) =>
    trackedResults(
      message,
      message.payload.urls.reduce((acc, url) => Object.assign(acc, { [url]: { id: url, status: "saved" } }), {}),
    ),
  );

  badger.scan();
  assert.equal(badgeTexts(doc).length, 1);

  const li = doc.createElement("li");
  li.innerHTML = '<a href="https://www.linkedin.com/jobs/view/2222222222/">Another role</a>';
  doc.querySelector("ul").appendChild(li);
  badger.scan();

  assert.equal(badgeTexts(doc).length, 2, "the new row is badged");
  assert.equal(sent.length, 2);
  assert.deepEqual(sent[1].payload.urls, ["https://www.linkedin.com/jobs/view/2222222222/"], "only the new row is asked about");
});

test("scanning an already-badged page sends nothing", () => {
  const doc = page(["https://www.linkedin.com/jobs/view/1111111111/"]);
  const { badger, sent } = badger_(doc, (message) => trackedResults(message, {}));

  badger.scan();
  badger.scan();
  badger.scan();

  assert.equal(sent.length, 1);
});

test("a row is never badged twice", () => {
  const doc = page(["https://www.linkedin.com/jobs/view/1111111111/"]);
  const { badger } = badger_(doc, (message) =>
    trackedResults(message, { "https://www.linkedin.com/jobs/view/1111111111/": { id: "acme", status: "saved" } }),
  );

  badger.scan();
  badger.scan();

  assert.equal(badgeTexts(doc).length, 1);
});

// Story 51: a results page is never broken by Sumisura being down.
test("an unreachable backend leaves the page exactly as it was", () => {
  const doc = page(["https://www.linkedin.com/jobs/view/1111111111/"]);
  const before = doc.body.innerHTML;
  const { badger } = badger_(doc, { ok: false, error: "Could not reach Sumisura." });

  badger.scan();

  assert.equal(badgeTexts(doc).length, 0);
  assert.equal(doc.body.innerHTML, before, "nothing about the page changed");
});

test("an answer in a shape the badger doesn't understand changes nothing", () => {
  const doc = page(["https://www.linkedin.com/jobs/view/1111111111/"]);
  const before = doc.body.innerHTML;

  for (const answer of [null, {}, { ok: true }, { ok: true, result: {} }, { ok: true, result: { results: "nope" } }]) {
    const { badger } = badger_(doc, answer);
    badger.scan();
  }

  assert.equal(doc.body.innerHTML, before);
});

test("a page with no rows to badge asks nothing", () => {
  const doc = page([]);
  const { badger, sent } = badger_(doc, (message) => trackedResults(message, {}));

  badger.scan();

  assert.equal(sent.length, 0);
});

// badger_ is the harness; named with a trailing underscore so it does not
// shadow the createBadger result inside each test.
function badger_(doc, answer) {
  return badger(doc, answer);
}
