package store

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"github.com/softwarity/meerkat/internal/icons"
)

// PortalConfig is THE CATALOGUE of applications this gateway offers, plus the
// way it is rendered (PORTAL-01, PORTAL-03). It is GLOBAL, like the theme and
// the branding - one arrangement for the installation, already personalised
// per visitor by the ROUTE ACCESS (an entry a caller may not open is not
// offered). A per-tenant arrangement is PORTAL-02 and deliberately not this: it
// would be the product's first per-tenant visual override, and the theme and
// branding are global today.
//
// ONE CATALOGUE, THREE RENDERINGS. The list of applications used to be DERIVED
// from the routes, each UI route carrying its own menu label. That put the name
// of an application in three places at once (the portal override, the route's
// label, the route's name) and forced the menu builder to GUESS which routes
// were the same application - an installation commonly fronts one product with
// several routes, one per organisation or per version, and they differ in what
// they proxy, never in where you go. An explicit list has nothing to guess, and
// it has an order somebody chose rather than the routing order, which exists
// for "first match wins" and means nothing to a reader.
//
// Mode decides what is drawn, and CHANGING IT KEEPS THE CATALOGUE: moving from
// a menu to a bar is a rendering decision, not a reason to retype the list.
//
// It is edited on its own screen (Application, Portal). In portal mode the bar
// is injected into every proxied UI page, the same way the user button is -
// which then rides INSIDE the bar rather than in a corner of its own.
type PortalConfig struct {
	// Mode is what the catalogue is drawn as: nothing, a menu, or a bar.
	Mode string `json:"mode"`
	// Layout says which surface carries the PARENTS: "header" puts them in a
	// top strip of tabs and their children in a rail; "rail" puts the parents
	// in a rail and their children in a top strip. The one axis portal-nav
	// (understory) could not swap.
	Layout string `json:"layout"`
	// Side is which edge the rail takes. It always means something - one of
	// the two surfaces is always a rail - so, unlike the page layout's side,
	// it is never cleared.
	Side string `json:"side"`
	// Display is how a nav entry renders: "icon" (the glyph alone), "label"
	// (the text alone) or "both". Global to the bar, like portal-svc's
	// tabDisplay but one setting for the whole portal.
	Display string `json:"display"`
	// ShowAppName puts the branding's app name in the bar. Its own decision,
	// like HideLogo beside it: the name used to appear on its own whenever no
	// logo was drawn, so on an installation without one this setting existed and
	// changed nothing. Off by default, and no it is off there really is nothing
	// at the head of the bar - which is width the tabs get back.
	ShowAppName bool `json:"showAppName,omitempty"`
	// HideLogo takes the branding's logo OUT of the bar. It is drawn by default,
	// which is what the bar has always done - an installation that never touched
	// this keeps its head as it was. Said in the negative for exactly that
	// reason: the zero value has to mean "as before".
	HideLogo bool `json:"hideLogo,omitempty"`
	// LogoRadius rounds it, 0 (as drawn) to 50 (a circle), in percent of its
	// box. The bar sits beside round avatars and pill-shaped tabs, so how much
	// the mark is rounded is a decision about THIS bar - the alternative was
	// re-cutting the image to change a corner.
	LogoRadius int `json:"logoRadius,omitempty"`
	// Entries are the applications, in the order they are offered. Read in
	// every mode but "none"; the icons and children below them are read only
	// when a bar is drawn.
	Entries []PortalEntry `json:"entries,omitempty"`
	// NotifyPage is where the users read their notifications (NOTIF-05), in
	// every mode: the bar's entry, the user button's, the toast and the mails
	// all lead there. Kept here because it is a place in the catalogue's
	// world - a route and a path under it - and the built-in page when empty.
	NotifyPage NotifyPage `json:"notifyPage,omitzero"`
}

// NotifyPage is the notifications page: the built-in one when RouteID is
// empty, or a page of the integrator's - a UI route and a path under it,
// built on the data plane's notifications API. One or the other, never both:
// the built-in page stays reachable at its address whatever is chosen here.
type NotifyPage struct {
	RouteID string `json:"routeId,omitempty"`
	// Path is under the route's address: "/inbox" on a route at /app is
	// /app/inbox. It may carry a query and a fragment, for an application
	// that routes in its fragment (/#/inbox).
	Path string `json:"path,omitempty"`
}

