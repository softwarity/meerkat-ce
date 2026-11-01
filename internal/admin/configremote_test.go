package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/confrepo"
	"github.com/softwarity/meerkat/internal/confrepo/confrepotest"
	"github.com/softwarity/meerkat/internal/store"
)

// A configuration in a git repository (CFG-07), against a repository that lives
// in memory.
//
// What every test here is really about is ONE sentence: a pull applies nothing.
// The gateway goes on serving what it served, the document lands on the shelf,
// and a human decides. Pull and push of a saved row are an agent's tools; the
// console reads a location on a side and pushes from there (compose_test.go).

// remoteID reads the id out of a created location.
func remoteID(t *testing.T, body string) string {
	t.Helper()
	var r store.ConfigRemote
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("not a git location: %s", body)
	}
	return r.ID
}

func TestAPullAppliesNothing(t *testing.T) {
	f := setupEE(t)
	repo := confrepotest.New()
	repo.Register(t)

	// A document in the repository, written by somebody else: one role, and a
	// route nothing here has ever heard of.
	repo.Put("platforms/acme/"+confrepo.DocumentName, []byte(
		"version: 1\nroles:\n  - name: from-git\n"))

	code, body := f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme prod","url":"https://example.invalid/acme.git","branch":"main",`+
			`"dir":"platforms/acme"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create location: %d %s", code, body)
	}
	acme := remoteID(t, body)

	ctx := mcpCtx(rootUser(t, f))
	out, err := f.api.toolPullConfiguration(ctx, json.RawMessage(`{"location":"Acme prod"}`))
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	// The row exists, bound, with the revision it was read at.
	saved := onlyConfiguration(t, f)
	if saved.Name != "Acme prod" {
		t.Errorf("the pulled configuration is called %q", saved.Name)
	}
	if saved.RemoteID != acme {
		t.Error("a pulled configuration came back unbound")
	}
	if saved.RemoteRev == "" {
		t.Error("a pulled configuration came back with no revision")
	}
	// And the plan says what activating it WOULD do, which is the only useful
	// next question.
	if a := jsonOf(t, out); !strings.Contains(a, `"applied":false`) || !strings.Contains(a, `"plan"`) {
		t.Fatalf("a pull must say nothing was applied and what would be: %s", a)
	}

	// THE SENTENCE: nothing was applied. The role is not in the gateway.
	code, body = f.call(t, "GET", "/api/roles", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("roles: %d %s", code, body)
	}
	if strings.Contains(body, "from-git") {
		t.Fatal("a pull applied the document: the role it carries is in the running gateway")
	}
}

// Pushing writes the saved copy into the location's directory, and the commit
// carries the operator.
func TestAPushWritesTheDirectory(t *testing.T) {
	f := setupEE(t)
	repo := confrepotest.New()
	repo.Register(t)

	code, body := f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme prod","url":"https://example.invalid/acme.git","branch":"main",`+
			`"dir":"platforms/acme"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create location: %d %s", code, body)
	}
	acme := remoteID(t, body)

	code, body = f.call(t, "POST", "/api/configurations", `{"name":"Acme"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("capture: %d %s", code, body)
	}
	ctx := mcpCtx(rootUser(t, f))

	// Unbound and with no location named, a push refuses with the cure.
	_, err := f.api.toolPushConfiguration(ctx, json.RawMessage(`{"configuration":"Acme"}`))
	if err == nil || !strings.Contains(err.Error(), "not bound") {
		t.Fatalf("pushing an unbound configuration: %v", err)
	}
	if _, err := f.api.toolPushConfiguration(ctx, json.RawMessage(`{"configuration":"Acme","location":"Acme prod"}`)); err != nil {
		t.Fatalf("push: %v", err)
	}
	if onlyConfiguration(t, f).RemoteID != acme {
		t.Error("naming a location must bind the configuration to it")
	}
	if repo.Commits != 1 {
		t.Errorf("the repository took %d commits", repo.Commits)
	}
	// In ITS directory, under the agreed name.
	if _, ok := repo.Files["platforms/acme/"+confrepo.DocumentName]; !ok {
		t.Errorf("the document is not where the location points: %v", repoPaths(repo))
	}
	// Attributed to the operator, by name.
	if len(repo.Messages) != 1 || !strings.Contains(repo.Messages[0], "Acme") {
		t.Errorf("the commit message is %q", repo.Messages)
	}
}

