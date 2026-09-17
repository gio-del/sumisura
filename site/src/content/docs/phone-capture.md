---
title: Capture from your phone
description: Send a job from the LinkedIn, Indeed or browser app on your phone to Sumisura with the share button.
---

On a computer, the [browser extension](./extension.md) captures the job you're
reading. On a phone you use the **Share** button instead. The job goes to your
Sumisura:

- **LinkedIn, Indeed and other links** wait in **To complete** until you add
  their description, or until you capture them with the extension on your
  computer, which completes them automatically.
- **Public Greenhouse, Lever and Ashby postings** are saved as Job Listings
  straight away.

See [Tracking](./tracking.md) for how To complete works.

You need:

- Sumisura reachable from your phone over **HTTPS**. Follow
  [Remote access](./remote-access.md), which also makes it work away from home.
- The access token (`LAN_AUTH_TOKEN`). You enter it once per device.

## Android: install Sumisura as an app

1. On your phone, open your Sumisura address (`https://….ts.net`) in **Chrome**
   and enter the access token.
2. Open Chrome's menu and choose **Add to Home screen**, then **Install**. On
   some devices it's called **Install app**.
3. In the LinkedIn app (or Indeed, or Chrome), open a job, tap **Share** and
   pick **Sumisura**. A page opens and tells you what happened: *Saved to To
   complete*, *Saved as a Job Listing*, *Already waiting in To complete* or
   *Already tracked as a Job Listing*.

Installing needs HTTPS. Over [LAN mode](./lan-mode.md)'s plain HTTP, Chrome
doesn't offer to install the app, so Sumisura never appears in the share sheet.

The installed app keeps no offline copy of your data. If the computer running
Sumisura can't be reached, it tells you so instead of showing stale records.

## iPhone: the "Save to Sumisura" Shortcut

Safari doesn't let web apps receive shares, so on iOS you use an Apple
**Shortcut** that shows up in the share sheet and sends the link to Sumisura.
Unlike the Android app, it works over plain-HTTP [LAN mode](./lan-mode.md) too,
but [Remote access](./remote-access.md) is still the recommended setup.

### Build it (about five minutes)

Open the **Shortcuts** app, tap **+**, name the shortcut **Save to Sumisura**,
and add these actions in order:

1. **Receive** input from **Share Sheet**. Tap the input types and keep only
   **URLs** and **Text**. Set *If there's no input* to **Stop and Respond**.
2. **Text**: your Sumisura address with no trailing slash, e.g.
   `https://sumisura-box.tail1234.ts.net`. Long-press the action's output and
   rename it **Server**.
3. **Text**: your access token (`LAN_AUTH_TOKEN`). Rename its output to
   **Token**.
4. **Get Contents of URL**:
   - URL: **Server**, followed by `/api/pending-captures`
   - Method: **POST**
   - Headers: add `X-Sumisura-Token` with the value **Token**
   - Request Body: **JSON**, with one Text field `text` set to **Shortcut
     Input**
5. **Get Dictionary Value**: get **Value** for key `message` in **Contents of
   URL**.
6. **If** **Dictionary Value** **has any value**:
   - **Show Notification** with **Dictionary Value**.
   - **Otherwise**: **Show Notification** with **Contents of URL**. This shows
     Sumisura's error text, e.g. a wrong token.
   - **End If**

Finally, open the shortcut's settings (the **ⓘ** button) and turn on **Show in
Share Sheet**.

### Use it

In the LinkedIn app, the Indeed app or Safari, open a job, tap **Share** and
pick **Save to Sumisura**. A notification tells you what happened: *Saved to To
complete.*, *Saved as a Job Listing.*, *Already waiting in To complete.* or
*Already tracked as a Job Listing.*

The first time, iOS asks whether the shortcut may connect to your Sumisura
address. Choose **Always Allow**.

### What it sends

One request to your own Sumisura: whatever the share sheet handed over (a URL,
or text with a URL in it) as `{"text": "…"}`, with the token in the
`X-Sumisura-Token` header. Nothing goes anywhere else. The token lives inside
the shortcut on your phone, so don't share your copy of the shortcut with
anyone. Rebuild it or use the import link below instead.

### Or import it

**Import link: not published yet.** Once it's available, importing asks for your
Sumisura address and your token. The shared link itself contains neither.

You can also add Sumisura to your home screen from Safari: open your address,
tap **Share → Add to Home Screen**.
