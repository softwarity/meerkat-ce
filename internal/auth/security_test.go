package auth

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// securityTrail reads the security half of the trail, oldest first - the
// order a reader follows an incident in.
func securityTrail(t *testing.T, st *store.Store) []store.AuditEvent {
	t.Helper()
	evs, err := st.ListAuditEvents(context.Background(), store.AuditFilter{Kind: store.AuditKindSecurity})
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(evs)-1; i < j; i, j = i+1, j-1 {
		evs[i], evs[j] = evs[j], evs[i]
	}
	return evs
}

// A sign-in, and the refusals before it, leave the lines an administrator
// needs to read an attack: who was tried, from where, why it was refused -
// the REAL reason, which the page never said - and when the account locked.
// The attempts refused after the lock write nothing, so hammering an account
// cannot grow the table at the attacker's pace.
func TestTheSecurityTrailRecordsSignInsAndRefusals(t *testing.T) {
	mux, _, st := mfaSetup(t)
	ctx := context.Background()
	if err := st.SetSetting(ctx, store.SettingRateLimit,
		store.RateLimitPolicy{LoginAttempts: 2, LoginWindow: "PT15M", TotpAttempts: 5}); err != nil {
		t.Fatal(err)
	}

	if rec := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("sign-in: %d", rec.Code)
	}
	do(t, mux, "POST", "/login", url.Values{"username": {"nobody"}, "password": {"x"}}, nil)
	for i := 0; i < 4; i++ {
		do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"wrong"}}, nil)
	}

	evs := securityTrail(t, st)
	type line struct{ action, name, detail string }
	want := []line{
		{secSignin, "admin", loginMethodPassword},
		{secSigninRefused, "nobody", refusedCredentials},
		{secSigninRefused, "admin", refusedCredentials},
		{secSigninRefused, "admin", refusedCredentials},
		{secSigninLocked, "admin", refusedThrottled},
	}
	if len(evs) != len(want) {
		for _, e := range evs {
			t.Logf("%s %s %s", e.Action, e.TargetName, e.Detail)
		}
		t.Fatalf("%d lines, want %d", len(evs), len(want))
	}
	for i, w := range want {
		e := evs[i]
		if e.Action != w.action || e.TargetName != w.name || e.Detail != w.detail {
			t.Errorf("line %d = %s %s %s, want %s %s %s", i, e.Action, e.TargetName, e.Detail, w.action, w.name, w.detail)
		}
		if e.Target != store.AuditTargetAccount {
			t.Errorf("line %d target = %q", i, e.Target)
		}
		if e.IP == "" {
			t.Errorf("line %d has no address", i)
		}
	}
	// Somebody unknown is nobody: the name is what was typed, not an actor.
	if evs[1].ActorID != "" {
		t.Errorf("an unknown name was recorded as actor %q", evs[1].ActorID)
	}
	if evs[0].ActorID != "u1" {
		t.Errorf("the sign-in's actor = %q", evs[0].ActorID)
	}

	// And the other half does not see them.
	admin, err := st.ListAuditEvents(ctx, store.AuditFilter{Kind: store.AuditKindAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if len(admin) != 0 {
		t.Fatalf("the changes' half holds %d security lines", len(admin))
	}
}

// A sign-in to the CONSOLE is its own kind: where and when the people who run
// the gateway work is root's business, not an application administrator's.
func TestAConsoleSignInIsItsOwnKind(t *testing.T) {
	_, _, st := mfaSetup(t)
	admin := http.NewServeMux()
	NewAdmin(st, session.NewManager(st, session.ForAdminPlane())).Register(admin)
	do(t, admin, "POST", "/login", url.Values{"username": {"admin"}, "password": {"wrong"}}, nil)
	evs := securityTrail(t, st)
	if len(evs) != 1 || evs[0].Target != store.AuditTargetConsole {
		t.Fatalf("console refusal: %+v", evs)
	}
	if kind, _ := store.AuditTargetOf(store.AuditTargetConsole); len(kind.Domains) != 0 {
		t.Fatalf("a console sign-in is visible to %v, want root alone", kind.Domains)
	}
}

// A second factor that is wrong is the line that matters most - whoever typed
// it had the password.
func TestAWrongSecondFactorIsRecorded(t *testing.T) {
	mux, _, st := mfaSetup(t)
	enrol(t, st)
	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	do(t, mux, "POST", "/totp", url.Values{"code": {"000000"}}, sessionCookieOf(login))
	evs := securityTrail(t, st)
	last := evs[len(evs)-1]
	if last.Action != secSigninRefused || last.Detail != refusedCode || last.ActorID != "u1" {
		t.Fatalf("last line = %s %s by %q", last.Action, last.Detail, last.ActorID)
	}
}

// A sign-in says which organisation it entered, and the line is stamped with
// it - which is what lets that organisation's administrators read it, the
// trail partitioning by tenant. A person with one organisation has it on the
// sign-in itself; a person with several enters one by choosing, and the
// choice is its own line.
func TestASignInNamesTheOrganisationItEntered(t *testing.T) {
	mux, _, st := groupSetup(t)
	ctx := context.Background()

	login := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	if login.Header().Get("Location") != "/select-tenant" {
		t.Fatalf("login went to %q", login.Header().Get("Location"))
	}
	do(t, mux, "POST", "/select-tenant", url.Values{"tenant": {"t2"}}, sessionCookieOf(login))

	evs := securityTrail(t, st)
	last := evs[len(evs)-1]
	if last.Action != secSigninTenant || last.TenantID != "t2" || last.Detail != "globex" {
		t.Fatalf("last line = %s tenant %q detail %q", last.Action, last.TenantID, last.Detail)
	}

	// Read as globex's administrator sees the trail: the entry is there, and
	// nothing of acme's.
	scoped, err := st.ListAuditEvents(ctx, store.AuditFilter{
		Kind: store.AuditKindSecurity, Scope: &store.AuditScope{TenantIDs: []string{"t2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped) != 1 || scoped[0].Action != secSigninTenant {
		t.Fatalf("globex's administrator reads %+v", scoped)
	}

	// One organisation: the sign-in itself carries it.
	if _, err := st.DeleteMembership(ctx, "u1", "t1"); err != nil {
		t.Fatal(err)
	}
	do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	evs = securityTrail(t, st)
	last = evs[len(evs)-1]
	if last.Action != secSignin || last.TenantID != "t2" {
		t.Fatalf("a one-organisation sign-in = %s tenant %q", last.Action, last.TenantID)
	}
}

// A password stored at a weaker cost is re-hashed at its owner's next sign-in
// (SEC-05) - and that is not a change of password: no history row, no new
// changed-at restarting the expiry. The test account is hashed at the minimum
// cost, which is exactly the case.
func TestAWeakHashIsMadeStrongAtSignIn(t *testing.T) {
	mux, _, st := mfaSetup(t)
	ctx := context.Background()
	before, err := st.GetUserByID(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if cost, _ := bcrypt.Cost([]byte(before.PasswordHash)); cost >= passwordCost {
		t.Fatalf("the fixture is already at cost %d", cost)
	}

	if rec := do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("sign-in: %d", rec.Code)
	}
	after, _ := st.GetUserByID(ctx, "u1")
	if cost, _ := bcrypt.Cost([]byte(after.PasswordHash)); cost != passwordCost {
		t.Fatalf("cost after sign-in = %d, want %d", cost, passwordCost)
	}
	if bcrypt.CompareHashAndPassword([]byte(after.PasswordHash), []byte("s3cret")) != nil {
		t.Fatal("the new hash is not of the same password")
	}
	if after.PasswordChangedAt != before.PasswordChangedAt {
		t.Fatal("a rehash restarted the password's expiry")
	}
	if old, _ := st.RecentPasswordHashes(ctx, "u1", 10); len(old) != 0 {
		t.Fatalf("a rehash was filed as a password change: %d rows", len(old))
	}
	// And the next sign-in has nothing to do: the hash is untouched.
	do(t, mux, "POST", "/login", url.Values{"username": {"admin"}, "password": {"s3cret"}}, nil)
	if again, _ := st.GetUserByID(ctx, "u1"); again.PasswordHash != after.PasswordHash {
		t.Fatal("a hash at the target cost was rewritten")
	}
}
