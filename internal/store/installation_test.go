package store

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// One identifier per installation: made on the first call, the same on every
// call after, and never part of an exported configuration.
func TestTheInstallationIDIsMadeOnceAndKept(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	a, err := st.InstallationID(ctx)
	if err != nil || len(a) != 8 {
		t.Fatalf("first: %q %v", a, err)
	}
	b, _ := st.InstallationID(ctx)
	if a != b {
		t.Fatalf("the identifier moved: %q then %q", a, b)
	}
}
