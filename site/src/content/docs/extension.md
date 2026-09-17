---
title: Browser extension
description: Capture the LinkedIn or Indeed job you are looking at, straight into your local Sumisura.
---

The extension adds one button to a job posting. Pressing it sends that posting
to **your own local Sumisura** as a Job Listing — the same save path as pasting
it by hand.

It works on LinkedIn (`/jobs/*`) and Indeed (`/viewjob*`).

## Installing it

The extension is not in any store yet (that is planned for the hosted version).
Load it unpacked:

**Chrome / Chromium**

1. Download `sumisura-extension-vX.Y.Z.zip` from the
   [latest release](https://github.com/gio-del/sumisura/releases) and unzip it —
   or use the `extension/` directory of your checkout.
2. Open `chrome://extensions`, enable **Developer mode**.
3. **Load unpacked**, and select that directory.

**Firefox**

1. Open `about:debugging#/runtime/this-firefox`.
2. **Load Temporary Add-on…**, and select `manifest.json` inside the directory.
   Firefox drops temporary add-ons on restart.

## Completing links shared from your phone

A LinkedIn or Indeed job you shared from your phone waits in **To complete**
(see [Tracking](./tracking.md)). On your computer, open that link and capture
it with the extension. The capture saves the Job Listing and removes the link
from To complete in one go, and the button's message tells you it did. The
link doesn't need to match exactly: a posting opened from a search results
pane or another country's Indeed site is still recognised.

## What it sends, and where

It reads the job posting on the tab you are looking at — title, company,
location, and the description, converted to Markdown — and POSTs it to
`http://localhost:8080`. Nothing is sent anywhere else: there is no server
behind the extension, no analytics, and no account.

It only acts when you press the button. It does not read pages in the
background, and it holds no credentials.

## If capture fails

The button reports its own failures in place. The usual causes:

- Sumisura is not running (`docker compose up`).
- The posting is behind a login wall, or rendered in a layout the extractor does
  not recognise — paste the description into the app instead.
