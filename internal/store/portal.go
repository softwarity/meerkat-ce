package store

import (
	"context"
	"fmt"
	"slices"
	"strings"

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
}

// PortalEntry is one application in the catalogue: a route, the name it is
// offered under, and - when a bar is drawn - an icon and the children shown in
// the secondary surface. When it has children, the bar offers a "home" back to
// the entry itself.
type PortalEntry struct {
	// RouteID binds the entry to an existing UI route; the entry inherits the
	// route's address and access from it, and a label/icon may override what
	// the route offers.
	RouteID string `json:"routeId"`
	// Icon is the chosen glyph as an SVG string (viewBox + path only, picked
	// from the console's icon bank), NOT a font ligature name: the bar renders
	// it as a CSS mask, so no icon font is ever loaded. Empty falls back to the
	// label's initial.
	Icon string `json:"icon,omitempty"`
	// Label is the name this application is offered under. Empty falls back to
	// the route's name - the LAST remaining fallback, and there is no third:
	// the route no longer carries a menu label of its own.
	Label string `json:"label,omitempty"`
	// HomeLabel is what the "back to this module" entry reads when the parent
	// has children (its own row in the secondary surface). Empty uses Label.
	HomeLabel string `json:"homeLabel,omitempty"`
	// Description is the entry's tooltip.
	Description string `json:"description,omitempty"`
	// Badge is the channel key a future notifier writes a count onto (the
	// mechanism is deferred; the slot is reserved so the shape does not move
	// under it later).
	Badge string `json:"badge,omitempty"`
	// Disabled turns the module off for everyone without removing it: kept in
	// the config but never served. Zero value (false) means enabled.
	Disabled bool             `json:"disabled,omitempty"`
	Children []PortalSubEntry `json:"children,omitempty"`
}

// PortalSubEntry is a sub-module of a parent, shown in the secondary surface.
type PortalSubEntry struct {
	RouteID string `json:"routeId"`
	// Icon is an SVG string, same as the parent's (see PortalEntry.Icon).
	Icon        string `json:"icon,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	Badge       string `json:"badge,omitempty"`
	// Disabled turns the sub-module off for everyone (see PortalEntry.Disabled).
	Disabled bool `json:"disabled,omitempty"`
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
func (s *Store) Portal(ctx context.Context) PortalConfig {
	cfg := DefaultPortalConfig()
	_ = s.GetSetting(ctx, SettingPortal, &cfg)
	return cfg
}

// SanitizePortalConfig normalises and validates in place against the routes
// that exist now. An unknown layout, side or route is an error that NAMES what
// is allowed - a refusal one cannot act on is a refusal reported as a bug.
//
// A route referenced here and deleted LATER is not this function's problem: it
// leaves a dangling id that the payload builder skips at read time. This one
// guards the WRITE: nothing enters the portal that is not, right now, an
// enabled UI route.
func SanitizePortalConfig(cfg *PortalConfig, routes []Route) error {
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

	// A catalogue entry points at an application: the route must exist, be
	// enabled and be a UI route. Anything else is refused, naming the id.
	uiRoutes := map[string]bool{}
	for _, rt := range routes {
		if rt.Enabled && rt.IsUI {
			uiRoutes[rt.ID] = true
		}
	}
	checkRoute := func(kind, id string) error {
		if id == "" {
			return fmt.Errorf("portal %s: an entry needs a route", kind)
		}
		if !uiRoutes[id] {
			return fmt.Errorf("portal %s route %q: not an enabled UI route", kind, id)
		}
		return nil
	}

	seenEntries := map[string]bool{}
	entries := cfg.Entries[:0]
	for i := range cfg.Entries {
		p := cfg.Entries[i]
		if err := checkRoute("entry", p.RouteID); err != nil {
			return err
		}
		if seenEntries[p.RouteID] {
			// A route listed twice is one choice offered twice: the second
			// is dropped, the first wins (the order the admin set).
			continue
		}
		seenEntries[p.RouteID] = true
		// The icon is stored as an SVG: a bare name is resolved to its
		// catalogue SVG, a pasted SVG is sanitized to viewBox + path, anything
		// else is dropped (icons.Resolve).
		p.Icon = icons.Resolve(p.Icon)
		p.Label = strings.TrimSpace(p.Label)
		p.HomeLabel = strings.TrimSpace(p.HomeLabel)
		p.Description = strings.TrimSpace(p.Description)
		p.Badge = strings.TrimSpace(p.Badge)

		seenChildren := map[string]bool{}
		children := p.Children[:0]
		for j := range p.Children {
			c := p.Children[j]
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
			c.Badge = strings.TrimSpace(c.Badge)
			children = append(children, c)
		}
		p.Children = children
		entries = append(entries, p)
	}
	cfg.Entries = entries
	return nil
}
