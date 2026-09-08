package store

import (
	"context"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The addresses a token may be used from (MCP-02). A typo here would silently
// allow nobody, and its owner would blame the token.
func TestSanitizeTokenCIDRs(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", want: ""},
		// A bare address is the obvious thing to type: it means that one.
		{in: "10.0.0.7", want: "10.0.0.7/32"},
		{in: "10.0.0.0/24", want: "10.0.0.0/24"},
		// Written loosely, stored canonically, so two identical ranges read as
		// one thing in the list.
		{in: "10.0.0.9/24", want: "10.0.0.0/24"},
		{in: " 192.168.1.0/24 , 10.0.0.7 ", want: "192.168.1.0/24,10.0.0.7/32"},
		{in: "::1", want: "::1/128"},
		{in: "not-an-address", wantErr: true},
		{in: "10.0.0.0/99", wantErr: true},
	}
	for _, tc := range cases {
		got, err := SanitizeTokenCIDRs(tc.in)
		switch {
		case tc.wantErr && err == nil:
			t.Errorf("%q was accepted, want a refusal naming what is expected", tc.in)
		case !tc.wantErr && err != nil:
			t.Errorf("%q: %v", tc.in, err)
		case !tc.wantErr && got != tc.want:
			t.Errorf("%q became %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAllowsAddress(t *testing.T) {
	cases := []struct {
		cidrs, addr string
		want        bool
	}{
		// No range at all is what every token has always been: anywhere.
		{"", "203.0.113.9:5000", true},
		{"10.0.0.0/24", "10.0.0.7:51000", true},
		{"10.0.0.0/24", "10.0.1.7:51000", false},
		{"10.0.0.0/24,192.168.0.0/16", "192.168.4.4:1", true},
		// A v4 address arriving on a dual-stack listener wears a v6 coat, and
		// a v4 range that did not unwrap it would never match.
		{"10.0.0.0/24", "[::ffff:10.0.0.7]:51000", true},
		{"::1/128", "[::1]:51000", true},
		// A host with no port, as some proxies and tests hand it over.
		{"10.0.0.0/24", "10.0.0.7", true},
		// Unparseable is refused: a restriction that cannot be evaluated must
		// not pass.
		{"10.0.0.0/24", "unix-socket", false},
	}
	for _, tc := range cases {
		if got := AllowsAddress(tc.cidrs, tc.addr); got != tc.want {
			t.Errorf("AllowsAddress(%q, %q) = %v, want %v", tc.cidrs, tc.addr, got, tc.want)
		}
	}
}

// The domain is a restriction someone adds, so its empty value is the WIDE
// one - unlike the scope, whose empty value is the safe one.
func TestSanitizeTokenDomain(t *testing.T) {
	for in, want := range map[string]string{"": DomainAll, "all": DomainAll, "GATEWAY": DomainGateway, "app": DomainApp} {
		got, err := SanitizeTokenDomain(in)
		if err != nil || got != want {
			t.Errorf("%q -> %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := SanitizeTokenDomain("routes"); err == nil {
		t.Error("an invented domain was accepted")
	}
}

// Editing a token changes what it MAY DO and never the token.
//
// That is the whole basis for allowing it: the secret is a hash in a column
// and encodes no perimeter, no domain, no address, no expiry and no name - so
// whoever holds the key keeps holding the same key, and a narrowing costs
// nobody a redeployment.
func TestEditingATokenLeavesTheSecretAlone(t *testing.T) {
	st, err := OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, User{ID: "u1", Username: "root", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	hash := "the-hash-nobody-may-touch"
	if err := st.AddAPIToken(ctx, NewToken{
		ID: "t1", UserID: "u1", Name: "prometheus", TokenHash: hash, Prefix: "mk_abc",
		Plane: PlaneAdmin, Scope: ScopeFull, ExpiresAt: 0,
	}); err != nil {
		t.Fatal(err)
	}

	ok, err := st.UpdateAPIToken(ctx, "u1", "t1", TokenEdit{
		Name: "prometheus (scraper)", Scope: ScopeMetrics, Domain: DomainGateway,
		FromCIDRs: "10.0.0.7", ExpiresAt: 0,
	})
	if err != nil || !ok {
		t.Fatalf("UpdateAPIToken: %v (existed=%v)", err, ok)
	}

	// The same secret still resolves, and to the narrowed token.
	got, err := st.ResolveAPIToken(ctx, hash, time.Now().Unix())
	if err != nil {
		t.Fatalf("the token stopped resolving after an edit: %v", err)
	}
	if got.Scope != ScopeMetrics {
		t.Errorf("perimeter is %q, want %q", got.Scope, ScopeMetrics)
	}
	if got.Domain != DomainGateway {
		t.Errorf("domain is %q, want %q", got.Domain, DomainGateway)
	}
	if got.Name != "prometheus (scraper)" {
		t.Errorf("name is %q, want the edited one", got.Name)
	}
	if got.FromCIDRs != "10.0.0.7/32" {
		t.Errorf("addresses are %q, want the sanitized 10.0.0.7/32", got.FromCIDRs)
	}

	// And a perimeter nobody can spell is refused by name rather than stored.
	if _, err := st.UpdateAPIToken(ctx, "u1", "t1", TokenEdit{Name: "x", Scope: "moon"}); err == nil {
		t.Error("an invented perimeter was accepted")
	}
	// A token belonging to somebody else is not this account's to edit.
	if ok, _ := st.UpdateAPIToken(ctx, "u2", "t1", TokenEdit{Name: "x", Scope: ScopeFull}); ok {
		t.Error("another account edited this token")
	}
}
