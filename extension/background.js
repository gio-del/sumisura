// Service worker: the only part of this extension that talks to the
// network. Fetching from here (instead of the content script) means the
// request carries the extension's host_permissions grant rather than being
// subject to the job board page's CORS policy.
//
// It runs only in response to a message from a content script, and the
// content scripts only send one about a posting the user has open
// (ADR-0043): a capture the user asked for, or the read-only lookup that
// tells the card what it already knows about that posting. Nothing is on a
// timer, and nothing is sent about a page the user is not looking at.
//
// Where captures go, and the access token sent with them, come from the
// options page (settings.js, issue #195). The default server is
// 127.0.0.1, not localhost: some Firefox setups (DNS-over-HTTPS enabled)
// fail to resolve "localhost" for an extension's fetch() even though it
// resolves fine for regular page navigation.

// Chrome runs this as a service worker, which loads settings.js here;
// Firefox lists settings.js ahead of this file in background.scripts.
if (typeof SumisuraSettings === "undefined" && typeof importScripts === "function") {
  importScripts("settings.js");
}

const UNREACHABLE = "Could not reach Sumisura. Is it running, and is the address in the extension's options right?";

// sendJSON sends one request with the stored connection settings, and
// hands back the response plus the origin the card builds deep links
// from.
async function sendJSON(method, toUrl, body) {
  const { serverUrl, token } = await SumisuraSettings.loadSettings(chrome.storage.local);
  const res = await fetch(toUrl(serverUrl), {
    method,
    headers: SumisuraSettings.requestHeaders(token, true),
    body: JSON.stringify(body),
  });
  return { res, serverUrl };
}

function postJSON(toUrl, body) {
  return sendJSON("POST", toUrl, body);
}

// conflictBody reads a 409 that carries a machine-readable reason — a
// duplicate posting, or the same-company question — so the card can act on
// it rather than showing a JSON blob. Any other body is not a conflict the
// card knows how to answer.
function conflictBody(text) {
  try {
    const parsed = JSON.parse(text);
    if (parsed && typeof parsed.reason === "string") return parsed;
  } catch {
    // Not JSON: an ordinary text error body.
  }
  return null;
}

async function handleCapture(payload) {
  const { res, serverUrl } = await postJSON(SumisuraSettings.captureUrl, payload);
  const text = await res.text().catch(() => "");

  if (res.status === 409) {
    const conflict = conflictBody(text);
    if (conflict) return { ok: false, refused: true, conflict, serverUrl };
  }
  if (!res.ok) {
    return { ok: false, error: SumisuraSettings.captureErrorMessage(res.status, text), serverUrl };
  }

  let saved = {};
  try {
    saved = JSON.parse(text);
  } catch {
    // A 2xx with an unreadable body still means the record was written;
    // the card just loses the deep link.
  }
  return { ok: true, saved, serverUrl };
}

async function handleLookup(payload) {
  const { res, serverUrl } = await postJSON(SumisuraSettings.lookupUrl, payload);
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    return { ok: false, error: SumisuraSettings.captureErrorMessage(res.status, text), serverUrl };
  }
  const result = await res.json().catch(() => null);
  if (!result) {
    return { ok: false, error: "Sumisura answered the lookup with something unreadable.", serverUrl };
  }
  return { ok: true, result, serverUrl };
}

// handleStatusMove moves an Application's Status from the card. The
// backend's own transition validation decides: the card offers only what
// the lookup called legal, and a refusal comes back as the reason rather
// than as a silent no-op (issue #206, stories 43-46).
async function handleStatusMove(payload) {
  const { res, serverUrl } = await sendJSON(
    "PATCH",
    (serverUrl) => SumisuraSettings.statusUrl(serverUrl, payload.applicationId),
    { status: payload.status },
  );
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    return { ok: false, error: SumisuraSettings.captureErrorMessage(res.status, text), serverUrl };
  }
  const application = await res.json().catch(() => null);
  return { ok: true, application: application || {}, serverUrl };
}

const HANDLERS = {
  SUMISURA_CAPTURE: handleCapture,
  SUMISURA_LOOKUP: handleLookup,
  SUMISURA_STATUS: handleStatusMove,
};

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  const handler = message && HANDLERS[message.type];
  if (!handler) {
    return false;
  }

  handler(message.payload)
    .then(sendResponse)
    .catch((err) => {
      console.error("[Sumisura] request failed", message.type, err);
      sendResponse({ ok: false, error: UNREACHABLE });
    });

  return true; // keep the message channel open for the async response above
});
