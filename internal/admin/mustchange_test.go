package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// Forcing a change KEEPS the password: the person signs in as usual and is
// sent to the change page. A reset would replace it with a temporary one
// somebody then has to carry to its owner - a phone call per person.
func TestForceAPasswordChange(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	local := store.User{ID: "u-local", Username: "local", PasswordHash: "$2a$10$fakehashfakehashfakeha",
		Enabled: true}
	if err := f.api.st.CreateUser(ctx, local); err != nil {
		t.Fatal(err)
	}
	// Born at an authority: no password here to change.
	external := store.User{ID: "u-ext", Username: "ext", Enabled: true}
	if err := f.api.st.CreateUser(ctx, external); err != nil {
		t.Fatal(err)
	}

	if code, out := f.call(t, "POST", "/api/users/u-local/must-change-password", "", f.rootC); code != http.StatusOK {
		t.Fatalf("flag: %d %s", code, out)
	}
	if u, _ := f.api.st.GetUserByID(ctx, "u-local"); !u.MustChangePassword {
		t.Error("the account was not flagged")
	}
	// And the password is untouched.
	if u, _ := f.api.st.GetUserByID(ctx, "u-local"); u.PasswordHash != local.PasswordHash {
		t.Error("forcing a change replaced the password")
	}

	// An account with no local password is refused, in words that say why.
	code, out := f.call(t, "POST", "/api/users/u-ext/must-change-password", "", f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("external account: %d %s", code, out)
	}
	if u, _ := f.api.st.GetUserByID(ctx, "u-ext"); u.MustChangePassword {
		t.Error("an account without a local password was flagged")
	}
}

// Everyone at once: local accounts only, and never the administrator firing
// it - being signed out of the console by one's own click, in the middle of an
// incident, is the wrong lesson at the wrong moment.
func TestForceAPasswordChangeForEveryone(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for _, u := range []store.User{
		{ID: "u-a", Username: "a", PasswordHash: "$2a$10$fakehashfakehashfakeha", Enabled: true},
		{ID: "u-b", Username: "b", PasswordHash: "$2a$10$fakehashfakehashfakehb", Enabled: true},
		{ID: "u-ext", Username: "ext", Enabled: true},
	} {
		if err := f.api.st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}

	code, out := f.call(t, "POST", "/api/users/must-change-password", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("flag all: %d %s", code, out)
	}
	var got struct {
		Users int `json:"users"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"u-a", "u-b"} {
		if u, _ := f.api.st.GetUserByID(ctx, id); !u.MustChangePassword {
			t.Errorf("%s was not flagged", id)
		}
	}
	if u, _ := f.api.st.GetUserByID(ctx, "u-ext"); u.MustChangePassword {
		t.Error("an account without a local password was flagged")
	}
	// Not the caller.
	if u, _ := f.api.st.GetUserByID(ctx, "root"); u.MustChangePassword {
		t.Error("the administrator flagged themselves")
	}

	// It reaches everybody at once, so root and nobody else.
	if code, _ := f.call(t, "POST", "/api/users/must-change-password", "", f.plainC); code != http.StatusForbidden {
		t.Fatalf("authz: %d, want 403", code)
	}
}
