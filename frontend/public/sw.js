// Sumisura's service worker exists for installability (issue #185): Chrome
// still looks for a fetch handler before it offers to install a web app,
// and only an installed app can receive shares. It deliberately caches
// nothing — this is a live personal tool, and a stale API response or app
// shell would be worse than an honest "you're offline".
const OFFLINE_PAGE = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sumisura — offline</title>
<style>body{font-family:system-ui,sans-serif;background:#F7F4EC;color:#1B2A4A;margin:0;padding:48px 24px;max-width:420px}
@media (prefers-color-scheme: dark){body{background:#132038;color:#F7F4EC}}</style></head>
<body><h1>Can't reach Sumisura</h1>
<p>Your device is offline, or the computer running Sumisura isn't reachable right now. If you use Tailscale, check that it's connected.</p>
<p><a href="" style="color:inherit">Try again</a></p></body></html>`;

self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (event) => event.waitUntil(self.clients.claim()));

self.addEventListener("fetch", (event) => {
  if (event.request.mode !== "navigate") return;
  event.respondWith(
    fetch(event.request).catch(
      () => new Response(OFFLINE_PAGE, { status: 503, headers: { "Content-Type": "text/html; charset=utf-8" } }),
    ),
  );
});
