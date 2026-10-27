package config

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// consoleRoutes are routes the way the console's form saves them: the whole ui
// shape, every block present, what is off still carrying its settings. The
// export trims all of that (trimRoute); the store keeps it. Each one is a
// shape seen on a real gateway, where a capture pushed to git and pulled back
// reported nine such routes as "update".
func consoleRoutes() []store.Route {
	path := func(p string) []routing.Spec {
		return []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{p}}}}
	}
	off := store.UserButton{Enabled: false, Height: 24, Position: "top-right", Shape: "round", PadX: 12, PadY: 12}
	return []store.Route{{
		// A page with its button off, the roles and user blocks off but
		// still holding the mechanism and tag the form defaults to.
		ID: "plug", Name: "plug", Order: 1, Enabled: true, IsUI: true, Upstream: "http://plug.invalid",
		Predicates: path("/plug/**"), Filters: []routing.Spec{{Type: "strip-prefix"}},
		UI: &store.RouteUI{
			Scheme:     &store.SchemeConfig{Mechanism: "none", Button: "dark"},
			Roles:      &store.RolesConfig{Mechanism: "class", Tag: "body"},
			UserInfo:   &store.UserInfoConfig{Mechanism: "attribute", Tag: "body", Fields: map[string]string{}},
			UserButton: off,
		},
	}, {
		// A page where nothing of the ui block is on: the export carries no
		// ui block at all.
		ID: "hubble", Name: "Hubble", Order: 2, Enabled: true, IsUI: true, Upstream: "http://hubble.invalid",
		Predicates: path("/hubble/**"),
		UI: &store.RouteUI{
			Scheme:     &store.SchemeConfig{},
			Roles:      &store.RolesConfig{Mechanism: "class", Tag: "body"},
			UserInfo:   &store.UserInfoConfig{Mechanism: "attribute", Tag: "body"},
			UserButton: off,
		},
	}, {
		// The button on, at its defaults; identity mapped under the field's
		// own name; an api block that says nothing.
		ID: "trap", Name: "trap", Order: 3, Enabled: true, IsUI: true, Upstream: "http://trap.invalid",
		Predicates: path("/**"),
		UI: &store.RouteUI{
			UserButton: store.UserButton{Enabled: true, Height: 24, Position: "top-right", Shape: "round", PadX: 12, PadY: 12},
			Roles:      &store.RolesConfig{Mechanism: "class", Tag: "body"},
		},
		Identity: &store.IdentityForward{Mechanism: "headers", Attributes: []store.IdentityAttr{
			{Field: "username", As: "username"}, {Field: "email", As: "X-Mail"},
		}},
		API: &store.RouteAPI{Spec: &store.RouteSpec{}},
	}}
}

func saveRoutes(t *testing.T, st *store.Store, routes []store.Route) {
	t.Helper()
	for _, r := range routes {
		if err := st.SaveRoute(context.Background(), r); err != nil {
			t.Fatalf("SaveRoute %s: %v", r.Name, err)
		}
	}
}

// roundTrip is what a capture pushed to git and pulled back is: the export,
// written and read again.
func roundTrip(t *testing.T, st *store.Store) *Document {
	t.Helper()
	doc, _, err := Export(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	file, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Unmarshal(file)
	if err != nil {
		t.Fatal(err)
	}
	return back
}

func actions(plan *Plan, kind string) map[string]string {
	out := map[string]string{}
	for _, c := range plan.Changes {
		if c.Kind == kind {
			out[c.Label] = c.Action
		}
	}
	return out
}

// Capture, then plan the same configuration: nothing moves. The store keeps
// what the console's form materialised, the document only what the export
// kept, and the plan compared the two raw - so every route saved from the
// console read "update" on every pull and every activation, and the change
// someone actually made drowned among them.
func TestPlanOfOwnCaptureIsAllSame(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)
	saveRoutes(t, st, consoleRoutes())

	doc := roundTrip(t, st)
	for _, preview := range []func() (*Plan, error){
		func() (*Plan, error) { return PreviewSwitch(ctx, st, doc) },
		func() (*Plan, error) { return Preview(ctx, st, doc, true) },
	} {
		plan, err := preview()
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range plan.Changes {
			if c.Action != ActionSame {
				t.Errorf("%s %s: %q for a document captured from this very state", c.Kind, c.Label, c.Action)
			}
		}
	}

	// A line kept out (CFG-02) is the export's version of it: same again.
	kept, err := Keep(ctx, st, doc, []Ref{{Kind: "route", ID: "plug"}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PreviewSwitch(ctx, st, kept)
	if err != nil {
		t.Fatal(err)
	}
	if got := actions(plan, "route")["plug"]; got != ActionSame {
		t.Errorf("a kept route reads %q, want same", got)
	}
}

// The trim on both sides must not hide a real change: what a person changed
// in the document still shows, whatever the stored row carries besides.
func TestPlanStillSeesARealRouteChange(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)
	saveRoutes(t, st, consoleRoutes())

	doc := roundTrip(t, st)
	for i := range doc.Routes {
		switch doc.Routes[i].ID {
		case "plug":
			doc.Routes[i].UI.UserButton.Enabled = true // the button comes on
		case "hubble":
			doc.Routes[i].UI = &store.RouteUI{Roles: &store.RolesConfig{Enabled: true}} // roles go out
		case "trap":
			doc.Routes[i].Identity.Attributes[1].As = "X-Email" // a mapping renamed
		}
	}
	plan, err := PreviewSwitch(ctx, st, doc)
	if err != nil {
		t.Fatal(err)
	}
	for name, action := range actions(plan, "route") {
		if action != ActionUpdate {
			t.Errorf("route %s: %q, want update", name, action)
		}
	}
}

// A role map is a decision even on a route with no spec of its own: the
// export dropped the api block that held it, so it never travelled - and
// with the comparison trimming like the export, a document bringing one would
// have read "same" and never been applied.
func TestRoleMapWithoutSpecTravels(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)
	r := consoleRoutes()[0]
	r.API = &store.RouteAPI{RoleMap: map[string]string{"ROLE_ADMIN": "ops"}}
	saveRoutes(t, st, []store.Route{r})

	doc := roundTrip(t, st)
	if got := doc.Routes[0].API; got == nil || got.RoleMap["ROLE_ADMIN"] != "ops" {
		t.Fatalf("the role map did not travel: %+v", got)
	}
	doc.Routes[0].API.RoleMap["ROLE_ADMIN"] = "admins"
	plan, err := PreviewSwitch(ctx, st, doc)
	if err != nil {
		t.Fatal(err)
	}
	if got := actions(plan, "route")["plug"]; got != ActionUpdate {
		t.Errorf("a changed role map reads %q, want update", got)
	}
}

