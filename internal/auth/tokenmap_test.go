package auth

import "testing"

// The map is what the locale editor lists beside a page, so it has to be
// neither empty nor everything: a page that reports nothing gives a translator
// an empty screen, and one that reports the whole catalogue gives them the
// list they were trying to escape.
// englishOnlyPages are the screens this product does not translate: a
// developer's tools, shown to whoever writes the service rather than to the
// people who use it. They MAY report nothing, which everywhere else is an
// error - a page whose strings nobody can list is a page nobody can correct.
//
// May, not must: they still carry the odd shared string, a back link or a
// title that belongs to a translated surface as much as to this one, and
// translating those twice would be the silliest possible saving.
var englishOnlyPages = map[string]bool{
	"oauth-consent":   true, // approving an agent (MCP-07)
	"profile-dev":     true, // the developer hub
	"profile-dev-key": true, // the plug key
	"plugged":         true, // served from a developer's machine
}

func TestEveryPageReportsSomeKeysAndNotAllOfThem(t *testing.T) {
	total := len(messages["en"])
	for _, p := range PreviewPages {
		own, _ := TemplateKeys(p.Key)
		if len(own) == 0 && !englishOnlyPages[p.Key] {
			t.Errorf("%s renders no catalogue key at all", p.Key)
		}

		if len(own) > total/2 {
			t.Errorf("%s reports %d keys of %d: that is not a page, that is the catalogue",
				p.Key, len(own), total)
		}
	}
}

// Asking the PAGE finds what reading the template cannot: half of these
// strings are chosen in Go and handed to the template, so they appear in no
// {{.T.x}} anywhere. The sign-in page's refusal is one of them.
func TestTheMapFindsKeysTheTemplateNeverSpells(t *testing.T) {
	own, _ := TemplateKeys("login")
	found := false
	for _, k := range own {
		if k == "errInvalidCreds" {
			found = true
		}
	}
	if !found {
		t.Error("the sign-in page's refusal is missing: it is handed in from Go, not spelled in the template")
	}
}

// No key is unreachable. Every one is either on a page or in the orphan list,
// and the editor shows both - a key in neither could never be corrected.
func TestNoKeyIsUnreachable(t *testing.T) {
	seen := map[string]bool{}
	// Every template the map knows, not every flow PAGE: the bar is previewable
	// and carries forty of these strings, and a list of families written out
	// here would go stale the first time a third one is added.
	keysOnce.Do(buildKeyMap)
	for key := range keysOf {
		own, shell := TemplateKeys(key)
		for _, k := range append(append([]string{}, own...), shell...) {
			seen[k] = true
		}
	}
	for _, k := range OrphanKeys() {
		seen[k] = true
	}
	for k := range messages["en"] {
		if !seen[k] {
			t.Errorf("key %q is on no page and in no orphan list: nothing can ever correct it", k)
		}
	}
}
