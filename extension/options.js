// Options page (issue #195): where captures go and the access token they
// carry. Everything that decides something lives in settings.js; this file
// only wires it to the form and to chrome.* APIs.

const form = document.getElementById("settings");
const serverInput = document.getElementById("server-url");
const tokenInput = document.getElementById("token");
const statusEl = document.getElementById("status");

function show(message, ok) {
  statusEl.textContent = message;
  statusEl.className = ok ? "" : "error";
  statusEl.hidden = false;
}

// readForm validates the address and, for a server other than the local
// default, asks for its host permission — which the browser only allows
// from a click, so this runs inside the button handlers.
async function readForm() {
  const serverUrl = SumisuraSettings.normalizeServerUrl(serverInput.value);
  if (SumisuraSettings.needsHostPermission(serverUrl)) {
    const granted = await chrome.permissions.request({ origins: [serverUrl + "/*"] });
    if (!granted) throw new Error(`The extension needs permission to reach ${serverUrl}.`);
  }
  return { serverUrl, token: tokenInput.value.trim() };
}

async function init() {
  const { serverUrl, token } = await SumisuraSettings.loadSettings(chrome.storage.local);
  serverInput.value = serverUrl === SumisuraSettings.DEFAULT_SERVER_URL ? "" : serverUrl;
  tokenInput.value = token;
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    const settings = await readForm();
    await chrome.storage.local.set(settings);
    show("Saved.", true);
  } catch (err) {
    show(err.message, false);
  }
});

document.getElementById("test").addEventListener("click", async () => {
  try {
    const { serverUrl, token } = await readForm();
    show("Checking…", true);
    const res = await fetch(serverUrl + "/api/auth/status", { headers: SumisuraSettings.requestHeaders(token, false) });
    const body = await res.json().catch(() => null);
    const { ok, message } = SumisuraSettings.describeConnection(res.status, body);
    show(message, ok);
  } catch (err) {
    show(err instanceof TypeError ? "Could not reach that address. Is Sumisura running there?" : err.message, false);
  }
});

init();
