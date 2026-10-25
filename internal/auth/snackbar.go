package auth

// The notifications' snackbar (NOTIF-05): what a reader sees of a new
// notification when the browser will not show it on their desktop - an
// insecure origin, a permission never given or refused, or no Notification
// API at all.
//
// ONE piece of script for two places. The account button splices it in and
// keeps the stack in its shadow root, so it appears on every proxied page, in
// the portal bar or not; the notifications page's preview in the theme editor
// draws three of them over the page with the very same code, still. A preview
// drawn by a copy is a preview of the copy.
//
// Material 3's snackbar on the data plane's theme roles: the inverse surface,
// its text, the inverse primary for the action, the extra-small corner and
// elevation 3. The roles are the theme's light-dark() tokens, so the stack
// follows the color scheme its host wears.
//
// mkSnackbar(o) returns { el, show(it), attach(root) }:
//
//	o.label(key)  a string of the catalogue: notifOpen, notifClose, notifNew,
//	              notifAck, notifFailed
//	o.nonce       the nonce the host page gave its scripts, for the <style>
//	o.still       a preview: no timer, no motion, the buttons do nothing
//	it            { id, level, title, go, ack } - go() opens the notifications
//	              page; ack(), when the notification asks the reader to
//	              acknowledge it, does and resolves to whether it took
//
// A plain function declaration, no backtick: it lives inside Go raw strings.
const snackbarJS = `function mkSnackbar(o) {
    // Material Symbols (outlined), inline: no font, no request.
    const P = {
      info: 'M440-280h80v-240h-80v240Zm40-320q17 0 28.5-11.5T520-640q0-17-11.5-28.5T480-680q-17 0-28.5 11.5T440-640q0 17 11.5 28.5T480-600Zm0 520q-83 0-156-31.5T197-197q-54-54-85.5-127T80-480q0-83 31.5-156T197-763q54-54 127-85.5T480-880q83 0 156 31.5T763-763q54 54 85.5 127T880-480q0 83-31.5 156T763-197q-54 54-127 85.5T480-80Zm0-80q134 0 227-93t93-227q0-134-93-227t-227-93q-134 0-227 93t-93 227q0 134 93 227t227 93Zm0-320Z',
      success: 'm424-296 282-282-56-56-226 226-114-114-56 56 170 170Zm56 216q-83 0-156-31.5T197-197q-54-54-85.5-127T80-480q0-83 31.5-156T197-763q54-54 127-85.5T480-880q83 0 156 31.5T763-763q54 54 85.5 127T880-480q0 83-31.5 156T763-197q-54 54-127 85.5T480-80Zm0-80q134 0 227-93t93-227q0-134-93-227t-227-93q-134 0-227 93t-93 227q0 134 93 227t227 93Zm0-320Z',
      warning: 'm40-120 440-760 440 760H40Zm138-80h604L480-720 178-200Zm302-40q17 0 28.5-11.5T520-280q0-17-11.5-28.5T480-320q-17 0-28.5 11.5T440-280q0 17 11.5 28.5T480-240Zm-40-120h80v-200h-80v200Zm40-100Z',
      error: 'M480-280q17 0 28.5-11.5T520-320q0-17-11.5-28.5T480-360q-17 0-28.5 11.5T440-320q0 17 11.5 28.5T480-280Zm-40-160h80v-240h-80v240Zm40 360q-83 0-156-31.5T197-197q-54-54-85.5-127T80-480q0-83 31.5-156T197-763q54-54 127-85.5T480-880q83 0 156 31.5T763-763q54 54 85.5 127T880-480q0 83-31.5 156T763-197q-54 54-127 85.5T480-80Zm0-80q134 0 227-93t93-227q0-134-93-227t-227-93q-134 0-227 93t-93 227q0 134 93 227t227 93Zm0-320Z',
      close: 'm256-200-56-56 224-224-224-224 56-56 224 224 224-224 56 56-224 224 224 224-56 56-224-224-224 224Z'
    };
    // How long one stays, and how many stand at once: the oldest goes when a
    // fourth comes.
    const STAY = 6000, MAX = 3;
    const label = (k) => (o.label && o.label(k)) || '';
    const NS = 'http://www.w3.org/2000/svg';
    const icon = (name, cls) => {
      const s = document.createElementNS(NS, 'svg');
      s.setAttribute('viewBox', '0 -960 960 960');
      s.setAttribute('fill', 'currentColor');
      s.setAttribute('aria-hidden', 'true');
      s.setAttribute('focusable', 'false');
      s.setAttribute('class', cls);
      const p = document.createElementNS(NS, 'path');
      p.setAttribute('d', P[name] || P.info);
      s.appendChild(p);
      return s;
    };
    const CSS =
      '.mk-snacks { position: fixed; inset-inline: 0; bottom: 16px; z-index: 4; display: grid; justify-content: center;' +
      ' gap: 8px; padding: 0 16px; pointer-events: none; }' +
      '.mk-snack { pointer-events: auto; box-sizing: border-box; display: flex; align-items: center; gap: 4px;' +
      ' min-width: min(344px, calc(100vw - 32px)); max-width: min(560px, calc(100vw - 32px)); min-height: 48px;' +
      ' padding: 4px 4px 4px 16px; border-radius: 4px;' +
      ' background: var(--mk-inverse-surface, light-dark(#322f35, #e6e0e9));' +
      ' color: var(--mk-inverse-on-surface, light-dark(#f5eff7, #322f35));' +
      ' box-shadow: 0 1px 3px 0 rgb(0 0 0 / .3), 0 4px 8px 3px rgb(0 0 0 / .15);' +
      ' font-family: var(--mk-font, system-ui, sans-serif); font-size: 14px; line-height: 20px; letter-spacing: .25px;' +
      ' animation: mk-snack-in .25s cubic-bezier(.05, .7, .1, 1) both; }' +
      '.mk-snack.out { animation: mk-snack-out .15s cubic-bezier(.3, 0, .8, .15) both; }' +
      '.mk-snack-ic { flex: none; width: 20px; height: 20px; margin-inline-end: 8px; }' +
      '.mk-snack-t { flex: 1; min-width: 0; padding: 10px 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }' +
      '.mk-snack-sr { position: absolute; width: 1px; height: 1px; margin: -1px; padding: 0; border: 0;' +
      ' overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }' +
      '.mk-snack button { position: relative; flex: none; margin: 0; border: 0; background: transparent; cursor: pointer;' +
      ' font: inherit; color: inherit; -webkit-tap-highlight-color: transparent; }' +
      '.mk-snack button::before { content: ""; position: absolute; inset: 0; border-radius: inherit; background: currentColor;' +
      ' opacity: 0; transition: opacity .15s; pointer-events: none; }' +
      '.mk-snack button:hover::before { opacity: .08; }' +
      '.mk-snack button:focus-visible::before, .mk-snack button:active::before { opacity: .1; }' +
      '.mk-snack button:focus-visible { outline: 2px solid var(--mk-inverse-primary, light-dark(#d0bcff, #6750a4)); outline-offset: 1px; }' +
      '.mk-snack .mk-snack-act { height: 40px; padding: 0 12px; border-radius: 20px; font-weight: 500; letter-spacing: .1px;' +
      ' color: var(--mk-inverse-primary, light-dark(#d0bcff, #6750a4)); }' +
      '.mk-snack-x { width: 40px; height: 40px; padding: 0; border-radius: 50%; display: grid; place-items: center; }' +
      '.mk-snack-x svg { width: 24px; height: 24px; }' +
      '.mk-snacks.still .mk-snack { animation: none; }' +
      '@keyframes mk-snack-in { from { opacity: 0; transform: translateY(12px); } to { opacity: 1; transform: none; } }' +
      '@keyframes mk-snack-out { to { opacity: 0; } }' +
      '@keyframes mk-snack-fade { from { opacity: 0; } to { opacity: 1; } }' +
      '@media (prefers-reduced-motion: reduce) {' +
      ' .mk-snack { animation-name: mk-snack-fade; animation-duration: .1s; }' +
      ' .mk-snack.out { animation-name: mk-snack-out; animation-duration: .1s; }' +
      ' .mk-snack button::before { transition: none; } }';
    const el = document.createElement('div');
    const st = document.createElement('style');
    if (o.nonce) st.nonce = o.nonce;
    st.textContent = CSS;
    // The live region exists before anything is said into it, or a screen
    // reader misses the first one.
    const list = document.createElement('div');
    list.className = 'mk-snacks' + (o.still ? ' still' : '');
    list.setAttribute('role', 'status');
    list.setAttribute('aria-live', 'polite');
    el.append(st, list);
    const shown = () => [...list.children].filter((s) => !s.classList.contains('out'));
    const show = (it) => {
      if (!it || !it.title) return;
      if (it.id && shown().some((s) => s.dataset.id === it.id)) return;
      const s = document.createElement('div');
      const level = ['info', 'success', 'warning', 'error'].includes(it.level) ? it.level : 'info';
      s.className = 'mk-snack ' + level;
      if (it.id) s.dataset.id = it.id;
      s.appendChild(icon(level, 'mk-snack-ic'));
      const t = document.createElement('span');
      t.className = 'mk-snack-t';
      t.title = it.title;
      const sr = document.createElement('span');
      sr.className = 'mk-snack-sr';
      sr.textContent = label('notifNew') + ': ';
      t.append(sr, document.createTextNode(it.title));
      const act = document.createElement('button');
      act.type = 'button';
      act.className = 'mk-snack-act';
      act.textContent = label('notifOpen');
      // Acknowledge, before Open, when it asks for one: done from here, the
      // snackbar goes; refused, it says so and stays.
      let ack = null;
      if (it.ack && label('notifAck')) {
        ack = document.createElement('button');
        ack.type = 'button';
        ack.className = 'mk-snack-act';
        ack.textContent = label('notifAck');
      }
      const x = document.createElement('button');
      x.type = 'button';
      x.className = 'mk-snack-x';
      x.title = label('notifClose');
      x.setAttribute('aria-label', label('notifClose'));
      x.appendChild(icon('close', ''));
      s.append(t);
      if (ack) s.append(ack);
      s.append(act, x);
      let timer = 0, left = STAY, since = 0, hover = false, focus = false;
      const close = () => {
        if (s.classList.contains('out')) return;
        clearTimeout(timer);
        s.classList.add('out');
        // animationend, or at once when nothing animates (a hidden tab,
        // an animation turned off).
        const gone = () => s.remove();
        s.addEventListener('animationend', gone, { once: true });
        setTimeout(gone, 400);
      };
      const run = () => { since = Date.now(); timer = setTimeout(close, left); };
      const hold = () => { clearTimeout(timer); left = Math.max(1500, left - (Date.now() - since)); };
      const moved = (h, f) => {
        const was = hover || focus;
        hover = h; focus = f;
        if (was && !(hover || focus)) run();
        if (!was && (hover || focus)) hold();
      };
      if (!o.still) {
        act.addEventListener('click', () => { close(); if (it.go) it.go(); });
        if (ack) {
          ack.addEventListener('click', () => {
            if (ack.disabled) return;
            ack.disabled = true;
            Promise.resolve(it.ack()).then((ok) => {
              if (ok) { close(); return; }
              ack.disabled = false;
              ack.textContent = label('notifFailed') || label('notifAck');
            }, () => { ack.disabled = false; ack.textContent = label('notifFailed') || label('notifAck'); });
          });
        }
        x.addEventListener('click', close);
        s.addEventListener('keydown', (e) => { if (e.key === 'Escape') { e.stopPropagation(); close(); } });
        s.addEventListener('mouseenter', () => moved(true, focus));
        s.addEventListener('mouseleave', () => moved(false, focus));
        s.addEventListener('focusin', () => moved(hover, true));
        s.addEventListener('focusout', (e) => { if (!s.contains(e.relatedTarget)) moved(hover, false); });
        run();
      }
      const now = shown();
      if (now.length >= MAX) {
        for (const old of now.slice(0, now.length - MAX + 1)) {
          if (o.still) old.remove(); else old.dispatchEvent(new CustomEvent('mk-snack-close'));
        }
      }
      s.addEventListener('mk-snack-close', close);
      list.appendChild(s);
    };
    return {
      el,
      show,
      // Put the stack back where it was: a host that redraws its shadow root
      // wipes it, and the same nodes keep their timers.
      attach: (root) => { if (el.parentNode !== root) root.appendChild(el); }
    };
  }`

// SnackbarScript is the snackbar's script for a page outside this package:
// the notifications page's preview draws its stack with it.
func SnackbarScript() string { return snackbarJS }
