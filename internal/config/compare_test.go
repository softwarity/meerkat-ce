package config

import (
	"testing"
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
