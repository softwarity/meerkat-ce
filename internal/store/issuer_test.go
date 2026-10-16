package store

import (
	"context"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// appNames declares the application plane's public names, as the Domains &
// TLS screen does.
func appNames(t *testing.T, st *Store, proxied bool, names ...string) {
	t.Helper()
	cfg := st.RawTLS(context.Background())
	cfg.AppDoor = certs.Door{Proxied: proxied, Names: names}
	if err := st.SetSetting(context.Background(), SettingTLS, cfg); err != nil {
		t.Fatal(err)
	}
}

// The issuer is chosen among the application plane's public addresses: none
// declared, the fallback every token said before; one, that one; several, the
// stored choice, else the first; a stored value that is none of them (the
// free-text issuer of before) gives way to the first.
func TestTheIssuerFollowsThePublicNames(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	ctx := context.Background()
	const data = ":8080"

	if got := st.JWTIssuer(ctx, data); got != DefaultJWTIssuer {
		t.Fatalf("no public name: %q", got)
	}
	if got := st.IssuerChoices(ctx, data); got == nil || len(got) != 0 {
		t.Fatalf("no public name, choices: %#v", got)
	}
	// Choosing is refused while there is nothing to choose, and the refusal
	// says where to declare one.
	if err := st.SetJWTIssuer(ctx, data, "https://gw.example.com"); err == nil ||
		!strings.Contains(err.Error(), "Domains & TLS, Application tab") {
		t.Fatalf("no public name, set: %v", err)
	}

	// The old free-text default, stored: one public name replaces it.
	if err := st.SetSetting(ctx, SettingJWTIssuer, "meerkat"); err != nil {
		t.Fatal(err)
	}
	appNames(t, st, false, "gw.example.com")
	if got := st.JWTIssuer(ctx, data); got != "https://gw.example.com" {
		t.Fatalf("one name: %q", got)
	}

	// Several: the first until one is chosen, then the choice.
	appNames(t, st, false, "gw.example.com", "id.example.com")
	if got := st.JWTIssuer(ctx, data); got != "https://gw.example.com" {
		t.Fatalf("several, none chosen: %q", got)
	}
	if err := st.SetJWTIssuer(ctx, data, "https://id.example.com"); err != nil {
		t.Fatal(err)
	}
	if got := st.JWTIssuer(ctx, data); got != "https://id.example.com" {
		t.Fatalf("several, chosen: %q", got)
	}
	// Anything else is refused, naming what is allowed.
	err = st.SetJWTIssuer(ctx, data, "http://meerkat:7070")
	if err == nil || !strings.Contains(err.Error(), "https://gw.example.com, https://id.example.com") {
		t.Fatalf("a value outside the public addresses: %v", err)
	}

	// The chosen name removed: the issuer follows to the first that is left.
	appNames(t, st, false, "gw.example.com")
	if got := st.JWTIssuer(ctx, data); got != "https://gw.example.com" {
		t.Fatalf("the chosen name removed: %q", got)
	}

	// A proxy in front serves the name over HTTPS: still https.
	appNames(t, st, true, "edge.example.com")
	if got := st.JWTIssuer(ctx, data); got != "https://edge.example.com" {
		t.Fatalf("a proxy in front: %q", got)
	}

	// No public name any more: the fallback, whatever was chosen.
	appNames(t, st, false)
	if got := st.JWTIssuer(ctx, data); got != DefaultJWTIssuer {
		t.Fatalf("the names removed: %q", got)
	}
}

// A loopback name is reached over plain http on the data plane's port: a
// developer's gateway and the test suite's.
func TestALoopbackNameIsHTTPOnTheDataPort(t *testing.T) {
	for _, c := range []struct{ name, addr, want string }{
		{"localhost", ":18082", "http://localhost:18082"},
		{"localhost", "0.0.0.0:8080", "http://localhost:8080"},
		{"127.0.0.1", ":8080", "http://127.0.0.1:8080"},
		{"::1", ":8080", "http://[::1]:8080"},
		{"[::1]", ":8080", "http://[::1]:8080"},
		{"localhost", "", "http://localhost"},
		{"gw.example.com", ":8080", "https://gw.example.com"},
		{"localhost.example.com", ":8080", "https://localhost.example.com"},
	} {
		if got := IssuerAddress(c.name, c.addr); got != c.want {
			t.Errorf("%s on %q: %q, want %q", c.name, c.addr, got, c.want)
		}
	}
}
