package store

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// An Injection is one block of a UI route's own CSS or JavaScript (UIF-02):
// written here, or a file uploaded on the route, placed at the start or the
// end of the head or at the end of the body.
//
// A LIST, because order is the whole question. In CSS the last rule of equal
// weight wins, so an override has to come AFTER the application's own
// stylesheets - the end of the head - and one injected at the start, where
// the two free blocks this list replaces used to go, lost to them. A script
// that uses a library comes after the library. The list is the order, within
// each place; the places are the order between them.
type Injection struct {
	// Kind is css or js.
	Kind string `json:"kind"`
	// Code is the block itself. Empty when File is set.
	Code string `json:"code,omitempty"`
	// File names a file uploaded on the route (route_files), served at
	// RouteAssetPath and linked rather than pasted in.
	File string `json:"file,omitempty"`
	// Position is where it lands: head-start, head-end or body-end.
	Position string `json:"position"`
	// Load is how a script is run: "" (classic, where it stands), defer,
	// async or module. defer and async mean nothing to a script written in
	// the page - a browser ignores them there - so they are a file's only.
	Load string `json:"load,omitempty"`
}

// The kinds, places and loads, in the order the console offers them.
const (
	InjectCSS = "css"
	InjectJS  = "js"

	InjectHeadStart = "head-start"
	InjectHeadEnd   = "head-end"
	InjectBodyEnd   = "body-end"

	LoadDefer  = "defer"
	LoadAsync  = "async"
	LoadModule = "module"
)

// InjectPositions are the places a block may land, in page order.
var InjectPositions = []string{InjectHeadStart, InjectHeadEnd, InjectBodyEnd}

// InjectLoads are the ways a script may run; empty is classic.
var InjectLoads = []string{"", LoadDefer, LoadAsync, LoadModule}

// MaxInjections and MaxInjectionCode bound a route's list: page tweaks, not a
// second application.
const (
	MaxInjections    = 32
	MaxInjectionCode = 64 << 10
)

// RouteAssetPath is where the data plane serves a file an injection names -
// not under the route's own path, which belongs to the application behind it.
const RouteAssetPath = "/meerkat/route-assets/"

// CheckInjections refuses a list the gateway could not write into a page, and
// fills the place a block left empty: the end of the head, the one that
// works for both kinds.
func CheckInjections(list []Injection) error {
	if len(list) > MaxInjections {
		return fmt.Errorf("custom code: %d blocks, the limit is %d", len(list), MaxInjections)
	}
	for i := range list {
		in := &list[i]
		where := fmt.Sprintf("custom code #%d", i+1)
		switch in.Kind {
		case InjectCSS, InjectJS:
		default:
			return fmt.Errorf("%s: kind %q is not allowed: allowed kinds are css, js", where, in.Kind)
		}
		if in.Position == "" {
			in.Position = InjectHeadEnd
		}
		if !slices.Contains(InjectPositions, in.Position) {
			return fmt.Errorf("%s: position %q is not allowed: allowed positions are %s",
				where, in.Position, strings.Join(InjectPositions, ", "))
		}
		switch {
		case in.File != "" && in.Code != "":
			return fmt.Errorf("%s: a block is written here or is a file, not both", where)
		case in.File != "":
			name, err := CheckRouteFileName(in.File)
			if err != nil {
				return fmt.Errorf("%s: %w", where, err)
			}
			in.File = name
		default:
			closing := "</style"
			if in.Kind == InjectJS {
				closing = "</script"
			}
			if strings.Contains(strings.ToLower(in.Code), closing) {
				return fmt.Errorf("%s: the %s must not contain %q", where, in.Kind, closing)
			}
			if len(in.Code) > MaxInjectionCode {
				return fmt.Errorf("%s: %d bytes, the limit is 64 KiB", where, len(in.Code))
			}
		}
		if in.Kind == InjectCSS && in.Load != "" {
			return fmt.Errorf("%s: a stylesheet has no load mode (%q)", where, in.Load)
		}
		if !slices.Contains(InjectLoads, in.Load) {
			return fmt.Errorf("%s: load %q is not allowed: allowed loads are classic (empty), defer, async, module", where, in.Load)
		}
		if in.File == "" && (in.Load == LoadDefer || in.Load == LoadAsync) {
			return fmt.Errorf("%s: %s only applies to a script loaded from a file - a browser ignores it on a script written in the page; use module, or a file", where, in.Load)
		}
	}
	return nil
}

// UnmarshalJSON reads a ui block, including the two free blocks a route had
// before the list (customCss, customJs). Those are what a configuration
// exported by v1.0 carries, and an import that dropped them would lose
// somebody's code without a word: they become the list's first two entries,
// where they used to land - the start of the head - so a page looks the same.
// Nothing writes them any more.
func (u *RouteUI) UnmarshalJSON(b []byte) error {
	type plain RouteUI
	var v struct {
		plain
		CustomCSS string `json:"customCss"`
		CustomJS  string `json:"customJs"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*u = RouteUI(v.plain)
	var legacy []Injection
	if v.CustomCSS != "" {
		legacy = append(legacy, Injection{Kind: InjectCSS, Code: v.CustomCSS, Position: InjectHeadStart})
	}
	if v.CustomJS != "" {
		legacy = append(legacy, Injection{Kind: InjectJS, Code: v.CustomJS, Position: InjectHeadStart})
	}
	if len(legacy) > 0 {
		u.Injections = append(legacy, u.Injections...)
	}
	return nil
}
