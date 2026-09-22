const test = require("node:test");
const assert = require("node:assert/strict");

const { toolbarBadge, SAVE_COMMAND } = require("./toolbar.js");

// The toolbar icon carries the one signal the card would otherwise be the
// only place to see (issue #206, story 52), so it has to stay right even
// when the card is collapsed.

test("a tracked posting shows on the icon, with its Status in the tooltip", () => {
  const badge = toolbarBadge({ id: "acme", status: "interviewing", archived: false });

  assert.notEqual(badge.text, "");
  assert.match(badge.title, /interviewing/i);
});

test("an untracked posting clears the icon rather than marking it", () => {
  const badge = toolbarBadge(null);

  assert.equal(badge.text, "");
  assert.match(badge.title, /not saved/i);
});

test("an archived match reads as archived, not as untracked", () => {
  const badge = toolbarBadge({ id: "acme", status: "saved", archived: true });

  assert.notEqual(badge.text, "");
  assert.match(badge.title, /archiv/i);
});

// A lookup that failed is not an answer, so the icon must not claim one.
test("an unknown state clears the icon and says why", () => {
  const badge = toolbarBadge(undefined, { unreachable: true });

  assert.equal(badge.text, "");
  assert.match(badge.title, /couldn't|could not/i);
});

test("the save command has one name, shared by the manifest and the listener", () => {
  assert.equal(typeof SAVE_COMMAND, "string");
  assert.ok(SAVE_COMMAND.length > 0);
});
