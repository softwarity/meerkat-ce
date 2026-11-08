package config

import (
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// Two documents compared object by object, what runs left out (CFG-04): an
// added route, a removed role, a changed theme with the field that moved, a
// changed setting, and what is the same stays quiet.
func TestCompareSaysWhatDiffersBetweenTwoDocuments(t *testing.T) {
	from, err := Unmarshal([]byte(`version: 1
roles:
  - {name: ops}
  - {name: sales}
routes:
  - {id: api, name: api, enabled: true, upstream: "http://a.invalid", predicates: [{type: path, args: {patterns: ["/api/**"]}}]}
settings:
  sessionTtl: PT30M
`))
	if err != nil {
		t.Fatal(err)
	}
	to, err := Unmarshal([]byte(`version: 1
roles:
  - {name: ops}
routes:
  - {id: api, name: api, enabled: true, upstream: "http://b.invalid", predicates: [{type: path, args: {patterns: ["/api/**"]}}]}
  - {id: web, name: web, enabled: true, upstream: "http://w.invalid", predicates: [{type: path, args: {patterns: ["/web/**"]}}]}
settings:
  sessionTtl: PT1H
`))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Change{}
	for _, c := range Compare(from, to) {
		got[c.Kind+" "+c.ID] = c
	}
	want := map[string]string{
		"role ops": ActionSame, "role sales": ActionRemove,
		"route api": ActionUpdate, "route web": ActionAdd,
		"setting sessionTtl": ActionUpdate,
	}
	for k, action := range want {
		if got[k].Action != action {
			t.Errorf("%s: %q, want %q", k, got[k].Action, action)
		}
	}
	if f := got["route api"].Fields; len(f) != 1 || f[0] != "upstream" {
		t.Errorf("the route's moved field: %v", f)
	}
}

// A route's uploaded files and its spec travel beside it, not in its JSON:
// two documents whose routes differ only by the content of a stylesheet must
// still compare as different, and say what moved.
func TestCompareReadsTheFilesBesideARoute(t *testing.T) {
	doc := func() *Document {
		d, err := Unmarshal([]byte(`version: 1
routes:
  - {id: web, name: web, enabled: true, upstream: "http://w.invalid", predicates: [{type: path, args: {patterns: ["/web/**"]}}]}
`))
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	from, to := doc(), doc()
	from.Files = map[string][]store.RouteFile{"web": {{Name: "theme.css", Data: []byte("a{color:red}")}}}
	to.Files = map[string][]store.RouteFile{"web": {{Name: "theme.css", Data: []byte("a{color:blue}")}}}
	to.Specs = map[string][]byte{"web": []byte("openapi: 3.0.0")}
	changes := Compare(from, to)
	if len(changes) != 1 || changes[0].Action != ActionUpdate || strings.Join(changes[0].Fields, ",") != "files,spec" {
		t.Fatalf("a rewritten file and a new spec make the route updated, got %+v", changes)
	}
	to.Files["web"][0].Data = []byte("a{color:red}")
	to.Specs = nil
	if c := Compare(from, to); c[0].Action != ActionSame {
		t.Errorf("the same files compare the same, got %+v", c)
	}
}
