package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// TestATrustedAuthorityIsANamedPEM: a name of its own and a PEM - and a second
// entry under the same name is refused by name.
func TestATrustedAuthorityIsANamedPEM(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	for _, bad := range []TrustedAuthority{{ID: "x", Name: "", PEM: "p"}, {ID: "x", Name: "no pem"}} {
		if err := st.SaveTrustedAuthority(ctx, &bad); err == nil {
			t.Errorf("%+v was accepted", bad)
		}
	}
	a := TrustedAuthority{ID: "a", Name: " company CA ", PEM: "p"}
	if err := st.SaveTrustedAuthority(ctx, &a); err != nil || a.Name != "company CA" {
		t.Fatalf("saved: %v %q", err, a.Name)
	}
	twin := TrustedAuthority{ID: "b", Name: "company CA", PEM: "p"}
	if err := st.SaveTrustedAuthority(ctx, &twin); err == nil || !strings.Contains(err.Error(), "already named") {
		t.Fatalf("the same name twice: %v", err)
	}
	if list, err := st.ListTrustedAuthorities(ctx); err != nil || len(list) != 1 {
		t.Fatalf("listed: %d %v", len(list), err)
	}
	if err := st.DeleteTrustedAuthority(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteTrustedAuthority(ctx, "a"); err != ErrTrustedAuthorityNotFound {
		t.Fatalf("deleting it twice: %v", err)
	}
}
