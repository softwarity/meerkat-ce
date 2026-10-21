package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/store"
)

// The navigation bar (PORTAL-01) is Enterprise, and the community image does
// not carry it: ee/portal registers it here from its init(), the way
// ee/layouts registers the page arrangements. What stays in this package is the
// SEAM - where the bar is mounted, injected and previewed when it is there -
// and the catalogue it shares with the Links mode, which is the community's
// way between applications.
//
// Absent, a portal-mode catalogue reads as links (store.Portal), a write that
// asks for it is refused, and nothing of the bar answers: no portal.js, no
// portal.json, no preview. Not a switch the community image keeps off - code
// it does not have.

// PortalBar is what ee/portal provides.
type PortalBar struct {
	// Script is the <meerkat-portal-nav> component. It is part of
	// ChromeVersion, the cache buster of the injected scripts.
	Script string
	// Serve mounts the bar's endpoints: portal.js and portal.json on the data
	// plane, portal.js and the editor's preview page on the admin plane.
	Serve func(h *Handler, dataPlane bool, handle func(pattern string, fn http.HandlerFunc))
	// Fragment is what a UI route injects at the top of <body> in portal
	// mode, in place of its own user button: the bar mounts the button.
	Fragment func(r store.Route, localeCodes []string) string
	// PreviewKinds are the bar's entries in the theme editor's picker, and
	// PreviewHTML renders one against a made-up catalogue. tr is the language
	// catalogue, handed in so the key map can render it against markers.
	PreviewKinds []PreviewKind
	PreviewHTML  func(key string, t store.Theme, b store.Branding, scheme string, tr map[string]string, lang string) (string, bool)
}

// PreviewKind is one entry of the theme editor's picker that is not a flow
// page: the report form, and the bar's two arrangements when it is there.
type PreviewKind struct{ Key, Label, Layout, Category string }

var portalBar *PortalBar

// RegisterPortalBar installs the bar, or removes it with nil (a test that
// stands one in). Called from ee/portal's init(), before any handler exists.
func RegisterPortalBar(b *PortalBar) {
	portalBar = b
	store.SetPortalBar(b != nil)
	ChromeVersion = chromeVersion()
}

// Bar is the registered bar, nil on the community image.
func Bar() *PortalBar { return portalBar }

// ChromeVersion is a short content hash of the scripts the gateway injects
// into proxied pages - the page agent, the user button and, when there is
// one, the bar - for the ?v= of their <script src>. They are cached five
// minutes, and without it a deployment that changed them left every open
// browser on the previous copy for that long: a fixed menu entry still missing
// after the rollout. Same content, same URL, so nothing is refetched for
// nothing.
var ChromeVersion = chromeVersion()

func chromeVersion() string {
	src := pageJS + userButtonScript()
	if portalBar != nil {
		src += portalBar.Script
	}
	sum := sha256.Sum256([]byte(src))
	return hex.EncodeToString(sum[:])[:12]
}

// ── What a component injected into somebody else's page needs ────────────────
//
// The bar is one such component; the user button is the other, and these are
// what both read. Exported for ee/portal, and nothing in them is the bar's.

// Store is the handler's store.
func (h *Handler) Store() *store.Store { return h.st }

// AdminPlane says the handler serves the control plane, where application
// routes are not offered.
func (h *Handler) AdminPlane() bool { return h.adminPlane }

// ComponentChrome is what an injected component wears for one request: the
// visitor's language and its strings, the data plane's palette, brand and
// colour scheme.
type ComponentChrome struct {
	Lang      string
	Strings   map[string]string
	Languages []string
	// ThemeCSS is the palette as :root rules; a shadow DOM rescopes it.
	ThemeCSS      string
	AppName       string
	Logo          string
	Scheme        string
	SchemeImposed bool
}

// ComponentChrome resolves it for r.
func (h *Handler) ComponentChrome(r *http.Request) ComponentChrome {
	offered := h.offeredLanguages()
	p := prefsOf(r, offered)
	css, brand, _ := h.chrome()
	c := ComponentChrome{
		Lang: p.Lang, Strings: catalogue(p.Lang), Languages: offered,
		ThemeCSS: string(css), AppName: brand.AppName, Logo: string(brand.LogoURL),
		Scheme: p.Scheme,
	}
	if forced := h.imposedScheme(); forced != "" {
		c.Scheme, c.SchemeImposed = forced, true
	}
	return c
}

// Caller is who r is, for a route's access: anonymous unless a complete
// session resolves, then the user, the active organisation and the roles it
// grants there.
func (h *Handler) Caller(r *http.Request) store.Caller {
	caller := store.Caller{}
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil || sess.Pending != "" {
		return caller
	}
	caller.Authenticated = true
	caller.TenantID = h.st.SessionTenant(r.Context(), sess.UserID, sess.TenantID)
	if u, err := h.st.GetUserByID(r.Context(), sess.UserID); err == nil {
		caller.Username = u.Username
	}
	if names, err := h.st.SessionRoleNames(r.Context(), sess.UserID, caller.TenantID, sess.GroupID); err == nil {
		caller.Roles = names
	}
	return caller
}

// RouteEntryPath is where a link to a UI route leads: its first path pattern,
// cut at the first wildcard. Empty when the route has nowhere to land.
func RouteEntryPath(rt store.Route) string { return routeEntryPath(rt) }

// EntryLabel is the name a catalogue entry is offered under: the catalogue's
// label when set, else the route's name.
//
// TWO SOURCES, NOT THREE. A route used to carry its own menu label, which sat
// between these two and meant the displayed name could come from any of three
// places - so finding where a wrong word came from meant looking in all of
// them. The catalogue is the one place a name is decided now, and the route's
// name is what an entry falls back to when nobody decided.
func EntryLabel(override string, rt store.Route) string {
	if override = strings.TrimSpace(override); override != "" {
		return override
	}
	return rt.Name
}

// Strings is the data-plane catalogue of one language.
func Strings(lang string) map[string]string { return catalogue(lang) }

// UserButtonScript is the <meerkat-user-button> component, which the bar mounts.
func UserButtonScript() string { return userButtonScript() }
