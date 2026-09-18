package auth

// portalNavJS is the <meerkat-portal-nav> custom element (PORTAL-01): CHROME,
// and nothing but. Vanilla, shadow DOM, themed by the data plane's own theme
// CSS (rescoped :root -> :host), system colors as a fallback. It fetches
// portal.json, filters nothing itself (the server already dropped what the
// caller may not open), and mounts the existing <meerkat-user-button> in its
// bar with in-portal so the account menu keeps working without a second
// component.
//
// The RAIL is modelled on @softwarity/rail-nav: it reserves its COLLAPSED width
// (72px) with page padding - never an overlay over content - and EXPANDS into a
// drawer over a backdrop, toggled by a burger, auto-collapsing on a click. The
// side (left/right) anchors the rail and mirrors the burger. Icons are the
// module's stored SVG rendered as a CSS mask (no icon font is ever loaded).
//
// It NEVER shows itself inside a frame (self !== top). No backtick may appear in
// this string: it is a Go raw literal. Every template is built by concatenation.
const portalNavJS = `(function () {
  'use strict';
  if (window.customElements && customElements.get('meerkat-portal-nav')) return;

  var SRC = '/meerkat/portal.json';
  var RAIL_W = 72, RAIL_EXP = 280, HEADER_H = 56, STRIP_H = 48;
  // The account button's height inside the bar: the same 40px box the launcher
  // and the chevrons use, so the row reads as one line of controls.
  var BTN_H = 40;
  // Below this viewport width the header drops its tabs and keeps only the app
  // selector (the waffle): a phone has no room for a tab strip.
  var NARROW = 560;

  // Chrome icons: Material Symbols path data (viewBox 0 -960 960 960).
  var P = {
    apps: 'M240-160q-33 0-56.5-23.5T160-240q0-33 23.5-56.5T240-320q33 0 56.5 23.5T320-240q0 33-23.5 56.5T240-160Zm240 0q-33 0-56.5-23.5T400-240q0-33 23.5-56.5T480-320q33 0 56.5 23.5T560-240q0 33-23.5 56.5T480-160Zm240 0q-33 0-56.5-23.5T640-240q0-33 23.5-56.5T720-320q33 0 56.5 23.5T800-240q0 33-23.5 56.5T720-160ZM240-400q-33 0-56.5-23.5T160-480q0-33 23.5-56.5T240-560q33 0 56.5 23.5T320-480q0 33-23.5 56.5T240-400Zm240 0q-33 0-56.5-23.5T400-480q0-33 23.5-56.5T480-560q33 0 56.5 23.5T560-480q0 33-23.5 56.5T480-400Zm240 0q-33 0-56.5-23.5T640-480q0-33 23.5-56.5T720-560q33 0 56.5 23.5T800-480q0 33-23.5 56.5T720-400ZM240-640q-33 0-56.5-23.5T160-720q0-33 23.5-56.5T240-800q33 0 56.5 23.5T320-720q0 33-23.5 56.5T240-640Zm240 0q-33 0-56.5-23.5T400-720q0-33 23.5-56.5T480-800q33 0 56.5 23.5T560-720q0 33-23.5 56.5T480-640Zm240 0q-33 0-56.5-23.5T640-720q0-33 23.5-56.5T720-800q33 0 56.5 23.5T800-720q0 33-23.5 56.5T720-640Z',
    menu: 'M120-240v-80h720v80H120Zm0-200v-80h720v80H120Zm0-200v-80h720v80H120Z',
    menuOpen: 'M120-240v-80h520v80H120Zm664-40L584-480l200-200 56 56-144 144 144 144-56 56ZM120-440v-80h400v80H120Zm0-200v-80h520v80H120Z',
    home: 'M240-200h120v-240h240v240h120v-360L480-740 240-560v360Zm-80 80v-480l320-240 320 240v480H520v-240h-80v240H160Z',
    account: 'M234-276q51-39 114-61.5T480-360q69 0 132 22.5T726-276q35-41 54.5-93T800-480q0-133-93.5-226.5T480-800q-133 0-226.5 93.5T160-480q0 59 19.5 111t54.5 93Zm246-164q-59 0-99.5-40.5T340-580q0-59 40.5-99.5T480-720q59 0 99.5 40.5T620-580q0 59-40.5 99.5T480-440Zm0 360q-83 0-156-31.5T197-197q-54-54-85.5-127T80-480q0-83 31.5-156T197-763q54-54 127-85.5T480-880q83 0 156 31.5T763-763q54 54 85.5 127T880-480q0 83-31.5 156T763-197q-54 54-127 85.5T480-80Z',
    left: 'M560-240 320-480l240-240 56 56-184 184 184 184-56 56Z',
    right: 'M504-480 320-664l56-56 240 240-240 240-56-56 184-184Z',
    lightMode: 'M480-360q50 0 85-35t35-85q0-50-35-85t-85-35q-50 0-85 35t-35 85q0 50 35 85t85 35Zm0 80q-83 0-141.5-58.5T280-480q0-83 58.5-141.5T480-680q83 0 141.5 58.5T680-480q0 83-58.5 141.5T480-280ZM200-440H40v-80h160v80Zm720 0H760v-80h160v80ZM440-760v-160h80v160h-80Zm0 720v-160h80v160h-80ZM256-650l-101-97 57-59 96 100-52 56Zm492 496-97-101 53-55 101 97-57 59Zm-98-550 97-101 59 57-100 96-56-52ZM154-212l101-97 55 53-97 101-59-57Z',
    darkMode: 'M480-120q-150 0-255-105T120-480q0-150 105-255t255-105q14 0 27.5 1t26.5 3q-41 29-65.5 75.5T444-660q0 90 63 153t153 63q55 0 101-24.5t75-65.5q2 13 3 26.5t1 27.5q0 150-105 255T480-120Z'
  };
  function chrome(path) {
    return '<svg viewBox="0 -960 960 960" aria-hidden="true" focusable="false"><path d="' + path + '"/></svg>';
  }
  function chromeC(path, cls) {
    return '<svg class="' + cls + '" viewBox="0 -960 960 960" aria-hidden="true" focusable="false"><path d="' + path + '"/></svg>';
  }
  var HOME_SVG = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 -960 960 960"><path d="' + P.home + '"/></svg>';

  var STYLE =
    ':host{all:initial;color-scheme:inherit;' +
      'font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;}' +
    '*{box-sizing:border-box;}' +
    // an author display rule beats the UA [hidden]{display:none}; restore it so
    // el.hidden actually hides (the launcher, shown only on overflow).
    '[hidden]{display:none!important;}' +
    // colour helpers keyed on the data-plane --mk-* tokens, system fallbacks
    '.mk{' +
      '--surface:var(--mk-surface,Canvas);' +
      '--surfacec:var(--mk-surface-container,Canvas);' +
      '--onsurface:var(--mk-on-surface,CanvasText);' +
      '--onvar:var(--mk-on-surface-variant,color-mix(in srgb,CanvasText 65%,transparent));' +
      '--outline:var(--mk-outline,color-mix(in srgb,CanvasText 22%,transparent));' +
      '--primary:var(--mk-primary,#3355dd);' +
      '--onprimary:var(--mk-on-primary,#fff);' +
      '--hover:color-mix(in srgb,var(--onsurface) 8%,transparent);' +
      '--active:color-mix(in srgb,var(--primary) 20%,transparent);' +
      '--error:var(--mk-error,#c0392b);}' +
    // shared icon (mask of the stored svg) and initial fallback
    '.ico{display:inline-block;width:22px;height:22px;flex:0 0 auto;background:currentColor;' +
      '-webkit-mask-size:contain;mask-size:contain;-webkit-mask-repeat:no-repeat;mask-repeat:no-repeat;' +
      '-webkit-mask-position:center;mask-position:center;}' +
    '.ini{display:inline-flex;align-items:center;justify-content:center;width:22px;height:22px;flex:0 0 auto;' +
      'border-radius:50%;background:var(--primary);color:var(--onprimary);font-size:12px;font-weight:600;}' +
    '.cr{display:inline-flex;align-items:center;justify-content:center;}' +
    '.cr svg{width:24px;height:24px;fill:currentColor;}' +
    '.badge{display:inline-flex;align-items:center;justify-content:center;min-width:16px;height:16px;padding:0 4px;' +
      'border-radius:8px;background:var(--error);color:#fff;font-size:11px;line-height:1;}' +
    // ---- header bar (parents as tabs) ----
    '.top{position:fixed;top:0;left:0;right:0;height:' + HEADER_H + 'px;z-index:2147483102;display:flex;align-items:center;' +
      'gap:8px;padding:0 10px;background:var(--surfacec);color:var(--onsurface);' +
      'border-bottom:1px solid var(--outline);}' +
    '.brand{display:flex;align-items:center;gap:8px;font-weight:600;font-size:15px;white-space:nowrap;' +
      'text-decoration:none;color:inherit;padding:0 6px;flex:0 0 auto;}' +
    '.brand img{height:26px;width:auto;display:block;}' +
    // padding gives the selected tab's outline room: overflow-x clips at the
    // padding box, so with none the first (and last) tab's outline was shaved.
    '.tabs{display:flex;align-items:center;gap:2px;overflow-x:auto;scrollbar-width:none;flex:1 1 auto;min-width:0;padding:4px 6px;}' +
    '.tabs::-webkit-scrollbar{display:none;}' +
    '.chev{flex:0 0 auto;display:inline-flex;align-items:center;justify-content:center;width:32px;height:32px;border:0;' +
      'background:transparent;color:inherit;cursor:pointer;border-radius:50%;}' +
    '.chev:hover{background:var(--hover);}' +
    '.chev[hidden]{display:none;}' +
    'a.tab,button.tab{display:inline-flex;align-items:center;gap:8px;text-decoration:none;color:var(--onvar);' +
      'border:0;background:transparent;font:inherit;cursor:pointer;white-space:nowrap;border-radius:10px;' +
      'padding:8px 12px;line-height:1;}' +
    'a.tab:hover,button.tab:hover{background:var(--hover);}' +
    'a.tab.cur,button.tab.cur{background:var(--active);color:var(--onsurface);}' +
    // edit-mode selection outline (console preview)
    '.tab.sel,.ritem.sel .pill{outline:2px solid var(--primary);outline-offset:1px;}' +
    '.ritem.sel{cursor:pointer;}' +
    '.spacer{flex:1 1 auto;}' +
    '.act{flex:0 0 auto;display:inline-flex;align-items:center;gap:4px;}' +
    '.launch{display:inline-flex;align-items:center;justify-content:center;width:40px;height:40px;border:0;' +
      'background:transparent;color:var(--onvar);cursor:pointer;border-radius:50%;}' +
    '.launch:hover{background:var(--hover);}' +
    // ---- children strip (rail mode) ----
    '.strip{position:fixed;top:0;height:' + STRIP_H + 'px;z-index:2147483101;display:flex;align-items:center;gap:2px;' +
      'padding:0 8px;overflow-x:auto;scrollbar-width:none;background:var(--surface);color:var(--onsurface);' +
      'border-bottom:1px solid var(--outline);}' +
    '.strip::-webkit-scrollbar{display:none;}' +
    // ---- rail (rail-nav) ----
    '.rail{position:fixed;top:0;bottom:0;width:' + RAIL_W + 'px;z-index:2147483100;display:flex;flex-direction:column;' +
      'background:var(--surface);color:var(--onsurface);transition:width .2s ease;overflow:visible;}' +
    // Expanded, the rail is a drawer over everything (backdrop below it): lift it
    // above the top bar and the children strip so its logo is not clipped by them.
    '.rail.exp{width:' + RAIL_EXP + 'px;z-index:2147483104;}' +
    '.rail.l{left:0;border-right:1px solid var(--outline);}' +
    '.rail.r{right:0;border-left:1px solid var(--outline);}' +
    '.rail.exp.l{box-shadow:4px 0 12px rgba(0,0,0,.22);}' +
    '.rail.exp.r{box-shadow:-4px 0 12px rgba(0,0,0,.22);}' +
    // The burger stays on the same axis as the item icons below it, collapsed
    // and expanded alike: those glyphs sit at 12px (ritems pad) + 13px (pill
    // pad) = centre ~36px. Collapsed centers the burger in the 72px rail (=36).
    // Expanded, flex-start with padding-left 24px puts the 24px burger's centre
    // back at 36px; the logo then sits beside it.
    '.rhead{display:flex;align-items:center;justify-content:center;gap:0;height:' + HEADER_H + 'px;' +
      'flex:0 0 auto;padding:0;cursor:pointer;color:var(--onsurface);}' +
    '.rail.exp .rhead{justify-content:flex-start;padding:0 24px;gap:10px;}' +
    '.rhead:hover{background:var(--hover);}' +
    '.rail.r.exp .rhead{flex-direction:row-reverse;}' +
    // Burger animates like @softwarity/rail-nav: the hamburger and the menu-open
    // glyphs are stacked and cross-fade with a quarter turn when the rail toggles
    // (no icon swap, so it never jumps). The right side mirrors the whole thing.
    '.burger{position:relative;width:24px;height:24px;flex:0 0 auto;color:inherit;}' +
    '.rail.r .burger{transform:scaleX(-1);}' +
    '.burger svg{position:absolute;top:0;left:0;width:24px;height:24px;fill:currentColor;' +
      'transition:opacity .3s ease,transform .3s ease;}' +
    '.burger .ic-menu{opacity:1;transform:rotate(0deg);}' +
    '.burger .ic-open{opacity:0;transform:rotate(-90deg);}' +
    '.rail.exp .burger .ic-menu{opacity:0;transform:rotate(90deg);}' +
    '.rail.exp .burger .ic-open{opacity:1;transform:rotate(0deg);}' +
    '.rlogo{display:flex;align-items:center;gap:8px;font-weight:600;font-size:14px;white-space:nowrap;overflow:hidden;' +
      'text-decoration:none;color:inherit;opacity:0;width:0;transition:opacity .15s ease .05s;}' +
    '.rail.exp .rlogo{opacity:1;width:auto;}' +
    '.rlogo img{height:24px;width:auto;display:block;}' +
    '.ritems{display:flex;flex-direction:column;flex:1 1 auto;min-height:0;overflow-y:auto;overflow-x:hidden;' +
      'scrollbar-width:none;padding:8px 12px;gap:0;}' +
    '.ritems::-webkit-scrollbar{display:none;}' +
    '.rfoot{flex:0 0 auto;display:flex;flex-direction:column;align-items:center;gap:4px;padding:8px 12px;}' +
    '.rail.exp .rfoot{align-items:stretch;}' +
    'a.ritem,button.ritem{display:flex;flex-direction:column;align-items:flex-start;gap:4px;text-decoration:none;' +
      'color:var(--onvar);border:0;background:transparent;font:inherit;cursor:pointer;min-height:56px;width:100%;' +
      'padding:0;transition:gap .2s ease;}' +
    '.rail.r a.ritem,.rail.r button.ritem{align-items:flex-end;}' +
    '.rail.exp a.ritem,.rail.exp button.ritem{gap:0;}' +
    '.pill{position:relative;display:flex;align-items:center;justify-content:flex-start;width:48px;height:32px;' +
      'border-radius:9999px;margin-top:12px;padding-left:13px;transition:background .2s ease,width .2s ease,' +
      'height .2s ease,margin .2s ease;}' +
    '.rail.r .pill{justify-content:flex-end;padding-left:0;padding-right:13px;}' +
    'a.ritem:hover .pill,button.ritem:hover .pill{background:var(--hover);}' +
    'a.ritem.cur .pill,button.ritem.cur .pill{background:var(--active);color:var(--onsurface);}' +
    '.rail.exp a.ritem .pill,.rail.exp button.ritem .pill{width:auto;height:48px;padding:0 16px 0 13px;gap:12px;margin-top:0;}' +
    '.rail.r.exp a.ritem .pill,.rail.r.exp button.ritem .pill{flex-direction:row-reverse;padding:0 13px 0 16px;}' +
    '.pill .badge{position:absolute;top:-6px;right:-6px;}' +
    '.lblo{font-size:12px;font-weight:500;line-height:16px;text-align:center;white-space:nowrap;overflow:hidden;' +
      'text-overflow:ellipsis;width:48px;max-height:16px;opacity:1;transition:opacity .1s ease,max-height .2s ease;}' +
    '.rail.exp a.ritem .lblo,.rail.exp button.ritem .lblo{opacity:0;max-height:0;}' +
    '.lbli{font-size:14px;font-weight:500;line-height:24px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;' +
      'width:0;opacity:0;}' +
    '.rail.exp a.ritem .lbli,.rail.exp button.ritem .lbli{width:auto;flex:1;opacity:1;transition:opacity .15s ease .1s;}' +
    '.rail.r .lbli{text-align:right;}' +
    // backdrop for an expanded rail
    '.backdrop{position:fixed;top:0;bottom:0;left:0;right:0;z-index:2147483099;background:rgba(0,0,0,.4);' +
      'opacity:0;pointer-events:none;transition:opacity .3s ease;}' +
    '.backdrop.on{opacity:1;pointer-events:auto;}' +
    // ---- launcher grid ----
    '.grid{position:fixed;z-index:2147483110;background:var(--surface);color:var(--onsurface);' +
      'border:1px solid var(--outline);border-radius:14px;padding:12px;box-shadow:0 12px 40px rgba(0,0,0,.28);' +
      'display:grid;grid-template-columns:repeat(3,92px);gap:4px;max-width:320px;}' +
    '.grid[hidden]{display:none;}' +
    '.grid a.tile,.grid button.tile{display:flex;flex-direction:column;align-items:center;gap:6px;text-decoration:none;' +
      'color:inherit;padding:12px 6px;border-radius:12px;text-align:center;border:0;background:transparent;' +
      'font:inherit;cursor:pointer;}' +
    '.grid a.tile:hover,.grid button.tile:hover{background:var(--surfacec);}' +
    '.grid .tlbl{font-size:12px;max-width:80px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}' +
    'meerkat-user-button{flex:0 0 auto;}';

  function iconMask(el, svgStr) {
    var uri = 'url("data:image/svg+xml;utf8,' + encodeURIComponent(svgStr) + '")';
    el.style.webkitMaskImage = uri;
    el.style.maskImage = uri;
  }
  function longestMatch(entries, path) {
    var best = null, len = -1;
    for (var i = 0; i < entries.length; i++) {
      var h = entries[i].href;
      if (!h) continue;
      var hit = h === '/' ? (path === '/') : (path === h || path.indexOf(h + '/') === 0);
      if (hit && h.length > len) { best = entries[i]; len = h.length; }
    }
    return best;
  }

  class MeerkatPortalNav extends HTMLElement {
    constructor() {
      super();
      this._root = this.attachShadow({ mode: 'open' });
      this._data = null;
      this._current = null;
      this._expanded = false;
      // Preview only: which scheme the editor forces so both looks can be seen.
      // Starts on the viewer's own preference; the bar's scheme button flips it.
      this._previewScheme = (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) ? 'dark' : 'light';
    }

    connectedCallback() {
      // Edit preview (PORTAL-01 console): the console's iframe drives the bar
      // with a payload over postMessage and reads back which entry was clicked,
      // so the SAME component renders the editor. No fetch, and the frame guard
      // is off precisely because this IS a frame on purpose.
      this._preview = this.hasAttribute('preview');
      this._edit = this.hasAttribute('edit');
      if (this._preview) {
        var self = this;
        this._onMsg = function (e) {
          var m = e && e.data;
          if (!m || m.type !== 'mk-portal') return;
          self._apply(m.payload || {});
        };
        window.addEventListener('message', this._onMsg);
        // Tell the host we are ready to receive a payload.
        try { window.parent && window.parent.postMessage({ type: 'mk-portal-ready' }, '*'); } catch (e2) {}
        return;
      }
      if (window.self !== window.top) return; // an embedded page is not a portal
      this._load();
    }
    disconnectedCallback() {
      this._clearOffset();
      if (this._onDoc) document.removeEventListener('click', this._onDoc);
      if (this._onResize) window.removeEventListener('resize', this._onResize);
      if (this._onMsg) window.removeEventListener('message', this._onMsg);
    }

    _load() {
      var self = this, cached = null;
      try { cached = sessionStorage.getItem('mk-portal'); } catch (e) {}
      if (cached) { try { self._apply(JSON.parse(cached)); } catch (e) {} }
      fetch(SRC, { credentials: 'same-origin', cache: 'no-store' })
        .then(function (r) { return r.ok ? r.json() : null; })
        .then(function (d) {
          if (!d) return;
          try { sessionStorage.setItem('mk-portal', JSON.stringify(d)); } catch (e) {}
          self._apply(d);
        }).catch(function () {});
    }

    _apply(d) {
      d = d || {};
      if (!d.layout) d.layout = 'header';
      if (!d.side) d.side = 'left';
      if (!d.display) d.display = 'both';
      if (!d.parents) d.parents = [];
      this._data = d;
      // On a served page an off or empty portal shows nothing; the editor
      // preview always renders the frame, even with no modules yet.
      if (!this._preview && (!d.enabled || !d.parents.length)) { this._clear(); return; }
      // In edit, the host tells us which entry is selected; otherwise the
      // current one is matched from the URL.
      this._current = this._preview
        ? (this._byId(d.selected) || d.parents[0] || null)
        : (longestMatch(d.parents, location.pathname) || d.parents[0]);
      this._render();
    }

    // Find the parent that owns the selected id ("i" or "i/j"), so its children
    // show while a child of it is selected.
    _byId(id) {
      if (!id) return null;
      var top = String(id).split('/')[0];
      var ps = (this._data && this._data.parents) || [];
      return ps[parseInt(top, 10)] || null;
    }
    _clear() { this._root.innerHTML = ''; this._clearOffset(); }
    _clearOffset() {
      try {
        var s = document.documentElement.style;
        s.paddingTop = ''; s.paddingLeft = ''; s.paddingRight = '';
        s.boxSizing = ''; s.height = '';
        s.removeProperty('--meerkat-top');
        s.removeProperty('--meerkat-left');
        s.removeProperty('--meerkat-right');
      } catch (e) {}
    }

    _showIcon() { return this._data.display !== 'label'; }
    _showLabel() { return this._data.display !== 'icon'; }

    // force=true means both icon and label regardless of the display setting -
    // the rail always shows both; the display setting only trims the header tabs.
    _glyph(entry, force) {
      var showIcon = force || this._showIcon(), showLabel = force || this._showLabel();
      if (showIcon && entry.icon) {
        var s = document.createElement('span'); s.className = 'ico'; iconMask(s, entry.icon); return s;
      }
      if (showIcon || !showLabel) {
        var i = document.createElement('span'); i.className = 'ini';
        i.textContent = (entry.label || '?').trim().charAt(0).toUpperCase(); return i;
      }
      return null;
    }
    _badgeEl(entry) {
      if (!entry.badge) return null;
      var b = document.createElement('span'); b.className = 'badge'; b.textContent = entry.badge; return b;
    }

    // ---- top bar (parents as tabs) --------------------------------------
    _tab(entry, cur) {
      var self = this;
      var a = document.createElement(entry.href ? 'a' : 'button');
      a.className = 'tab' + (cur ? ' cur' : '');
      if (entry.href) a.setAttribute('href', entry.href); else a.setAttribute('type', 'button');
      if (entry.description) a.setAttribute('title', entry.description);
      var g = this._glyph(entry); if (g) a.appendChild(g);
      if (this._showLabel()) {
        var l = document.createElement('span'); l.textContent = entry.label || ''; a.appendChild(l);
      }
      var bd = this._badgeEl(entry); if (bd) a.appendChild(bd);
      this._wireSelect(a, entry);
      return a;
    }

    // In edit mode a click selects the entry (told back to the console) instead
    // of navigating, and the selected one is outlined.
    _wireSelect(el, entry) {
      // Editing is off when the attribute is absent OR the host turned it off in
      // the payload (the console does this at tablet/phone widths - a preview,
      // not an editor). Then a click selects nothing and no outline is drawn.
      if (!this._edit || (this._data && this._data.edit === false) || entry.id == null) return;
      if (entry.id === (this._data && this._data.selected)) el.classList.add('sel');
      el.addEventListener('click', function (e) {
        e.preventDefault(); e.stopPropagation();
        try { window.parent && window.parent.postMessage({ type: 'mk-portal-select', id: entry.id }, '*'); } catch (x) {}
      });
    }

    _brand() {
      // The mark is NOT a link: on a data-plane page a href to "/" walks out of
      // the current app (often back to the console). It is decoration; the tabs
      // and rail do the navigating. In the rail it sits in the burger head, so a
      // click there still bubbles up to toggle the drawer.
      var d = this._data, el = document.createElement('span'); el.className = 'brand';
      var appName = (d.brand && d.brand.appName) || '';
      if (d.brand && d.brand.logo) {
        var img = document.createElement('img'); img.setAttribute('src', d.brand.logo);
        img.setAttribute('alt', appName); el.appendChild(img);
        // Optionally the branding app name sits beside the logo, like the icon.
        if (d.showName && appName) {
          var nm = document.createElement('span'); nm.className = 'bname'; nm.textContent = appName; el.appendChild(nm);
        }
      } else {
        // With no logo the name IS the mark - always shown.
        var t = document.createElement('span'); t.textContent = appName; el.appendChild(t);
      }
      return el;
    }

    _userBtn(position) {
      // In the console preview there is no session to speak of, and mounting the
      // real button would fetch cross-plane. Stand in for it with the piece that
      // is useful here: the color-scheme selector the real user button carries,
      // wired to flip the whole preview between light and dark so both are seen.
      if (this._preview) {
        var self = this, dark = this._previewScheme === 'dark';
        var sc = document.createElement('button'); sc.className = 'launch'; sc.setAttribute('type', 'button');
        var lbl = dark ? 'Preview in light' : 'Preview in dark';
        sc.setAttribute('title', lbl); sc.setAttribute('aria-label', lbl);
        sc.innerHTML = chrome(dark ? P.lightMode : P.darkMode);
        sc.addEventListener('click', function (e) { e.stopPropagation(); self._togglePreviewScheme(); });
        return sc;
      }
      var d = this._data, b = document.createElement('meerkat-user-button');
      b.setAttribute('in-portal', '');
      // The button's own default (24px) is a discreet corner badge; in the bar
      // it is one control among the tabs, and it has to read at their scale -
      // sized off the surface it sits in (a 56px header, a 72px rail).
      b.setAttribute('height', String(BTN_H));
      // position's first word is the anchored edge and decides which way the
      // menu opens: at the bottom of a rail it must open UPWARD, not down.
      b.setAttribute('position', position || 'top-right');
      if (d.languages && d.languages.length) b.setAttribute('languages', d.languages.join(','));
      if (d.schemeImposed && (d.scheme === 'light' || d.scheme === 'dark')) b.setAttribute('scheme-wear', d.scheme);
      else b.setAttribute('scheme', 'select');
      return b;
    }

    _launcher() {
      var self = this, b = document.createElement('button');
      b.className = 'launch'; b.setAttribute('type', 'button');
      b.setAttribute('aria-label', (this._data.labels && this._data.labels.applications) || 'Applications');
      b.innerHTML = chrome(P.apps);
      // Shown only when the primary nav does not fit (decided in _updateOverflow):
      // when everything is reachable, the app switcher would be a click to where
      // one already is.
      b.hidden = true;
      b.addEventListener('click', function (e) {
        e.stopPropagation();
        self._grid.hidden = !self._grid.hidden;
        if (!self._grid.hidden) self._placeGrid(b);
      });
      this._launchEl = b;
      return b;
    }
    _buildGrid() {
      var grid = document.createElement('div'); grid.className = 'grid'; grid.hidden = true;
      for (var i = 0; i < this._data.parents.length; i++) {
        var p = this._data.parents[i];
        var t = document.createElement(p.href ? 'a' : 'button'); t.className = 'tile';
        if (p.href) t.setAttribute('href', p.href); else t.setAttribute('type', 'button');
        if (p.description) t.setAttribute('title', p.description);
        var g = this._glyph(p) || document.createElement('span'); if (g.className === 'ico') g.style.width = g.style.height = '24px';
        var wrap = document.createElement('span'); wrap.className = 'cr'; wrap.appendChild(g); t.appendChild(wrap);
        var l = document.createElement('span'); l.className = 'tlbl'; l.textContent = p.label || ''; t.appendChild(l);
        grid.appendChild(t);
      }
      return grid;
    }
    _placeGrid(anchor) {
      var r = anchor.getBoundingClientRect(), g = this._grid;
      g.style.top = (r.bottom + 6) + 'px';
      g.style.right = Math.max(8, window.innerWidth - r.right) + 'px';
      g.style.left = 'auto';
    }

    // ---- rail (rail-nav) ------------------------------------------------
    _railItem(entry, cur) {
      var self = this;
      var a = document.createElement(entry.href ? 'a' : 'button');
      a.className = 'ritem' + (cur ? ' cur' : '');
      if (entry.href) a.setAttribute('href', entry.href); else a.setAttribute('type', 'button');
      if (entry.description) a.setAttribute('title', entry.description);
      var pill = document.createElement('span'); pill.className = 'pill';
      var g = this._glyph(entry, true); // rail: always the icon
      if (g) pill.appendChild(g);
      var bd = this._badgeEl(entry); if (bd) pill.appendChild(bd);
      // Both labels always exist so the collapse/expand is a CSS cross-fade (no
      // rebuild): the inline one shows expanded, the stacked one collapsed.
      var li = document.createElement('span'); li.className = 'lbli'; li.textContent = entry.label || ''; pill.appendChild(li);
      a.appendChild(pill);
      var lo = document.createElement('span'); lo.className = 'lblo'; lo.textContent = entry.label || ''; a.appendChild(lo);
      a.addEventListener('click', function () { if (self._expanded) self._setExpanded(false); });
      this._wireSelect(a, entry);
      return a;
    }

    // Build a rail. entries render as items; primary rails also carry a logo,
    // the launcher and the user button. homeFor, when set, prepends a "home"
    // row pointing back at that parent.
    _buildRail(entries, opts) {
      var self = this, d = this._data;
      var rail = document.createElement('nav');
      rail.className = 'rail ' + (d.side === 'right' ? 'r' : 'l') + (this._expanded ? ' exp' : '');
      this._railEl = rail; // toggled (not rebuilt) on expand so the CSS animates

      var head = document.createElement('div'); head.className = 'rhead';
      // Both glyphs are mounted; the CSS cross-fades between them on toggle.
      var burger = document.createElement('span'); burger.className = 'burger';
      burger.innerHTML = chromeC(P.menu, 'ic-menu') + chromeC(P.menuOpen, 'ic-open');
      head.appendChild(burger);
      if (opts.logo) {
        var lg = this._brand(); lg.className = 'rlogo'; head.appendChild(lg);
      }
      head.addEventListener('click', function () { self._setExpanded(!self._expanded); });
      rail.appendChild(head);

      var items = document.createElement('div'); items.className = 'ritems';
      this._ritemsEl = items;
      if (opts.homeFor) items.appendChild(this._railItem(this._homeEntry(opts.homeFor), opts.homeCurrent));
      for (var i = 0; i < entries.length; i++) items.appendChild(this._railItem(entries[i], entries[i] === opts.currentEntry));
      rail.appendChild(items);

      if (opts.footer) {
        var foot = document.createElement('div'); foot.className = 'rfoot';
        foot.appendChild(this._launcher());
        foot.appendChild(this._userBtn('bottom-' + (d.side === 'right' ? 'right' : 'left')));
        rail.appendChild(foot);
      }
      return rail;
    }

    _setExpanded(v) {
      if (this._expanded === v) return;
      this._expanded = v;
      // Toggle the class on the standing DOM (do NOT rebuild): the CSS then
      // animates the width, the burger cross-fade and the labels. A rebuild would
      // snap to the end state. The offset stays put - collapsed width is always
      // reserved, the drawer just overlays. The backdrop fades with it.
      if (this._railEl) this._railEl.classList.toggle('exp', v);
      if (this._backdropEl) this._backdropEl.classList.toggle('on', v);
      var self = this;
      requestAnimationFrame(function () { self._updateArrows(); });
    }

    // Preview scheme switch: force light or dark on the host AND the iframe root,
    // overriding the theme's "light dark" so light-dark() resolves to the chosen
    // one - the bar and the page behind it flip together.
    _togglePreviewScheme() {
      this._previewScheme = this._previewScheme === 'dark' ? 'light' : 'dark';
      this._render();
    }

    // The "back to this module" row for a parent that has children: its own
    // label (or homeLabel) and a home glyph, so it reads as the way back.
    _homeEntry(parent) {
      return {
        id: parent.id, // selecting the home row selects its parent
        label: parent.homeLabel || parent.label,
        href: parent.href,
        description: parent.description,
        icon: HOME_SVG,
      };
    }

    // ---- children strip (rail mode) -------------------------------------
    _buildStrip(parent, children, currentChild) {
      var d = this._data;
      var strip = document.createElement('nav'); strip.className = 'strip';
      strip.style[d.side === 'right' ? 'right' : 'left'] = RAIL_W + 'px';
      strip.style[d.side === 'right' ? 'left' : 'right'] = '0';
      strip.appendChild(this._tab(this._homeEntry(parent), !currentChild && !!parent.href));
      for (var i = 0; i < children.length; i++) strip.appendChild(this._tab(children[i], children[i] === currentChild));
      return strip;
    }

    // ---- render ---------------------------------------------------------
    _render() {
      var self = this, d = this._data;
      this._root.innerHTML = '';
      if (this._preview) {
        // Inline color-scheme beats the theme's ":host/:root {color-scheme:light dark}"
        // rules, so the chosen scheme wins for the bar and the page behind it.
        this.style.colorScheme = this._previewScheme;
        try { document.documentElement.style.colorScheme = this._previewScheme; } catch (e) {}
      }
      var style = document.createElement('style');
      style.textContent = '.mk{}' + (d.themeCss || '') + STYLE;
      this._root.appendChild(style);
      var wrap = document.createElement('div'); wrap.className = 'mk'; this._root.appendChild(wrap);
      this._tabsEl = this._leftEl = this._rightEl = this._ritemsEl = this._launchEl = null;
      this._railEl = this._backdropEl = this._spacerEl = null;

      this._grid = this._buildGrid(); wrap.appendChild(this._grid);
      var backdrop = document.createElement('div'); backdrop.className = 'backdrop' + (this._expanded ? ' on' : '');
      backdrop.addEventListener('click', function () { self._setExpanded(false); });
      wrap.appendChild(backdrop);

      var children = (this._current && this._current.children) || [];
      var currentChild = longestMatch(children, location.pathname);

      if (d.layout === 'rail') {
        // parents in the rail, children in a top strip
        var rail = this._buildRail(d.parents, {
          logo: true, footer: true, currentEntry: this._current
        });
        wrap.appendChild(rail);
        var hasStrip = children.length > 0;
        if (hasStrip) wrap.appendChild(this._buildStrip(this._current, children, currentChild));
        this._offset('rail', d.side, hasStrip);
      } else {
        // parents in the header, children in the rail
        wrap.appendChild(this._buildHeader(d.parents));
        var hasRail = children.length > 0;
        if (hasRail) {
          var crail = this._buildRail(children, {
            logo: false, footer: false, homeFor: this._current,
            homeCurrent: !currentChild && !!this._current.href, currentEntry: currentChild
          });
          crail.style.top = HEADER_H + 'px';
          wrap.appendChild(crail);
        }
        this._offset('header', d.side, hasRail);
      }

      if (this._onDoc) document.removeEventListener('click', this._onDoc);
      this._onDoc = function () { if (self._grid) self._grid.hidden = true; };
      document.addEventListener('click', this._onDoc);
      if (this._onResize) window.removeEventListener('resize', this._onResize);
      this._onResize = function () { self._updateArrows(); };
      window.addEventListener('resize', this._onResize);
      requestAnimationFrame(function () { self._updateArrows(); });
    }

    _buildHeader(parents) {
      var self = this, bar = document.createElement('nav'); bar.className = 'top';
      bar.appendChild(this._brand());
      var left = document.createElement('button'); left.className = 'chev'; left.type = 'button';
      left.setAttribute('aria-label', 'Scroll left'); left.innerHTML = chrome(P.left);
      var tabs = document.createElement('div'); tabs.className = 'tabs';
      for (var i = 0; i < parents.length; i++) tabs.appendChild(this._tab(parents[i], parents[i] === this._current));
      var right = document.createElement('button'); right.className = 'chev'; right.type = 'button';
      right.setAttribute('aria-label', 'Scroll right'); right.innerHTML = chrome(P.right);
      left.addEventListener('click', function (e) { e.stopPropagation(); tabs.scrollLeft -= 200; });
      right.addEventListener('click', function (e) { e.stopPropagation(); tabs.scrollLeft += 200; });
      tabs.addEventListener('scroll', function () { self._updateArrows(); });
      bar.appendChild(left); bar.appendChild(tabs); bar.appendChild(right);
      // When the tabs are hidden (phone), this grows in their place so the app
      // selector is pushed to the far edge instead of hugging the brand.
      var spacer = document.createElement('span'); spacer.className = 'spacer'; spacer.hidden = true;
      bar.appendChild(spacer);
      var act = document.createElement('span'); act.className = 'act';
      act.appendChild(this._launcher()); act.appendChild(this._userBtn('top-right'));
      bar.appendChild(act);
      this._tabsEl = tabs; this._leftEl = left; this._rightEl = right; this._spacerEl = spacer;
      return bar;
    }

    _updateArrows() {
      var t = this._tabsEl, l = this._leftEl, r = this._rightEl, launch = this._launchEl;
      if (t) {
        // Phone width: hide the tab strip (and its chevrons) entirely and leave
        // only the app selector, which lists every module.
        if (window.innerWidth < NARROW) {
          t.hidden = true;
          if (l) l.hidden = true;
          if (r) r.hidden = true;
          if (this._spacerEl) this._spacerEl.hidden = false; // push the selector right
          if (launch) launch.hidden = false;
          return;
        }
        t.hidden = false;
        if (this._spacerEl) this._spacerEl.hidden = true;
        // Otherwise the scroll chevrons and the app switcher appear only when the
        // parent tabs do not fit.
        var over = t.scrollWidth > t.clientWidth + 2;
        if (l) l.hidden = !over || t.scrollLeft <= 0;
        if (r) r.hidden = !over || (t.scrollLeft + t.clientWidth >= t.scrollWidth - 1);
        if (launch) launch.hidden = !over;
        return;
      }
      // Rail: the app switcher (in the footer) appears only when the parent
      // items overflow the rail's height - otherwise every parent is in view.
      var ri = this._ritemsEl;
      if (launch && ri) launch.hidden = ri.scrollHeight <= ri.clientHeight + 2;
    }

    _offset(layout, side, hasSecondary) {
      try {
        var s = document.documentElement.style;
        s.paddingTop = ''; s.paddingLeft = ''; s.paddingRight = '';
        var padSide = side === 'right' ? 'paddingRight' : 'paddingLeft';
        var top = 0, left = 0, right = 0;
        if (layout === 'header') {
          top = HEADER_H;
          if (hasSecondary) { if (side === 'right') right = RAIL_W; else left = RAIL_W; }
        } else {
          if (side === 'right') right = RAIL_W; else left = RAIL_W;
          if (hasSecondary) top = STRIP_H;
        }
        if (top) s.paddingTop = top + 'px';
        if (left) s.paddingLeft = left + 'px';
        if (right) s.paddingRight = right + 'px';
        // The padding alone left the page too TALL: height:100% resolves against
        // the container's CONTENT box, which is still the whole viewport, so an
        // element filling the page ran 56px past the bottom and raised a scroll
        // bar over nothing. Pinning <html> to the viewport and counting the
        // padding inside it (border-box) makes 100% mean "what is left".
        s.boxSizing = 'border-box';
        s.height = '100dvh';
        // What the bar takes, for the rules CSS cannot fix from here: 100vh is
        // the VIEWPORT whatever we do to <html>, so an application that asks for
        // it slides under the bar. It can subtract these instead:
        //   height: calc(100dvh - var(--meerkat-top, 0px))
        s.setProperty('--meerkat-top', top + 'px');
        s.setProperty('--meerkat-left', left + 'px');
        s.setProperty('--meerkat-right', right + 'px');
      } catch (e) {}
    }
  }

  customElements.define('meerkat-portal-nav', MeerkatPortalNav);
})();`
