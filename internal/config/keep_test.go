package config

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// A configuration that removes one route, changes another and adds a third,
// switched to with all three lines kept out: the gateway ends as it was. And
// the same configuration switched to whole does all three - the keep is what
// made the difference, not a broken switch.
func TestALineKeptOutLeavesTheGatewayAsItIs(t *testing.T) {
	ctx := context.Background()
	route := func(id, upstream string) store.Route {
		return store.Route{ID: id, Name: id, Order: 1, Enabled: true, Upstream: upstream,
			Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/" + id + "/**"}}}}}
	}
	prepare := func() *store.Store {
		s := openTemp(t)
		for _, r := range []store.Route{route("api", "http://api:8080"), route("old", "http://old:8080")} {
			if err := s.SaveRoute(ctx, r); err != nil {
				t.Fatal(err)
			}
		}
		return s
	}
	doc := &Document{Version: Version, Routes: []store.Route{route("api", "http://api-v2:8080"), route("new", "http://new:8080")}}
	upstreams := func(s *store.Store) map[string]string {
		t.Helper()
		rs, err := s.ListRoutes(ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, r := range rs {
			out[r.ID] = r.Upstream
		}
		return out
	}

	whole := prepare()
	if _, err := Switch(ctx, whole, doc); err != nil {
		t.Fatal(err)
	}
	if got := upstreams(whole); len(got) != 2 || got["api"] != "http://api-v2:8080" || got["new"] == "" || got["old"] != "" {
		t.Fatalf("the whole switch: %v", got)
	}

	s := prepare()
	keep, err := ParseRefs([]string{"route:old", "route:api", "route:new"})
	if err != nil {
		t.Fatal(err)
	}
	kept, err := Keep(ctx, s, doc, keep)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Switch(ctx, s, kept)
	if err != nil {
		t.Fatal(err)
	}
	// The routes only: this test document declares no authority, so a switch
	// - which is the whole configuration - removes the local one as it should.
	for _, c := range plan.Changes {
		if c.Kind == "route" && c.Action != ActionSame {
			t.Fatalf("every route line kept out, and the plan still changes one: %+v", c)
		}
	}
	if got := upstreams(s); len(got) != 2 || got["api"] != "http://api:8080" || got["old"] != "http://old:8080" {
		t.Fatalf("kept out, and the gateway moved: %v", got)
	}
	if len(doc.Routes) != 2 || doc.Routes[0].Upstream != "http://api-v2:8080" {
		t.Fatal("the caller's document was changed")
	}
}

// What is not a line refuses with what is.
func TestKeepNamesWhatItTakes(t *testing.T) {
	for _, bad := range []string{"route", "user:alice", "route.spec:api", ""} {
		if _, err := ParseRefs([]string{bad}); err == nil {
			t.Errorf("%q was taken", bad)
		}
	}
	if refs, err := ParseRefs([]string{"mailRelay", "setting:branding", "role:Ops read"}); err != nil || len(refs) != 3 || refs[2].ID != "Ops read" {
		t.Fatalf("%v %v", refs, err)
	}
}
