// The card the extension injects on a job posting: the rendering layer for
// card-model.js (issue #206). It owns the Shadow DOM, the posting-change
// watching and the lookup cache; every decision about what to show comes
// from SumisuraCard.cardModel, so this file has no branching of its own
// beyond "draw what the model said".
//
// It lives in a Shadow DOM because LinkedIn and Indeed ship global CSS that
// reaches anything in their page (and their own class names are hashed, so
// there is nothing to be specific against). Inside a shadow root their
// styles cannot reach the card and the card's cannot reach them — which is
// also why content.css is gone: the styles below are the whole of it.

(function (root) {
  const HOST_ID = "sumisura-card";
  const COLLAPSED_KEY = "cardCollapsed";
  // How often the card re-reads the posting URL off the page. LinkedIn's
  // split pane swaps postings by pushState, which fires no event a content
  // script can hear, so there is nothing to subscribe to. This is a DOM
  // read on a timer, not network polling: a lookup leaves only when the
  // posting actually changed, and then at most once per posting
  // (ADR-0043).
  const POSTING_WATCH_MS = 700;

  const STYLES = `
    :host { all: initial; }
    * { box-sizing: border-box; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
    .card {
      position: fixed; right: 24px; bottom: 24px; z-index: 2147483647;
      width: 320px; max-width: calc(100vw - 32px); max-height: calc(100vh - 48px); overflow-y: auto;
      padding: 14px; border-radius: 12px;
      background: #fff; color: #1a1a1a; border: 1px solid rgba(0,0,0,.12);
      box-shadow: 0 6px 24px rgba(0,0,0,.18); font-size: 13px; line-height: 1.45;
    }
    .pill {
      position: fixed; right: 24px; bottom: 24px; z-index: 2147483647;
      padding: 10px 16px; border: none; border-radius: 999px;
      background: #0a66c2; color: #fff; font-weight: 600; font-size: 14px;
      cursor: pointer; box-shadow: 0 2px 8px rgba(0,0,0,.25);
    }
    .head { display: flex; align-items: flex-start; gap: 8px; }
    .headline { flex: 1; margin: 0; font-weight: 600; font-size: 13px; }
    .collapse {
      flex: none; padding: 2px 6px; border: none; border-radius: 6px;
      background: transparent; color: #555; font-size: 16px; line-height: 1; cursor: pointer;
    }
    .collapse:hover { background: rgba(0,0,0,.06); }
    .detail { margin: 6px 0 0; color: #555; }
    .problems { margin: 8px 0 0; padding-left: 18px; color: #a61b1b; }
    .fields { margin-top: 10px; display: grid; gap: 6px; }
    .fields label { display: grid; gap: 2px; font-size: 11px; color: #555; text-transform: uppercase; letter-spacing: .03em; }
    .fields input {
      padding: 6px 8px; border: 1px solid rgba(0,0,0,.2); border-radius: 6px;
      font-size: 13px; color: #1a1a1a; background: #fff;
    }
    .siblings { margin: 10px 0 0; padding: 0; list-style: none; display: grid; gap: 6px; }
    .sibling { padding: 8px; border: 1px solid rgba(0,0,0,.1); border-radius: 8px; background: #fafafa; }
    .sibling-title { font-weight: 600; }
    .sibling-meta { color: #555; font-size: 12px; }
    .sibling a { color: #0a66c2; }
    .actions { margin-top: 12px; display: flex; flex-wrap: wrap; gap: 6px; }
    button.action {
      padding: 7px 12px; border-radius: 8px; border: 1px solid #0a66c2;
      background: #0a66c2; color: #fff; font-size: 13px; font-weight: 600; cursor: pointer;
    }
    button.action.secondary { background: #fff; color: #0a66c2; }
    button.action.tiny { padding: 5px 9px; font-size: 12px; font-weight: 500; }
    button.action:disabled { opacity: .6; cursor: default; }
    .link { display: inline-block; margin-top: 10px; color: #0a66c2; font-weight: 600; }
    @media (prefers-color-scheme: dark) {
      .card { background: #1f1f1f; color: #f0f0f0; border-color: rgba(255,255,255,.14); }
      .detail, .sibling-meta, .fields label, .collapse { color: #b9b9b9; }
      .sibling { background: #272727; border-color: rgba(255,255,255,.1); }
      .fields input { background: #272727; color: #f0f0f0; border-color: rgba(255,255,255,.2); }
      .collapse:hover { background: rgba(255,255,255,.08); }
      button.action.secondary { background: transparent; color: #7ab8f5; border-color: #7ab8f5; }
      .link, .sibling a { color: #7ab8f5; }
    }
  `;

  // createCard wires one card to a page. capture/postingUrl/validate are
  // the board's own functions; deps is injected so the whole thing can be
  // driven without chrome.* in a test.
  function createCard(options) {
    const doc = options.doc || document;
    const capture = options.capture;
    const postingUrl = options.postingUrl;
    const validate = options.validate;
    const send = options.send || defaultSend;
    const storage = options.storage || defaultStorage();

    // lookups caches one answer per posting for this page session, so
    // clicking back and forth between search results costs nothing
    // (stories 29, 32).
    const lookups = new Map();

    const state = {
      postingUrl: "",
      serverUrl: "",
      capture: null,
      validation: [],
      lookup: { state: "idle" },
      outcome: { state: "idle" },
      collapsed: false,
      // edits survive a re-render but not a change of posting: they are
      // corrections to this capture (stories 41, 42).
      edits: {},
    };

    let shadow = null;

    function render() {
      if (!shadow) return;
      const view = SumisuraCard.cardModel({
        serverUrl: state.serverUrl,
        capture: editedCapture(),
        validation: state.validation,
        lookup: state.lookup,
        outcome: state.outcome,
      });
      draw(shadow, view, handlers, state.collapsed);
    }

    function editedCapture() {
      if (!state.capture) return null;
      return Object.assign({}, state.capture, state.edits);
    }

    const handlers = {
      onToggleCollapse() {
        state.collapsed = !state.collapsed;
        storage.set({ [COLLAPSED_KEY]: state.collapsed });
        render();
      },
      onEdit(field, value) {
        state.edits[field] = value;
        // Re-validating as the user types is what makes a corrected
        // capture stop complaining (story 42).
        state.validation = validate ? validate(editedCapture()) : [];
      },
      onAction(action) {
        if (action.id === "cancel") {
          state.outcome = { state: "idle" };
          render();
          return;
        }
        save(action.resolution);
      },
      onStatusMove(move) {
        moveStatus(move);
      },
    };

    // moveStatus changes a tracked Application's Status from the card. The
    // cached lookup for this posting is dropped either way: after a
    // successful move it is out of date, and after a refused one it may be
    // why the move was refused.
    function moveStatus(move) {
      state.outcome = { state: "moving", status: move.status };
      render();

      send({ type: "SUMISURA_STATUS", payload: { applicationId: move.applicationId, status: move.status } }, (response) => {
        lookups.delete(state.postingUrl);
        if (response && response.serverUrl) state.serverUrl = response.serverUrl;
        if (response && response.ok) {
          state.outcome = { state: "moved", status: (response.application && response.application.status) || move.status };
        } else {
          state.outcome = { state: "move-failed", error: (response && response.error) || "Could not move that Status." };
        }
        render();
      });
    }

    function save(resolution) {
      const payload = editedCapture();
      if (!payload) return;
      if (resolution) payload.resolution = resolution;

      state.outcome = { state: "saving" };
      render();

      send({ type: "SUMISURA_CAPTURE", payload }, (response) => {
        state.outcome = outcomeFrom(response);
        if (response && response.serverUrl) state.serverUrl = response.serverUrl;
        // A save changes what is tracked, so the cached answer for this
        // posting is now wrong.
        lookups.delete(state.postingUrl);
        render();
      });
    }

    // refresh re-reads the page. It fires a lookup only when the posting
    // actually changed and this session has not already asked about it.
    function refresh(force) {
      let url = "";
      try {
        url = postingUrl(doc) || "";
      } catch (err) {
        console.error("[Sumisura] reading the posting url threw", err);
      }
      if (!force && url === state.postingUrl) return;

      state.postingUrl = url;
      state.outcome = { state: "idle" };
      state.edits = {};
      state.capture = readCapture();
      state.validation = state.capture && validate ? validate(state.capture) : [];

      if (!url || !state.capture) {
        state.lookup = { state: "idle" };
        render();
        return;
      }

      const cached = lookups.get(url);
      if (cached) {
        state.lookup = cached;
        render();
        return;
      }

      // Story 31: clear to loading the moment the posting changes, so the
      // previous posting's answer is never on screen next to this one.
      state.lookup = { state: "loading" };
      render();

      send({ type: "SUMISURA_LOOKUP", payload: { url, company: state.capture.company } }, (response) => {
        const answer =
          response && response.ok
            ? { state: "ready", result: response.result }
            : { state: "failed", error: (response && response.error) || "Could not reach Sumisura." };
        lookups.set(url, answer);
        if (response && response.serverUrl) state.serverUrl = response.serverUrl;
        // The user may have moved on while this was in flight; only draw
        // it if it is still the posting on screen.
        if (state.postingUrl === url) {
          state.lookup = answer;
          render();
        }
      });
    }

    function readCapture() {
      try {
        return capture(doc);
      } catch (err) {
        console.error("[Sumisura] captureJobPosting threw", err);
        return null;
      }
    }

    function ensureMounted() {
      if (doc.getElementById(HOST_ID)) return false;
      const host = doc.createElement("div");
      host.id = HOST_ID;
      shadow = host.attachShadow({ mode: "open" });
      const style = doc.createElement("style");
      style.textContent = STYLES;
      shadow.appendChild(style);
      shadow.appendChild(doc.createElement("div"));
      doc.body.appendChild(host);
      return true;
    }

    function start() {
      storage.get([COLLAPSED_KEY]).then((stored) => {
        state.collapsed = Boolean(stored && stored[COLLAPSED_KEY]);
        render();
      });

      ensureMounted();
      refresh(true);

      // Job boards are single-page apps that re-render over our node, so
      // the card is re-attached if it disappears — the same guard the
      // button already had. Re-mounting means a fresh shadow root, so the
      // current state is drawn into it straight away.
      new MutationObserver(() => {
        if (ensureMounted()) render();
        refresh(false);
      }).observe(doc.body, { childList: true, subtree: false });

      root.setInterval(() => refresh(false), POSTING_WATCH_MS);
    }

    // mount, save and moveStatus are exposed alongside start so the card's
    // traffic and caching can be driven without the timer and the chrome.*
    // runtime start() brings with it (card-view.test.js).
    return { start, mount: ensureMounted, refresh, render, save, moveStatus, state, lookups };
  }

  // outcomeFrom turns background.js's answer into what the model reads.
  function outcomeFrom(response) {
    if (!response) return { state: "error", error: "No answer from the extension." };
    if (response.ok) return { state: "saved", saved: response.saved || {} };
    if (response.refused && response.conflict) return { state: "refused", conflict: response.conflict };
    return { state: "error", error: response.error || "Failed to save." };
  }

  // draw replaces the card's body with the view. It rebuilds rather than
  // patching: the card is small, and a rebuild has no stale-node class of
  // bug. The one thing it preserves is focus in an edited field, so typing
  // a company name is not interrupted by the re-render that typing causes.
  function draw(shadow, view, handlers, collapsed) {
    const doc = shadow.ownerDocument || document;
    const active = shadow.activeElement;
    const focusedField = active && active.dataset ? active.dataset.field : null;
    const caret = active && active.selectionStart;

    const body = shadow.lastElementChild;
    body.textContent = "";

    if (collapsed) {
      const pill = doc.createElement("button");
      pill.type = "button";
      pill.className = "pill";
      pill.textContent = pillLabel(view);
      pill.addEventListener("click", handlers.onToggleCollapse);
      body.appendChild(pill);
      return;
    }

    const card = doc.createElement("div");
    card.className = "card";

    const head = doc.createElement("div");
    head.className = "head";
    const headline = doc.createElement("p");
    headline.className = "headline";
    headline.textContent = view.headline;
    const collapse = doc.createElement("button");
    collapse.type = "button";
    collapse.className = "collapse";
    collapse.title = "Collapse";
    collapse.setAttribute("aria-label", "Collapse");
    collapse.textContent = "–";
    collapse.addEventListener("click", handlers.onToggleCollapse);
    head.appendChild(headline);
    head.appendChild(collapse);
    card.appendChild(head);

    if (view.detail) {
      const detail = doc.createElement("p");
      detail.className = "detail";
      detail.textContent = view.detail;
      card.appendChild(detail);
    }

    if (view.problems.length > 0) {
      const problems = doc.createElement("ul");
      problems.className = "problems";
      view.problems.forEach((problem) => {
        const li = doc.createElement("li");
        li.textContent = problem;
        problems.appendChild(li);
      });
      card.appendChild(problems);
    }

    if (view.fields.editable) {
      card.appendChild(fieldsBlock(doc, view, handlers));
    }

    if (view.siblings.length > 0) {
      card.appendChild(siblingsBlock(doc, view, handlers));
    }

    if (view.statusMoves.length > 0) {
      const moves = doc.createElement("div");
      moves.className = "actions";
      view.statusMoves.forEach((move) => {
        const button = doc.createElement("button");
        button.type = "button";
        button.className = "action secondary tiny";
        button.textContent = move.label;
        button.addEventListener("click", () => handlers.onStatusMove(move));
        moves.appendChild(button);
      });
      card.appendChild(moves);
    }

    if (view.actions.length > 0) {
      const actions = doc.createElement("div");
      actions.className = "actions";
      view.actions.forEach((action, i) => {
        actions.appendChild(actionButton(doc, action, handlers, i > 0 || action.id === "cancel"));
      });
      card.appendChild(actions);
    }

    if (view.link) {
      const link = doc.createElement("a");
      link.className = "link";
      link.href = view.link.href;
      link.target = "_blank";
      link.rel = "noreferrer";
      link.textContent = view.link.label;
      card.appendChild(link);
    }

    body.appendChild(card);

    if (focusedField) {
      const restored = body.querySelector('[data-field="' + focusedField + '"]');
      if (restored) {
        restored.focus();
        if (caret != null && restored.setSelectionRange) restored.setSelectionRange(caret, caret);
      }
    }
  }

  function fieldsBlock(doc, view, handlers) {
    const fields = doc.createElement("div");
    fields.className = "fields";
    [
      { field: "title", label: "Job Title", value: view.fields.title },
      { field: "company", label: "Company", value: view.fields.company },
    ].forEach((spec) => {
      const label = doc.createElement("label");
      label.textContent = spec.label;
      const input = doc.createElement("input");
      input.type = "text";
      input.value = spec.value;
      input.dataset.field = spec.field;
      input.addEventListener("input", (e) => handlers.onEdit(spec.field, e.target.value));
      label.appendChild(input);
      fields.appendChild(label);
    });
    return fields;
  }

  function siblingsBlock(doc, view, handlers) {
    const list = doc.createElement("ul");
    list.className = "siblings";
    view.siblings.forEach((sibling) => {
      const item = doc.createElement("li");
      item.className = "sibling";

      const title = doc.createElement("div");
      title.className = "sibling-title";
      title.textContent = sibling.title;
      item.appendChild(title);

      const meta = doc.createElement("div");
      meta.className = "sibling-meta";
      meta.textContent = sibling.statusLabel + " · saved " + sibling.savedAtLabel;
      item.appendChild(meta);

      const open = doc.createElement("a");
      open.href = sibling.link;
      open.target = "_blank";
      open.rel = "noreferrer";
      open.textContent = "Open";
      item.appendChild(open);

      if (view.state === "untracked" || view.state === "needs-decision") {
        const replace = actionButton(doc, sibling.replaceAction, handlers, true);
        replace.classList.add("tiny");
        item.appendChild(replace);
      }
      list.appendChild(item);
    });
    return list;
  }

  function actionButton(doc, action, handlers, secondary) {
    const button = doc.createElement("button");
    button.type = "button";
    button.className = "action" + (secondary ? " secondary" : "");
    button.textContent = action.label;
    button.addEventListener("click", () => handlers.onAction(action));
    return button;
  }

  // pillLabel keeps the one signal that matters visible while collapsed,
  // so collapsing the card does not mean losing the answer it gave.
  function pillLabel(view) {
    switch (view.state) {
      case "tracked":
        return "✓ In Sumisura";
      case "saved":
        return "✓ Saved";
      case "refused-duplicate":
        return "✓ Already saved";
      case "loading":
        return "Sumisura…";
      case "unreachable":
        return "Sumisura ?";
      default:
        return "Sumisura";
    }
  }

  function defaultSend(message, callback) {
    chrome.runtime.sendMessage(message, (response) => {
      if (chrome.runtime.lastError) {
        callback({ ok: false, error: chrome.runtime.lastError.message });
        return;
      }
      callback(response);
    });
  }

  function defaultStorage() {
    return {
      get: (keys) => chrome.storage.local.get(keys),
      set: (values) => chrome.storage.local.set(values),
    };
  }

  const SumisuraCardView = { createCard, draw, outcomeFrom, pillLabel, HOST_ID };

  root.SumisuraCardView = SumisuraCardView;
  if (typeof module !== "undefined" && module.exports) {
    module.exports = SumisuraCardView;
  }
})(typeof window !== "undefined" ? window : globalThis);
