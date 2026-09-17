---
title: Remote access
description: Reach your Sumisura from your phone, at home or away, over HTTPS, without opening a port to the internet.
---

By default Sumisura only answers on `127.0.0.1`. [LAN mode](./lan-mode.md) opens
it to the devices on your home network over plain HTTP. This page covers the
next step: using the app from your phone **anywhere**, over **HTTPS**, while it
keeps running on a computer at home.

The recommended setup is [Tailscale](https://tailscale.com) with
`tailscale serve`:

- The app **stays bound to `127.0.0.1`**. Tailscale runs on the same machine and
  forwards to it.
- Only devices signed in to **your own tailnet** can reach it. Nothing is
  reachable from the public internet, and nothing listens on your LAN.
- You get a real HTTPS address like `https://sumisura-box.tail1234.ts.net`, with
  a certificate Tailscale manages for you. Installing Sumisura as an app on a
  phone requires HTTPS.
- **No router configuration**: no port forwarding, no dynamic DNS. It works even
  if your connection is behind CGNAT.

Tailscale's Personal plan is free for non-commercial use. Check
[tailscale.com/pricing](https://tailscale.com/pricing) for the current limits.

:::caution[Don't expose Sumisura to the internet]
Don't forward a router port to Sumisura, don't put it behind a public reverse
proxy, and don't use `tailscale funnel` (which publishes a service to the whole
internet). The access token is a single shared secret, not a login system, and
the app holds your whole career history and your Anthropic API key's spending
power.
:::

## 1. Run the published image

Remote access is meant for the [published image](./quickstart.md), which serves
the whole app on one port and restarts on its own (`restart: unless-stopped`).
The development compose file (Vite on port 5173) isn't meant for this.

In `.env`, **leave `BIND_ADDR` unset** and set an access token:

```sh
# BIND_ADDR stays unset: the app keeps listening on 127.0.0.1 only.
LAN_AUTH_TOKEN=<a long random string, e.g. from `openssl rand -hex 32`>
```

Then start it:

```sh
docker compose -f docker-compose.release.yml up -d
```

With a token set, the browser extension on this computer needs it too: enter it
in the extension's options (see [Browser extension](./extension.md#address-and-access-token)).
The `tailor-cv` skill picks it up from `.env` on its own.

The token is defence in depth. Only your own devices can reach the app, but a
tailnet can grow over time (a shared node, a family member's laptop), and the
token keeps every one of them out unless you've given it to them. You enter it
once per device, and the browser keeps an access cookie. See
[LAN mode](./lan-mode.md) for how it works.

## 2. Set up Tailscale

1. [Create a tailnet](https://login.tailscale.com/start) and, in the admin
   console under **DNS**, make sure **MagicDNS** is on and **HTTPS
   Certificates** are enabled.
2. [Install Tailscale](https://tailscale.com/download) on the computer running
   Sumisura and sign in (`sudo tailscale up` on Linux).
3. Install the Tailscale app on your phone and sign in to the same tailnet.

## 3. Serve Sumisura over HTTPS

On the computer running Sumisura:

```sh
sudo tailscale serve --bg --https=443 http://127.0.0.1:8080
```

`--bg` makes this persistent: it survives reboots and `tailscale down`/`up`.
Check it with `tailscale serve status`. It prints the address to open, for
example `https://sumisura-box.tail1234.ts.net`. To stop serving, run
`sudo tailscale serve --https=443 off`.

`tailscale serve` terminates HTTPS and tells Sumisura so in
`X-Forwarded-Proto: https`. Sumisura then marks the access cookie `Secure`.

## 4. Open it from your phone

With the Tailscale app connected, open the `https://….ts.net` address on your
phone, enter the access token once, and you're in. Try it with Wi-Fi off to
confirm it works away from home.

## Leaving a computer on as a server

If Sumisura lives on a computer you leave on, check these once so it keeps
working after a power cut or a reboot, weeks later:

- **Docker starts at boot**: `sudo systemctl enable docker`. The container
  itself comes back on its own thanks to `restart: unless-stopped`.
- **Tailscale starts at boot**: `sudo systemctl enable tailscaled`. Your
  `serve --bg` configuration is restored with it.
- **The machine never suspends.** On GNOME, set *Settings → Power → Automatic
  Suspend* to off. On any systemd machine, to be sure:
  `sudo systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target`.
- **Disable key expiry** for this machine in the Tailscale admin console
  (*Machines → … → Disable key expiry*). Otherwise it drops off the tailnet when
  its key expires, by default after 180 days, until someone signs in again.
- **Back up `data/`.** It's your Master Data and your tracking history, and
  Sumisura doesn't back it up for you.

After a reboot, check with `docker ps` and `tailscale serve status`, then open
the address from your phone on mobile data.

## Alternatives

- **[Headscale](https://github.com/juanfont/headscale)** is an open-source,
  self-hosted Tailscale coordination server that works with the regular Tailscale
  apps. Use it if you don't want to depend on Tailscale's hosted service. You
  then run and expose the coordination server yourself.
- **Plain WireGuard** works too. You manage keys, a reachable endpoint (which
  usually means a port forward for WireGuard itself, not for Sumisura) and your
  own HTTPS certificate.

Whichever you choose, the rule is the same: Sumisura stays on `127.0.0.1` or a
private network, and the private network is the only way in.
