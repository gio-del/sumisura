# Palette

Sumisura's identity is sartorial: a tailor's tape measure (yellow) marked in ink
(navy) against tailor's chalk (off-white). Three colours plus two surface shades
— deliberately small, not a design-token system. `frontend/src/index.css` reads
its shadcn tokens from here; so does `site/`.

## Core

| Role | Hex | Where it is used |
|---|---|---|
| Primary / ink | `#1B2A4A` (navy) | Body text and headings on light surfaces, primary buttons, the logo's tick marks, the dark-mode background. |
| Accent | `#F2B705` (tape yellow) | The tape in the logo, and anything that should read as *selected* / *tailored* in the UI (the role amber played in the old palette). **Fill and highlight only — never text on a light surface.** |
| Surface — light | `#F7F4EC` (chalk) | Page background in light mode. Warmer than white; it is what makes the brand read as atelier rather than dashboard. |

## Supporting shades

| Role | Hex | Notes |
|---|---|---|
| Surface — dark | `#132038` | Page background in dark mode (one step deeper than the ink, so ink-coloured cards separate from it). |
| Card — dark | `#1F3054` | Cards/popovers in dark mode. |
| Muted text — light | `#4A5A72` | Secondary text on chalk. |
| Muted text — dark | `#C3CBD9` | Secondary text on the dark surface. |
| Border — light | `#DCD6C8` | Hairlines on chalk (decorative; contrast rules below do not apply to it). |

## Contrast (WCAG 2.1, measured)

| Foreground | Background | Ratio | Verdict |
|---|---|---|---|
| Navy `#1B2A4A` | Chalk `#F7F4EC` | **12.94:1** | AAA |
| Muted `#4A5A72` | Chalk `#F7F4EC` | **6.38:1** | AA (AAA for large text) |
| Chalk `#F7F4EC` | Dark surface `#132038` | **14.80:1** | AAA |
| Chalk `#F7F4EC` | Dark card `#1F3054` | **11.87:1** | AAA |
| Muted `#C3CBD9` | Dark surface `#132038` | **8.71:1** | AAA |
| Yellow `#F2B705` | Navy `#1B2A4A` | **7.82:1** | AAA — the one pairing where yellow may carry text |
| Navy `#1B2A4A` | Yellow `#F2B705` | **7.82:1** | AAA — navy label on a yellow fill (badges, primary buttons) |
| Yellow `#F2B705` | Chalk `#F7F4EC` | **1.65:1** | ✗ **fails** — yellow is never text, an icon, or a hairline on chalk |

## Rules

1. Yellow is a **fill**, not a text colour, unless it sits on navy or the dark surface.
2. Every text/background pair used in the product must be in the table above (or measured and added to it) at ≥ 4.5:1.
3. Yellow keeps one meaning across the app: *this is the thing that was selected / tailored*. It is not a generic highlight, and never means "warning".
4. Status colours (error, success) are intentionally not brand colours; keep them functional and check them against these surfaces when they are introduced.

## Typography

| Role | Face | Notes |
|---|---|---|
| Display | **Instrument Serif** (SIL OFL) | The wordmark, landing-page headings, and the app's page titles (`h1`) only. In the logo it is converted to outlines, so the lockup never depends on an installed font. |
| Text | **Inter** (SIL OFL) | Everything else: UI body text, tables, forms, docs body. |

The wordmark and the docs/landing site use these faces. The **web app itself
uses the system font stack** (issue #305): no font is bundled or fetched, so it
works offline (ADR-0004) and adds no dependency. The app's colours, including
the dark theme and the per-Status badge pairs, are the CSS variables in
`frontend/src/index.css`.

## Files

| File | What it is |
|---|---|
| `logo-mark.svg` | The tape-measure **S**, transparent background. Works on both light and dark surfaces. |
| `logo-lockup.svg` | Mark + "Sumisura" wordmark in navy, for light surfaces. Wordmark is outlined, not live text. |
| `logo-lockup-dark.svg` | Same lockup with a chalk wordmark, for dark surfaces. |
| `logo-tile.svg` | App-icon tile: navy rounded square, yellow tape S with sparse ticks. |
| `favicon.svg` | The tile with the ticks dropped — the ticks turn to mush below ~24px. Source for the favicon and the extension icons. |
| `icons/icon-{16,32,48,128}.png` | Browser-extension icons, exported from `favicon.svg`. |
| `social-preview.svg` / `.png` | 1280×640 GitHub social preview (upload via repo Settings → Social preview; the API cannot set it). |

The Tailored CV and Cover Letter templates (`template/*.typ`) are **not** branded
— they produce the user's documents, not Sumisura marketing.
