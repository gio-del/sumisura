// Service worker: the only part of this extension that talks to the
// network. Fetching from here (instead of the content script) means the
// request carries the extension's host_permissions grant rather than being
// subject to the LinkedIn page's CORS policy — no backend CORS headers
// needed. It only ever runs in response to a SUMISURA_CAPTURE message,
// which content.js only sends when the user clicks the capture button
// (story 6: no background polling, no unsolicited requests).

// 127.0.0.1, not localhost: some Firefox setups (DNS-over-HTTPS enabled)
// fail to resolve "localhost" for an extension's fetch() even though it
// resolves fine for regular page navigation.
const API_URL = "http://127.0.0.1:8080/api/job-listings/from-extension";

console.log("[Sumisura] background script loaded");

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  console.log("[Sumisura] background received message", message);
  if (!message || message.type !== "SUMISURA_CAPTURE") {
    return false;
  }

  fetch(API_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(message.payload),
  })
    .then(async (res) => {
      console.log("[Sumisura] fetch responded", res.status);
      if (!res.ok) {
        const body = await res.text().catch(() => "");
        sendResponse({ ok: false, error: body || `Request failed (${res.status})` });
        return;
      }
      const body = await res.json().catch(() => ({}));
      sendResponse({ ok: true, completedPendingCapture: Boolean(body && body.completedPendingCaptureId) });
    })
    .catch((err) => {
      console.error("[Sumisura] fetch threw", err);
      sendResponse({ ok: false, error: err.message || "Could not reach the local app — is it running?" });
    });

  return true; // keep the message channel open for the async response above
});
