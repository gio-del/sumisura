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

## iPhone

Safari doesn't let web apps receive shares, so on iOS you share through a
Shortcut instead (coming separately). You can still open your Sumisura address
in Safari and use **Share → Add to Home Screen** to put the app on your home
screen.
