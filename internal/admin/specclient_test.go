package admin

import (
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// TestASpecIsFetchedWithTheRoutesTrust: a service signed by an authority of
// our own served its pages through the route and refused its spec - "signed
// by unknown authority" (2026-10-16): the spec was fetched by a client that
// knew only the system's roots. It trusts what the router trusts now.
func TestASpecIsFetchedWithTheRoutesTrust(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/svc/api/v1/openapi" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"openapi":"3.0.0","info":{"title":"t","version":"1"},"paths":{"/api/v1/ping":{"get":{"responses":{"200":{"description":"ok"}}}}}}`)
	}))
	t.Cleanup(upstream.Close)
	route := store.Route{ID: "r", Name: "svc", Enabled: true, Upstream: upstream.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/svc/**"}}}},
		API:        &store.RouteAPI{Spec: &store.RouteSpec{Type: store.SpecUpstream, Path: "api/v1/openapi"}}}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	rt := gateway.New(st, session.NewManager(st))
	a := &API{st: st}

	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := a.readSpec(ctx, route); err == nil {
		t.Fatal("a spec behind an authority nobody trusts was read")
	}

	authority := store.TrustedAuthority{ID: "a1", Name: "test CA",
		PEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw}))}
	if err := st.SaveTrustedAuthority(ctx, &authority); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTrustedAuthority(ctx, "a1"); _ = rt.Reload(ctx) })
	spec, err := a.readSpec(ctx, route)
	if err != nil {
		t.Fatalf("the spec of a trusted upstream was refused: %v", err)
	}
	if spec == nil {
		t.Fatal("no spec")
	}
}
