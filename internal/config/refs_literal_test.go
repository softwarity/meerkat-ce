package config

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// A respond template writes Go range variables - $i, $r - and they are not
// vault references. Reading them as such reserved two empty entries at the
// first start, on a gateway nobody had asked anything of.
func TestRefsIgnoresLiteralArguments(t *testing.T) {
	doc := &Document{
		Version: Version,
		Routes: []store.Route{{
			ID:   "user",
			Name: "user",
			Filters: []routing.Spec{{
				Type: "respond",
				Args: map[string]any{
					"contentType": "application/json",
					"body": `{"name": {{json .Username}}, "authorities": ` +
						`[{{range $i, $r := .Roles}}{{if $i}},{{end}}{"authority": {{json $r}}}{{end}}]}`,
				},
			}},
		}},
	}
	refs, err := Refs(doc)
	if err != nil {
		t.Fatalf("Refs: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("a template's own variables were read as vault references: %v", refs)
	}

	// The rest of the same filter is still read: only the declared literal is
	// out of scope, not the whole spec.
	doc.Routes[0].Filters[0].Args["contentType"] = "$mediaType"
	refs, err = Refs(doc)
	if err != nil {
		t.Fatalf("Refs: %v", err)
	}
	if !slices.Contains(refs, "mediaType") {
		t.Errorf("a reference outside the literal argument was missed: %v", refs)
	}
	if _, err := json.Marshal(doc); err != nil {
		t.Fatal(err)
	}
}
