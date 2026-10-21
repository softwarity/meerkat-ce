package auth

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/softwarity/meerkat/internal/store"
)

// The theme editor's previews that are not flow pages: the components the
// gateway injects into somebody else's HTML (THEME-07).
//
// The report form (ISSUE-01) is the account button's, so it is always here.
// The navigation bar's two arrangements are ee/portal's (PORTAL-01), offered
// when the bar is registered (PortalBar.PreviewKinds).
//
// NOTHING HERE CALLS A HANDLER. Like the pages, a preview renders the REAL
// component against made-up data: asking the live endpoints would put the
// installation being administered into a palette screenshot.

// PortalPreviewPrefix marks the keys this file answers, the way "mail:" marks
// the messages.
const PortalPreviewPrefix = "portal:"

// issuePreviewKey is the report form, alone. Eighteen strings of its own,
// which listed under the account menu would drown the ones that name it - and
// it is a screen a visitor fills in, not a row they walk past.
const issuePreviewKey = "portal:issue"

// PortalPreviewKinds is what the picker offers, in the order it walks them:
// the bar's arrangements when there is a bar, then the report form.
func PortalPreviewKinds() []PreviewKind {
	var out []PreviewKind
	if portalBar != nil {
		out = append(out, portalBar.PreviewKinds...)
	}
	return append(out, PreviewKind{Key: issuePreviewKey, Label: "Report an issue", Category: cAccount})
}

// WritePortalPreview renders one of them. Reports false for a key it does not
// know, so the caller can answer 404 with the list of what it does.
func WritePortalPreview(w http.ResponseWriter, key string, t store.Theme, b store.Branding, scheme, locale string) bool {
	lang := locale
	if lang == "" || !IsKnownLanguage(lang) {
		lang = "en"
	}
	page, ok := portalPreviewHTML(key, t, b, scheme, catalogue(lang), lang)
	if !ok {
		return false
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(page))
	return true
}

// portalPreviewHTML builds the page. The catalogue is handed IN rather than
// resolved here, because the key map renders this same page against a
// catalogue of markers to learn which strings it draws - see tokenmap.go.
func portalPreviewHTML(key string, t store.Theme, b store.Branding, scheme string, tr map[string]string, lang string) (string, bool) {
	if key != issuePreviewKey {
		if portalBar == nil {
			return "", false
		}
		return portalBar.PreviewHTML(key, t, b, scheme, tr, lang)
	}
	forced := PreviewScheme(scheme)
	// Each screen carries the labels IT draws and no others. That is what makes
	// the editor's list per-screen rather than one list of two hundred: the key
	// map is built by rendering these pages and reading what comes out, so a
	// label in the payload is a label on the screen.
	user := SampleAccount(tr, true)
	// scheme-wear, because the button carries its own copy of the palette in its
	// shadow root and that copy declares "color-scheme: light dark" on the host -
	// which beats the scheme the pane put on the document, so the form came out
	// in the VIEWER's scheme while the page behind it obeyed the pane.
	wear := ""
	if forced != "auto" {
		wear = ` scheme-wear="` + forced + `"`
	}
	// The button itself has no business here: it exists to host the panel,
	// which is position: fixed, so it does not go with it.
	mount := `<meerkat-user-button open="issue" in-frame position="top-right"` + wear + `></meerkat-user-button>`
	return ComponentPreview(lang, t, b.AppName, forced, user, mount, ""), true
}

// PreviewScheme is the scheme a preview pane forces: "light", "dark", or
// "auto" for a pane that follows the viewer.
func PreviewScheme(scheme string) string { return previewScheme(scheme) }

