package auth

import (
	"sync"

	"github.com/softwarity/meerkat/internal/store"
)

// The ARRANGEMENT of the served pages (PAGE-02), one CSS block per layout.
//
// One skeleton, many layouts. Every flow page is flowTop + body + flowBottom,
// and the shared chrome now wraps its two halves - <div class="brand"> (logo,
// name, tagline) and <div class="pane"> (whatever the page is, plus the foot
// and the language/scheme buttons). That is all a layout needs to move things
// around: no page knows which one is active, and adding one is a block of CSS
// rather than eighteen new templates to keep in step.
//
// Only the ACTIVE layout's block is emitted. The framed block always is: it
// answers a context, not a choice.
//
// Every arrangement beyond `centered` is REGISTERED, from ee/layouts, and the
// community image does not link that package. So it does not merely refuse a
// split sign-in page - it cannot draw one, and says so by falling back. That
// fallback is not a nicety: the body class is written by the template, so a
// page that declared itself mk-split with no rule to dress it would be a
// BROKEN page, not a centred one.
func layoutCSS(l store.PageLayout) string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	b, ok := registry[l.Name]
	if !ok {
		return framedCSS // centered: the chrome's own arrangement
	}
	return b.css + b.right + framedCSS
}

// Known reports whether THIS build can draw an arrangement. The chrome asks
// before stamping the class, so a value saved by an Enterprise image and read
// by a community one degrades to centred instead of breaking.
func Known(name string) bool {
	if name == "" || name == store.LayoutCentered {
		return true
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[name]
	return ok
}

// allLayoutsCSS carries the WHOLE catalogue, and only the console's specimen
// gets it: the Layout tab flips a class on <body> and the arrangement changes
// under the cursor. Reloading the frame to fetch another block would flash a
// blank page at every candidate, which is precisely when someone is comparing
// them. A served page never pays for this - it gets its own block alone.
func allLayoutsCSS() string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	css := ""
	for _, name := range store.PageLayouts {
		if b, ok := registry[name]; ok {
			css += b.css + b.right
		}
	}
	return css + framedCSS
}

type layoutBlock struct{ css, right string }

var (
	registryMu sync.RWMutex
	registry   = map[string]layoutBlock{}
)

// RegisterLayout adds one arrangement. Called from ee/layouts' init(): the
// blank import in the Enterprise edition file is the whole seam.
func RegisterLayout(name, css, right string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = layoutBlock{css: css, right: right}
}

// framed: what a page does when it is somebody's iframe. Not a choice in the
// catalogue - a response to a context, decided by the CLIENT (see the script
// in flowBottom) exactly as the user button decides it (UIF-03), so the HTML
// served stays identical for everyone and no cache can hand one context the
// other's page.
//
// The mark stays. Removing it is what the white-label licence buys, and a
// layout that dropped it in a frame would be a way to buy it with an iframe.
const framedCSS = `
    body.mk-framed { padding: 0; display: block; }
    body.mk-framed::before { display: none; }
    body.mk-framed .brand { display: none; }
    body.mk-framed .foot { display: none; }
    body.mk-framed .watch {
      width: 100%; max-width: 440px; min-height: 100vh; margin: 0 auto;
      align-content: center; padding: 16px 16px 34px; background: none;
      border: 0; box-shadow: none; backdrop-filter: none;
    }
    body.mk-framed .pane { width: 100%; }
    body.mk-framed form, body.mk-framed .panel { margin-top: 0; }`
