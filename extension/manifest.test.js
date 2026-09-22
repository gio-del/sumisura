const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

// A class-of-bug test, at the manifest seam rather than the click path.
//
// The per-field capture validation added by issue #58 was wired into the
// LinkedIn bundle only; when the Indeed board was added later, nothing
// extended it there, so Indeed captures silently fell back to
// capture-common.js's bare "is company or description empty" check — the
// exact behaviour #58 was filed to replace. Nothing failed loudly, and no
// extraction test could have caught it, because the defect was in the
// wiring rather than in either script.
//
// Asserting on the shipped manifest catches that shape of defect for any
// board, including one added in future, without faking the chrome.*
// runtime to reach the click path.

const SHARED_SCRIPTS = new Set(["turndown.js", "capture-common.js", "validate-capture.js", "card-model.js", "card-view.js", "badges.js"]);
const VALIDATION_SCRIPT = "validate-capture.js";

function manifest() {
  return JSON.parse(fs.readFileSync(path.join(__dirname, "manifest.json"), "utf8"));
}

// A board capture script is any content script in a bundle that isn't one of
// the board-agnostic ones — i.e. the file holding that board's own selectors.
function boardCaptureScripts(js) {
  return js.filter((file) => !SHARED_SCRIPTS.has(file));
}

test("every content-script bundle shipping a board capture script also ships the validation script", () => {
  for (const entry of manifest().content_scripts) {
    const boardScripts = boardCaptureScripts(entry.js);
    if (boardScripts.length === 0) continue;

    assert.ok(
      entry.js.includes(VALIDATION_SCRIPT),
      `content_scripts bundle for ${entry.matches.join(", ")} ships ${boardScripts.join(", ")} ` +
        `but not ${VALIDATION_SCRIPT}, so its captures silently fall back to the bare emptiness check`
    );
  }
});

test("the validation script is loaded before the board capture script that uses it", () => {
  for (const entry of manifest().content_scripts) {
    const boardScripts = boardCaptureScripts(entry.js);
    if (boardScripts.length === 0) continue;

    // Content scripts in a bundle execute in declaration order, and each
    // board script reads the `validateCapture` global at init.
    const validationIndex = entry.js.indexOf(VALIDATION_SCRIPT);
    for (const boardScript of boardScripts) {
      assert.ok(
        validationIndex !== -1 && validationIndex < entry.js.indexOf(boardScript),
        `${VALIDATION_SCRIPT} must precede ${boardScript} in the bundle for ${entry.matches.join(", ")}`
      );
    }
  }
});

test("every board capture script referenced by the manifest exists on disk", () => {
  for (const entry of manifest().content_scripts) {
    for (const file of entry.js) {
      assert.ok(fs.existsSync(path.join(__dirname, file)), `manifest references a missing content script: ${file}`);
    }
  }
});

// Issue #195: the background script uses SumisuraSettings, which Chrome loads
// via importScripts and Firefox only if settings.js is listed first.
test("background scripts load their dependencies before background.js, and the options page exists", () => {
  const m = manifest();
  // toolbar.js joined settings.js here for the same reason (issue #206):
  // Chrome importScripts them, Firefox only loads them if they are listed
  // ahead of background.js.
  assert.deepEqual(m.background.scripts, ["settings.js", "toolbar.js", "background.js"]);
  assert.ok(m.permissions.includes("storage"), "chrome.storage needs the storage permission");
  assert.equal(m.options_ui.page, "options.html");
  assert.ok(fs.existsSync(path.join(__dirname, m.options_ui.page)));
  const html = fs.readFileSync(path.join(__dirname, "options.html"), "utf8");
  assert.ok(html.indexOf("settings.js") < html.indexOf("options.js"), "options.html must load settings.js before options.js");
});

// Issue #206: the card is two files — what to show (card-model.js) and how
// to draw it (card-view.js) — and capture-common.js calls into the view at
// init. Content scripts in a bundle execute in declaration order, so a
// wrong order is a silent ReferenceError on a real posting page, which no
// pure test could catch.
test("the card scripts load, in order, before the shared script that mounts them", () => {
  for (const entry of manifest().content_scripts) {
    if (boardCaptureScripts(entry.js).length === 0) continue;
    const model = entry.js.indexOf("card-model.js");
    const view = entry.js.indexOf("card-view.js");
    const common = entry.js.indexOf("capture-common.js");
    const where = entry.matches.join(", ");
    assert.ok(model !== -1 && view !== -1, `${where}: the bundle must ship both card scripts`);
    assert.ok(model < view, `${where}: card-model.js must precede card-view.js, which reads SumisuraCard`);
    assert.ok(view < common, `${where}: card-view.js must precede capture-common.js, which mounts the card at init`);
  }
});

// The card styles itself inside its shadow root, so a page-level
// stylesheet would be dead weight the board's CSS could still fight with.
test("no content-script bundle injects a page-level stylesheet any more", () => {
  for (const entry of manifest().content_scripts) {
    assert.equal(entry.css, undefined, `${entry.matches.join(", ")}: the card styles itself inside its shadow root`);
  }
});

// Issue #206: badging is board-agnostic (the row selector is the board
// script's), so it only has to load before the board script that starts it.
test("a bundle that badges search results loads badges.js before its board script", () => {
  for (const entry of manifest().content_scripts) {
    const badges = entry.js.indexOf("badges.js");
    if (badges === -1) continue;
    for (const boardScript of boardCaptureScripts(entry.js)) {
      assert.ok(
        badges < entry.js.indexOf(boardScript),
        `badges.js must precede ${boardScript}, which reads SumisuraBadges at init`
      );
    }
  }
});

// Issue #206, stories 52-54: the capture shortcut is a browser command
// relayed to the content script, so the command has to be declared, its id
// has to match the one background.js listens for, and the extension needs
// host permission for the pages it relays to.
test("the capture shortcut is declared under the id background.js listens for", () => {
  const { SAVE_COMMAND } = require("./toolbar.js");
  const commands = manifest().commands;

  assert.ok(commands, "manifest must declare a commands entry");
  assert.ok(commands[SAVE_COMMAND], `manifest must declare the "${SAVE_COMMAND}" command`);
  assert.ok(commands[SAVE_COMMAND].description, "a command with no description is unlabelled in the browser's shortcut list");
  assert.ok(commands[SAVE_COMMAND].suggested_key, "without a suggested key the shortcut ships unbound");
});

test("every page a content script runs on is one the extension may message", () => {
  const m = manifest();
  for (const entry of m.content_scripts) {
    for (const match of entry.matches) {
      assert.ok(
        m.host_permissions.includes(match),
        `chrome.tabs.sendMessage needs host permission for ${match}, or the shortcut silently does nothing there`
      );
    }
  }
});