// A push REPLACES what the location holds, whatever somebody else pushed
// there since - as a pull replaces the saved configuration.
func TestAPushReplacesWhatTheRepositoryHolds(t *testing.T) {
	f := setupEE(t)
	repo := confrepotest.New()
	repo.Register(t)

	code, body := f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme","url":"https://example.invalid/acme.git","branch":"main"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create location: %d %s", code, body)
	}

	code, body = f.call(t, "POST", "/api/configurations", `{"name":"Acme"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("capture: %d %s", code, body)
	}
	ctx := mcpCtx(rootUser(t, f))
	push := json.RawMessage(`{"configuration":"Acme","location":"Acme"}`)
	if _, err := f.api.toolPushConfiguration(ctx, push); err != nil {
		t.Fatalf("first push: %v", err)
	}

	// Somebody else pushes to the branch.
	repo.Move()

	if _, err := f.api.toolPushConfiguration(ctx, push); err != nil {
		t.Fatalf("pushing over somebody else's push: %v", err)
	}
	if repo.Commits != 2 {
		t.Errorf("the push did not commit: %d commits", repo.Commits)
	}
}

// A location is a directory, so two of them in one repository never see each
// other - which is the whole one-platform-per-customer story.
func TestTwoLocationsOneRepository(t *testing.T) {
	f := setupEE(t)
	repo := confrepotest.New()
	repo.Register(t)

	ctx := mcpCtx(rootUser(t, f))
	names := []string{}
	for name, dir := range map[string]string{"Acme": "platforms/acme", "Foo": "platforms/foo"} {
		code, body := f.call(t, "POST", "/api/config-remotes",
			`{"name":"`+name+`","url":"https://example.invalid/all.git","branch":"main","dir":"`+dir+`"}`,
			f.rootC)
		if code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", name, code, body)
		}
		names = append(names, name)
	}
	// One configuration per location, each pushed.
	for _, name := range names {
		code, body := f.call(t, "POST", "/api/configurations", `{"name":"`+name+` config"}`, f.rootC)
		if code != http.StatusCreated {
			t.Fatalf("capture %s: %d %s", name, code, body)
		}
		push := json.RawMessage(`{"configuration":"` + name + ` config","location":"` + name + `"}`)
		if _, err := f.api.toolPushConfiguration(ctx, push); err != nil {
			t.Fatalf("push %s: %v", name, err)
		}
	}
	for _, want := range []string{"platforms/acme/" + confrepo.DocumentName, "platforms/foo/" + confrepo.DocumentName} {
		if _, ok := repo.Files[want]; !ok {
			t.Errorf("%s is missing: %v", want, repoPaths(repo))
		}
	}
}

// The token is a vault REFERENCE or nothing: a literal in this table would be a
// credential in clear in a row the console reads and a snapshot carries.
func TestTheTokenMustBeAVaultReference(t *testing.T) {
	f := setupEE(t)
	repo := confrepotest.New()
	repo.Register(t)

	code, body := f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme","url":"https://example.invalid/a.git","branch":"main",`+
			`"tokenRef":"ghp_averyrealsecret"}`, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("a literal token was accepted: %d %s", code, body)
	}
	if !strings.Contains(body, "vault") {
		t.Errorf("the refusal does not say where a token goes: %s", body)
	}

	// And a reference the vault does not hold is refused where the hole is,
	// rather than passed on as an empty password for a forge to blame.
	code, body = f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme","url":"https://example.invalid/a.git","branch":"main",`+
			`"tokenRef":"${nowhere}"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	id := remoteID(t, body)
	code, body = f.call(t, "POST", "/api/config-remotes/"+id+"/check", "", f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("checking a location whose vault entry is missing: %d %s", code, body)
	}
	if !strings.Contains(body, "nowhere") {
		t.Errorf("the refusal does not name the missing entry: %s", body)
	}
}

// Only https, and the refusal says what is supported rather than failing later
// with a transport error nobody can read.
func TestOnlyHTTPSWithAToken(t *testing.T) {
	f := setupEE(t)
	repo := confrepotest.New()
	repo.Register(t)

	code, body := f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme","url":"git@github.com:acme/conf.git","branch":"main"}`, f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("an ssh URL was accepted: %d %s", code, body)
	}
	if !strings.Contains(body, "https") {
		t.Errorf("the refusal does not say what is allowed: %s", body)
	}
}

