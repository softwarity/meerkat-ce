package auth

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/evalmark"
	"github.com/softwarity/meerkat/internal/icons"
	"github.com/softwarity/meerkat/internal/store"
)

// The portal bar, beside a palette (PORTAL-01, THEME-07).
//
// The bar is the one surface of this product that is not a page: it is injected
// into somebody else's HTML and draws itself in a shadow DOM from the theme. It
// was therefore the one surface a palette could not be judged on, which is
// backwards - it is the piece a visitor sees on EVERY screen of every
// application, where a flow page is seen twice a year.
//
// Two entries and not one, because the layout is not a detail of the bar but
// its shape: parents in a top strip with children in a rail, or the reverse.
// A palette that reads well as tabs can read badly as a rail (the rail carries
// its own surface behind sixty pixels of icon), and nobody would find that out
// from the other one.
//
// NOTHING HERE CALLS A HANDLER. Like the pages, the preview renders the REAL
// component against made-up applications: asking portal.json would put the
// catalogue of the installation being administered into a palette screenshot,
// and would show an empty bar on a gateway whose portal is off.

// PortalPreviewKinds is what the picker offers, in the order it walks them.
var PortalPreviewKinds = []struct{ Key, Label, Layout string }{
	{"portal:header", "Portal, header first", store.PortalHeader},
	{"portal:rail", "Portal, rail first", store.PortalRail},
	// The report form, alone. Eighteen strings of its own, which listed under
	// the bar would drown the six that name the bar - and it is a screen a
	// visitor fills in, not a row they walk past.
	{"portal:issue", "Report an issue", ""},
}

// PortalPreviewPrefix marks the keys this file answers, the way "mail:" marks
// the messages.
const PortalPreviewPrefix = "portal:"

