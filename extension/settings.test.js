const test = require("node:test");
const assert = require("node:assert/strict");
const settings = require("./settings.js");

test("normalizeServerUrl: blank means the local backend", () => {
  assert.equal(settings.normalizeServerUrl(""), "http://127.0.0.1:8080");
  assert.equal(settings.normalizeServerUrl("   "), "http://127.0.0.1:8080");
});

test("normalizeServerUrl: keeps only the origin, adding http:// to a bare host", () => {
  assert.equal(settings.normalizeServerUrl("https://sumisura-box.tail1234.ts.net/jobs"), "https://sumisura-box.tail1234.ts.net");
  assert.equal(settings.normalizeServerUrl("192.168.1.20:8080"), "http://192.168.1.20:8080");
  assert.equal(settings.normalizeServerUrl(" http://127.0.0.1:8080/ "), "http://127.0.0.1:8080");
});

test("normalizeServerUrl: rejects non-http addresses", () => {
  assert.throws(() => settings.normalizeServerUrl("ftp://example.com"), /http/);
  assert.throws(() => settings.normalizeServerUrl("http://"), /address/);
});

test("needsHostPermission: only for servers the manifest doesn't already grant", () => {
  assert.equal(settings.needsHostPermission("http://127.0.0.1:8080"), false);
  assert.equal(settings.needsHostPermission("http://localhost:8080"), false);
  assert.equal(settings.needsHostPermission("https://sumisura-box.tail1234.ts.net"), true);
});

test("requestHeaders: the token header only when a token is set", () => {
  assert.deepEqual(settings.requestHeaders("", true), { "Content-Type": "application/json" });
  assert.deepEqual(settings.requestHeaders("s3cret", true), { "Content-Type": "application/json", "X-Sumisura-Token": "s3cret" });
  assert.deepEqual(settings.requestHeaders("s3cret", false), { "X-Sumisura-Token": "s3cret" });
});

test("captureErrorMessage: a 401 points at the options page", () => {
  assert.match(settings.captureErrorMessage(401, "missing or invalid LAN auth token"), /extension's options/);
  assert.equal(settings.captureErrorMessage(400, "validation failed: company is required\n"), "validation failed: company is required");
  assert.equal(settings.captureErrorMessage(500, ""), "Request failed (500)");
});

test("describeConnection: each auth status the backend can report", () => {
  assert.deepEqual(settings.describeConnection(200, { required: false, authenticated: true }).ok, true);
  assert.deepEqual(settings.describeConnection(200, { required: true, authenticated: true }).ok, true);
  const wrongToken = settings.describeConnection(200, { required: true, authenticated: false });
  assert.equal(wrongToken.ok, false);
  assert.match(wrongToken.message, /token is missing or wrong/);
  assert.equal(settings.describeConnection(404, null).ok, false);
  assert.equal(settings.describeConnection(502, null).ok, false);
});

test("loadSettings: defaults, and a stored address that no longer parses falls back", async () => {
  const storage = (stored) => ({ get: async () => stored });
  assert.deepEqual(await settings.loadSettings(storage({})), { serverUrl: "http://127.0.0.1:8080", token: "" });
  assert.deepEqual(await settings.loadSettings(storage({ serverUrl: "https://box.ts.net", token: "t" })), { serverUrl: "https://box.ts.net", token: "t" });
  assert.deepEqual(await settings.loadSettings(storage({ serverUrl: "ftp://nope" })), { serverUrl: "http://127.0.0.1:8080", token: "" });
});
