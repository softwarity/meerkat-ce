package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
)

// The Developer hub lists its tools: the certificate always, the API docs
// only when the data-plane switch exposes them. The cert lives on its own
// sub-page; a non-dev is refused everywhere.
func TestDeveloperHub(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{ID: "d", Username: "devon", PasswordHash: string(hash), Enabled: true, Dev: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{ID: "b", Username: "bob", PasswordHash: string(hash), Enabled: true}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	New(st, session.NewManager(st)).Register(mux)

	devC := postLogin(t, mux, url.Values{"username": {"devon"}, "password": {"s3cret"}}).Result().Cookies()[0]
	bobC := postLogin(t, mux, url.Values{"username": {"bob"}, "password": {"s3cret"}}).Result().Cookies()[0]

	// The profile shows the Developer entry for a dev, and the hub links the cert.
	if prof := do(t, mux, "GET", "/profile", nil, devC); !strings.Contains(bodyString(prof), `href="/profile/dev"`) {
		t.Fatal("profile missing the Developer link for a dev")
	}
	// Read the body ONCE: bodyString drains the recorder, so a second call
	// answers "" and every Contains on it passes for the wrong reason - which
	// is how the assertion that used to guard the API entry was green while
	// testing nothing at all.
	// The tunnel ships off: no key is offered for a door that is not there,
	// and a bookmarked key page says why rather than looking broken.
	if body := bodyString(do(t, mux, "GET", "/profile/dev", nil, devC)); strings.Contains(body, `href="/profile/dev/key"`) {
		t.Fatal("the hub offered the plug key with the tunnel off")
	}
	if res := do(t, mux, "GET", "/profile/dev/key", nil, devC); res.Code != http.StatusForbidden || !strings.Contains(bodyString(res), "Infra, Plug") {
		t.Fatalf("key page with the tunnel off: code=%d", res.Code)
	}
	if body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, devC)); strings.Contains(body, `"plug":true`) {
		t.Fatalf("the user button offers the key with the tunnel off: %s", body)
	}
	if err := st.SetSetting(ctx, store.SettingPlug, store.PlugSetting{Enabled: true, Host: "dev.example.com", Port: 30222}); err != nil {
		t.Fatal(err)
	}

	hub := do(t, mux, "GET", "/profile/dev", nil, devC)
	hubBody := bodyString(hub)
	if hub.Code != http.StatusOK || !strings.Contains(hubBody, `href="/profile/dev/key"`) {
		t.Fatalf("hub: code=%d, cert link missing", hub.Code)
	}

	// The API entry: the dev capability is the whole gate, so a dev sees it.
	if !strings.Contains(hubBody, `href="/meerkat/apidocs/"`) {
		t.Fatal("API entry missing from the hub of a dev")
	}

	// The user-button payload mirrors the same gate (the Developer submenu):
	// a dev gets the flag, a non-dev never does.
	if body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, devC)); !strings.Contains(body, `"devDocs":true`) {
		t.Fatalf("user-button payload misses the devDocs flag for a dev: %s", body)
	}
	if body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, bobC)); strings.Contains(body, `"devDocs"`) {
		t.Fatalf("user-button payload advertises devDocs to a non-dev: %s", body)
	}

	// The key sub-page renders its form, and something that is not a public
	// key is refused there.
	key := do(t, mux, "GET", "/profile/dev/key", nil, devC)
	keyBody := bodyString(key)
	if key.Code != http.StatusOK || !strings.Contains(keyBody, "ssh-ed25519") {
		t.Fatalf("key page: code=%d", key.Code)
	}
	// The commands carry the address developers use, as Infra, Plug records
	// it - copied, not translated - and the profile named after the
	// application (the default branding's "MY APP").
	for _, want := range []string{
		"-p 30222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null get@dev.example.com install my-app | sh",
		"install-windows | bash -s -- dev.example.com 30222 my-app",
		"plug keygen -p my-app",
		"plug pubkey -p my-app",
	} {
		if !strings.Contains(keyBody, want) {
			t.Errorf("the key page does not carry %q", want)
		}
	}
	bad := do(t, mux, "POST", "/profile/dev-key", url.Values{"key": {"not a key"}}, devC)
	if bad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad key: code=%d, want 422", bad.Code)
	}

	// Two workstations, two keys: both listed, each removable on its own, and
	// each step a line of the trail named by the fingerprint.
	var lines []string
	for range 2 {
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		pk, _ := ssh.NewPublicKey(pub)
		line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pk)))
		lines = append(lines, line)
		if res := do(t, mux, "POST", "/profile/dev-key", url.Values{"key": {line}}, devC); res.Code != http.StatusSeeOther {
			t.Fatalf("adding a key: code=%d", res.Code)
		}
	}
	if res := do(t, mux, "POST", "/profile/dev-key", url.Values{"key": {lines[0]}}, devC); res.Code != http.StatusConflict {
		t.Fatalf("the same key twice: code=%d, want 409", res.Code)
	}
	keys, _ := st.ListDevKeys(ctx, "d")
	if len(keys) != 2 {
		t.Fatalf("keys after two additions: %d", len(keys))
	}
	// Unescaped: a fingerprint's "+" is written as an entity in the HTML.
	page := html.UnescapeString(bodyString(do(t, mux, "GET", "/profile/dev/key", nil, devC)))
	for _, k := range keys {
		if !strings.Contains(page, k.Fingerprint) {
			t.Errorf("the page does not list %s", k.Fingerprint)
		}
	}
	do(t, mux, "POST", "/profile/dev-key", url.Values{"action": {"remove"}, "id": {keys[0].ID}}, devC)
	if left, _ := st.ListDevKeys(ctx, "d"); len(left) != 1 || left[0].ID != keys[1].ID {
		t.Fatalf("after removing one: %+v", left)
	}
	trail, _ := st.ListAuditEvents(ctx, store.AuditFilter{Kind: store.AuditKindSecurity})
	seen := map[string]int{}
	for _, e := range trail {
		seen[e.Action]++
	}
	if seen[secDevKeyAdd] != 2 || seen[secDevKeyRemove] != 1 {
		t.Fatalf("the trail holds %v", seen)
	}

	// A non-dev is refused at the hub and the sub-page.
	for _, p := range []string{"/profile/dev", "/profile/dev/key"} {
		if res := do(t, mux, "GET", p, nil, bobC); res.Code != http.StatusForbidden {
			t.Fatalf("non-dev on %s: code=%d, want 403", p, res.Code)
		}
	}
}

