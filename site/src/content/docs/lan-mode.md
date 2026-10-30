---
title: LAN-reachable mode
description: An opt-in way to reach your Sumisura from another device on a network you trust — and the trade-offs that come with it.
---

By default Sumisura binds to `127.0.0.1` and has no authentication. That is the
design (nothing but your own machine can reach it), not an oversight.

LAN mode is an explicit exception, for checking your applications from a phone
on your own home network.

## Turning it on

In `.env`:

```sh
BIND_ADDR=0.0.0.0
LAN_AUTH_TOKEN=<a long random string>
```

Then start with the `lan` profile:

```sh
docker compose --profile lan up
```

Every `/api/*` request now needs access, and there are two ways to give it:

- **In a browser**, open the app and you'll get an **Enter access token**
  screen. Enter the token once on that device. Sumisura keeps an HttpOnly cookie
  derived from the token, so you stay signed in for about a year, and PDFs and
  company logos load as usual. **Forget this device** in the navigation bar
  signs that device out.
- **From other clients** (scripts, an iOS Shortcut), send the token in a
  header:

  ```
  X-Sumisura-Token: <your token>
  ```

Requests with neither get `401`. Changing `LAN_AUTH_TOKEN` and restarting signs
every device out. Leaving it unset skips the check entirely, so a plain
`docker compose up` behaves exactly as before.

## Signing in with a username and password

A token is awkward to type on a phone and is not what a password manager
expects. Set a username and password next to the token:

```sh
LAN_AUTH_USER=<your name>
LAN_AUTH_PASSWORD=<a long random string>
```

Browsers then get a **Sign in** screen with a Username and a Password field
instead of the token screen, marked up so your password manager offers to save
and fill it. The token does not go away: the extension, the `tailor-cv` skill
and an iOS Shortcut keep sending it in the header, and `POST /api/auth/session`
still accepts it. Changing the password and restarting signs every browser out
without touching those clients. Set both variables or neither; one alone leaves
token sign-in as it was.

## Failed attempts are limited

Sumisura counts failed sign-ins per source address, in memory. After 5 wrong
attempts within 10 minutes (a wrong token or password on
`POST /api/auth/session`, or a wrong `X-Sumisura-Token` header), that address
gets `429 Too Many Requests` with a `Retry-After` header, even if the next
attempt carries the right credentials. The wait starts at 30 seconds and doubles
with each further failure, up to 15 minutes. A successful sign-in clears the
count, other addresses are not affected, and requests that already carry a valid
access cookie are neither slowed nor counted, nor is `GET /api/auth/status`.
Restarting the backend forgets the counts. The limits are fixed, not
configurable.

Behind a local reverse proxy such as `tailscale serve`, every request reaches
Sumisura from the proxy's address, so all devices share one count.

## Know what you are turning on

:::caution
This is a single static shared secret, not a login system, and there is **no
TLS**. The token travels in plaintext across your network. Turn this on only on
a network you control and trust, and treat it as "my phone can reach my laptop",
not "this is exposed safely".
:::

To reach Sumisura from outside your home network, or over HTTPS, don't widen LAN
mode: follow [Remote access](./remote-access.md) instead. It keeps the app on
`127.0.0.1` behind Tailscale.