// WritePortalPreview renders one of the two bars. Reports false for a key it
// does not know, so the caller can answer 404 with the list of what it does.
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
// resolved here, because the key map renders this same page against a catalogue
// of markers to learn which strings it draws - see tokenmap.go.
func portalPreviewHTML(key string, t store.Theme, b store.Branding, scheme string, tr map[string]string, lang string) (string, bool) {
	known := false
	layout := ""
	for _, k := range PortalPreviewKinds {
		if k.Key == key {
			known, layout = true, k.Layout
		}
	}
	if !known {
		return "", false
	}
	issueOnly := layout == ""
	css := t.CSS()
	forced := previewScheme(scheme)

	payload := portalPayload{
		Enabled:  true,
		Layout:   layout,
		Side:     "left",
		Display:  store.PortalDisplayBoth,
		ShowName: true,
		Brand:    portalBrand{AppName: b.AppName, Logo: b.Logo},
		// The bar draws in a shadow DOM, so its copy of the palette is rescoped
		// - the same substitution portal.json makes.
		ThemeCSS: strings.Replace(css, ":root", ":host", 1),
		Scheme:   forced,
		// A pane forces its scheme, so the bar wears it instead of offering the
		// switch: two panes showing the same palette would be one pane.
		SchemeImposed: forced != "auto",
		Labels:        map[string]string{"applications": tr["applications"]},
		Parents:       portalSampleEntries(),
		// Two languages, so the account menu draws its language submenu: the
		// bar forwards this list to the button exactly as an injected page
		// would, and one language draws no menu at all.
		Languages: portalSampleLanguages(lang),
	}
	if forced == "auto" {
		payload.Scheme = "light"
		payload.SchemeImposed = false
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	// The account menu's own payload, handed over the way an injected page
	// hands it: the button reads window.meerkatPage first and only falls back
	// to fetching, and there is nothing to fetch on the admin plane.
	//
	// Each screen carries the labels IT draws and no others. That is what makes
	// the editor's list per-screen rather than one list of two hundred: the key
	// map is built by rendering these pages and reading what comes out, so a
	// label in the payload is a label on the screen.
	sample := portalSampleUser(tr)
	if issueOnly {
		sample.Labels = userButtonIssueLabels(tr)
	} else {
		sample.Labels = userButtonMenuLabels(tr)
	}
	user, err := json.Marshal(sample)
	if err != nil {
		return "", false
	}

	langJSON, err := json.Marshal(lang)
	if err != nil {
		return "", false
	}

	// The bar, or the report form on its own: the panel lives inside the account
	// button's shadow root, so the button is what mounts it - with nothing else
	// of the portal, since the form is the subject here.
	mount := `<meerkat-portal-nav preview open-account></meerkat-portal-nav>`
	bars := `<script>` + portalNavJS + `</script>` +
		`<script>window.postMessage({type:"mk-portal",payload:` + string(data) + `},location.origin);</script>`
	if issueOnly {
		// scheme-wear, because the button carries its own copy of the palette in
		// its shadow root and that copy declares "color-scheme: light dark" on
		// the host - which beats the scheme the pane put on the document, so
		// the form came out in the VIEWER's scheme while the page behind it
		// obeyed the pane. It is the same attribute the bar sets on it when an
		// integrator has settled the question.
		wear := ""
		if forced != "auto" {
			wear = ` scheme-wear="` + forced + `"`
		}
		mount = `<meerkat-user-button open="issue" in-frame position="top-right"` + wear + `></meerkat-user-button>`
		bars = ""
	}

	schemeRule := ""
	if forced != "auto" {
		schemeRule = ":root { color-scheme: " + forced + "; }"
	}

	page := `<!doctype html><html lang="` + template.HTMLEscapeString(lang) + `" dir="` + Dir(lang) + `">` +
		`<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<title>` + template.HTMLEscapeString(b.AppName) + `</title><style>` + css + schemeRule + `
    * { box-sizing: border-box; }
    html, body { margin: 0; min-height: 100%; }
    body {
      font-family: var(--mk-font); color: var(--mk-on-surface);
      background: var(--mk-surface);
    }
    /* An application behind the bar, because a bar with nothing under it is a
       strip of colour: the question being answered here is whether the two
       surfaces sit together. */
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
		// NOTHING LEAVES THIS PAGE. The bar draws real links and the account
		// menu really signs out: in a preview served from the admin plane, a
		// click on a tab walks the frame into the console, and Sign out posts
		// to the console's own /logout and ends the session of the person
		// tuning the palette. Anchors, forms and fetch are stopped at the door
		// rather than one by one - a preview has no business emitting anything,
		// and the next call added to the button would be the next thing to find
		// out about the hard way.
		//
		// The stubbed fetch NEVER SETTLES, and that is the point rather than a
		// shortcut: every navigation these components make sits in a .then or a
		// .catch of a call (location.href = "/login" after logging out is the
		// loud one). A promise that resolves empty lets all of them run; one
		// that never does leaves the page exactly as it is, which is what a
		// preview is.
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
		`<script>` + strings.Replace(userButtonJS, "__MK_EVAL__", evalmark.MenuEntry, 1) + `</script>` + bars + `</body></html>`

	return page, true
}

// portalSampleEntries is the catalogue the sample bar draws: enough parents to
// make the strip work for its living, one of them with children so the second
// surface is not empty, and a badge so its shape can be judged.
func portalSampleEntries() []portalEntry {
	icon := func(name string) string { return icons.Resolve(name) }
	return []portalEntry{
		{Label: "Orders", Icon: icon("shopping_cart"), Href: "/orders", Description: "Everything being shipped"},
		{
			Label: "Billing", Icon: icon("receipt_long"), Href: "/billing/invoices",
			Description: "Invoices and plans",
			Children: []portalEntry{
				{Label: "Invoices", Icon: icon("description"), Href: "/billing/invoices"},
				{Label: "Plans", Icon: icon("sell"), Href: "/billing/plans"},
				{Label: "Usage", Icon: icon("bar_chart"), Href: "/billing/usage", Badge: "3"},
			},
		},
		{Label: "Catalogue", Icon: icon("inventory_2"), Href: "/catalogue"},
		{Label: "Docs", Icon: icon("menu_book"), Href: "/docs"},
		{Label: "Support", Icon: icon("support_agent"), Href: "/support", Badge: "2"},
	}
}

// portalSampleUser is the account menu's payload: signed in, in an
// organisation, with somewhere to switch to - the state that draws the most of
// the menu, since an anonymous button is a single word.
func portalSampleUser(t map[string]string) userButtonPayload {
	return userButtonPayload{
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
			{Name: "Billing", Href: "/billing"},
			{Name: "Docs", Href: "/docs"},
		},
		Issues:  true,
		DevDocs: true,
		// Every label the menu draws, from the one list the served button uses:
		// a preview that knew fewer would render blank rows, which is precisely
		// the screen somebody opens to fix a wording.
		Labels: userButtonLabels(t),
		Scheme: "auto",
	}
}

// portalSampleLanguages is the language offer the sample bar forwards. The
// preview's own language first, so the menu names it as the active one, then
// two others to make a menu rather than a line.
func portalSampleLanguages(lang string) []string {
	out := []string{lang}
	for _, l := range []string{"en", "fr"} {
		if l != lang {
			out = append(out, l)
		}
	}
	return out
}