// The same switch, seen from the pages: with developer mode off the profile
// stops offering the hub, the hub refuses, and the user button never mentions
// a Developer submenu. The account keeps its capability throughout - that is
// the point: the installation answers, not the flag.
func TestDeveloperModeHidesTheDeveloperSurface(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{ID: "d", Username: "devon", PasswordHash: string(hash), Enabled: true, Dev: true}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	New(st, session.NewManager(st)).Register(mux)
	devC := postLogin(t, mux, url.Values{"username": {"devon"}, "password": {"s3cret"}}).Result().Cookies()[0]

	if err := st.SetDevMode(ctx, false); err != nil {
		t.Fatal(err)
	}
	if body := bodyString(do(t, mux, "GET", "/profile", nil, devC)); strings.Contains(body, `href="/profile/dev"`) {
		t.Fatal("the profile offered the Developer entry with developer mode off")
	}
	for _, p := range []string{"/profile/dev", "/profile/dev/key"} {
		if res := do(t, mux, "GET", p, nil, devC); res.Code != http.StatusForbidden {
			t.Fatalf("%s with developer mode off: code=%d, want 403", p, res.Code)
		}
	}
	if body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, devC)); strings.Contains(body, `"devDocs"`) {
		t.Fatalf("the user button advertised the Developer submenu with developer mode off: %s", body)
	}

	// Back on, everything the developer had returns - no re-flagging, no
	// re-login.
	if err := st.SetDevMode(ctx, true); err != nil {
		t.Fatal(err)
	}
	if body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, devC)); !strings.Contains(body, `"devDocs":true`) {
		t.Fatalf("the Developer submenu did not come back: %s", body)
	}
}

