package auth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A correction has to reach EVERY rendered string, and for a while it did not.
// Five places indexed the embedded map directly - the user button, the portal
// bar, the maintenance page, its stripe, the API tokens page - so an integrator
// could rewrite a label in the console and watch the old one keep coming back.
// The override layer only exists behind catalogue().
//
// This refuses the pattern rather than the symptom: a new page written the same
// way would be silently untranslatable again, and nothing would say so.
func TestNoPageReadsThroughTheOverrideLayer(t *testing.T) {
	// Where indexing the embedded map IS the point: the layer itself, the
	// English reference, and the probe that builds the key map.
	allowed := map[string]bool{
		"localeoverride.go": true,
		"i18n.go":           true,
		"tokenmap.go":       true,
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || allowed[name] {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			idx, ok := n.(*ast.IndexExpr)
			if !ok {
				return true
			}
			if id, ok := idx.X.(*ast.Ident); ok && id.Name == "messages" {
				t.Errorf("%s indexes the embedded catalogue directly - "+
					"corrections made in the console would not reach it. Use catalogue(lang).",
					fset.Position(idx.Pos()))
			}
			return true
		})
	}
}

// And the same thing proved on the wire: a wording corrected in the console
// comes out of the user button's payload. The AST check above says nothing
// indexes the map any more; this says the layer is actually consulted.
func TestACorrectionReachesTheUserButton(t *testing.T) {
	mux, _, _ := setupFlow(t)
	t.Cleanup(func() { SetLocaleOverrides(nil) })

	before := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, nil))
	if !strings.Contains(before, messages["en"]["signIn"]) {
		t.Fatalf("the payload does not carry the shipped wording: %s", before)
	}
	SetLocaleOverrides(map[string]map[string]string{"en": {"signIn": "Step inside"}})
	after := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, nil))
	if !strings.Contains(after, "Step inside") {
		t.Errorf("the correction never reached the user button: %s", after)
	}
}
