package openapi

import (
	"encoding/json"
	"strings"
	"testing"
)

// YAML is one of the two forms the specification admits, and the one people
// write by hand. It has to come out as JSON, because that is the only form
// Rewrite and InjectSimulation can edit - a spec that stayed YAML used to sail
// through them untouched, silently costing an upstream its retargeting.
func TestNormalizeTurnsYAMLIntoJSON(t *testing.T) {
	const src = `openapi: 3.0.0
info:
  title: Orders
  version: "1"
paths:
  /orders:
    get:
      responses:
        200:
          description: ok
        "404": {description: nope}
`
	out, err := Normalize([]byte(src))
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	// Response codes are written unquoted, so YAML reads them as INTEGERS.
	// JSON has no such keys, and the specification calls for strings: a
	// conversion that hands those keys over as they are fails outright.
	if !strings.Contains(string(out), `"200"`) {
		t.Errorf("the 200 lost its string form:\n%s", out)
	}
	// And it is a spec on the way out, not just any JSON.
	spec, err := Parse(out)
	if err != nil {
		t.Fatalf("the converted document no longer parses: %v", err)
	}
	if len(spec.Operations) != 1 || spec.Operations[0].Path != "/orders" {
		t.Fatalf("operations = %+v", spec.Operations)
	}
}

// A spec already in JSON is handed back as it stands: reformatting a document
// nobody asked to change is how a diff fills up with noise.
func TestNormalizeLeavesJSONAlone(t *testing.T) {
	const src = `{"openapi":"3.0.0","info":{"title":"t","version":"1"},"paths":{}}`
	out, err := Normalize([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != src {
		t.Fatalf("JSON was rewritten:\n%s", out)
	}
}

// A key YAML admits and JSON does not, pinned rather than left to the
// decoder's mood.
//
// This is the shape that broke a build and nothing else: the same version of
// the same YAML library returned map[string]any under one Go release and
// map[any]any under the one before it, so every test was green on a
// workstation a minor version ahead of the runner. What a customer's
// specification does must not depend on which toolchain compiled the gateway,
// so the conversion is the product's own now, and this says so.
func TestNonStringKeysBecomeStrings(t *testing.T) {
	const src = `responses:
  200: ok
  true: yes
  3.5: fine
nested:
  - 1: one
`
	out, err := Normalize([]byte(src))
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	responses, ok := doc["responses"].(map[string]any)
	if !ok {
		t.Fatalf("responses is %T", doc["responses"])
	}
	for _, key := range []string{"200", "true", "3.5"} {
		if _, ok := responses[key]; !ok {
			t.Errorf("no key %q in %v", key, responses)
		}
	}
	// Inside a list too: the conversion has to walk the whole document.
	list, ok := doc["nested"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("nested is %T", doc["nested"])
	}
	if _, ok := list[0].(map[string]any)["1"]; !ok {
		t.Errorf("the mapping inside the list kept a non-string key: %v", list[0])
	}
}