// PortalSpecialNotifications is the notifications entry (NOTIF-05): an entry
// of the catalogue that opens no route but the notifications page, and wears
// the unread badge. Placed, moved and removed like the others; its presence
// is what puts it in the bar.
const PortalSpecialNotifications = "notifications"

// NotificationsEntry says the catalogue places the notifications entry where
// the bar shows it: not turned off, nor inside a container turned off.
func (c PortalConfig) NotificationsEntry() bool {
	for _, e := range c.Entries {
		if e.Disabled {
			continue
		}
		if e.Special == PortalSpecialNotifications {
			return true
		}
		for _, ch := range e.Children {
			if !ch.Disabled && ch.Special == PortalSpecialNotifications {
				return true
			}
		}
	}
	return false
}

// PortalEntry is one entry of the catalogue, and it is one of two things.
//
//   - A MODULE: an application - RouteID names its UI route, whose address and
//     access it inherits; a label and an icon may override what the route
//     offers. A module holds no sub-modules.
//   - A CONTAINER: no route, a label, an icon and a description of its own, and
//     modules inside it - its sub-modules. Clicking it opens the first
//     sub-module the visitor may open, and a container none of whose
//     sub-modules the visitor may open is not offered at all.
//
// Two kinds and not a parent that is also an application: a parent that was a
// route AND held children needed a second name for its own row ("home"), and
// a container needs none - it has no page of its own.
type PortalEntry struct {
	// RouteID binds a MODULE to its UI route. Empty makes the entry a
	// container.
	RouteID string `json:"routeId,omitempty"`
	// Icon is the chosen glyph as an SVG string (viewBox + path only, picked
	// from the console's icon bank), NOT a font ligature name: the bar renders
	// it as a CSS mask, so no icon font is ever loaded. Empty falls back to the
	// label's initial.
	Icon string `json:"icon,omitempty"`
	// Label is the name this application is offered under. Empty falls back to
	// the route's name - the LAST remaining fallback, and there is no third:
	// the route no longer carries a menu label of its own.
	Label string `json:"label,omitempty"`
	// Description is the entry's tooltip.
	Description string `json:"description,omitempty"`
	// Disabled turns the module off for everyone without removing it: kept in
	// the config but never served. Zero value (false) means enabled.
	Disabled bool `json:"disabled,omitempty"`
	// Children are a container's modules, its sub-modules. Empty on a module.
	Children []PortalSubEntry `json:"children,omitempty"`
	// Special makes the entry one the gateway provides rather than a route:
	// "notifications" (PortalSpecialNotifications) is the only one. It has no
	// route and no children.
	Special string `json:"special,omitempty"`
}

// IsContainer says the entry is a container rather than a module.
func (e PortalEntry) IsContainer() bool { return e.RouteID == "" && e.Special == "" }

