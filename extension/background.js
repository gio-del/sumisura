// Service worker: the only part of this extension that talks to the
// network. Fetching from here (instead of the content script) means the
// request carries the extension's host_permissions grant rather than being
// subject to the job board page's CORS policy. It only ever runs in
// response to a SUMISURA_CAPTURE message, which the content scripts only
// send when the user clicks the capture button (story 6: no background
// polling, no unsolicited requests).
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

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (!message || message.type !== "SUMISURA_CAPTURE") {
    return false;
  }

  SumisuraSettings.loadSettings(chrome.storage.local)
    .then(({ serverUrl, token }) =>
      fetch(SumisuraSettings.captureUrl(serverUrl), {
        method: "POST",
        headers: SumisuraSettings.requestHeaders(token, true),
        body: JSON.stringify(message.payload),
      }),
    )
    .then(async (res) => {
      if (!res.ok) {
        const body = await res.text().catch(() => "");
        sendResponse({ ok: false, error: SumisuraSettings.captureErrorMessage(res.status, body) });
        return;
      }
      const body = await res.json().catch(() => ({}));
      sendResponse({ ok: true, completedPendingCapture: Boolean(body && body.completedPendingCaptureId) });
    })
    .catch((err) => {
      console.error("[Sumisura] capture request failed", err);
      sendResponse({ ok: false, error: "Could not reach Sumisura. Is it running, and is the address in the extension's options right?" });
    });

  return true; // keep the message channel open for the async response above
});
