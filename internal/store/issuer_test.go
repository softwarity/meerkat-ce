package store

import (
	"context"
	"testing"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// TestTheIssuerIsMeerkatUntilSet: the default is what every token said
// before, a valid one is kept as given, and a stored value this build would
// refuse reads as the default rather than as an empty issuer.
func TestTheIssuerIsMeerkatUntilSet(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	if got := st.JWTIssuer(ctx); got != DefaultJWTIssuer {
		t.Fatalf("unset: %q", got)
	}
	if err := st.SetJWTIssuer(ctx, "http://meerkat:7070"); err != nil {
		t.Fatal(err)
	}
	if got := st.JWTIssuer(ctx); got != "http://meerkat:7070" {
		t.Fatalf("set: %q", got)
	}
	if err := st.SetSetting(ctx, SettingJWTIssuer, "two words"); err != nil {
		t.Fatal(err)
	}
	if got := st.JWTIssuer(ctx); got != DefaultJWTIssuer {
		t.Fatalf("an invalid stored value: %q", got)
	}
}

func TestCheckJWTIssuer(t *testing.T) {
	for v, ok := range map[string]bool{
		"meerkat": true, "acme-gateway": true, "http://meerkat:7070": true,
		"https://id.acme.example/gw": true,
		"":                           false, "two words": false, "http://meerkat:7070/": false,
		"ftp://meerkat": false, "http://": false, "https://x.example?a=1": false,
	} {
		if err := CheckJWTIssuer(v); (err == nil) != ok {
			t.Errorf("%q: %v", v, err)
		}
	}
}
