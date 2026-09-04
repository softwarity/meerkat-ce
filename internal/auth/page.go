package auth

import (
	"net/http"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// The page agent: what the gateway does TO a proxied page, as opposed to what
// it draws ON it. The two used to be one file, and the user button carried
// both - which meant a route that offered the light/dark switch got nothing
// when the button was turned off, and a page inside a portal's iframe had to
// be excepted by hand from a guard written for chrome it does not draw.
//
// The split is along that line: the button is chrome (a menu, an avatar, a way
// out), the agent is behaviour - the language the person chose, the scheme they
// picked, and whether the session is still there. It is injected on EVERY UI
// route and never asks whether a button is there. The button, when there is
// one, asks the agent to act.
//
// window.meerkatPage is the seam:
//
//	pickLanguage(code)  someone chose a language here
//	applyLanguage(code) this page acts on a language, whoever chose it
//	onLanguage(fn)      called when it does, BEFORE the page's own hook
//	applyScheme(v)      light | dark | auto, applied to the document
//	pickScheme(v)       someone chose a scheme here
//	resolvedLanguage()  the language this page was served in
//	signedOut()         tell the other pages the session just ended
//	data()              the shared /meerkat/user-button.json read
//	onEvent(type, fn)   what the GATEWAY has to say, live
//
// It also draws ONE thing of its own, and only one: the strip naming what a
// developer's machine is serving (DEV-11). Chrome belongs to the button; this
// is here because it is not chrome - it says the page in front of you is not
// the deployed version, and that has to reach a route whose button is off.
//
// Three channels, and the split between them is by WHO knows the thing.
//
// Two are BroadcastChannel, between the pages of one browser: meerkat-locale
// carries a language someone picked, meerkat-session carries "alive" and
// "ended". Deliberately not sockets - one message between tabs, and a
// connection per open page would buy reaching a second machine that nobody
// needs for a setting touched twice a year.
//
// The third IS a socket (/meerkat/events), because what it carries only the
// gateway knows: a service served from a developer's machine, a configuration
// reloaded. It opens on the first onEvent listener, so a page that asks for
// nothing pays nothing, and it is never required - every listener already has
// its state from the fetch at load.
func (h *Handler) pageJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write([]byte(pageJS))
}

