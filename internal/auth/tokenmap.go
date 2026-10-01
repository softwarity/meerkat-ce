package auth

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/softwarity/meerkat/internal/store"
)

// Which catalogue keys a screen actually shows (I18N).
//
// The locale editor puts the strings of the page you are looking at beside the
// page, which means something has to know that the organisation chooser reads
// six keys and not the other two hundred and ninety-six. Nothing did.
//
// THE PAGE IS ASKED RATHER THAN READ. Rendering it once against a catalogue
// whose every value is a unique marker, then looking at which markers came
// out, gives the exact set - whatever route a string took to get there. That
// matters: reading {{.T.x}} out of the template text finds only what the
// template spells, and a good half of these strings are chosen in Go and
// handed in (an error message, a title, a status line). Walking the parse tree
// found 131 keys of 302; asking the page finds what it really draws.
//
// The cost is one render per template, once, behind a sync.Once.

// probePrefix and probeSuffix bracket a key inside a rendered page. Letters
// only: the marker travels through HTML escaping, attribute quoting and URL
// encoding without changing shape, which a punctuation-based sentinel would
// not.
const (
	probePrefix = "mkIEighteenNKeY"
	probeSuffix = "eNdKeY"
)

var (
	keysOnce sync.Once
	keysOf   map[string][]string
	shellSet []string
)

// TemplateKeys returns the catalogue keys one previewable template renders,
// and the keys every page renders whatever it is (the shell: the brand line,
// the footer, the mark).
//
// A page's own list is what it OWNS, shell excluded: thirty pieces of
// furniture in front of five real strings is a list nobody reads.
func TemplateKeys(previewKey string) (own, shell []string) {
	keysOnce.Do(buildKeyMap)
	return keysOf[previewKey], shellSet
}

// ScreenCount is how many previewable screens render a string.
//
// A catalogue key exists ONCE - the screens share it, they do not each own a
// copy - so correcting "Cancel" on the report form corrects it in the account
// menu and on four other screens at the same time. The editor says so with
// this, because a wording tuned for one context and moved for five others is
// the kind of damage nobody notices until somebody else reports it.
func ScreenCount(key string) int {
	keysOnce.Do(buildKeyMap)
	n := 0
	for _, ks := range keysOf {
		for _, k := range ks {
			if k == key {
				n++
				break
			}
		}
	}
	for _, k := range shellSet {
		if k == key {
			// The shell is on every page, so it is on every screen.
			return len(keysOf)
		}
	}
	return n
}

// ShellKeys is what every page carries.
func ShellKeys() []string {
	keysOnce.Do(buildKeyMap)
	return shellSet
}

// OrphanKeys are the catalogue keys no previewable screen renders: the strings
// of a page the catalogue does not carry, and the ones only a mail or a log
// ever shows. Listed so that no key becomes untranslatable for want of a
// screen to show it on.
func OrphanKeys() []string {
	keysOnce.Do(buildKeyMap)
	seen := map[string]bool{}
	for _, ks := range keysOf {
		for _, k := range ks {
			seen[k] = true
		}
	}
	for _, k := range shellSet {
		seen[k] = true
	}
	var out []string
	for k := range messages["en"] {
		if !seen[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func buildKeyMap() {
	probe := probeCatalogue()
	// The shell first: a page with an empty body renders the top and the
	// bottom, and nothing else.
	shellSet = renderedKeys(probe, func(c flowChrome) []byte {
		var b bytes.Buffer
		_ = flowPage("shell", "").Execute(&b, struct{ flowChrome }{c})
		return b.Bytes()
	})
	inShell := make(map[string]bool, len(shellSet))
	for _, k := range shellSet {
		inShell[k] = true
	}

	keysOf = make(map[string][]string, len(PreviewPages)+len(PortalPreviewKinds))
	// The bar draws its text from a JSON payload the page carries, not from a
	// Go template, so the probe finds those strings the same way it finds a
	// page's: they are IN the rendered document. Which is the whole reason the
	// catalogue is handed to portalPreviewHTML instead of resolved inside it.
	for _, k := range PortalPreviewKinds {
		kind := k
		keysOf[kind.Key] = renderedKeys(probe, func(c flowChrome) []byte {
			html, ok := portalPreviewHTML(kind.Key, store.DefaultTheme(), store.DefaultBranding(),
				"light", c.T, c.Lang)
			if !ok {
				return nil
			}
			return []byte(html)
		})
	}
	// The MAILS. They are previewable - the picker offers four of them - and
	// their nineteen strings came out of no screen at all, for the same reason
	// the titles did: this loop did not walk them. The spec is where a message
	// keeps its words, and it is built from the catalogue, so scanning it finds
	// them without a store, a palette or a rendered HTML body.
	for _, k := range MailSampleKinds {
		if k.Operator {
			// An operator's message is not offered by the picker (it wears the
			// console's own colours), so it has no screen to be corrected on.
			continue
		}
		spec, ok := sampleSpec(k.Key, probe, probePrefix+"appName"+probeSuffix, "https://example.test")
		if !ok {
			continue
		}
		// Marshalled rather than printed: the spec's button is a POINTER, and
		// %+v prints its address - which is how the two call-to-action labels
		// sat on no screen while the rest of the message was found.
		raw, err := json.Marshal(spec)
		if err != nil {
			continue
		}
		all := scanProbes(string(raw))
		own := make([]string, 0, len(all))
		for _, key := range all {
			if !inShell[key] && key != "appName" {
				own = append(own, key)
			}
		}
		sort.Strings(own)
		keysOf["mail:"+k.Key] = own
	}
	for _, p := range PreviewPages {
		page := p
		all := renderedKeys(probe, func(c flowChrome) []byte {
			tmpl, data := page.build(page.dress(c, true))
			if tmpl == nil {
				return nil
			}
			var b bytes.Buffer
			_ = tmpl.Execute(&b, data)
			return b.Bytes()
		})
		own := make([]string, 0, len(all))
		for _, k := range all {
			if !inShell[k] {
				own = append(own, k)
			}
		}
		keysOf[page.Key] = own
	}
}

// renderedKeys renders once with the probe catalogue and reports which keys
// came out, sorted.
func renderedKeys(probe map[string]string, render func(flowChrome) []byte) []string {
	// A chrome with everything TURNED ON. These templates hide most of
	// themselves behind conditions, and a zero value takes every "off" branch -
	// which renders a page that shows almost nothing and reports almost no
	// keys. Two languages so the switcher draws; the Meerkat brand so the mark
	// and its footer do.
	out := scanProbes(string(render(flowChrome{
		T: probe, Lang: "en", Dir: "ltr", Langs: []string{"en", "fr"},
		Brand:     brandView{AppName: "App", Meerkat: true},
		PoweredBy: true, MarkText: MarkText, MarkURL: MarkURL,
		Preview: true,
	})))
	sort.Strings(out)
	return out
}

func probeCatalogue() map[string]string {
	en := messages["en"]
	probe := make(map[string]string, len(en))
	for k := range en {
		probe[k] = probePrefix + k + probeSuffix
	}
	return probe
}

// scanProbes pulls the keys out of a rendered page. A key can appear many
// times and in any order; the set is what matters.
func scanProbes(page string) []string {
	seen := map[string]bool{}
	for i := 0; ; {
		j := strings.Index(page[i:], probePrefix)
		if j < 0 {
			break
		}
		start := i + j + len(probePrefix)
		end := strings.Index(page[start:], probeSuffix)
		if end < 0 {
			break
		}
		seen[page[start:start+end]] = true
		i = start + end + len(probeSuffix)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}
