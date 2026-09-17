package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/store"
)

// portalAssetVersion is a short content hash of the bar's JS, used as a cache
// buster on the console preview's <script src> so a rebuilt bar is never hidden
// behind a stale cached copy (the data plane still caches it by max-age).
var portalAssetVersion = func() string {
	sum := sha256.Sum256([]byte(portalNavJS))
	return hex.EncodeToString(sum[:])[:12]
}()

// The <meerkat-portal-nav> web component (PORTAL-01): a header-or-rail bar the
// gateway injects into the proxied UI pages when a portal is configured, in
// place of the per-route user button - which then rides INSIDE the bar. It is
// the "Applications" submenu of the user button (reachableLinks) grown into a
// navigation surface: parents and their children, filtered by the same route
// access, themed by the data plane.
//
// Like the user button, it is a vanilla custom element served from the DATA
// plane (portal.js), fetching its data and localized labels from portal.json.
// Everything is same-origin and offline-first.

// registerPortal mounts the component's endpoints on the DATA plane.
func (h *Handler) registerPortal(mux *http.ServeMux) {
	mux.HandleFunc("GET /meerkat/portal.js", h.portalJS)
	mux.HandleFunc("GET /meerkat/portal.json", h.portalJSON)
}

func (h *Handler) portalJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	// The data plane caches it (served on every proxied page); the admin plane
	// serves the SAME file to the console's live editor, where a stale copy would
	// show yesterday's bar after a rebuild - so there it is never cached.
	if h.adminPlane {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}
	_, _ = w.Write([]byte(portalNavJS))
}

// registerPortalPreview mounts, on the ADMIN plane, the pieces the console's
// portal editor needs same-origin: the component itself and a host page that
// runs it in edit mode, driven over postMessage. The served DATA-plane portal
// still comes from registerPortal on the data plane; this is the editor's mirror.
func (h *Handler) registerPortalPreview(mux *http.ServeMux) {
	mux.HandleFunc("GET /meerkat/portal.js", h.portalJS)
	mux.HandleFunc("GET /meerkat/portal-preview", h.portalPreviewPage)
}

