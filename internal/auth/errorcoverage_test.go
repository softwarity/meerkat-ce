package auth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// EVERY REFUSAL A HANDLER CAN GIVE HAS A SCREEN IT CAN BE READ ON.
//
// A served page shows one refusal at a time and a preview shows the page once,
// so a wording that no preview stacks is a wording nobody can see rendered -
// and a wording nobody can see rendered is a wording nobody corrects. Of the
// thirty this product carries, two were reachable before previewErrors existed.
//
// The handlers are the authority, not a list kept by hand: this reads them. A
// new refusal rendered that way fails the build until its page claims it, or
// written down as answered in plain text - which is a decision, not an
// oversight, and one that has to be made in writing.
func TestEveryRefusalHasAScreen(t *testing.T) {
	claimed := map[string]bool{}
	for page, keys := range previewErrors {
		for _, k := range keys {
			claimed[k] = true
		}
		if _, ok := previewPageByKey(page); !ok {
			t.Errorf("previewErrors names %q, which is not a page the picker offers", page)
		}
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	// EVERY "errXxx" literal in the package, read off the syntax tree rather
	// than matched in the text.
	//
	// It used to look for h.tr(r, "errX") and nothing else, which missed three
	// refusals of the reset page: they go through a local helper that takes the
	// KEY, so the literal sits at the call site and h.tr sees a variable. A
	// catalogue key only ever appears in this package to be rendered, so the
	// literal is the honest thing to look for - and the tree, not a regexp,
	// because the first version of this matched its own comment.
	key := regexp.MustCompile(`^err[A-Z][A-Za-z0-9]*$`)
	fset := token.NewFileSet()
	found := map[string][]string{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "pagepreview.go" {
			// pagepreview.go IS the table: every key in it is claimed by
			// construction, and reading it back would prove only that.
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, err := strconv.Unquote(lit.Value)
			if err != nil || !key.MatchString(v) {
				return true
			}
			found[v] = append(found[v], name)
			return true
		})
	}
	if len(found) < 20 {
		t.Fatalf("only %d refusals found in the handlers - the pattern stopped matching", len(found))
	}

	var loose []string
	for key, where := range found {
		if claimed[key] || errorsWithNoPage[key] {
			continue
		}
		loose = append(loose, key+" ("+strings.Join(uniq(where), ", ")+")")
	}
	sort.Strings(loose)
	if len(loose) > 0 {
		t.Errorf("%d refusals belong to no preview, so nothing can show them:\n  %s\n"+
			"Add each to previewErrors under the page its handler re-renders, or to "+
			"errorsWithNoPage when it is answered as plain text.",
			len(loose), strings.Join(loose, "\n  "))
	}

	// And the other way: a key claimed by a page that no handler gives is a
	// line left behind by a refusal that was removed.
	for key := range claimed {
		if _, ok := found[key]; ok {
			continue
		}
		if notSpelledAtTheCallSite[key] {
			continue
		}
		t.Errorf("previewErrors claims %q, which no handler gives any more", key)
	}
}

// notSpelledAtTheCallSite are refusals the scan above cannot see, each for a
// reason worth writing down rather than widening the pattern for: the passkey
// failure is printed by the page's own script, and the two access-window ones
// are chosen into a variable first (which of the pair, and the date that goes
// with it) before h.tr is asked.
var notSpelledAtTheCallSite = map[string]bool{
	"errPasskey":       true,
	"errAccessEnded":   true,
	"errAccessNotOpen": true,
}

func previewPageByKey(key string) (PreviewPage, bool) {
	for _, p := range PreviewPages {
		if p.Key == key {
			return p, true
		}
	}
	return PreviewPage{}, false
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
