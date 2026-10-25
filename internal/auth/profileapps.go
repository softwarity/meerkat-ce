package auth

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// The profile's way back into the applications: ONE control.
//
// It used to be a card listing every application the person could open, the
// one they came from pointing back where they were. On a portal installation
// that list was one flat link under a heading, and on the others a cloud of
// chips that said nothing about which one to take. Someone on their profile
// has finished with it and wants to go back - so the page offers the one way
// back, chosen in this order:
//
//  1. they came from a page of the applications, still open to them in the
//     tenant now active: Back, to that very page;
//  2. portal mode: the portal, under the application's name - the first
//     module of the catalogue they may open, which is where a sign-in lands;
//  3. links mode: a list of the catalogue's links, containers as groups;
//  4. otherwise: a list of the UI routes they may open.
//
// A step with nothing to offer gives way to the next one, and a list of one
// is a plain button with that one's name. When nothing at all is open to
// them, the button still says Applications, and leads to the page that tells
// them to ask their administrator (refused?why=apps): a profile with no way
// out but signing out reads as a dead end, and the honest answer is that
// nothing is granted yet.

// noAppsHref is where the Applications button leads someone who may open
// nothing: the refusal page, told why.
const noAppsHref = "/refused?why=apps"

// appsControl is the one control: a button when Href is set, a list otherwise
// - Label is then the list's own line, the name of what it holds.
type appsControl struct {
	Label  string
	Href   string
	Groups []appsGroup
}

// appsGroup is one run of the list: a container's modules under its label,
// or modules of their own under none.
type appsGroup struct {
	Label string
	Links []publicLink
}

// appsChoice is everything the choice is made from, already resolved for the
// caller: what is open to them is decided before, so this part is a rule and
// nothing else - and a table test reads it as one.
type appsChoice struct {
	// Back is the page they came from, kept only when it is still open to
	// them in the active tenant.
	Back string
	Mode string
	// Catalogue is the portal catalogue as the caller may open it (in portal
	// mode, at most its first module).
	Catalogue []publicLink
	// Routes are the UI routes the caller may open, in routing order.
	Routes []publicLink
	// AppName names the portal (the branding's application name).
	AppName string
}

// chooseApps applies the rule above. t is the page's catalogue of strings.
func chooseApps(c appsChoice, t map[string]string) appsControl {
	if c.Back != "" {
		return appsControl{Label: t["back"], Href: c.Back}
	}
	if c.Mode == store.PortalModePortal && len(c.Catalogue) > 0 {
		name := strings.TrimSpace(c.AppName)
		if name == "" {
			name = c.Catalogue[0].Name
		}
		return appsControl{Label: name, Href: c.Catalogue[0].Href}
	}
	if c.Mode == store.PortalModeLinks && len(c.Catalogue) > 0 {
		return appsList(c.Catalogue, t)
	}
	if len(c.Routes) > 0 {
		return appsList(c.Routes, t)
	}
	return appsControl{Label: t["applications"], Href: noAppsHref}
}

// appsList is a list of links as the control draws it: the runs the
// catalogue marked (Head) become groups, and a list of one is its button.
func appsList(links []publicLink, t map[string]string) appsControl {
	if len(links) == 1 {
		return appsControl{Label: links[0].Name, Href: links[0].Href}
	}
	ctl := appsControl{Label: t["applications"]}
	for i, l := range links {
		if i == 0 || l.Head {
			ctl.Groups = append(ctl.Groups, appsGroup{Label: l.Group})
		}
		g := &ctl.Groups[len(ctl.Groups)-1]
		g.Links = append(g.Links, publicLink{Name: l.Name, Href: l.Href})
	}
	return ctl
}

// profileApps resolves the choice for this request and this session.
func (h *Handler) profileApps(r *http.Request, sess store.Session, brand string, t map[string]string) appsControl {
	ctx := r.Context()
	caller := h.sessionCaller(ctx, sess)
	grants := func(rt store.Route) bool { return rt.Access.Grants(caller) }
	addr := func(rt store.Route) string { return routeAddress(r, rt) }
	routes, _ := h.st.ListRoutes(ctx)
	c := appsChoice{
		Mode:    h.st.Portal(ctx).Mode,
		AppName: brand,
		Back:    h.backIfOpen(r, sess, r.URL.Query().Get("from"), routes, grants),
	}
	// The lists are only worked out when the step before did not answer.
	if c.Back == "" {
		c.Catalogue = h.catalogueAt(ctx, grants, addr)
		if len(c.Catalogue) == 0 || c.Mode == store.PortalModeNone {
			c.Routes = openRoutes(routes, grants, addr)
		}
	}
	return chooseApps(c, t)
}