// portalPreviewPage is the iframe the console mounts: the REAL bar in edit mode,
// wearing the DATA-plane theme and brand (read here, since the admin plane's own
// chrome is Meerkat's, not the application's). The console posts a draft
// ("mk-portal-draft"); this page adds theme/brand and forwards it to the
// component, and relays the component's selections back to the console.
func (h *Handler) portalPreviewPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, err := h.st.GetActiveTheme(ctx)
	if err != nil || len(t.Light) == 0 {
		t = store.DefaultTheme()
	}
	rawCSS := t.CSS()
	b := store.DefaultBranding()
	if err := h.st.GetSetting(ctx, store.SettingBranding, &b); err != nil || b.AppName == "" {
		b = store.DefaultBranding()
	}
	hostCSS, _ := json.Marshal(strings.Replace(rawCSS, ":root", ":host", 1))
	appName, _ := json.Marshal(b.AppName)
	logo, _ := json.Marshal(b.Logo)

	page := `<!doctype html><html><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1"><title>Portal preview</title>` +
		`<style>html,body{margin:0;height:100%;}` + rawCSS +
		`body{background:var(--mk-surface,Canvas);color:var(--mk-on-surface,CanvasText);}</style></head><body>` +
		`<script>window.__mkTheme={css:` + string(hostCSS) + `,brand:{appName:` + string(appName) + `,logo:` + string(logo) + `}};</script>` +
		`<meerkat-portal-nav preview edit></meerkat-portal-nav>` +
		`<script defer src="/meerkat/portal.js?v=` + portalAssetVersion + `"></script>` +
		`<script>(function(){window.addEventListener("message",function(e){` +
		`if(e.origin!==location.origin)return;var m=e.data;if(!m||m.type!=="mk-portal-draft")return;` +
		`var p=m.payload||{};p.enabled=true;p.themeCss=window.__mkTheme.css;p.brand=window.__mkTheme.brand;` +
		`window.postMessage({type:"mk-portal",payload:p},location.origin);});})();</script>` +
		`</body></html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(page))
}

// portalBrand is the identity the bar wears: the application's name and, when
// it has one, its logo (a sanitized data URI). The Meerkat mark is never
// carried - the portal is the integrator's, not the product's.
type portalBrand struct {
	AppName string `json:"appName"`
	Logo    string `json:"logo,omitempty"`
}

// portalEntry is one module as the bar shows it - a parent or a child. It
// carries NO access rule: what a caller may not open is not in the list at
// all (the same security boundary as portal-svc's NavPillar). An empty Href
// marks a parent that groups children the caller can reach but is not itself
// navigable.
type portalEntry struct {
	Label string `json:"label"`
	// Icon is an SVG string (viewBox + path), rendered as a CSS mask by the
	// bar - no icon font. Empty falls back to the label's initial.
	Icon string `json:"icon,omitempty"`
	Href string `json:"href,omitempty"`
	// HomeLabel is the label of the parent's own "home" row in the secondary
	// surface when it has children; empty means use Label. Parents only.
	HomeLabel   string        `json:"homeLabel,omitempty"`
	Description string        `json:"description,omitempty"`
	Badge       string        `json:"badge,omitempty"`
	Children    []portalEntry `json:"children,omitempty"`
}

// portalPayload is what portal.json returns, per session, no-store.
type portalPayload struct {
	Enabled       bool              `json:"enabled"`
	Layout        string            `json:"layout"`
	Side          string            `json:"side"`
	Display       string            `json:"display"`
	ShowName      bool              `json:"showName,omitempty"`
	Brand         portalBrand       `json:"brand"`
	ThemeCSS      string            `json:"themeCss"`
	Scheme        string            `json:"scheme"`
	SchemeImposed bool              `json:"schemeImposed,omitempty"`
	Labels        map[string]string `json:"labels"`
	Parents       []portalEntry     `json:"parents,omitempty"`
	// Languages carries the flow-page locale offer, forwarded to the user
	// button the bar mounts (its language submenu) exactly as the per-route
	// injection would have set it.
	Languages []string `json:"languages,omitempty"`
}

func (h *Handler) portalJSON(w http.ResponseWriter, r *http.Request) {
	offered := h.offeredLanguages()
	p := prefsOf(r, offered)
	t := messages[p.Lang]

	css, brand, _ := h.chrome()
	scheme, imposed := p.Scheme, false
	if forced := h.imposedScheme(); forced != "" {
		scheme, imposed = forced, true
	}
	payload := portalPayload{
		Layout:        store.PortalHeader,
		Side:          "left",
		Display:       store.PortalDisplayBoth,
		Brand:         portalBrand{AppName: brand.AppName, Logo: string(brand.LogoURL)},
		ThemeCSS:      strings.Replace(string(css), ":root", ":host", 1),
		Scheme:        scheme,
		SchemeImposed: imposed,
		Labels:        map[string]string{"applications": t["applications"]},
		Languages:     offered,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	// The caller a route's access is evaluated against: anonymous by default,
	// filled in when a session resolves. An anonymous caller sees only the
	// routes whose access poses no condition - the same rule the injection
	// applies to decide the bar shows at all.
	caller := store.Caller{}
	if sess, err := h.sm.Resolve(r.Context(), r); err == nil && sess.Pending == "" {
		caller.Authenticated = true
		caller.TenantID = sess.TenantID
		if u, err := h.st.GetUserByID(r.Context(), sess.UserID); err == nil {
			caller.Username = u.Username
		}
		if names, err := h.st.SessionRoleNames(r.Context(), sess.UserID, sess.TenantID, sess.GroupID); err == nil {
			caller.Roles = names
		}
	}

	cfg, parents := h.portalNav(r.Context(), caller)
	payload.Enabled = cfg.Enabled
	payload.Layout = cfg.Layout
	payload.Side = cfg.Side
	if cfg.Display != "" {
		payload.Display = cfg.Display
	}
	payload.ShowName = cfg.ShowAppName
	payload.Parents = parents

	b, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(b)
}

// portalNav resolves the configured portal against a caller: it keeps the
// parents and children whose bound route the caller may open, resolves each
// entry's address, and drops the access rules. It is reachableLinks grown a
// second level and a caller that can be anonymous.
//
// A route referenced by the config but since deleted (or turned off, or no
// longer UI) is simply skipped - the write-time SanitizePortalConfig guards
// what ENTERS the config; this guards what a stale reference does at READ.
func (h *Handler) portalNav(ctx context.Context, caller store.Caller) (store.PortalConfig, []portalEntry) {
	cfg := h.st.Portal(ctx)
	if !cfg.Enabled || h.adminPlane {
		return cfg, nil
	}
	routes, err := h.st.ListRoutes(ctx)
	if err != nil {
		return cfg, nil
	}
	byID := make(map[string]store.Route, len(routes))
	for _, rt := range routes {
		byID[rt.ID] = rt
	}

	entry := func(routeID, icon, label, desc, badge string) (portalEntry, bool, bool) {
		rt, ok := byID[routeID]
		if !ok || !rt.Enabled || !rt.IsUI {
			return portalEntry{}, false, false
		}
		visible := rt.Access.Grants(caller)
		href := routeEntryPath(rt)
		navigable := visible && href != ""
		e := portalEntry{
			Label:       portalLabel(label, rt),
			Icon:        icon,
			Description: desc,
			Badge:       badge,
		}
		if navigable {
			e.Href = href
		}
		return e, visible, navigable
	}

	var out []portalEntry
	for _, p := range cfg.Parents {
		if p.Disabled { // turned off for everyone, kept in config but not served
			continue
		}
		pe, pVisible, pNavigable := entry(p.RouteID, p.Icon, p.Label, p.Description, p.Badge)
		pe.HomeLabel = p.HomeLabel
		if pVisible || pNavigable { // a resolvable, reachable route existed
			for _, c := range p.Children {
				if c.Disabled {
					continue
				}
				if ce, _, cNav := entry(c.RouteID, c.Icon, c.Label, c.Description, c.Badge); cNav {
					pe.Children = append(pe.Children, ce)
				}
			}
		}
		// Keep a parent that is itself reachable, or that gathers at least one
		// reachable child. A parent neither reachable nor with reachable
		// children is a heading that leads nowhere: dropped.
		if pNavigable || len(pe.Children) > 0 {
			out = append(out, pe)
		}
	}
	return cfg, out
}

// portalLabel is the entry's label: the override when set, else the route's
// own apps-menu label, else its name.
func portalLabel(override string, rt store.Route) string {
	if override = strings.TrimSpace(override); override != "" {
		return override
	}
	if rt.UI != nil && rt.UI.Link != "" {
		return rt.UI.Link
	}
	return rt.Name
}