// The developer key is reachable from the button's Developer submenu, not only
// from two levels down in the profile - it is the prerequisite for the tunnel,
// and where it lived is where nobody found it.
func TestTheButtonOffersTheDeveloperKey(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{ID: "d", Username: "devon", PasswordHash: string(hash), Enabled: true, Dev: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, store.User{ID: "b", Username: "bob", PasswordHash: string(hash), Enabled: true}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	New(st, session.NewManager(st)).Register(mux)
	devC := postLogin(t, mux, url.Values{"username": {"devon"}, "password": {"s3cret"}}).Result().Cookies()[0]
	bobC := postLogin(t, mux, url.Values{"username": {"bob"}, "password": {"s3cret"}}).Result().Cookies()[0]

	// The developer rows name themselves in English in the component - they
	// open a developer's tools, which this product does not translate - so what
	// travels is the STATE, not the wording: whether a key is deposited.
	body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, devC))
	if !strings.Contains(body, `"devDocs":true`) {
		t.Fatalf("the developer submenu is not offered to a dev: %s", body)
	}
	// No key deposited yet, so no check mark to claim otherwise.
	if strings.Contains(body, `"devKey":true`) {
		t.Fatalf("a key is announced before one was deposited: %s", body)
	}

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pk, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddDevKey(ctx, "d", strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pk)))); err != nil {
		t.Fatal(err)
	}
	if body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, devC)); !strings.Contains(body, `"devKey":true`) {
		t.Fatalf("the deposited key is not reflected: %s", body)
	}

	// A non-dev gets neither the submenu nor the flag - the capability is the
	// whole gate, as it is everywhere else on the developer surface.
	if body := bodyString(do(t, mux, "GET", "/meerkat/user-button.json", nil, bobC)); strings.Contains(body, `"devKey":true`) {
		t.Fatalf("a non-dev is told about a key: %s", body)
	}
}

// The profile offers a way back to where the person was, when the page that
// sent them said so - and never anywhere else, whatever the query string says.
func TestTheProfileReturnsWhereYouWere(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("s3cret"), bcrypt.MinCost)
	if err := st.CreateUser(ctx, store.User{ID: "u", Username: "alice", PasswordHash: string(hash), Enabled: true}); err != nil {
		t.Fatal(err)
	}
	// An application to come back TO: without one there is nothing for a
	// return to land on, which is the rule this exercises.
	if err := st.SaveRoute(ctx, store.Route{
		ID: "embed", Name: "embed", Order: 1, Enabled: true, IsUI: true,
		Upstream:   "http://example.invalid",
		Predicates: []routing.Spec{{Type: "path", Args: map[string]any{"patterns": []any{"/embed/**"}}}},
	}); err != nil {
		t.Fatal(err)
	}
	// And catalogued, which is what puts an application in the menu now: the
	// route existing is no longer enough (PORTAL-03).
	portal := store.PortalConfig{Mode: store.PortalModeLinks,
		Entries: []store.PortalEntry{{RouteID: "embed", Label: "NEO"}}}
	routes, err := st.ListRoutes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SanitizePortalConfig(&portal, routes); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, store.SettingPortal, portal); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	New(st, session.NewManager(st)).Register(mux)
	c := postLogin(t, mux, url.Values{"username": {"alice"}, "password": {"s3cret"}}).Result().Cookies()[0]

	deep := "/embed/ZmxpZ2h0LWZvbGRlci1mcm9udGVuZA"
	body := bodyString(do(t, mux, "GET", "/profile?from="+url.QueryEscape(deep), nil, c))
	if !strings.Contains(body, `href="`+deep+`"`) {
		t.Fatalf("the profile does not return to the page that sent us: %s", deep)
	}

	// A path is inside an entry by SEGMENTS, not by characters: /embedded is
	// not inside /embed, and a plain prefix test said it was.
	for _, in := range []string{"/embed", "/embed/x", "/embed?a=1"} {
		if !underEntry(in, "/embed") {
			t.Fatalf("%s should be inside /embed", in)
		}
	}
	for _, out := range []string{"/embedded", "/embedding/x", "/other"} {
		if underEntry(out, "/embed") {
			t.Fatalf("%s is not inside /embed", out)
		}
	}

	// Nobody said: the application is still listed, at its entry, and nothing
	// pretends to know where the person was.
	plain := bodyString(do(t, mux, "GET", "/profile", nil, c))
	if !strings.Contains(plain, `href="/embed"`) {
		t.Fatalf("the application is no longer listed at its entry: %s", plain)
	}
	if strings.Contains(plain, deep) {
		t.Fatal("a deep return appeared with nothing to return to")
	}

	// An absolute URL is not a place on this site: safeNext refuses it, which
	// is what stops this from being an open redirect in a link.
	body = bodyString(do(t, mux, "GET", "/profile?from="+url.QueryEscape("https://elsewhere.example/steal"), nil, c))
	if strings.Contains(body, "elsewhere.example") {
		t.Fatalf("the profile offered to send someone off-site: %s", body)
	}

	// And a page of the sign-in flow is not a place anyone wants to go back
	// to while they are signed in. safeNext accepts it - it IS this site - so
	// the guard has to be that a return only ever lands on an APPLICATION.
	body = bodyString(do(t, mux, "GET", "/profile?from=%2Flogin", nil, c))
	if strings.Contains(body, `href="/login"`) {
		t.Fatalf("the profile offered to send a signed-in person back to the sign-in page: %s", body)
	}
}