// ComponentPreview is the frame an injected component is previewed in: an
// application behind it, the page agent stood in for, and nothing that leaves
// the page. mount is the component's element; after comes after the user
// button's script (the bar's own script and its payload).
func ComponentPreview(lang string, t store.Theme, appName, forced string, user []byte, mount, after string) string {
	css := t.CSS()
	schemeRule := ""
	if forced != "auto" {
		schemeRule = ":root { color-scheme: " + forced + "; }"
	}
	langJSON, _ := json.Marshal(lang)
	return `<!doctype html><html lang="` + template.HTMLEscapeString(lang) + `" dir="` + Dir(lang) + `">` +
		`<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<title>` + template.HTMLEscapeString(appName) + `</title><style>` + css + schemeRule + `
    * { box-sizing: border-box; }
    html, body { margin: 0; min-height: 100%; }
    body {
      font-family: var(--mk-font); color: var(--mk-on-surface);
      background: var(--mk-surface);
    }
    /* An application behind the component, because a bar or a form with
       nothing under it is a strip of colour: the question being answered here
       is whether the two surfaces sit together. */
    .app { padding: 28px 32px 48px; }
    .app h1 { margin: 0 0 6px; font-size: 24px; font-weight: 600; }
    .app p.sub { margin: 0 0 28px; color: var(--mk-on-surface-variant); }
    .cards { display: grid; gap: 16px; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); }
    .card {
      border: 1px solid var(--mk-outline-variant); border-radius: 14px;
      background: var(--mk-surface-container-low); padding: 16px 18px;
    }
    .card h2 { margin: 0 0 4px; font-size: 13px; font-weight: 600; color: var(--mk-on-surface-variant); }
    .card .n { font-size: 26px; font-weight: 600; }
    .card .d { margin-top: 6px; font-size: 12px; color: var(--mk-on-surface-variant); }
    .rows { margin-top: 24px; border: 1px solid var(--mk-outline-variant); border-radius: 14px; overflow: hidden; }
    .row { display: flex; align-items: center; gap: 12px; padding: 12px 18px; }
    .row + .row { border-top: 1px solid var(--mk-outline-variant); }
    .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--mk-tertiary); flex: 0 0 auto; }
    .row .g { flex: 1 1 auto; color: var(--mk-on-surface-variant); font-size: 13px; }
    </style></head><body>` +
		// The page agent, stood in for. The button asks the agent for its data
		// before it would fetch, and there is nothing to fetch on the admin
		// plane - but it also CALLS the agent (onLanguage, pickLanguage,
		// resolvedLanguage, signedOut), so a stub that only answers data throws
		// on the first of them and the menu never draws. Every method it uses,
		// doing nothing: a preview has nowhere to navigate to.
		//
		// NOTHING LEAVES THIS PAGE. The components draw real links and the
		// account menu really signs out: in a preview served from the admin
		// plane, a click on a tab walks the frame into the console, and Sign out
		// posts to the console's own /logout and ends the session of the person
		// tuning the palette. Anchors, forms and fetch are stopped at the door
		// rather than one by one.
		//
		// The stubbed fetch NEVER SETTLES, and that is the point rather than a
		// shortcut: every navigation these components make sits in a .then or a
		// .catch of a call. A promise that resolves empty lets all of them run;
		// one that never does leaves the page exactly as it is.
		`<script>(function(){` +
		`window.fetch=function(){return new Promise(function(){});};` +
		`addEventListener("click",function(e){` +
		`var p=e.composedPath?e.composedPath():[];` +
		`for(var i=0;i<p.length;i++){var n=p[i];if(n&&n.tagName==="A"){e.preventDefault();return;}}` +
		`},true);` +
		`addEventListener("submit",function(e){e.preventDefault();},true);` +
		`})();</script>` +
		`<script>window.meerkatPage={` +
		`data:function(){return Promise.resolve(` + string(user) + `);},` +
		`onLanguage:function(){},pickLanguage:function(){},pickScheme:function(){},` +
		`resolvedLanguage:function(){return ` + string(langJSON) + `;},signedOut:function(){}` +
		`};</script>` +
		mount +
		`<main class="app"><h1>Orders</h1><p class="sub">An application the gateway serves.</p>` +
		`<div class="cards">` +
		`<div class="card"><h2>Open</h2><div class="n">128</div><div class="d">+12 since yesterday</div></div>` +
		`<div class="card"><h2>Shipped</h2><div class="n">1 402</div><div class="d">this month</div></div>` +
		`<div class="card"><h2>Returned</h2><div class="n">17</div><div class="d">1.2 per cent</div></div>` +
		`</div><div class="rows">` +
		`<div class="row"><span class="dot"></span><b>AC-4821</b><span class="g">Acme, two lines</span></div>` +
		`<div class="row"><span class="dot"></span><b>AC-4822</b><span class="g">Northwind, one line</span></div>` +
		`<div class="row"><span class="dot"></span><b>AC-4823</b><span class="g">Initech, six lines</span></div>` +
		`</div></main>` +
		`<script>` + userButtonScript() + `</script>` + after + `</body></html>`
}

// SampleAccount is the account menu's payload in a preview: signed in, in an
// organisation, with somewhere to switch to - the state that draws the most of
// the menu, since an anonymous button is a single word. issue carries the
// report form's labels instead of the menu's.
func SampleAccount(tr map[string]string, issue bool) []byte {
	sample := userButtonPayload{
		Authenticated: true,
		Username:      "alice",
		Fullname:      "Alice Nkemelu",
		Email:         "alice@example.com",
		Initials:      "AN",
		TenantID:      "acme",
		TenantName:    "Acme",
		Tenants: []userButtonTenant{
			{ID: "acme", Name: "Acme"},
			{ID: "northwind", Name: "Northwind"},
		},
		// The submenus, switched on: a menu of two rows previews two rows, and
		// the strings somebody came to read are in the ones that fold out.
		Apps: []userButtonLink{
			{Name: "Orders", Href: "/orders"},
			{Name: "Invoices", Href: "/billing/invoices", Group: "Billing", Head: true},
			{Name: "Plans", Href: "/billing/plans", Group: "Billing"},
			{Name: "Docs", Href: "/docs", Head: true},
		},
		Issues:  true,
		DevDocs: true,
		Scheme:  "auto",
	}
	if issue {
		sample.Labels = userButtonIssueLabels(tr)
	} else {
		sample.Labels = userButtonMenuLabels(tr)
	}
	out, _ := json.Marshal(sample)
	return out
}

// SampleLanguages is the language offer a preview forwards to the button. The
// preview's own language first, so the menu names it as the active one, then
// two others to make a menu rather than a line.
func SampleLanguages(lang string) []string {
	out := []string{lang}
	for _, l := range []string{"en", "fr"} {
		if l != lang {
			out = append(out, l)
		}
	}
	return out
}
