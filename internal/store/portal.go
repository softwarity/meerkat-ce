package store

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/softwarity/meerkat/internal/icons"
)

// PortalConfig is the navigation portal (PORTAL-01): a header or a rail that
// moves between the UI routes this gateway serves, as if they were one
// application. It is GLOBAL, like the theme and the branding - one arrangement
// for the installation, already personalised per visitor by the ROUTE ACCESS
// (a module a caller may not open is not offered). A per-tenant arrangement is
// PORTAL-02 and deliberately not this: it would be the product's first
// per-tenant visual override, and the theme and branding are global today.
//
// It is edited on its own screen (Application, Portal) and shown, when on, on
// every proxied UI page - injected the same way the user button is, which then
// rides INSIDE the portal bar rather than in a corner of its own.
type PortalConfig struct {
	// Enabled off is the state of every installation until someone builds a
	// portal: the per-route user button behaves exactly as before.
	Enabled bool `json:"enabled"`
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
	// ShowAppName writes the branding app name beside the data-plane logo in
	// the bar. Off by default: the logo alone is the mark. Only meaningful when
	// a logo is set - with none, the name is already the brand it falls back to.
	ShowAppName bool `json:"showAppName,omitempty"`
	// Parents are the top-level modules, in display order.
	Parents []ModuleParent `json:"parents,omitempty"`
}

// ModuleParent is one top-level module: a full application in its own right
// (it has a route and is navigable), which may gather children shown in the
// secondary surface. When it has children, the bar offers a "home" back to the
// parent itself.
type ModuleParent struct {
	// RouteID binds the entry to an existing UI route; the entry inherits the
	// route's address and access from it, and a label/icon may override what
	// the route offers.
	RouteID string `json:"routeId"`
	// Icon is the chosen glyph as an SVG string (viewBox + path only, picked
	// from the console's icon bank), NOT a font ligature name: the bar renders
	// it as a CSS mask, so no icon font is ever loaded. Empty falls back to the
	// label's initial.
	Icon string `json:"icon,omitempty"`
	// Label overrides the route's own name in the bar; empty uses the route.
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
	Disabled bool          `json:"disabled,omitempty"`
	Children []ModuleChild `json:"children,omitempty"`
}

// ModuleChild is a sub-module of a parent, shown in the secondary surface.
type ModuleChild struct {
	RouteID string `json:"routeId"`
	// Icon is an SVG string, same as the parent's (see ModuleParent.Icon).
	Icon        string `json:"icon,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	Badge       string `json:"badge,omitempty"`
	// Disabled turns the sub-module off for everyone (see ModuleParent.Disabled).
	Disabled bool `json:"disabled,omitempty"`
}

// Portal layouts: which surface carries the parents.
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

// DefaultPortalConfig is what an installation that never built a portal gets:
// off, and the header arrangement with icon-and-label entries as a starting
// point once turned on.
func DefaultPortalConfig() PortalConfig {
	return PortalConfig{Enabled: false, Layout: PortalHeader, Side: "left", Display: PortalDisplayBoth}
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

	// A UI route is one that can wear the portal bar: it must exist, be enabled
	// and be a UI route. Anything else is refused, naming the id.
	uiRoutes := map[string]bool{}
	for _, rt := range routes {
		if rt.Enabled && rt.IsUI {
			uiRoutes[rt.ID] = true
		}
	}
	checkRoute := func(kind, id string) error {
		if id == "" {
			return fmt.Errorf("portal %s: a module needs a route", kind)
		}
		if !uiRoutes[id] {
			return fmt.Errorf("portal %s route %q: not an enabled UI route", kind, id)
		}
		return nil
	}

	seenParents := map[string]bool{}
	parents := cfg.Parents[:0]
	for i := range cfg.Parents {
		p := cfg.Parents[i]
		if err := checkRoute("module", p.RouteID); err != nil {
			return err
		}
		if seenParents[p.RouteID] {
			// A route listed twice as a parent is one choice offered twice:
			// the second is dropped, the first wins (the order the admin set).
			continue
		}
		seenParents[p.RouteID] = true
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
			if err := checkRoute("sub-module", c.RouteID); err != nil {
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
		parents = append(parents, p)
	}
	cfg.Parents = parents
	return nil
}
