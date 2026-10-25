package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// An authority says how many accounts came in through it: deleting it
// strands them. Accounts, not identities - a person linked twice is one.
func TestAnAuthorityCountsThePeopleLinkedToIt(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	if code, body := f.call(t, "PUT", "/api/auth-providers/acme", oidcBody("${acme-secret}"), f.rootC); code != http.StatusOK {
		t.Fatalf("put: %d %s", code, body)
	}
	for _, id := range []string{"carol", "dave"} {
		if err := f.api.st.CreateUser(ctx, store.User{ID: id, Username: id, PasswordHash: "x", Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range [][2]string{{"sub-carol", "carol"}, {"sub-carol-2", "carol"}, {"sub-dave", "dave"}} {
		if err := f.api.st.LinkIdentity(ctx, "acme", l[0], l[1], nil); err != nil {
			t.Fatal(err)
		}
	}
	var listed []struct {
		ID     string `json:"id"`
		Linked int    `json:"linked"`
	}
	if err := json.Unmarshal([]byte(get(t, f, "/api/auth-providers")), &listed); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range listed {
		switch p.ID {
		case "acme":
			found = true
			if p.Linked != 2 {
				t.Errorf("acme lists %d linked, want 2 (carol twice, dave once)", p.Linked)
			}
		default:
			if p.Linked != 0 {
				t.Errorf("%s lists %d linked, want 0", p.ID, p.Linked)
			}
		}
	}
	if !found {
		t.Fatalf("acme is not listed: %+v", listed)
	}
	// The save answers the same count.
	code, body := f.call(t, "PUT", "/api/auth-providers/acme", oidcBody("${acme-secret}"), f.rootC)
	if code != http.StatusOK || !strings.Contains(body, `"linked":2`) {
		t.Fatalf("save: %d %s, want linked 2", code, body)
	}
}

// A kind it does not know is refused with every kind it does.
func TestAnUnknownAuthorityKindNamesEveryKind(t *testing.T) {
	f := setup(t)
	code, body := f.call(t, "PUT", "/api/auth-providers/x", `{"kind":"kerberos","name":"X"}`, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("%d %s, want 422", code, body)
	}
	for _, kind := range store.ProviderKinds {
		if !strings.Contains(body, kind) {
			t.Errorf("the refusal leaves out %s: %s", kind, body)
		}
	}
}
