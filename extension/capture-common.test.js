const test = require("node:test");
const assert = require("node:assert/strict");
const { successMessage } = require("./capture-common.js");

test("successMessage: a plain capture", () => {
  assert.equal(successMessage({ ok: true }), "Saved to Sumisura.");
  assert.equal(successMessage({ ok: true, completedPendingCapture: false }), "Saved to Sumisura.");
});

test("successMessage: a capture that completed a link shared from a phone", () => {
  assert.equal(
    successMessage({ ok: true, completedPendingCapture: true }),
    "Saved to Sumisura — and completed the link you shared from your phone.",
  );
});