// sessionCaller is the session as an access rule reads it, in the tenant
// active NOW. Memberships are not filled in: what one may open is what one
// may open right now, not after a detour through the tenant chooser.
func (h *Handler) sessionCaller(ctx context.Context, sess store.Session) store.Caller {
	caller := store.Caller{Authenticated: true, TenantID: h.st.SessionTenant(ctx, sess.UserID, sess.TenantID)}
	if u, err := h.st.GetUserByID(ctx, sess.UserID); err == nil {
		caller.Username = u.Username
	}
	if names, err := h.st.SessionRoleNames(ctx, sess.UserID, caller.TenantID, sess.GroupID); err == nil {
		caller.Roles = names
	}
	return caller
}

// backIfOpen keeps the page someone came from when a UI route open to them
// serves it on this host - which is what "still open" means after a tenant
// switch: the route that let them in a minute ago may refuse them now.
//
// safeNext proves it is a path of this site - the value arrives in a query
// string, so it is a stranger until it passes - and that does not make it a
// page anyone wants to go back to. A page this handler draws itself (the
// sign-in, the profile) is not an application, whatever a catch-all route
// says. The one exception is the notifications page: a page of the
// applications' side people do come from, open while the notifications are
// on for their tenant.
func (h *Handler) backIfOpen(r *http.Request, sess store.Session, from string, routes []store.Route, grants func(store.Route) bool) string {
	back := safeNext(from)
	if from == "" || back != from {
		return "" // nothing said, or refused by safeNext (which answers "/")
	}
	u, err := url.Parse(back)
	if err != nil {
		return ""
	}
	if u.Path == NotificationsPagePath {
		if h.NotifyBadge != nil {
			if _, ok := h.NotifyBadge(r.Context(), sess); ok {
				return back
			}
		}
		return ""
	}
	if h.drawsItself(u.Path, r.Host) {
		return ""
	}
	host := hostOnly(r.Host)
	for _, rt := range routes {
		if rt.Enabled && rt.IsUI && servesPath(rt, host, u.Path) && grants(rt) {
			return back
		}
	}
	return ""
}

// drawsItself says whether a path is one of this handler's own pages rather
// than one a route serves.
func (h *Handler) drawsItself(path, host string) bool {
	if h.pages == nil {
		return false
	}
	_, pattern := h.pages.Handler(&http.Request{Method: http.MethodGet, Host: host, URL: &url.URL{Path: path}})
	return pattern != "" && pattern != "/"
}

// servesPath says whether a route's host and path predicates take this page.
// The other predicates (a header, a time window) say nothing about a page and
// are not read: the question is whether the page is still theirs to open.
func servesPath(rt store.Route, host, path string) bool {
	for _, names := range routing.HostNames(rt.Predicates) {
		if !routing.HostMatches(names, host) {
			return false
		}
	}
	for _, spec := range rt.Predicates {
		if spec.Type != "path" {
			continue
		}
		matched := false
		for _, p := range routing.MatchPaths([]routing.Spec{spec}) {
			if cp, err := routing.CompilePath(p); err == nil && cp.Match(path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// openRoutes are the UI routes the caller may open, in routing order, each
// once by its address.
func openRoutes(routes []store.Route, grants func(store.Route) bool, addr func(store.Route) string) []publicLink {
	var links []publicLink
	seen := map[string]bool{}
	for _, rt := range routes {
		if !rt.Enabled || !rt.IsUI || !grants(rt) {
			continue
		}
		href := addr(rt)
		if href == "" || seen[href] {
			continue
		}
		seen[href] = true
		links = append(links, publicLink{Name: rt.Name, Href: href})
	}
	return links
}

// routeAddress is where a link to a route takes a browser that is on this
// request's page: its entry path when the route answers on this host, the
// full address on the route's own host otherwise - the same port and scheme,
// since it is the same gateway answering. "" when no address can be built: a
// path that starts with a wildcard, or a route that only answers on names
// given as wildcards. Such a route is left out of a list rather than offered
// as a link that lands nowhere.
func routeAddress(r *http.Request, rt store.Route) string {
	entry := routeEntryPath(rt)
	if entry == "" {
		if len(routing.MatchPaths(rt.Predicates)) > 0 {
			return ""
		}
		entry = "/" // no path predicate: the route takes every path
	}
	hosts := routing.HostNames(rt.Predicates)
	here := hostOnly(r.Host)
	answersHere := true
	for _, names := range hosts {
		if !routing.HostMatches(names, here) {
			answersHere = false
			break
		}
	}
	if answersHere {
		return entry
	}
	name := ""
	for _, candidate := range hosts[0] {
		if strings.HasPrefix(candidate, "*") {
			continue
		}
		ok := true
		for _, names := range hosts[1:] {
			if !routing.HostMatches(names, candidate) {
				ok = false
				break
			}
		}
		if ok {
			name = candidate
			break
		}
	}
	if name == "" {
		return ""
	}
	if _, port, err := net.SplitHostPort(r.Host); err == nil && port != "" {
		name = net.JoinHostPort(name, port)
	}
	return filters.Scheme(r) + "://" + name + entry
}
