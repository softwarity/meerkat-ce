package gateway

import (
	"context"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// TestAnUpstreamSignedByAnAuthorityOfOurOwn: an upstream in HTTPS whose
// certificate no system authority signed is a 502 until its authority is
// trusted, and again once it is removed.
func TestAnUpstreamSignedByAnAuthorityOfOurOwn(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close(); setUpstreamRoots(nil) })
	ctx := context.Background()
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "over https")
	}))
	t.Cleanup(upstream.Close)
	authority := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw}))

	if err := st.SaveRoute(ctx, store.Route{ID: "r", Name: "orders", Enabled: true, Upstream: upstream.URL,
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/**"}}}}}); err != nil {
		t.Fatal(err)
	}
	rt := New(st, session.NewManager(st))
	call := func() int {
		t.Helper()
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		return rec.Code
	}
	reload := func() {
		t.Helper()
		if err := rt.Reload(ctx); err != nil {
			t.Fatal(err)
		}
	}
	reload()
	if code := call(); code != http.StatusBadGateway {
		t.Fatalf("an unknown authority: %d, want 502", code)
	}

	// Pasted.
	pasted := store.TrustedAuthority{ID: "a1", Name: "test CA", PEM: authority}
	if err := st.SaveTrustedAuthority(ctx, &pasted); err != nil {
		t.Fatal(err)
	}
	reload()
	if code := call(); code != http.StatusOK {
		t.Fatalf("its authority trusted: %d, want 200", code)
	}
	if err := st.DeleteTrustedAuthority(ctx, "a1"); err != nil {
		t.Fatal(err)
	}
	reload()
	if code := call(); code != http.StatusBadGateway {
		t.Fatalf("its authority removed: %d, want 502", code)
	}
}
