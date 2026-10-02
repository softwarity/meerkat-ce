package config

import (
	"context"
	"reflect"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/store"
)

// A route's endpoint audit (AUD-04) is part of its configuration: exported,
// imported elsewhere, the same operations are audited the same way.
func TestARoutesEndpointAuditTravelsWithTheConfiguration(t *testing.T) {
	ctx := context.Background()
	from := openTemp(t)
	audit := []store.EndpointAudit{{
		Method: "POST", Path: "/orders/{id}/refund", Description: "Refund",
		Fields: []store.AuditField{{Name: "order", From: "path", Key: "id"}, {Name: "amount", From: "body", Key: "/total/amount"}},
		Body:   true, Mask: []string{"card"},
	}}
	if err := from.SaveRoute(ctx, store.Route{
		ID: "orders", Name: "Orders", Order: 1, Enabled: true, Upstream: "http://orders:8080",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/orders/**"}}}},
		API:        &store.RouteAPI{Audit: audit},
	}); err != nil {
		t.Fatal(err)
	}
	doc, _, err := Export(ctx, from)
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
	to := openTemp(t)
	if _, err := Apply(ctx, to, back, false); err != nil {
		t.Fatal(err)
	}
	r, err := to.GetRoute(ctx, "orders")
	if err != nil {
		t.Fatal(err)
	}
	if r.API == nil || !reflect.DeepEqual(r.API.Audit, audit) {
		t.Fatalf("the audit did not travel:\n%s", file)
	}
}
