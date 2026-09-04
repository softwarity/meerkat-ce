package gateway

import (
	"context"
	"testing"

	filtering "github.com/softwarity/meerkat/internal/filters"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The setting is read at Reload, which is the one line that makes the screen
// mean anything: without it the number is saved, shown back, and ignored.
//
// Reload is also what the control plane announces to the other gateways, so
// applying it here is what carries the change across a cluster - no second
// mechanism.
func TestReloadAppliesTheStoredCeiling(t *testing.T) {
	t.Cleanup(func() { filtering.SetMaxRewritableBody(0) })

	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	rt := New(st, session.NewManager(st))

	// A fresh installation runs what the product always ran.
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if got := filtering.MaxRewritableBody(); got != filtering.DefaultMaxRewritableBody {
		t.Fatalf("fresh install: ceiling = %d, want the default %d", got, filtering.DefaultMaxRewritableBody)
	}

	if err := st.SetSetting(ctx, store.SettingProxyLimits, store.ProxyLimits{BodyRewriteMiB: 64}); err != nil {
		t.Fatal(err)
	}
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if got, want := filtering.MaxRewritableBody(), int64(64)<<20; got != want {
		t.Fatalf("after a save: ceiling = %d, want %d - the setting is stored and ignored", got, want)
	}
}
