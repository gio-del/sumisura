// Connection settings for the extension (issue #195): which Sumisura to
// send captures to, and the access token to send with them when that
// installation runs with LAN_AUTH_TOKEN set. Stored in chrome.storage.local
// on this browser only. Loaded by background.js (importScripts in Chrome's
// service worker, the background "scripts" list in Firefox) and by the
// options page; the pure functions are also required by settings.test.js.

(function (root) {
  const DEFAULT_SERVER_URL = "http://127.0.0.1:8080";
  // Origins manifest.json grants up front; any other server needs its
  // host permission requested from the options page.
  const BUILT_IN_ORIGINS = ["http://127.0.0.1:8080", "http://localhost:8080"];
  const TOKEN_HEADER = "X-Sumisura-Token";

  // normalizeServerUrl turns what the user typed into an origin: blank means
  // the default local backend, a bare host gets http://, and any path is
  // dropped (the API paths are fixed). Throws on anything that isn't an
  // http(s) address.
  function normalizeServerUrl(input) {
    const trimmed = String(input || "").trim();
    if (!trimmed) return DEFAULT_SERVER_URL;
    const withScheme = /^[a-z][a-z0-9+.-]*:\/\//i.test(trimmed) ? trimmed : "http://" + trimmed;
    let url;
    try {
      url = new URL(withScheme);
    } catch {
      throw new Error("That doesn't look like an address, e.g. http://127.0.0.1:8080");
    }
    if (url.protocol !== "http:" && url.protocol !== "https:") {
      throw new Error("The address must start with http:// or https://");
    }
    return url.origin;
  }

  function needsHostPermission(origin) {
    return !BUILT_IN_ORIGINS.includes(origin);
  }

  function captureUrl(serverUrl) {
    return serverUrl + "/api/job-listings/from-extension";
  }

  // lookupUrl is the read-only route the card asks before the user clicks
  // (issue #206, ADR-0043): is this posting tracked, and what else do I
  // track at this company.
  function lookupUrl(serverUrl) {
    return serverUrl + "/api/job-listings/capture-lookup";
  }

  // statusUrl is the Application's Status route, which the card calls to
  // move a Status from the job board (issue #206).
  function statusUrl(serverUrl, applicationId) {
    return serverUrl + "/api/applications/" + encodeURIComponent(applicationId) + "/status";
  }

  // jobListingUrl is the deep link into the app for one Job Listing. Built
  // from the stored serverUrl, which normalizeServerUrl has already
  // reduced to an origin.
  function jobListingUrl(serverUrl, id) {
    return serverUrl + "/jobs/" + encodeURIComponent(id);
  }

  function requestHeaders(token, withJsonBody) {
    const headers = {};
    if (withJsonBody) headers["Content-Type"] = "application/json";
    if (token) headers[TOKEN_HEADER] = token;
    return headers;
  }

  // captureErrorMessage is what the capture button says when the backend
  // refuses a capture. A 401 always means the access token: say where to
  // fix it rather than echoing the backend's generic text.
  function captureErrorMessage(status, body) {
    if (status === 401) {
      return "Sumisura asked for an access token. Set it in the extension's options.";
    }
    return (body && String(body).trim()) || `Request failed (${status})`;
  }

  // describeConnection turns GET /api/auth/status's answer into what the
  // options page's Test connection button shows. ok is false whenever a
  // capture would fail.
  function describeConnection(status, body) {
    if (status === 200 && body && typeof body.required === "boolean") {
      if (!body.required) return { ok: true, message: "Connected. This Sumisura doesn't require an access token." };
      if (body.authenticated) return { ok: true, message: "Connected, and the access token is correct." };
      return { ok: false, message: "Connected, but the access token is missing or wrong." };
    }
    if (status === 404) {
      return { ok: false, message: "Reached a server, but it doesn't answer like Sumisura (or runs an older version)." };
    }
    return { ok: false, message: `Unexpected answer from the server (${status}).` };
  }

  async function loadSettings(storage) {
    const stored = await storage.get(["serverUrl", "token"]);
    let serverUrl = DEFAULT_SERVER_URL;
    try {
      serverUrl = normalizeServerUrl(stored.serverUrl);
    } catch {
      // A stored value that no longer parses falls back to the default.
    }
    return { serverUrl, token: typeof stored.token === "string" ? stored.token : "" };
  }

  const SumisuraSettings = {
    DEFAULT_SERVER_URL,
    TOKEN_HEADER,
    normalizeServerUrl,
    needsHostPermission,
    captureUrl,
    lookupUrl,
    statusUrl,
    jobListingUrl,
    requestHeaders,
    captureErrorMessage,
    describeConnection,
    loadSettings,
  };

  root.SumisuraSettings = SumisuraSettings;
  if (typeof module !== "undefined" && module.exports) {
    module.exports = SumisuraSettings;
  }
})(typeof self !== "undefined" ? self : globalThis);