// PortalSubEntry is a module inside a container, shown in the secondary
// surface.
type PortalSubEntry struct {
	RouteID string `json:"routeId"`
	// Icon is an SVG string, same as the parent's (see PortalEntry.Icon).
	Icon        string `json:"icon,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	// Disabled turns the sub-module off for everyone (see PortalEntry.Disabled).
	Disabled bool `json:"disabled,omitempty"`
	// Special: see PortalEntry.Special. The notifications entry may sit in a
	// container too.
	Special string `json:"special,omitempty"`
}

// Portal modes: what the catalogue is drawn as.
const (
	// PortalModeNone offers no catalogue at all. The user button and the
	// built-in pages show the branding name and nothing to click: an
	// installation with one application has no menu to draw.
	PortalModeNone = "none"
	// PortalModeLinks offers the catalogue as a flat list - the user button's
	// Applications submenu, and the built-in data-plane pages.
	PortalModeLinks = "links"
	// PortalModePortal draws the navigation bar on every proxied UI page. The
	// built-in pages then offer ONE link, the first entry the caller may open:
	// once there is a bar, the bar is the navigation, and a page outside the
	// applications only needs a way back in.
	PortalModePortal = "portal"
)

// PortalModes is the closed catalogue in the order the console offers it.
var PortalModes = []string{PortalModeNone, PortalModeLinks, PortalModePortal}

// Portal layouts: which surface carries the top-level entries.
const (
	// PortalHeader: parents in a top strip of tabs, children in a rail.
	PortalHeader = "header"
	// PortalRail: parents in a rail, children in a top strip of tabs.
	PortalRail = "rail"
)

// PortalLayouts is the closed catalogue in the order the console offers it.
var PortalLayouts = []string{PortalHeader, PortalRail}

// Portal display modes: what a nav entry shows.
const (
	PortalDisplayIcon  = "icon"
	PortalDisplayLabel = "label"
	PortalDisplayBoth  = "both"
)

// PortalDisplays is the closed catalogue in the order the console offers it.
var PortalDisplays = []string{PortalDisplayBoth, PortalDisplayIcon, PortalDisplayLabel}

// DefaultPortalConfig is what an installation that never built a catalogue
// gets: no catalogue, and the header arrangement with icon-and-label entries as
// a starting point for whoever later draws a bar.
func DefaultPortalConfig() PortalConfig {
	return PortalConfig{
		Mode: PortalModeNone, Layout: PortalHeader, Side: "left", Display: PortalDisplayBoth,
	}
}

// Portal reads the stored portal configuration, falling back to the default on
// any read error - a broken setting must not take the injection decision with
// it (a gateway that cannot read the portal simply serves the per-route
// buttons, as if the portal were off).
//
// The bar is Enterprise (ee/portal). An image without it reads a portal-mode
// catalogue as links: the list is the same one, so nothing is lost and nothing converted -
// an imported configuration, or a database carried over from the other image,
// offers its applications in the account button's menu instead of a bar.
func (s *Store) Portal(ctx context.Context) PortalConfig {
	cfg := DefaultPortalConfig()
	_ = s.GetSetting(ctx, SettingPortal, &cfg)
	if cfg.Mode == PortalModePortal && !portalBar {
		cfg.Mode = PortalModeLinks
	}
	return cfg
}

// portalBar says the image carries the navigation bar: ee/portal registers it
// (auth.RegisterPortalBar), and the community image never does.
var portalBar bool

// SetPortalBar is how the registration reaches the store. Not a switch an
// operator turns: the bar is code one image has and the other does not.
func SetPortalBar(on bool) { portalBar = on }

// RequirePortalMode refuses a write that asks for the bar on an image that
// does not carry it - said rather than saved as links behind the writer's back.
func RequirePortalMode(cfg PortalConfig) error {
	if cfg.Mode != PortalModePortal || portalBar {
		return nil
	}
	return fmt.Errorf("portal mode %q: the navigation bar is part of the Enterprise edition, and this image does not carry it; allowed modes are %s, %s",
		cfg.Mode, PortalModeNone, PortalModeLinks)
}

// SanitizePortalConfig normalises and validates in place against the routes
// that exist now. An unknown layout, side or route is an error that NAMES what
// is allowed - a refusal one cannot act on is a refusal reported as a bug.
//
// The routes are checked for what they ARE, a UI route, not for what they are
// doing: a route turned off for a while (maintenance, an upstream not deployed
// here) keeps its place, label and icon, and the bar greys it out until it
// comes back. Requiring it enabled made one such entry refuse every save of
// the whole portal, including saves that only added something valid.
//
// stored is the portal as saved (nil for none). A reference it already holds
// passes even when its route was deleted or stopped being a UI route since:
// the payload builder skips it at read time, and the console names it so the
// entry can be removed. Refusing it would turn a change made ELSEWHERE into a
// portal nobody can save any more. Only what a write ADDS must be a UI route.
func SanitizePortalConfig(cfg *PortalConfig, routes []Route, stored *PortalConfig) error {
	cfg.Mode = strings.TrimSpace(cfg.Mode)
	if cfg.Mode == "" {
		cfg.Mode = PortalModeNone
	}
	if !slices.Contains(PortalModes, cfg.Mode) {
		return fmt.Errorf("portal mode %q: allowed are %s", cfg.Mode, strings.Join(PortalModes, ", "))
	}
	cfg.Layout = strings.TrimSpace(cfg.Layout)
	if cfg.Layout == "" {
		cfg.Layout = PortalHeader
	}
	if !slices.Contains(PortalLayouts, cfg.Layout) {
		return fmt.Errorf("portal layout %q: allowed are %s", cfg.Layout, strings.Join(PortalLayouts, ", "))
	}
	cfg.Side = strings.TrimSpace(cfg.Side)
	switch cfg.Side {
	case "":
		cfg.Side = "left"
	case "left", "right":
	default:
		return fmt.Errorf("portal side %q: allowed are left, right", cfg.Side)
	}
	cfg.Display = strings.TrimSpace(cfg.Display)
	if cfg.Display == "" {
		cfg.Display = PortalDisplayBoth
	}
	if !slices.Contains(PortalDisplays, cfg.Display) {
		return fmt.Errorf("portal display %q: allowed are %s", cfg.Display, strings.Join(PortalDisplays, ", "))
	}
	// Past a half the corners meet and more only distorts the mark, so the
	// refusal names the two ends rather than clamping in silence.
	if cfg.LogoRadius < 0 || cfg.LogoRadius > 50 {
		return fmt.Errorf("portal logo radius %d: allowed are 0 (as drawn) to 50 (circle)", cfg.LogoRadius)
	}

	// A catalogue entry points at an application: the route must exist and be
	// a UI route, enabled or not. Anything else is refused, naming the id.
	known := map[string]Route{}
	for _, rt := range routes {
		known[rt.ID] = rt
	}
	held := map[string]bool{}
	if stored != nil {
		for _, e := range stored.Entries {
			held[e.RouteID] = true
			for _, c := range e.Children {
				held[c.RouteID] = true
			}
		}
	}
	checkRoute := func(kind, id string) error {
		if id == "" {
			return fmt.Errorf("portal %s: an entry needs a route", kind)
		}
		rt, ok := known[id]
		switch {
		case held[id] && (!ok || !rt.IsUI):
			return nil // see stored above
		case !ok:
			return fmt.Errorf("portal %s route %q: no such route", kind, id)
		case !rt.IsUI:
			return fmt.Errorf("portal %s route %q (%s): not a UI route - turn UI on in the route, or pick another", kind, id, rt.Name)
		}
		return nil
	}

	if err := sanitizeNotifyPage(&cfg.NotifyPage, known, stored); err != nil {
		return err
	}

	// The notifications entry is offered once: a second one is dropped, the
	// first wins, like a route listed twice.
	seenSpecial := false
	special := func(kind, name, routeID string, children int) (keep bool, err error) {
		if name != PortalSpecialNotifications {
			return false, fmt.Errorf("portal %s special %q: allowed is %s", kind, name, PortalSpecialNotifications)
		}
		if routeID != "" {
			return false, fmt.Errorf("portal %s %q: it opens the notifications page, not a route - leave its route empty", kind, name)
		}
		if children > 0 {
			return false, fmt.Errorf("portal %s %q: it holds no modules - add a container and move it in", kind, name)
		}
		if seenSpecial {
			return false, nil
		}
		seenSpecial = true
		return true, nil
	}
	// The bell when no icon was chosen: the entry is drawn like any other,
	// and an entry without an icon is drawn with the label's initial.
	specialIcon := func(icon string) string {
		if icon = icons.Resolve(icon); icon != "" {
			return icon
		}
		return icons.Resolve("notifications")
	}

	seenEntries := map[string]bool{}
	entries := cfg.Entries[:0]
	for i := range cfg.Entries {
		p := cfg.Entries[i]
		p.RouteID = strings.TrimSpace(p.RouteID)
		p.Label = strings.TrimSpace(p.Label)
		p.Special = strings.TrimSpace(p.Special)
		if p.Special != "" {
			keep, err := special("entry", p.Special, p.RouteID, len(p.Children))
			if err != nil {
				return err
			}
			if keep {
				p.Icon = specialIcon(p.Icon)
				p.Description = strings.TrimSpace(p.Description)
				entries = append(entries, p)
			}
			continue
		}
		if p.IsContainer() {
			// A container has no route to name it, so it needs a label of its
			// own: an untitled heading is one nobody can find again.
			if p.Label == "" {
				return fmt.Errorf("portal container #%d: a container needs a label", i+1)
			}
		} else {
			if err := checkRoute("entry", p.RouteID); err != nil {
				return err
			}
			if len(p.Children) > 0 {
				return fmt.Errorf("portal entry %q: a module holds no sub-modules - "+
					"add a container and move the modules into it", p.RouteID)
			}
			if seenEntries[p.RouteID] {
				// A route listed twice is one choice offered twice: the second
				// is dropped, the first wins (the order the admin set).
				continue
			}
			seenEntries[p.RouteID] = true
		}
		// The icon is stored as an SVG: a bare name is resolved to its
		// catalogue SVG, a pasted SVG is sanitized to viewBox + path, anything
		// else is dropped (icons.Resolve).
		p.Icon = icons.Resolve(p.Icon)
		p.Description = strings.TrimSpace(p.Description)

		seenChildren := map[string]bool{}
		children := p.Children[:0]
		for j := range p.Children {
			c := p.Children[j]
			c.RouteID = strings.TrimSpace(c.RouteID)
			if c.Special = strings.TrimSpace(c.Special); c.Special != "" {
				keep, err := special("sub-entry", c.Special, c.RouteID, 0)
				if err != nil {
					return err
				}
				if keep {
					c.Icon = specialIcon(c.Icon)
					c.Label = strings.TrimSpace(c.Label)
					c.Description = strings.TrimSpace(c.Description)
					children = append(children, c)
				}
				continue
			}
			if err := checkRoute("sub-entry", c.RouteID); err != nil {
				return err
			}
			if seenChildren[c.RouteID] {
				continue
			}
			seenChildren[c.RouteID] = true
			c.Icon = icons.Resolve(c.Icon)
			c.Label = strings.TrimSpace(c.Label)
			c.Description = strings.TrimSpace(c.Description)
			children = append(children, c)
		}
		p.Children = children
		entries = append(entries, p)
	}
	cfg.Entries = entries
	return nil
}

// sanitizeNotifyPage checks the notifications page: the built-in one (both
// fields empty), or a UI route and a path under it. A route the stored portal
// already names passes even when it was deleted or stopped being a UI route
// since, as for the entries: the built-in page takes over at read time.
func sanitizeNotifyPage(n *NotifyPage, known map[string]Route, stored *PortalConfig) error {
	n.RouteID, n.Path = strings.TrimSpace(n.RouteID), strings.TrimSpace(n.Path)
	if n.RouteID == "" {
		if n.Path != "" {
			return fmt.Errorf("notifications page path %q: pick the UI route it is under, or leave both empty for the built-in page", n.Path)
		}
		return nil
	}
	rt, ok := known[n.RouteID]
	held := stored != nil && stored.NotifyPage.RouteID == n.RouteID
	switch {
	case held && (!ok || !rt.IsUI):
	case !ok:
		return fmt.Errorf("notifications page route %q: no such route", n.RouteID)
	case !rt.IsUI:
		return fmt.Errorf("notifications page route %q (%s): not a UI route - turn UI on in the route, or pick another", n.RouteID, rt.Name)
	}
	if n.Path == "" {
		n.Path = "/"
	}
	return checkNotifyPath(n.Path)
}

// checkNotifyPath refuses what is not a path under a route: another host, a
// way out of the route (..), a backslash or a space a browser reads its own
// way.
func checkNotifyPath(p string) error {
	bad := func(why string) error {
		return fmt.Errorf("notifications page path %q: %s; give a path under the route, like /inbox or /#/inbox", p, why)
	}
	switch {
	case len(p) > 512:
		return bad("longer than 512 characters")
	case !strings.HasPrefix(p, "/"):
		return bad("it must start with /")
	case strings.HasPrefix(p, "//"):
		return bad("// names another host")
	case strings.ContainsAny(p, "\\<>\"'"):
		return bad("no backslash, quote or angle bracket")
	case strings.IndexFunc(p, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0:
		return bad("no space or control character")
	}
	u, err := url.Parse(p)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return bad("not a path")
	}
	if slices.Contains(strings.Split(u.Path, "/"), "..") {
		return bad(".. leaves the route")
	}
	return nil
}