// Comparing two saved documents (CFG-04) reads them as the plan does: a file
// edited by hand that spells out the form's defaults is the same file.
func TestCompareReadsDefaultsAsThePlanDoes(t *testing.T) {
	st := openTemp(t)
	saveRoutes(t, st, consoleRoutes())
	exported := roundTrip(t, st)

	// The same routes, spelled the way the store holds them.
	raw, err := json.Marshal(struct {
		Version int           `json:"version"`
		Routes  []store.Route `json:"routes"`
	}{Version, consoleRoutes()})
	if err != nil {
		t.Fatal(err)
	}
	spelled, err := Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Routes only: the export carries the rest of the installation too.
	exported = &Document{Version: Version, Routes: exported.Routes}
	for _, c := range Compare(exported, spelled) {
		if c.Action != ActionSame {
			t.Errorf("%s %s: %q %v, want same", c.Kind, c.Label, c.Action, c.Fields)
		}
	}
}

// What "this saved configuration is what runs" means (CFG-02, CFG-06): the
// text the same, pictures out; and a picture the saved one carries, the one
// shown now. A document without pictures keeps those in place when applied,
// so their absence is no difference.
func TestMatchesJudgesPicturesApart(t *testing.T) {
	logo := `"data:image/png;base64,iVBORw0KGgo="`
	other := `"data:image/png;base64,AAAAAAAAAAA="`
	doc := func(branding string) *Document {
		d := &Document{Version: Version, Routes: consoleRoutes()[:1],
			Settings: map[string]json.RawMessage{"branding": json.RawMessage(branding)}}
		return d
	}
	running := doc(`{"title":"Acme","logo":` + logo + `}`)

	cases := []struct {
		name  string
		saved *Document
		want  bool
	}{
		{"the same, pictures in", doc(`{"title":"Acme","logo":` + logo + `}`), true},
		{"the same, pictures out", doc(`{"title":"Acme","logo":""}`), true},
		{"another picture", doc(`{"title":"Acme","logo":` + other + `}`), false},
		{"another title", doc(`{"title":"Other","logo":` + logo + `}`), false},
	}
	for _, c := range cases {
		got, err := Matches(c.saved, running)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}

	// The fingerprint of a capture is the fingerprint of its own text, which
	// is what a restore point stores and is keyed by.
	text, err := Text(running)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(text), "base64") {
		t.Fatalf("the text kept a picture:\n%s", text)
	}
	back, err := Unmarshal(text)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := Fingerprint(running)
	b, _ := Fingerprint(back)
	if a != b {
		t.Fatal("a document and its own text fingerprint differently")
	}
}

// A group added by hand in a document - no hours mentioned, its roles in the
// order someone typed them, listed before one with a smaller id - is saved as
// inheriting the hours, its roles a set. Once applied, the same document
// plans as "same" and matches what runs: it read "update" on every pull after
// the first, and the gateway it had just been activated on as modified since.
func TestHandWrittenGroupComparesAsSaved(t *testing.T) {
	ctx := context.Background()
	st := openTemp(t)
	for _, r := range []string{"viewer", "ROLE_ADMIN", "editor"} {
		if err := st.SaveRole(ctx, store.Role{Name: r}); err != nil {
			t.Fatal(err)
		}
	}
	doc, err := Unmarshal([]byte(`version: 1
groups:
  - {id: zz, name: night, tenantId: default, roles: [viewer, ROLE_ADMIN, editor]}
  - {id: aa, name: day, tenantId: default, roles: [editor]}
`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, st, doc, false); err != nil {
		t.Fatal(err)
	}
	plan, err := Preview(ctx, st, doc, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range plan.Changes {
		if c.Action != ActionSame {
			t.Errorf("%s %s: %q once applied, want same", c.Kind, c.Label, c.Action)
		}
	}
	running, _, err := Export(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	// The document's groups against what runs: the rest of the gateway is not
	// in this fragment.
	groups := slices.DeleteFunc(running.Groups, func(g store.Group) bool { return g.ID != "zz" && g.ID != "aa" })
	if ok, err := Matches(doc, &Document{Version: Version, Groups: groups}); err != nil || !ok {
		t.Errorf("the document it was applied from does not match what runs (%v)", err)
	}
}
