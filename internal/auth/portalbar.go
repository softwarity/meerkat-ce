package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"html/template"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/evalmark"
	"github.com/softwarity/meerkat/internal/mail"
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

// NotifyBadge is what the user button and the portal bar show of the
// notifications (NOTIF-05): every unread one the reader may still act on, and
// where the reader reads them.
type NotifyBadge struct {
	Total int    `json:"total"`
	Page  string `json:"page"`
	// InBar says the portal bar carries the notifications entry for this
	// reader: the entry and the badge are then the bar's, and the user button
	// inside it shows neither.
	InBar bool `json:"inBar,omitempty"`
}

// Notifications is the badge for the session a request carries, ok false
// when there is none, when the binary carries no notifications, or when they
// are off for the reader's tenant.
func (h *Handler) Notifications(r *http.Request) (NotifyBadge, bool) {
	if h.NotifyBadge == nil || h.adminPlane {
		return NotifyBadge{}, false
	}
	sess, err := h.sm.Resolve(r.Context(), r)
	if err != nil || sess.Pending != "" {
		return NotifyBadge{}, false
	}
	return h.NotifyBadge(r.Context(), sess)
}

// NotificationsPagePath is the built-in notifications page. It answers
// whatever page the portal names: the address everything leads to may be
// the integrator's, the built-in one stays reachable.
const NotificationsPagePath = "/meerkat/notifications"

// The notifications' desktop helper (Enterprise): a service worker, because a
// notification a page makes itself can carry no button, and a phone refuses
// it. Its scope is a path that serves nothing, so it controls no page and
// sees no request; the account button registers it, the first time it has
// news for the desktop.
const (
	NotificationsWorkerPath  = "/meerkat/notify-sw.js"
	NotificationsWorkerScope = "/meerkat/notify-sw/"
)

// NotifyPlace is where one reader reads their notifications.
type NotifyPlace struct {
	// Page is the address: the integrator's page when the portal names one
	// and the reader may open its route, the built-in page otherwise.
	Page string
	// Route is the integrator's route serving Page, "" for the built-in page.
	Route store.Route
	// InBar says the bar carries the notifications entry for this reader:
	// portal mode, the entry placed, and the integrator's route open to them.
	InBar bool
}

// NotifyPlace resolves it for a caller. A reader the integrator's route
// refuses is led to the built-in page, through the user button: the bar's
// entry is filtered like any other. A route turned off, gone or no longer UI
// leads everybody to the built-in page, and the entry stays where it is.
func (h *Handler) NotifyPlace(ctx context.Context, caller store.Caller) NotifyPlace {
	cfg := h.st.Portal(ctx)
	place := NotifyPlace{Page: NotificationsPagePath}
	allowed := true
	if np := cfg.NotifyPage; np.RouteID != "" {
		if rt, err := h.st.GetRoute(ctx, np.RouteID); err == nil && rt.IsUI {
			entry := routeEntryPath(rt)
			switch {
			case !rt.Access.Grants(caller):
				allowed = false
			case rt.Enabled && entry != "":
				place.Page, place.Route = NotifyHref(entry, np.Path), rt
			}
		}
	}
	place.InBar = allowed && !h.adminPlane && cfg.Mode == store.PortalModePortal && cfg.NotificationsEntry()
	return place
}

// NotifyHref is a path under a route's address: "/inbox" under "/app" is
// "/app/inbox", and under "/" it is "/inbox".
func NotifyHref(entry, path string) string {
	if path == "" {
		path = "/"
	}
	if entry == "/" || entry == "" {
		return path
	}
	return strings.TrimSuffix(entry, "/") + path
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

// ── A page of the data plane, for a package outside this one ─────────────────
//
// The notifications page (NOTIF-05) is drawn by Enterprise code in the very
// frame the sign-in pages wear: the theme, the arrangement, the language and
// the scheme switch. These are the three pieces it needs, and nothing in them
// is the notifications'.

// FlowChrome is the model every flow page embeds.
type FlowChrome = flowChrome

// FlowPage builds a page from its body, in the flow pages' frame.
func FlowPage(name, body string) *template.Template { return flowPage(name, body) }

// FlowData is the frame's data for one request, titled by a catalogue key.
func (h *Handler) FlowData(r *http.Request, titleKey string) FlowChrome {
	return h.flowData(r, titleKey)
}

// WriteFlow writes a page built by FlowPage.
func WriteFlow(w http.ResponseWriter, page *template.Template, data any, status int) {
	writeFlow(w, page, data, status)
}

// AppPage makes a flow page a screen of the product (see flowChrome.App): the
// brand, the background picture and the arrangement go, the theme, the
// language and the scheme stay. The arrangement's block is dropped with it -
// a split or a drawer is a way to dress a sign-in, not a list - and only the
// evaluation banner's placement is kept.
func AppPage(c FlowChrome) FlowChrome {
	c.App = true
	c.Layout = store.PageLayout{Name: store.LayoutCentered}
	c.LayoutCSS = template.CSS(evalmark.FlowCSS) //nolint:gosec // a constant from ee/eval, never anyone's input
	return c
}

// BehindPortal puts a flow page behind the portal the way a route page stands
// there: in portal mode, on an image that carries the bar, the page agent and
// the bar ride at the top of its body - the same fragment, so the same bar, in
// the same place, with its badges kept live by the agent's channel. The
// page's own language and scheme switches give way to the account menu the
// bar mounts: one surface, not two corners. Anything else leaves the page as
// it was.
//
// The agent drives this page's light/dark in place, as a route's mechanism
// would: the body class the frame stamps (mk-scheme-*), and a rule that lets
// it win over the scheme the page was rendered in, for "auto" picked on a page
// served light or dark.
func (h *Handler) BehindPortal(r *http.Request, c FlowChrome) FlowChrome {
	if portalBar == nil || portalBar.Fragment == nil || h.adminPlane {
		return c
	}
	if h.st.Portal(r.Context()).Mode != store.PortalModePortal {
		return c
	}
	langs := h.offeredLanguages()
	agent := `<script defer src="/meerkat/page.js?v=` + ChromeVersion + `" data-scheme="select"` +
		` data-scheme-mechanism="class" data-scheme-tag="body" data-scheme-light="mk-scheme-light" data-scheme-dark="mk-scheme-dark"` +
		` data-languages="` + template.HTMLEscapeString(strings.Join(langs, ",")) + `"></script>` +
		`<style>:root:has(> body.mk-scheme-light) { color-scheme: light; } :root:has(> body.mk-scheme-dark) { color-scheme: dark; }</style>`
	c.BodyStart = template.HTML(agent + portalBar.Fragment(store.Route{IsUI: true}, langs)) //nolint:gosec // constants, and language codes validated and escaped
	c.SchemeSwitch = false
	c.Langs = nil
	return c
}

// ── A mail in the gateway's own look, for a package outside this one ─────────
//
// The notifications (NOTIF-05) are doubled by mail when a service asks: the
// single template NOTIF-01 dresses every mail in, the strings in the reader's
// language, and the relay the console configured.

// BuildMail composes a message in the gateway's look.
func (h *Handler) BuildMail(ctx context.Context, to string, spec mail.Spec) mail.Message {
	return h.buildMail(ctx, to, spec)
}

// SendMail sends it through the configured relay.
func (h *Handler) SendMail(ctx context.Context, msg mail.Message) error { return h.sendMail(ctx, msg) }
