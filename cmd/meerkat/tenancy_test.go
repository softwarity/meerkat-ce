package main

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// The flag SEEDS the mode on a first boot and steps aside afterwards: the
// console owns it, because a mode read per request can be switched and undone
// in a click. Nothing here refuses to start - a gateway that will not boot
// because a flag disagrees with its database turns a configuration mistake
// into an outage.
func TestSettleTenancy(t *testing.T) {
	ctx := context.Background()

	t.Run("single is the default and needs no license", func(t *testing.T) {
		st := openStore(t)
		if err := settleTenancy(ctx, st, store.TenancySingle); err != nil {
			t.Fatal(err)
		}
		if mode := st.Tenancy(ctx); mode != store.TenancySingle {
			t.Fatalf("recorded %q, want single", mode)
		}
	})

	// A gateway must START. Asking the community image for multi is a
	// configuration mistake, and turning it into an outage at boot - at the
	// worst possible hour - would be the wrong answer.
	t.Run("multi on the community image starts single instead of refusing", func(t *testing.T) {
		if edition.Enterprise {
			t.Skip("this image sells multi: the case above covers it")
		}
		st := openStore(t)
		if err := settleTenancy(ctx, st, store.TenancyMulti); err != nil {
			t.Fatalf("a gateway must start: %v", err)
		}
		if mode := st.Tenancy(ctx); mode != store.TenancySingle {
			t.Fatalf("recorded %q, want single", mode)
		}
	})

	// The mode a build can seed IS its edition: multi belongs to the Enterprise
	// image, and the community one starts single whatever the flag says rather
	// than refusing to boot over it.
	t.Run("multi is seeded by the image that sells it", func(t *testing.T) {
		st := openStore(t)
		if err := settleTenancy(ctx, st, store.TenancyMulti); err != nil {
			t.Fatal(err)
		}
		want := store.TenancySingle
		if edition.Enterprise {
			want = store.TenancyMulti
		}
		if mode := st.Tenancy(ctx); mode != want {
			t.Fatalf("recorded %q, want %q", mode, want)
		}
	})

	t.Run("once chosen, the flag no longer decides", func(t *testing.T) {
		st := openStore(t)
		// The console had the last word...
		if err := st.SetTenancy(ctx, store.TenancyMulti); err != nil {
			t.Fatal(err)
		}
		// ...and a compose file still says single. The database wins.
		if err := settleTenancy(ctx, st, store.TenancySingle); err != nil {
			t.Fatal(err)
		}
		if mode := st.Tenancy(ctx); mode != store.TenancyMulti {
			t.Fatalf("recorded %q: the flag must not override a chosen mode", mode)
		}
	})

	// The first boot IS a choice. Without that, this branch runs again on the
	// next start and a compose file with no -tenancy (so: single, the default)
	// quietly takes a multi-organisation gateway back to one - its
	// organisations still in the database, none of them named anywhere.
	t.Run("a restart without the flag keeps the mode", func(t *testing.T) {
		if !edition.Enterprise {
			t.Skip("the community image never seeds multi, so there is no mode to keep")
		}
		st := openStore(t)
		if err := settleTenancy(ctx, st, store.TenancyMulti); err != nil {
			t.Fatal(err)
		}
		// Second boot, nothing on the command line: the flag defaults to single.
		if err := settleTenancy(ctx, st, store.TenancySingle); err != nil {
			t.Fatal(err)
		}
		if mode := st.Tenancy(ctx); mode != store.TenancyMulti {
			t.Fatalf("recorded %q: a restart must not undo the mode", mode)
		}
	})

	t.Run("an unknown mode is refused rather than guessed", func(t *testing.T) {
		st := openStore(t)
		if err := settleTenancy(ctx, st, "mono"); err == nil {
			t.Fatal("a typo must not silently fall back to a mode")
		}
	})
}
