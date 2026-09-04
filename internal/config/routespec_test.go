package config

import (
	"archive/zip"
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

const depositedSpec = `openapi: 3.0.0
info: {title: Orders, version: "1"}
paths: {/orders: {get: {}}}
`

func withDepositedSpec(t *testing.T, s *store.Store) store.Route {
	t.Helper()
	ctx := context.Background()
	route := store.Route{
		ID: "r-api", Name: "orders", Order: 1, Enabled: true, Upstream: "http://svc:8080",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/api/**"}}}},
		API: &store.RouteAPI{Spec: &store.RouteSpec{
			Type: store.SpecFile, Path: "openapi.json", Filename: "orders.yaml",
		}},
	}
	if err := s.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRouteSpec(ctx, route.ID, []byte(depositedSpec)); err != nil {
		t.Fatal(err)
	}
	return route
}

// A deposited spec travels like a picture: named by the document, carried by
// the package, and never inlined in a file people read and diff.
func TestTheDepositedSpecTravelsInThePackage(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	seed(t, s)
	withDepositedSpec(t, s)

	doc, _, err := Export(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Specs) != 1 {
		t.Fatalf("the export carries %d specs, want 1", len(doc.Specs))
	}
	plain, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "paths:") {
		t.Fatal("the spec's own content leaked into the plain YAML")
	}
	// What the YAML DOES say is enough to put it back: the source, the served
	// path, and the name the file takes in a package.
	for _, want := range []string{"type: file", "openapi.json", "orders.yaml"} {
		if !strings.Contains(string(plain), want) {
			t.Errorf("the document does not name %q:\n%s", want, plain)
		}
	}

	pkg, err := MarshalBundle(doc)
	if err != nil {
		t.Fatalf("MarshalBundle: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	// One directory per route: two routes may both deposit an "openapi.yaml",
	// and only the id cannot be edited between an export and its import.
	const want = "assets/specs/r-api/orders.yaml"
	if !slices.Contains(names, want) {
		t.Fatalf("package holds %v, want %s", names, want)
	}

	back, err := UnmarshalBundle(pkg)
	if err != nil {
		t.Fatalf("UnmarshalBundle: %v", err)
	}
	if string(back.Specs["r-api"]) != depositedSpec {
		t.Fatalf("the spec did not come back whole: %q", back.Specs["r-api"])
	}
}

// A document without its package names a file it does not carry. It is
// reported, and it destroys nothing: not the spec already in place, and above
// all not the per-endpoint policies, which are text and always travel.
func TestAnImportWithoutTheFileSaysSoAndKeepsWhatIsThere(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	seed(t, s)
	route := withDepositedSpec(t, s)

	doc, _, err := Export(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	doc.Specs = nil // what an unzipped YAML, or a plain export, amounts to

	plan, err := Preview(ctx, s, doc, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.MissingFiles) != 1 {
		t.Fatalf("missing files = %+v, want the deposited spec named", plan.MissingFiles)
	}
	if plan.MissingFiles[0].Route != route.Name || plan.MissingFiles[0].Name != "orders.yaml" {
		t.Errorf("missing file = %+v, want the route and the file named", plan.MissingFiles[0])
	}

	if _, err := Apply(ctx, s, doc, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	content, ok, err := s.RouteSpecContent(ctx, route.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || string(content) != depositedSpec {
		t.Fatal("an import that carried no file destroyed the one in place")
	}
}
