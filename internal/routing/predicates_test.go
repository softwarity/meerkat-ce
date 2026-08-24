package routing

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The SCG time predicates: after/before/between evaluate against now; bad
// datetimes and inverted bounds fail at compile time.
func TestTimeWindow(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	past, future := "2000-01-01T00:00:00Z", "2100-01-01T00:00:00Z"

	match := func(args map[string]any) bool {
		t.Helper()
		cp, err := CompilePredicates([]Spec{{Type: "time-window", Args: args}})
		if err != nil {
			t.Fatalf("compile %v: %v", args, err)
		}
		return cp.Match(req)
	}
	// One end open on either side, which is what after and before used to be.
	if !match(map[string]any{"from": past}) {
		t.Fatal("a window opened in the past must match now")
	}
	if match(map[string]any{"from": future}) {
		t.Fatal("a window opening in the future must not match now")
	}
	if !match(map[string]any{"to": future}) {
		t.Fatal("a window closing in the future must match now")
	}
	if match(map[string]any{"to": past}) {
		t.Fatal("a window closed in the past must match nothing")
	}
	// Both ends, which is what between used to be.
	if !match(map[string]any{"from": past, "to": future}) {
		t.Fatal("now is inside past..future")
	}
	if match(map[string]any{"from": "2000-01-01T00:00:00Z", "to": "2000-06-01T00:00:00Z"}) {
		t.Fatal("now is not inside a window that closed in 2000")
	}

	// Refused at save rather than never matching at request time.
	bad := []map[string]any{
		{},                           // a window open at both ends is no window
		{"from": "tomorrow"},         // not a datetime
		{"to": past, "from": future}, // closes before it opens
		{"from": past, "to": past},   // accepts nothing
	}
	for _, args := range bad {
		if _, err := CompilePredicates([]Spec{{Type: "time-window", Args: args}}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestXForwardedRemoteAddr(t *testing.T) {
	cp, err := CompilePredicates([]Spec{{Type: "x-forwarded-remote-addr",
		Args: map[string]any{"cidrs": []any{"10.0.0.0/8"}}}})
	if err != nil {
		t.Fatal(err)
	}
	mk := func(xff string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	if !cp.Match(mk("203.0.113.9, 10.1.2.3")) {
		t.Fatal("rightmost 10.x must match")
	}
	if cp.Match(mk("10.1.2.3, 203.0.113.9")) {
		t.Fatal("rightmost public address must not match")
	}
	if cp.Match(mk("")) {
		t.Fatal("no X-Forwarded-For must not match")
	}
}

// The reason this predicate exists at all: to a regexp, 1.10 reads as smaller
// than 1.9, because a regexp compares characters. Here they are numbers.
func TestVersionComparesNumbersNotCharacters(t *testing.T) {
	cp, err := CompilePredicates([]Spec{{Type: "version", Args: map[string]any{
		"from": "1.9", "pattern": `v?(\d+(?:\.\d+)*)`}}})
	if err != nil {
		t.Fatal(err)
	}
	p := cp.Match
	req := func(v string) *http.Request {
		r := httptest.NewRequest("GET", "http://gw.example/x", nil)
		if v != "" {
			r.Header.Set("X-API-Version", v)
		}
		return r
	}
	for _, v := range []string{"1.9", "1.10", "2", "v2.1", "1.9.1"} {
		if !p(req(v)) {
			t.Fatalf("%q was refused by from=1.9", v)
		}
	}
	for _, v := range []string{"1.8", "1.8.9", "0.1"} {
		if p(req(v)) {
			t.Fatalf("%q passed from=1.9", v)
		}
	}
	// No version, or nonsense: no match. The route without a version predicate
	// underneath is what answers those.
	for _, v := range []string{"", "next"} {
		if p(req(v)) {
			t.Fatalf("%q passed as if it were a version", v)
		}
	}
	// A pre-release DOES match, because the pattern extracts 2.1 out of it -
	// and the pattern is written on the brick, so someone who wants 2.1-beta
	// turned away anchors it. This is the difference an extraction makes, and
	// it is visible rather than decided in the engine.
	if !p(req("2.1-beta")) {
		t.Fatal("2.1-beta did not read as 2.1, which is what the pattern extracts")
	}
}

func TestVersionRangeIsHalfOpen(t *testing.T) {
	cp, err := CompilePredicates([]Spec{{Type: "version",
		Args: map[string]any{"from": "2.0", "to": "3.0", "source": "query", "name": "v",
			"pattern": `v?(\d+(?:\.\d+)*)`}}})
	if err != nil {
		t.Fatal(err)
	}
	p := cp.Match
	req := func(v string) *http.Request {
		return httptest.NewRequest("GET", "http://gw.example/x?v="+v, nil)
	}
	for _, v := range []string{"2.0", "2.9.9", "2.99"} {
		if !p(req(v)) {
			t.Fatalf("%q was refused by [2.0, 3.0)", v)
		}
	}
	// to is EXCLUSIVE: 3.0 belongs to the next route, which is the point of
	// naming it as the first version refused.
	for _, v := range []string{"3.0", "3.1", "1.9"} {
		if p(req(v)) {
			t.Fatalf("%q passed [2.0, 3.0)", v)
		}
	}
}

func TestVersionRefusesWhatCannotMatch(t *testing.T) {
	p := `v?(\d+(?:\.\d+)*)`
	bad := []Spec{
		{Type: "version", Args: map[string]any{}},
		// The pattern is written out even when it is the obvious one, so a
		// route cannot read an order id as a version without anyone deciding.
		{Type: "version", Args: map[string]any{"from": "2.0"}},
		{Type: "version", Args: map[string]any{"from": "2.0", "to": "2.0", "pattern": p}},
		{Type: "version", Args: map[string]any{"from": "3.0", "to": "2.0", "pattern": p}},
		{Type: "version", Args: map[string]any{"from": "deux", "pattern": p}},
		{Type: "version", Args: map[string]any{"from": "1.2.3.4", "pattern": p}},
		{Type: "version", Args: map[string]any{"from": "2.0", "source": "body", "pattern": p}},
	}
	for _, s := range bad {
		if _, err := CompilePredicates([]Spec{s}); err == nil {
			t.Fatalf("accepted %v", s.Args)
		}
	}
}

// The pattern is what makes a version in the PATH usable at all, and a version
// buried in a media type on top of it.
func TestVersionReadsThroughAPattern(t *testing.T) {
	inPath := func(from, pattern string) Predicate {
		cp, err := CompilePredicates([]Spec{{Type: "version", Args: map[string]any{
			"source": "path", "pattern": pattern, "from": from}}})
		if err != nil {
			t.Fatal(err)
		}
		return cp.Match
	}
	p := inPath("2.0", `/v(?<version>[0-9.]+)/`)
	get := func(path string) *http.Request { return httptest.NewRequest("GET", "http://gw.example"+path, nil) }

	if !p(get("/api/v2.1/orders")) {
		t.Fatal("/api/v2.1/orders was refused by from=2.0")
	}
	if p(get("/api/v1.2/orders")) {
		t.Fatal("/api/v1.2/orders passed from=2.0")
	}
	// Nothing to extract: no match, and the route without a version predicate
	// underneath is what answers.
	if p(get("/api/orders")) {
		t.Fatal("a path carrying no version matched")
	}

	// The same on a header whose value carries more than the version.
	cp, err := CompilePredicates([]Spec{{Type: "version", Args: map[string]any{
		"name": "Accept", "pattern": `vnd\.acme\.v(?<version>[0-9.]+)`, "from": "2.0"}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "http://gw.example/x", nil)
	req.Header.Set("Accept", "application/vnd.acme.v2+json")
	if !cp.Match(req) {
		t.Fatal("a version inside a media type was not extracted")
	}

	// No named group: the first capturing group is the version.
	cp2, err := CompilePredicates([]Spec{{Type: "version", Args: map[string]any{
		"source": "path", "pattern": `/v([0-9.]+)/`, "from": "2.0"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !cp2.Match(get("/api/v2.5/orders")) {
		t.Fatal("the first capturing group was not read as the version")
	}
}

func TestVersionPatternRefusals(t *testing.T) {
	// Reading the path without saying where the version is would compile into a
	// predicate that never matches - refused at save instead.
	if _, err := CompilePredicates([]Spec{{Type: "version",
		Args: map[string]any{"source": "path", "from": "2.0"}}}); err == nil {
		t.Fatal("a version predicate without a pattern was accepted")
	}
	if _, err := CompilePredicates([]Spec{{Type: "version",
		Args: map[string]any{"pattern": "v(", "from": "2.0"}}}); err == nil {
		t.Fatal("a broken pattern was accepted")
	}
}

// The preview the console shows while someone types runs the engine itself.
func TestPreviewVersionIsTheEngine(t *testing.T) {
	args := map[string]any{"source": "path", "pattern": `/v(?<version>[0-9.]+)/`, "from": "2.0", "to": "3.0"}

	got, ok, err := PreviewVersion(args, "/api/v2.1/orders")
	if err != nil || got != "2.1" || !ok {
		t.Fatalf("extracted %q match=%v err=%v", got, ok, err)
	}
	// Extracted but outside the range: a preview has to tell that apart from
	// having found nothing, so the number comes back with a false.
	if got, ok, _ = PreviewVersion(args, "/api/v1.2/orders"); got != "1.2" || ok {
		t.Fatalf("extracted %q match=%v, want 1.2 and no match", got, ok)
	}
	if got, ok, _ = PreviewVersion(args, "/api/orders"); got != "" || ok {
		t.Fatalf("extracted %q match=%v, want nothing", got, ok)
	}
	// A pattern RE2 refuses must fail here exactly as it fails at save.
	if _, _, err = PreviewVersion(map[string]any{"pattern": "(?=x)", "from": "2.0"}, "x"); err == nil {
		t.Fatal("a lookahead was accepted, so the preview would promise what the gateway refuses")
	}
	// The preview names the brick the way the route editor does, since its
	// message is shown verbatim next to the field.
	_, _, err = PreviewVersion(map[string]any{"from": "2.0"}, "2.1")
	if err == nil || !strings.Contains(err.Error(), "predicate version") {
		t.Fatalf("a missing pattern reported %v", err)
	}
}

// A list of values is the everyday case a regexp was standing in for, and it
// cannot be got wrong: prod does not match preprod, which an unanchored
// pattern would have.
func TestMatcherValues(t *testing.T) {
	match := func(spec Spec, header string) bool {
		t.Helper()
		cp, err := CompilePredicates([]Spec{spec})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		req := httptest.NewRequest("GET", "http://gw.example/x", nil)
		if header != "" {
			req.Header.Set("X-Env", header)
		}
		return cp.Match(req)
	}
	list := Spec{Type: "header", Args: map[string]any{"name": "X-Env", "values": []string{"prod", "staging"}}}
	for _, v := range []string{"prod", "staging"} {
		if !match(list, v) {
			t.Fatalf("%q was refused by the list", v)
		}
	}
	for _, v := range []string{"preprod", "PROD", "dev", ""} {
		if match(list, v) {
			t.Fatalf("%q passed a list of prod and staging", v)
		}
	}
	// Presence alone still works, and so does the regexp.
	if !match(Spec{Type: "header", Args: map[string]any{"name": "X-Env"}}, "anything") {
		t.Fatal("presence-only stopped matching")
	}
	if !match(Spec{Type: "header", Args: map[string]any{"name": "X-Env", "regexp": "pro."}}, "prod") {
		t.Fatal("the regexp form stopped matching")
	}
	// Both at once is refused: two answers to the same question.
	if _, err := CompilePredicates([]Spec{{Type: "header", Args: map[string]any{
		"name": "X-Env", "values": []string{"prod"}, "regexp": "pro."}}}); err == nil {
		t.Fatal("a list and a regexp together were accepted")
	}
}
