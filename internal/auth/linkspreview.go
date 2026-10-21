package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/store"
)

// The links mode's preview (PORTAL-03): what the catalogue looks like where
// it is offered, the Applications submenu of the account button.
//
// The bar has its own preview, and a list of links had none: the editor showed
// the catalogue as a numbered list, which says what is in it and nothing of
// where a visitor finds it. This is the REAL button, menu open, the submenu
// unfolded, fed the list being edited - the console posts it over postMessage,
// already resolved to what the menu draws (name, group, head), so the page
// holds no catalogue logic of its own.
//
// NOTHING LEAVES THIS PAGE, for the reasons the bar preview gives: fetch never
// settles, anchors and forms are stopped at the door.

// registerLinksPreview mounts the frame on the ADMIN plane, same-origin with
// the console that drives it.
func (h *Handler) registerLinksPreview(mux routes) {
	mux.HandleFunc("GET /meerkat/links-preview", h.linksPreviewPage)
}

// linksPreviewPage is the frame: the data-plane theme (read here, since the
// admin plane's own chrome is Meerkat's) and the button, mounted again each
// time a draft arrives - it reads its payload once, when it connects.
func (h *Handler) linksPreviewPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, err := h.st.GetActiveTheme(ctx)
	if err != nil || len(t.Light) == 0 {
		t = store.DefaultTheme()
	}
	css := t.CSS()
	p := prefsOf(r, h.offeredLanguages())
	tr := catalogue(p.Lang)
	// Signed in and nothing else: the submenu is the subject, and every other
	// row folded beside it would be a second one.
	sample := userButtonPayload{
		Authenticated: true,
		Username:      "alice",
		Fullname:      "Alice Nkemelu",
		Initials:      "AN",
		Labels:        userButtonMenuLabels(tr),
		ThemeCSS:      strings.Replace(css, ":root", ":host", 1),
		Scheme:        "auto",
	}
	user, err := json.Marshal(sample)
	if err != nil {
		http.Error(w, "links preview: "+err.Error(), http.StatusInternalServerError)
		return
	}
	lang, _ := json.Marshal(p.Lang)

	page := `<!doctype html><html><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1"><title>Links preview</title>` +
		`<style>` + css + `
    html, body { margin: 0; height: 100%; }
    body { font-family: var(--mk-font); background: var(--mk-surface); color: var(--mk-on-surface); }
    .app { padding: 24px 28px; }
    .app h1 { margin: 0 0 6px; font-size: 20px; font-weight: 600; }
    .app p { margin: 0; color: var(--mk-on-surface-variant); font-size: 13px; }
    </style></head><body>` +
		`<main class="app"><h1>Orders</h1><p>An application the gateway serves.</p></main>` +
		`<script>(function(){` +
		`window.fetch=function(){return new Promise(function(){});};` +
		`addEventListener("click",function(e){` +
		`var p=e.composedPath?e.composedPath():[];` +
		`for(var i=0;i<p.length;i++){var n=p[i];if(n&&n.tagName==="A"){e.preventDefault();return;}}` +
		`},true);` +
		`addEventListener("submit",function(e){e.preventDefault();},true);` +
		`var apps=[];` +
		`window.meerkatPage={` +
		`data:function(){var u=` + string(user) + `;u.apps=apps;return Promise.resolve(u);},` +
		`onLanguage:function(){},pickLanguage:function(){},pickScheme:function(){},` +
		`resolvedLanguage:function(){return ` + string(lang) + `;},signedOut:function(){}` +
		`};` +
		`function mount(){` +
		`var o=document.querySelector("meerkat-user-button");if(o)o.remove();` +
		`var b=document.createElement("meerkat-user-button");` +
		`b.setAttribute("open","apps");b.setAttribute("in-frame","");b.setAttribute("position","top-right");` +
		`document.body.appendChild(b);}` +
		`addEventListener("message",function(e){` +
		`if(e.origin!==location.origin)return;var m=e.data;if(!m||m.type!=="mk-links-draft")return;` +
		`apps=Array.isArray(m.apps)?m.apps:[];mount();});` +
		`window.__mkMount=mount;` +
		`})();</script>` +
		`<script>` + userButtonScript() + `</script>` +
		`<script>window.__mkMount();parent.postMessage({type:"mk-links-ready"},location.origin);</script>` +
		`</body></html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(page))
}