// Deleting a location keeps the configurations that came from it - they are
// copies of a state, and losing a destination is not losing them.
func TestDeletingALocationKeepsItsConfigurations(t *testing.T) {
	f := setupEE(t)
	repo := confrepotest.New()
	repo.Register(t)
	repo.Put(confrepo.DocumentName, []byte("version: 1\n"))

	code, body := f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme","url":"https://example.invalid/a.git","branch":"main"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	acme := remoteID(t, body)
	if _, err := f.api.toolPullConfiguration(mcpCtx(rootUser(t, f)), json.RawMessage(`{"location":"Acme"}`)); err != nil {
		t.Fatalf("pull: %v", err)
	}
	pulled := onlyConfiguration(t, f)

	if code, body = f.call(t, "DELETE", "/api/config-remotes/"+acme, "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", code, body)
	}
	code, body = f.call(t, "GET", "/api/configurations/"+pulled.ID, "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("the configuration went with its location: %d %s", code, body)
	}
	var still store.Configuration
	if err := json.Unmarshal([]byte(body), &still); err != nil {
		t.Fatal(err)
	}
	if still.RemoteID != "" {
		t.Error("the configuration still points at a location that no longer exists")
	}
}

// A location with nothing in it yet is a destination waiting for a push, not a
// failure - and the message says so.
func TestAnEmptyLocationSaysSo(t *testing.T) {
	f := setupEE(t)
	repo := confrepotest.New()
	repo.Register(t)

	code, body := f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme","url":"https://example.invalid/a.git","branch":"main","dir":"platforms/acme"}`,
		f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	acme := remoteID(t, body)
	code, body = f.call(t, "POST", "/api/configurations/compose",
		`{"left":{"source":"running"},"right":{"source":"remote","id":"`+acme+`"}}`, f.rootC)
	if code != http.StatusNotFound {
		t.Fatalf("reading an empty location: %d %s", code, body)
	}
	if !strings.Contains(body, "push one there first") {
		t.Errorf("the message does not say what to do: %s", body)
	}
}

// onlyConfiguration is the one saved row the test has made, as the store holds
// it: the list does not carry the binding.
func onlyConfiguration(t *testing.T, f fixture) store.Configuration {
	t.Helper()
	ctx := context.Background()
	rows, err := f.api.st.ListConfigurations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("%d saved configurations, want one", len(rows))
	}
	c, err := f.api.st.GetConfiguration(ctx, rows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func repoPaths(repo *confrepotest.Fake) []string {
	out := make([]string, 0, len(repo.Files))
	for path := range repo.Files {
		out = append(out, path)
	}
	return out
}

// An agent creates a location and changes it by name, with the console's rules:
// a literal token is refused, and a change touches only what it names.
func TestAnAgentSavesAGitLocation(t *testing.T) {
	f := setupEE(t)
	confrepotest.New().Register(t)
	ctx := context.WithValue(context.Background(), mcpActorKey{}, store.User{ID: "root", Username: "root", Root: true})

	if _, err := f.api.toolSaveGitLocation(ctx, json.RawMessage(
		`{"name":"Acme","url":"https://example.invalid/a.git","branch":"main","tokenRef":"ghp_real"}`)); err == nil ||
		!strings.Contains(err.Error(), "vault") {
		t.Fatalf("a literal token: %v", err)
	}
	if _, err := f.api.toolSaveGitLocation(ctx, json.RawMessage(
		`{"name":"Acme","url":"https://example.invalid/a.git","branch":"main","directory":"prod","tokenRef":"${git}"}`)); err != nil {
		t.Fatalf("create: %v", err)
	}
	out, err := f.api.toolSaveGitLocation(ctx, json.RawMessage(`{"name":"Acme","branch":"release"}`))
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	if m, _ := out.(map[string]any); m["changed"] != "Acme" {
		t.Fatalf("the change did not find the location by name: %v", out)
	}
	list, err := f.api.st.ListConfigRemotes(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("locations: %v %v", list, err)
	}
	if r := list[0]; r.Branch != "release" || r.Dir != "prod" || r.TokenRef != "${git}" {
		t.Fatalf("a change of branch touched the rest: %+v", r)
	}
}