const pageJS = `(() => {
  if (window.meerkatPage) return;

  const COOKIE_LANG = 'MEERKAT_LANG', COOKIE_SCHEME = 'MEERKAT_SCHEME';
  const setCookie = (k, v) => { document.cookie = k + '=' + v + ';path=/;max-age=31536000;SameSite=Lax'; };
  const getCookie = (k) => (document.cookie.split('; ').find(c => c.startsWith(k + '=')) || '').split('=')[1] || '';
  const darkMedia = matchMedia('(prefers-color-scheme: dark)');
  // What the ROUTE declared, written on the script tag by the gateway.
  const cfg = (document.currentScript && document.currentScript.dataset) || {};
  const languages = () => (cfg.languages || '').split(',').filter(Boolean);
  const listeners = [];
  let schemeImposed = false;

  // One read of the session payload for the whole page: the agent needs it
  // only to learn whether the integrator settled light/dark, the button needs
  // all of it, and neither should pay for the other's request.
  let pending = null;
  const data = () => pending || (pending = fetch('/meerkat/user-button.json', { credentials: 'same-origin' }).then(r => r.json()));

  // The person's light/dark choice, reflected for the APPLICATION: the CSS
  // color-scheme + data-meerkat-scheme, plus the app's own mechanism - an
  // attribute (light/dark values) or a class pair - as configured on the
  // route. Silent unless the route offers the switch.
  const applyScheme = (v) => {
    if (cfg.scheme !== 'select') return;
    const root = document.documentElement;
    if (v === 'light' || v === 'dark') {
      root.style.colorScheme = v;
      root.setAttribute('data-meerkat-scheme', v);
    } else {
      root.style.colorScheme = '';
      root.removeAttribute('data-meerkat-scheme');
    }
    // The strip lives outside this inheritance, in its own root: it has to be
    // told, or a toggle leaves it in the scheme the page just left.
    syncStripScheme();
    const mech = cfg.schemeMechanism;
    if (!mech) return;
    const light = cfg.schemeLight || '', dark = cfg.schemeDark || '';
    const resolved = (v === 'light' || v === 'dark') ? v : (darkMedia.matches ? 'dark' : 'light');
    if (mech === 'attribute') {
      if (cfg.schemeAttribute) root.setAttribute(cfg.schemeAttribute, resolved === 'dark' ? dark : light);
    } else if (mech === 'class') {
      if (light) root.classList.remove(light);
      if (dark) root.classList.remove(dark);
      const cls = resolved === 'dark' ? dark : light;
      if (cls) root.classList.add(cls);
    }
  };

  // The language this page was served in, resolved the way the gateway does
  // it: the cookie, then what the browser asks for, within THIS route's offer.
  const resolvedLanguage = () => {
    const codes = languages();
    const pick = (tag) => {
      const t = (tag || '').toLowerCase();
      return codes.find(c => c.toLowerCase() === t)
        || codes.find(c => c.split('-')[0].toLowerCase() === t.split('-')[0]) || '';
    };
    let code = pick(getCookie(COOKIE_LANG));
    if (!code) for (const nav of (navigator.languages || [navigator.language || ''])) {
      code = pick(nav); if (code) break;
    }
    return code;
  };

  // Where the language lives in the URL, changing it is a NAVIGATION, not a
  // reload: an application built per locale carries the segment in its own
  // <base href>, so reloading would ask for the same French page again and
  // the menu would look broken. The segment sits right after the route's
  // prefix, which the gateway named in data-locale-paths; the rest of the
  // path, the query and the fragment are carried over untouched.
  const followLanguage = (code) => {
    // The language lives in the query string: rewrite it and go there.
    if (cfg.localeParam) {
      const u = new URL(location.href);
      u.searchParams.set(cfg.localeParam, code);
      location.assign(u.toString());
      return;
    }
    if (cfg.localePaths === undefined) { location.reload(); return; }
    const paths = cfg.localePaths.split(',').filter(Boolean);
    const codes = languages();
    const path = location.pathname;
    let at = 0;
    for (const p of paths) {
      if (p.length > at && (path === p || path.startsWith(p + '/'))) at = p.length;
    }
    const rest = path.slice(at).replace(/^\//, '');
    const slash = rest.indexOf('/');
    const first = slash < 0 ? rest : rest.slice(0, slash);
    const tail = slash < 0 ? '' : rest.slice(slash + 1);
    const known = codes.some(c => c.toLowerCase() === first.toLowerCase());
    // Replace the locale segment when there is one, insert it when there is
    // not: a deep link without the language is as valid as one with it.
    const next = path.slice(0, at) + '/' + code + (known ? (tail ? '/' + tail : '') : (rest ? '/' + rest : ''));
    location.assign(next + location.search + location.hash);
  };

  // What this page DOES about a new language, whoever asked. The route's
  // mechanism decides - its script, else the segment in its URL, else a
  // reload - and it decides the same way whether the click happened here or
  // in another tab. Anything else would be arbitrary: a portal that reloads
  // itself while the application next to it stays in the old language is not
  // a design, it is where the message happened to stop.
  //
  // The listeners run FIRST, and that ordering is the point: the button's menu
  // names the active language, and a script that redraws the page in place
  // never reloads, so a menu redrawn afterwards would go on naming the old
  // language under a page that had moved on.
  //
  // From the hook on, the page is ITS business: the hook REPLACES the reload,
  // and reloading is one of the things it may decide to do. We do not reload
  // behind its back, not even when it throws - it may well have redrawn
  // already, and the reload would destroy exactly the state this exists to
  // keep. A throw is reported and left at that.
  const applyLanguage = (code) => {
    for (const fn of listeners) {
      try { fn(code); } catch (e) { console.error('meerkat: a language listener threw', e); }
    }
    const hook = window['` + store.LocaleHookName + `'];
    if (typeof hook !== 'function') { followLanguage(code); return; }
    try {
      hook(code);
    } catch (e) {
      console.error('meerkat: the locale script threw; the page is its to handle', e);
    }
  };

  // Picking a language, in the order that matters: the cookie FIRST, so
  // whatever the page fetches next already asks in the new one; the profile
  // next, so the choice follows the person to their other browser; and only
  // then the page. The profile call is not waited on: it is a preference,
  // not a step.
  const pickLanguage = (code) => {
    if (!code) return;
    setCookie(COOKIE_LANG, code);
    // Tell the other pages of this origin at once: the portal and the
    // application it frames, or two applications side by side.
    try { new BroadcastChannel('meerkat-locale').postMessage({ locale: code }); } catch { /* no channel */ }
    fetch('/meerkat/locale', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ locale: code }),
      credentials: 'same-origin',
    }).catch(() => {});
    applyLanguage(code);
  };

  // Picking a scheme, in the same order and for the same reasons as a
  // language: the cookie first (the server renders the next page in it), the
  // account next (so it follows the person to their other browser), the page
  // last. The integrator may have settled light or dark for everyone, and then
  // there is nothing to pick - the switch is not drawn at all.
  const pickScheme = (v) => {
    if (!v) return;
    setCookie(COOKIE_SCHEME, v);
    try { new BroadcastChannel('meerkat-scheme').postMessage({ scheme: v }); } catch { /* no channel */ }
    fetch('/meerkat/scheme', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ scheme: v }),
      credentials: 'same-origin',
    }).catch(() => {});
    applyScheme(v);
  };

  // A scheme picked in one page reaches the other pages of this browser, the
  // way a language does: the portal and the application it frames must not
  // disagree on light or dark for the time it takes to reload.
  const listenForScheme = () => {
    if (typeof BroadcastChannel !== 'function') return;
    let channel;
    try { channel = new BroadcastChannel('meerkat-scheme'); } catch { return; }
    channel.onmessage = (e) => {
      const v = e.data && e.data.scheme;
      if (v && !schemeImposed) applyScheme(v);
    };
  };

  // The other pages of this browser, live. A language picked in one page
  // reaches every other page of this origin at once - the portal and the
  // application it frames, two applications side by side - so they all act
  // as if their own button had been clicked. An iframe is one of those pages,
  // and this is why the agent has no frame guard: a framed page draws no menu,
  // but it is still an application that has to follow.
  //
  // Deliberately NOT a socket to the gateway: this is one message between
  // pages of one browser. A persistent connection per open page would buy
  // reaching a SECOND machine, which nobody needs for a setting touched
  // twice a year, and would owe a bus the day two gateways run side by side.
  const listenForLanguage = () => {
    if (typeof BroadcastChannel !== 'function') return;
    let channel;
    try { channel = new BroadcastChannel('meerkat-locale'); } catch { return; }
    channel.onmessage = (e) => {
      const code = e.data && e.data.locale;
      if (code) applyLanguage(code);
    };
  };

  // The language the gateway resolved for this person, applied to the page as
  // soon as it is up. Replaying the sequence the admin wrote, with locale set,
  // is all there is to do - the same one call as a click, so nothing has to be
  // written twice, in the route and again in the application's bootstrap.
  //
  // After the load event, because an application is not there while its own
  // scripts are still being parsed. ONLY for the script mechanism: the gateway
  // already served this page in the right language, so a route that navigates
  // or reloads to change language has nothing to do here.
  //
  // No comparison with what the page is showing: applying a language it is
  // already in costs nothing. The rule is the reload, not the comparison, and
  // this watches the rate to catch a script that reloads anyway - calls
  // milliseconds apart are a loop, not a person.
  const applyLanguageAtLoad = () => {
    if (typeof window['` + store.LocaleHookName + `'] !== 'function') return;
    const run = () => {
      const code = resolvedLanguage();
      if (!code) return;
      try {
        const last = Number(sessionStorage.getItem('meerkat-locale-at') || 0);
        if (Date.now() - last < 3000) {
          console.warn('meerkat: the locale script reloads the page; it must apply the language in place');
          return;
        }
        sessionStorage.setItem('meerkat-locale-at', String(Date.now()));
      } catch { /* private mode: no guard, the call still happens */ }
      applyLanguage(code);
    };
    if (document.readyState === 'complete') run();
    else addEventListener('load', run, { once: true });
  };

  // ---- the session ----------------------------------------------------
  // The deadline lives in a readable cookie the gateway reposes every time it
  // slides. Watching it costs nothing and answers the question a page could
  // not answer before: is the session still there? Without it a tab left open
  // shows a live-looking screen until someone clicks and gets an error - or
  // gets nothing at all, which is worse.
  const until = () => Number(getCookie('` + session.UntilCookieName + `') || 0) * 1000;

  let sessionChannel = null;
  const sessionOpen = () => {
    if (sessionChannel) return sessionChannel;
    if (typeof BroadcastChannel !== 'function') return null;
    try { sessionChannel = new BroadcastChannel('meerkat-session'); } catch { return null; }
    return sessionChannel;
  };
  const tellSession = (state) => { const c = sessionOpen(); if (c) c.postMessage({ state }); };

  // Only the top window navigates. A framed page is inside a portal, and the
  // portal is leaving too - it takes the frame with it. A login form drawn
  // inside someone else's iframe is a phishing lesson, not a feature.
  const toLogin = () => {
    if (window.self !== window.top) return;
    location.assign('/login?next=' + encodeURIComponent(location.pathname + location.search + location.hash));
  };

  const watchSession = () => {
    // No deadline at load means no session, and a public page has every right
    // to one: there is nothing to watch and nobody to send to a login form.
    if (!until()) return;
    tellSession('alive');
    const check = () => {
      if (until() > Date.now()) return;
      tellSession('ended');
      toLogin();
    };
    setInterval(check, 30000);
    // A laptop that slept through the deadline: timers do not fire while it
    // is away, so the question is asked again the moment it comes back.
    addEventListener('visibilitychange', () => { if (!document.hidden) check(); });
    const c = sessionOpen();
    if (c) c.onmessage = (e) => {
      // Signing out in one tab ends it everywhere, at once, instead of leaving
      // the other pages looking signed in until their next call fails.
      if (e.data && e.data.state === 'ended' && !until()) toLogin();
    };
  };

  // ---- the live channel -----------------------------------------------
  // What the GATEWAY knows and the page cannot: a service being served from a
  // developer's machine right now, a configuration reloaded, a message an
  // operator put on every screen. Not the same need as the BroadcastChannels
  // above - those carry one message between the tabs of one browser, this one
  // carries what only the server has.
  //
  // Opened LAZILY, on the first subscriber: a page nobody listens on pays
  // nothing, which is what let this be added to pages that were already
  // working. And never required - every listener already has its state from
  // the fetch at load, so a proxy that eats WebSockets costs the freshness,
  // not the feature.
  const handlers = new Map();
  let socket = null, attempt = 0, retryAt = 0;

  const openEvents = () => {
    if (socket || typeof WebSocket !== 'function') return;
    const url = (location.protocol === 'https:' ? 'wss://' : 'ws://') + location.host + '/meerkat/events';
    try { socket = new WebSocket(url); } catch { socket = null; return; }
    socket.onopen = () => { attempt = 0; };
    socket.onmessage = (e) => {
      let msg;
      try { msg = JSON.parse(e.data); } catch { return; }
      for (const fn of handlers.get(msg && msg.type) || []) {
        try { fn(msg.data); } catch (err) { console.error('meerkat: an event listener threw', err); }
      }
    };
    // One handler for both ways out: a refusal never fires onclose without
    // onerror, and a reconnection scheduled twice is a reconnection storm.
    socket.onclose = socket.onerror = () => {
      if (!socket) return;
      socket = null;
      if (!handlers.size) return;
      // Backoff with jitter, capped: a gateway that just restarted has every
      // page of every browser waiting on it, and they must not come back in
      // step. The state a page shows meanwhile is the one it fetched, which
      // is exactly what makes waiting affordable.
      const wait = Math.min(30000, 1000 * Math.pow(2, attempt++)) * (0.75 + Math.random() / 2);
      clearTimeout(retryAt);
      retryAt = setTimeout(openEvents, wait);
    };
  };

  const onEvent = (type, fn) => {
    if (!type || typeof fn !== 'function') return;
    const list = handlers.get(type) || [];
    list.push(fn);
    handlers.set(type, list);
    openEvents();
  };

  // A tab brought back after a sleep finds its socket dead and nothing
  // pending: the timer did not fire while it was away.
  addEventListener('visibilitychange', () => {
    if (!document.hidden && !socket && handlers.size) { attempt = 0; openEvents(); }
  });

  // ---- what this application IS right now (DEV-11) --------------------
  // A strip saying which names answer from somebody's machine instead of the
  // cluster. Shown to EVERY visitor, signed in or not: it changes what the
  // page in front of them is.
  //
  // It lives in the AGENT and not in the user button, and that is not where it
  // started - the button is injected only when a route enables it, so on a
  // route without one the message that must reach everyone reached nobody.
  // The split at the top of this file already said where it belongs: the
  // button is chrome, the agent is behaviour, and "this is not the deployed
  // version" is behaviour.
  //
  // Its own shadow root, like the button's, so the application's CSS cannot
  // reach in and the strip cannot leak out. position: fixed, so it adds no
  // height to the document and moves nothing.
  let servedNow = [], stripHost = null, stripRoot = null;

  const stripCSS =
    // all: initial cuts the inheritance that would let the application reach
    // in - and it cuts color-scheme with it, so Canvas and CanvasText would
    // resolve light on a dark desktop. Said again here, and overridden below
    // when the integrator settled the question (THEME-05).
    ':host { all: initial; color-scheme: light dark; }' +
    ':host([data-scheme="dark"]) { color-scheme: dark; }' +
    ':host([data-scheme="light"]) { color-scheme: light; }' +
    // Draggable along its edge, so it can be moved off whatever it happens to
    // cover. --pb-x is the anchor: unset means centred, which is where it
    // starts and where it stays for anyone who never touches it.
    '.pb { position: fixed; bottom: 0; left: var(--pb-x, 50%); transform: translateX(-50%); z-index: 2147483000;' +
    ' display: flex; align-items: center; flex-wrap: wrap; gap: 10px; padding: 6px 12px;' +
    ' max-width: min(760px, calc(100vw - 20px));' +
    ' background: Canvas; color: CanvasText; border: 1px solid #f59e0b; border-bottom: 0;' +
    ' border-radius: 10px 10px 0 0; box-shadow: 0 -8px 30px rgba(0,0,0,.25);' +
    ' font-family: system-ui, sans-serif; font-size: 12px; line-height: 1.4; }' +
    '.pb.min > *:not(.pb-tab) { display: none; }' +
    '.pb.min { padding: 0; border: 0; box-shadow: none; }' +
    '.pb-tab { position: absolute; bottom: 100%; left: 50%; transform: translateX(-50%);' +
    ' display: flex; align-items: center; gap: 5px; padding: 2px 10px; font: inherit;' +
    ' background: Canvas; color: inherit; border: 1px solid #f59e0b; border-bottom: 0;' +
    ' border-radius: 8px 8px 0 0;' +
    // Grab, not pointer: the handle both collapses the strip and slides it,
    // and the cursor is what says the second one is there at all.
    ' cursor: grab; touch-action: none; user-select: none; }' +
    '.pb-tab:active { cursor: grabbing; }' +
    '.pb-badge { background: #f59e0b; color: #1c1c22; font-weight: 700; border-radius: 4px;' +
    ' padding: 1px 6px; font-size: 11px; }' +
    '.pb-title { font-weight: 700; }' +
    '.pb-list { display: flex; flex-wrap: wrap; gap: 6px; margin: 0; padding: 0; list-style: none; }' +
    // One entry per DEVELOPER, their names inside it: with four services
    // plugged by one person, four separate chips repeated the same name four
    // times and buried the only thing that varies.
    '.pb-list li { display: flex; align-items: center; gap: 5px; border: 1px solid rgba(128,128,128,.4);' +
    ' border-radius: 6px; padding: 1px 7px; }' +
    // A badge, not just a bolder word: side by side in one line, the person
    // and the names they serve read as one run of text, and the name of the
    // person is the part that changes.
    '.pb-who { font-weight: 600; padding: 1px 8px; border-radius: 999px;' +
    ' background: rgba(128,128,128,.28); }' +
    // A service name is an identifier: it wraps BETWEEN names, never inside
    // one - a name split across two lines reads as two that do not exist.
    '.pb-names > span { white-space: nowrap; }' +
    '.pb-names { font-family: ui-monospace, monospace; }' +
    '';

  const esc = (v) => String(v == null ? '' : v).replace(/[&<>"']/g, (c) =>
    ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

  // Where the strip sits, as a fraction of the viewport width. Clamped on
  // every placement rather than on save: a position that fitted yesterday's
  // window has to keep the strip reachable in today's.
  let anchor = 0.5, dragged = false;

  const placeStrip = (fraction, bar) => {
    const half = bar.getBoundingClientRect().width / 2;
    const min = half + 8, max = Math.max(min, window.innerWidth - half - 8);
    anchor = Math.min(Math.max(fraction * window.innerWidth, min), max) / window.innerWidth;
    bar.style.setProperty('--pb-x', (anchor * 100).toFixed(3) + '%');
  };

  // What look the strip wears. Its own shadow root cuts colour-scheme along
  // with everything else, so it has to be told - and told the same thing the
  // rest of the page was.
  //
  // The integrator's choice comes FIRST (THEME-05). Unchecking dark for the
  // built-in pages made them light and left this strip dark on a dark desktop,
  // because it was only ever reading the per-route switch, which a route that
  // does not offer one never writes.
  let imposedScheme = '';
  const syncStripScheme = () => {
    if (!stripHost) return;
    const scheme = imposedScheme || document.documentElement.getAttribute('data-meerkat-scheme');
    if (scheme) stripHost.setAttribute('data-scheme', scheme);
    else stripHost.removeAttribute('data-scheme');
  };

  const renderServed = (labels) => {
    const L = labels || {};
    const lb = (k, d) => L[k] || d;
    if (!servedNow.length) {
      if (stripHost) { stripHost.remove(); stripHost = null; stripRoot = null; }
      return;
    }
    if (!stripHost) {
      stripHost = document.createElement('div');
      stripHost.setAttribute('data-meerkat-plugged', '');
      stripRoot = stripHost.attachShadow({ mode: 'open' });
      const style = document.createElement('style');
      style.textContent = stripCSS;
      stripRoot.appendChild(style);
      stripRoot.appendChild(document.createElement('div'));
      document.body.appendChild(stripHost);
      try {
        const saved = parseFloat(localStorage.getItem('mk-plugbar-x'));
        if (saved >= 0 && saved <= 1) anchor = saved;
      } catch { /* opaque storage: it stays centred */ }
      // A window that narrows must not leave the strip half off the screen.
      addEventListener('resize', () => {
        const b = stripRoot && stripRoot.lastElementChild;
        if (b) placeStrip(anchor, b);
      });
    }
    // Follow whatever the page settled on: the agent writes the resolved
    // choice on <html>, and the strip is outside that inheritance.
    syncStripScheme();
    let collapsed = false;
    try { collapsed = sessionStorage.getItem('mk-plugbar-min') === '1'; } catch { /* opaque storage */ }
    // Grouped by whoever is serving, and sorted on both levels: an unchanged
    // state must look unchanged between two renders, or the list appears to
    // shuffle on every event.
    const byWho = new Map();
    for (const s of servedNow.slice().sort((a, b) => a.name.localeCompare(b.name))) {
      const k = s.who || '';
      if (!byWho.has(k)) byWho.set(k, []);
      byWho.get(k).push(s);
    }
    const groups = [...byWho.entries()].sort((a, b) => a[0].localeCompare(b[0]));
    const bar = stripRoot.lastElementChild;
    bar.className = 'pb' + (collapsed ? ' min' : '');
    bar.innerHTML =
      '<button class="pb-tab" title="' + esc(lb('pluggedTitle', 'Served from a developer machine')) + '">' +
      '<span class="pb-badge">plug</span><span class="pb-chev">' + (collapsed ? '\u25b4' : '\u25be') + '</span></button>' +
      '<span class="pb-title">' + esc(lb('pluggedTitle', 'Served from a developer machine')) + '</span>' +
      '<ul class="pb-list">' + groups.map(([who, list]) =>
        '<li>' +
        (who ? '<span class="pb-who">' + esc(who) + '</span>' : '') +
        '<span class="pb-names">' + list.map((s) => '<span>' + esc(s.name) + '</span>').join(', ') + '</span>' +
        '</li>').join('') + '</ul>';
    placeStrip(anchor, bar);
    const tab = bar.querySelector('.pb-tab');
    tab.addEventListener('click', () => {
      // A drag ends with a click the browser fires anyway; swallowing it here
      // is what keeps moving the strip from also collapsing it.
      if (dragged) { dragged = false; return; }
      const min = !bar.classList.contains('min');
      bar.classList.toggle('min', min);
      bar.querySelector('.pb-chev').textContent = min ? '\u25b4' : '\u25be';
      try { sessionStorage.setItem('mk-plugbar-min', min ? '1' : '0'); } catch { /* ditto */ }
    });
    // Dragging the handle slides the strip along its edge. The only real
    // defect of a fixed strip is that it covers something; letting it be moved
    // costs a few lines and removes the whole complaint.
    //
    // The position is kept as a FRACTION of the viewport, in localStorage: a
    // pixel offset saved on a wide screen puts the strip off a narrow one, and
    // this is a preference rather than a session's business.
    tab.addEventListener('pointerdown', (e) => {
      const start = e.clientX;
      const rect = bar.getBoundingClientRect();
      const grip = start - (rect.left + rect.width / 2);
      dragged = false;
      tab.setPointerCapture(e.pointerId);
      const move = (ev) => {
        // Below a few pixels it is a click, not a drag: a handle that slides
        // when someone means to collapse it is a handle that fights back.
        if (!dragged && Math.abs(ev.clientX - start) < 4) return;
        dragged = true;
        placeStrip((ev.clientX - grip) / window.innerWidth, bar);
      };
      const up = () => {
        tab.removeEventListener('pointermove', move);
        tab.removeEventListener('pointerup', up);
        if (dragged) {
          try { localStorage.setItem('mk-plugbar-x', String(anchor)); } catch { /* ditto */ }
        }
      };
      tab.addEventListener('pointermove', move);
      tab.addEventListener('pointerup', up);
    });
  };

  // The payload is the state; the channel only keeps it fresh. A page whose
  // socket never opens still shows the truth it was served with, and simply
  // stops learning - it never shows nothing by accident.
  const watchServed = () => {
    // A framed page draws no strip, exactly as the user button draws no menu
    // there (UIF-03). A portal and the application it frames are two pages of
    // this origin, both carrying the agent, and both drew one - the same strip
    // twice, one of them inside the frame. The portal is the page someone is
    // actually looking at, so it keeps it.
    //
    // The CLIENT decides, never the server: the HTML served is identical for
    // everyone, so no cache can hand one context the other's page.
    if (window.self !== window.top) return;
    data().then((d) => {
      if (d && d.schemeImposed && d.scheme) imposedScheme = d.scheme;
      servedNow = Array.isArray(d.served) ? d.served.slice() : [];
      renderServed(d.labels);
      onEvent('served', (s) => {
        if (!s || !s.name) return;
        servedNow = servedNow.filter((x) => x.name !== s.name).concat([s]);
        renderServed(d.labels);
      });
      onEvent('unserved', (s) => {
        if (!s || !s.name) return;
        servedNow = servedNow.filter((x) => x.name !== s.name);
        renderServed(d.labels);
      });
    }).catch(() => {});
  };

  window.meerkatPage = {
    data, applyScheme, pickScheme, applyLanguage, pickLanguage, resolvedLanguage, onEvent,
    onLanguage: (fn) => { if (typeof fn === 'function') listeners.push(fn); },
    // Called by the button after /logout: the cookie is already gone here, so
    // the other tabs learn it from the message rather than from a failure.
    signedOut: () => tellSession('ended'),
  };

  watchSession();
  watchServed();

  if (cfg.scheme === 'select') {
    applyScheme(getCookie(COOKIE_SCHEME) || 'auto');
    // In auto, follow the system live - unless the integrator settled it.
    darkMedia.addEventListener('change', () => {
      if (schemeImposed) return;
      if ((getCookie(COOKIE_SCHEME) || 'auto') === 'auto') applyScheme('auto');
    });
    // The integrator settled light/dark (THEME-05): wear it, whatever the
    // cookie or the system say.
    data().then(d => {
      if (d && d.schemeImposed) { schemeImposed = true; applyScheme(d.scheme); }
    }).catch(() => {});
    listenForScheme();
  }
  if (languages().length) {
    listenForLanguage();
    applyLanguageAtLoad();
  }
})();
`
