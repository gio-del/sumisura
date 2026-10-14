# ADR-0046: Browsers may sign in with a username and password; clients keep the token

- Status: accepted
- Date: 2026-10-14
- Relates to: [ADR-0004](0004-standalone-web-app.md), [ADR-0036](0036-renamed-to-sumisura.md), [ADR-0039](0039-backend-serves-the-production-frontend.md)

## Context

Token mode (issue #57, cookie exchange in #181) has one secret for everything: the browser's sign-in screen, the extension, the `tailor-cv` skill and an iOS Shortcut all use `LAN_AUTH_TOKEN`. A person signing in on a phone has to paste a long random string, and a password manager does not treat a lone "Access token" field as a login it can save and fill. An install that lives on an always-on home server, reached from several devices, wants a normal sign-in instead.

Sumisura stays single-tenant and free of accounts (ADR-0036): this is not a user system.

## Decision

Two optional variables, `LAN_AUTH_USER` and `LAN_AUTH_PASSWORD`, set **together and alongside** `LAN_AUTH_TOKEN`:

- **Browsers** get a Username + Password form (`autocomplete="username"` / `"current-password"`), and `POST /api/auth/session` accepts `{"username", "password"}` for the same HttpOnly access cookie. `GET /api/auth/status` reports `passwordLogin` so the frontend picks the form.
- **Clients keep the token.** The header path is unchanged, and the session route still takes `{"token"}`. The password is for people; the token is for programs. Rotating one never forces the other to change.
- **The cookie is keyed by all three** (token, username, password) when password sign-in is on, so changing the password signs every browser out, as changing the token always has. With token-only sign-in the cookie value is unchanged, so no existing device is signed out by upgrading.
- **One credential, compared in constant time**, both fields always compared. No password hashing: the value lives in the same environment as the token, which is just as sensitive, and a hash would add a dependency without protecting anything the environment does not already expose.
- **Both or neither.** One alone leaves token sign-in as it was (the server logs it), rather than accepting a username against an empty password.

## Consequences

- A home-server install can keep the sign-in in a password manager (Vaultwarden/Bitwarden) and rotate it without re-configuring the extension or the skill.
- Still a single static credential, not a login system: no lockout, no per-device revocation beyond "rotate and every browser signs in again". The tradeoffs on the LAN-reachable mode page still apply.
