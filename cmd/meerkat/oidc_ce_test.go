//go:build !ee

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/softwarity/meerkat/internal/auth"
	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/oidcprovider"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The community binary carries no OpenID provider: nothing registers one, and
// the data plane mounted the way run() mounts it answers none of its paths -
// they fall through to the routes, which have nothing there. Asserted on the
// binary, where the linking is decided.
func TestThisBinaryHasNoOpenIDProvider(t *testing.T) {
	if oidcprovider.Linked() {
		t.Fatal("an OpenID provider was linked into the community binary")
	}
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	sm := session.NewManager(st)
	router := gateway.New(st, sm)
	if err := router.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	auth.New(st, sm).Register(mux)
	prov := oidcprovider.New(oidcprovider.Deps{Store: st, Sessions: sm, Router: router})
	prov.MountData(mux)
	mux.Handle("/", prov.WrapData(router))
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, oidcprovider.DiscoveryPath},
		{http.MethodGet, oidcprovider.AuthorizePath},
		{http.MethodPost, oidcprovider.TokenPath},
		{http.MethodGet, oidcprovider.UserinfoPath},
		{http.MethodGet, oidcprovider.JWKSPath},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s answered %d in the community binary, want 404", c.method, c.path, rec.Code)
		}
	}
}
