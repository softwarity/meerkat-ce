package admin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// TestSaveMemberIsTheWholeGrant: holding a group in an organisation somebody
// does not belong to grants nothing, so joining and setting groups is ONE
// decision that took two endpoints in a fixed order - the shape a half-done
// grant comes in. The tool does both, and answers with the roles the person now
// carries, which is the question behind the call.
func TestSaveMemberIsTheWholeGrant(t *testing.T) {
	a, ctx := grantAPI(t)

	// A role, a group that grants it, and somebody to give it to.
	if _, err := a.toolSaveRole(ctx, json.RawMessage(`{"name":"ROLE_ADMIN"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.toolSaveRole(ctx, json.RawMessage(`{"name":"ROLE_USER","grantedBy":"ROLE_ADMIN"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.toolSaveGroup(ctx, json.RawMessage(`{"tenant":"Acme","name":"operators","roles":["ROLE_ADMIN"]}`)); err != nil {
		t.Fatalf("save_group: %v", err)
	}

	out, err := a.toolSaveMember(ctx, json.RawMessage(`{"tenant":"Acme","username":"jo","groups":["operators"]}`))
	if err != nil {
		t.Fatalf("save_member: %v", err)
	}
	m := out.(map[string]any)
	if m["joined"] != true {
		t.Errorf("the membership was not created: %+v", m)
	}
	// ROLE_ADMIN implies ROLE_USER, and what the person CARRIES is both.
	roles, _ := m["roles"].([]string)
	if len(roles) != 2 || !contains(roles, "ROLE_ADMIN") || !contains(roles, "ROLE_USER") {
		t.Errorf("want both roles through the hierarchy, got %v", roles)
	}

	// A group the organisation does not hold is refused, and says the
	// membership was saved and the groups were not - a half-done grant that
	// says so beats one that does not.
	_, err = a.toolSaveMember(ctx, json.RawMessage(`{"tenant":"Acme","username":"jo","groups":["ghosts"]}`))
	if err == nil {
		t.Fatal("an unknown group was accepted")
	}
	if !strings.Contains(err.Error(), "list_groups") {
		t.Errorf("the refusal helps nobody: %v", err)
	}

	// And the group cannot be deleted while somebody holds it.
	_, err = a.toolDeleteGroup(ctx, json.RawMessage(`{"tenant":"Acme","name":"operators"}`))
	if err == nil {
		t.Fatal("a group somebody holds was deleted")
	}
	if !strings.Contains(err.Error(), "1 member") {
		t.Errorf("the refusal does not count them: %v", err)
	}
}

func grantAPI(t *testing.T) (*API, context.Context) {
	t.Helper()
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.WithValue(context.Background(), mcpActorKey{}, store.User{Username: "admin", Root: true})
	if err := st.SetTenancy(ctx, store.TenancyMulti); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveTenant(ctx, store.Tenant{ID: "t1", Name: "Acme", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "jo", PasswordHash: "x", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	return &API{st: st, router: gateway.New(st, nil)}, ctx
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestSaveUserDrawsTheFirstPasswordAndMarksIt: an agent must not CHOOSE a
// password - what it writes it also remembers, in a transcript nobody treats as
// a vault. So the gateway draws one and marks it to be changed at the first
// sign-in: the secret in the answer stops working the moment it is used.
func TestSaveUserDrawsTheFirstPasswordAndMarksIt(t *testing.T) {
	a, ctx := grantAPI(t)

	out, err := a.toolSaveUser(ctx, json.RawMessage(`{"username":"rita","fullname":"Rita","email":"rita@example.test"}`))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	m := out.(map[string]any)
	if m["created"] != true {
		t.Errorf("want a new account: %+v", m)
	}
	pw, _ := m["password"].(string)
	if len(pw) < 12 {
		t.Errorf("no first credential was issued: %+v", m)
	}
	if m["passwordMustChange"] != true {
		t.Errorf("the credential was not marked to be changed: %+v", m)
	}
	// It really is that password, and it really is theirs.
	user, err := a.st.GetUserByUsername(ctx, "rita")
	if err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(pw)) != nil {
		t.Error("the answer does not carry the password that was set")
	}

	// A second save that says nothing about the password issues none: an
	// account whose credential rotates every time somebody fixes a typo in a
	// full name is an account nobody can sign into.
	out, err = a.toolSaveUser(ctx, json.RawMessage(`{"username":"rita","fullname":"Rita R."}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, issued := out.(map[string]any)["password"]; issued {
		t.Error("an ordinary edit issued a new password")
	}

	// And an agent cannot choose one.
	_, err = a.toolSaveUser(ctx, json.RawMessage(`{"username":"rita","password":"hunter2"}`))
	if err == nil {
		t.Fatal("a chosen password was accepted")
	}
	if !strings.Contains(err.Error(), "temporary") {
		t.Errorf("the refusal does not say what to pass: %v", err)
	}
}
