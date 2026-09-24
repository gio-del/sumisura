// Minimal test harness for the pure `validateCapture` function (issue #58).
// No test runner/config exists yet under extension/ — Node's built-in
// `node:test` needs no dependency or build step, matching the extension's
// "no toolchain" posture, so it's used as-is: `node --test extension/`.
//
// Fixture payloads below mimic the shape captureJobPosting() in content.js
// actually produces (title/company/location/url/description/logoUrl/
// listingSalaryText), covering both a well-formed capture and the broken
// scenarios called out in the PRD (missing title, stale-nav "LinkedIn" as
// company, a too-short description, everything broken at once).

const test = require("node:test");
const assert = require("node:assert/strict");
const { validateCapture } = require("./validate-capture.js");

const REALISTIC_DESCRIPTION =
  "We are looking for a Senior Machine Learning Engineer to join our platform team. ".repeat(4);

function wellFormedPayload(overrides) {
  return Object.assign(
    {
      title: "Senior Machine Learning Engineer",
      company: "Prima",
      location: "",
      url: "https://www.linkedin.com/jobs/view/12345/",
      description: REALISTIC_DESCRIPTION,
      logoUrl: "",
      listingSalaryText: "",
    },
    overrides
  );
}

test("well-formed payload has no problems", () => {
  const problems = validateCapture(wellFormedPayload());
  assert.deepEqual(problems, []);
});

test("missing title is reported", () => {
  const problems = validateCapture(wellFormedPayload({ title: "" }));
  assert.equal(problems.length, 1);
  assert.match(problems[0], /title/i);
});

test("whitespace-only title is treated as missing", () => {
  const problems = validateCapture(wellFormedPayload({ title: "   " }));
  assert.match(problems.join(", "), /missing title/i);
});

test("stale-nav company ('LinkedIn') is rejected", () => {
  const problems = validateCapture(wellFormedPayload({ company: "LinkedIn" }));
  assert.equal(problems.length, 1);
  assert.match(problems[0], /company/i);
  assert.match(problems[0], /LinkedIn/);
});

test("generic company blocklist is case-insensitive", () => {
  const problems = validateCapture(wellFormedPayload({ company: "jobs" }));
  assert.match(problems.join(", "), /company/i);
});

test("too-short company name is rejected", () => {
  const problems = validateCapture(wellFormedPayload({ company: "X" }));
  assert.match(problems.join(", "), /company/i);
});

test("missing company is reported distinctly from a bad company", () => {
  const problems = validateCapture(wellFormedPayload({ company: "" }));
  assert.deepEqual(problems, ["missing company"]);
});

test("20-character description is rejected as too short", () => {
  const problems = validateCapture(wellFormedPayload({ description: "Short job description" }));
  assert.equal(problems.length, 1);
  assert.match(problems[0], /description/i);
  assert.match(problems[0], /short/i);
});

test("missing description is reported distinctly from a too-short one", () => {
  const problems = validateCapture(wellFormedPayload({ description: "" }));
  assert.deepEqual(problems, ["missing description"]);
});

test("everything broken at once reports one problem per field", () => {
  const problems = validateCapture({
    title: "",
    company: "Home",
    location: "",
    url: "https://www.linkedin.com/jobs/search-results/",
    description: "too short",
    logoUrl: "",
    listingSalaryText: "",
  });
  assert.equal(problems.length, 3);
});

test("location/logoUrl/listingSalaryText are never validated", () => {
  const problems = validateCapture(
    wellFormedPayload({ location: "", logoUrl: "", listingSalaryText: "" })
  );
  assert.deepEqual(problems, []);
});

test("validateCapture: a still-loading pane reports one actionable problem, not three", () => {
  const problems = validateCapture({
    title: "",
    company: "",
    description: "",
    paneReady: false,
  });

  assert.deepEqual(problems, ["the posting is still loading — capture again in a moment"]);
});

test("validateCapture: a ready pane is judged on its fields as before", () => {
  const problems = validateCapture({
    title: "",
    company: "",
    description: "",
    paneReady: true,
  });

  assert.deepEqual(problems, ["missing title", "missing company", "missing description"]);
});

// LinkedIn virtualises the description: it is mounted only while scrolled
// into view, so it is genuinely absent after you scroll the posting past it
// — which is exactly when someone who just read the posting would capture.
// Reporting that as "missing description" sends the reader hunting for a
// capture bug that isn't there.
test("validateCapture: an unmounted description names its own remedy", () => {
  const problems = validateCapture({
    title: "Data Platform Engineer",
    company: "MC Engineering",
    description: "",
    paneReady: true,
    descriptionRendered: false,
  });

  assert.deepEqual(problems, [
    "the posting's description isn't on the page right now — scroll back up to it and capture again",
  ]);
});

test("validateCapture: a still-loading pane outranks an unmounted description", () => {
  const problems = validateCapture({
    title: "",
    company: "",
    description: "",
    paneReady: false,
    descriptionRendered: false,
  });

  assert.deepEqual(problems, ["the posting is still loading — capture again in a moment"]);
});

test("validateCapture: a rendered-but-short description is still judged on length", () => {
  const problems = validateCapture({
    title: "Data Platform Engineer",
    company: "MC Engineering",
    description: "Too short.",
    paneReady: true,
    descriptionRendered: true,
  });

  assert.deepEqual(problems, ["description looks too short to be a real job posting"]);
});
