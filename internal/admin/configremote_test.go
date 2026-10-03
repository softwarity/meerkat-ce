package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/confrepo"
	"github.com/softwarity/meerkat/internal/confrepo/confrepotest"
	"github.com/softwarity/meerkat/internal/store"
)

// A configuration in a git repository (CFG-07), against a repository that lives
// in memory.
//
// What every test here is really about is ONE sentence: a pull applies nothing.
// The gateway goes on serving what it served, the document lands on the shelf,
// and a human decides. Everything else - the binding, the revision, the refusal
// to force - exists to keep that sentence true when two people and a branch are
// involved.

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
	f := setup(t)
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

	code, body = f.call(t, "POST", "/api/configurations/pull",
		`{"remoteId":"`+acme+`"}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("pull: %d %s", code, body)
	}
	var out pullAnswer
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("not a pull answer: %s", body)
	}
	// The row exists, bound, with the revision it was read at.
	if out.Configuration.Name != "Acme prod" {
		t.Errorf("the pulled configuration is called %q", out.Configuration.Name)
	}
	if out.Configuration.RemoteID != acme {
		t.Error("a pulled configuration came back unbound")
	}
	if out.Configuration.RemoteRev == "" {
		t.Error("a pulled configuration came back with no revision")
	}
	// And the plan says what activating it WOULD do, which is the only useful
	// next question.
	if out.Plan == nil {
		t.Fatalf("a pull answered no plan: %s", body)
	}
	if !planMentions(out.Plan, "from-git") {
		t.Errorf("the plan does not mention the role the document carries: %+v", out.Plan.Changes)
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
	f := setup(t)
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
	var saved store.Configuration
	if err := json.Unmarshal([]byte(body), &saved); err != nil {
		t.Fatal(err)
	}

	// Unbound, a push refuses with the cure rather than a 404.
	code, body = f.call(t, "POST", "/api/configurations/"+saved.ID+"/push", "", f.rootC)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("pushing an unbound configuration: %d %s", code, body)
	}
	if !strings.Contains(body, "not bound") {
		t.Errorf("the refusal does not say what is wrong: %s", body)
	}

	code, body = f.call(t, "PUT", "/api/configurations/"+saved.ID+"/remote",
		`{"remoteId":"`+acme+`"}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("bind: %d %s", code, body)
	}
	code, body = f.call(t, "POST", "/api/configurations/"+saved.ID+"/push", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("push: %d %s", code, body)
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
	f := setup(t)
	repo := confrepotest.New()
	repo.Register(t)

	code, body := f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme","url":"https://example.invalid/acme.git","branch":"main"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create location: %d %s", code, body)
	}
	acme := remoteID(t, body)

	code, body = f.call(t, "POST", "/api/configurations", `{"name":"Acme"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("capture: %d %s", code, body)
	}
	var saved store.Configuration
	if err := json.Unmarshal([]byte(body), &saved); err != nil {
		t.Fatal(err)
	}
	if code, body = f.call(t, "PUT", "/api/configurations/"+saved.ID+"/remote",
		`{"remoteId":"`+acme+`"}`, f.rootC); code != http.StatusOK {
		t.Fatalf("bind: %d %s", code, body)
	}
	if code, body = f.call(t, "POST", "/api/configurations/"+saved.ID+"/push", "", f.rootC); code != http.StatusOK {
		t.Fatalf("first push: %d %s", code, body)
	}

	// Somebody else pushes to the branch.
	repo.Move()

	code, body = f.call(t, "POST", "/api/configurations/"+saved.ID+"/push", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("pushing over somebody else's push: %d %s", code, body)
	}
	if repo.Commits != 2 {
		t.Errorf("the push did not commit: %d commits", repo.Commits)
	}
}

// A location is a directory, so two of them in one repository never see each
// other - which is the whole one-platform-per-customer story.
func TestTwoLocationsOneRepository(t *testing.T) {
	f := setup(t)
	repo := confrepotest.New()
	repo.Register(t)

	ids := map[string]string{}
	for name, dir := range map[string]string{"Acme": "platforms/acme", "Foo": "platforms/foo"} {
		code, body := f.call(t, "POST", "/api/config-remotes",
			`{"name":"`+name+`","url":"https://example.invalid/all.git","branch":"main","dir":"`+dir+`"}`,
			f.rootC)
		if code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", name, code, body)
		}
		ids[name] = remoteID(t, body)
	}
	// One configuration per location, each pushed.
	for name := range ids {
		code, body := f.call(t, "POST", "/api/configurations", `{"name":"`+name+` config"}`, f.rootC)
		if code != http.StatusCreated {
			t.Fatalf("capture %s: %d %s", name, code, body)
		}
		var saved store.Configuration
		if err := json.Unmarshal([]byte(body), &saved); err != nil {
			t.Fatal(err)
		}
		if code, body = f.call(t, "PUT", "/api/configurations/"+saved.ID+"/remote",
			`{"remoteId":"`+ids[name]+`"}`, f.rootC); code != http.StatusOK {
			t.Fatalf("bind %s: %d %s", name, code, body)
		}
		if code, body = f.call(t, "POST", "/api/configurations/"+saved.ID+"/push", "",
			f.rootC); code != http.StatusOK {
			t.Fatalf("push %s: %d %s", name, code, body)
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
	f := setup(t)
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
	f := setup(t)
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
	f := setup(t)
	repo := confrepotest.New()
	repo.Register(t)
	repo.Put(confrepo.DocumentName, []byte("version: 1\n"))

	code, body := f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme","url":"https://example.invalid/a.git","branch":"main"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	acme := remoteID(t, body)
	code, body = f.call(t, "POST", "/api/configurations/pull", `{"remoteId":"`+acme+`"}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("pull: %d %s", code, body)
	}
	var out pullAnswer
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}

	if code, body = f.call(t, "DELETE", "/api/config-remotes/"+acme, "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", code, body)
	}
	code, body = f.call(t, "GET", "/api/configurations/"+out.Configuration.ID, "", f.rootC)
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
	f := setup(t)
	repo := confrepotest.New()
	repo.Register(t)

	code, body := f.call(t, "POST", "/api/config-remotes",
		`{"name":"Acme","url":"https://example.invalid/a.git","branch":"main","dir":"platforms/acme"}`,
		f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, body)
	}
	acme := remoteID(t, body)
	code, body = f.call(t, "POST", "/api/configurations/pull", `{"remoteId":"`+acme+`"}`, f.rootC)
	if code != http.StatusNotFound {
		t.Fatalf("pulling an empty location: %d %s", code, body)
	}
	if !strings.Contains(body, "push one there first") {
		t.Errorf("the message does not say what to do: %s", body)
	}
}

func planMentions(plan *config.Plan, label string) bool {
	for _, c := range plan.Changes {
		if strings.Contains(c.ID, label) || strings.Contains(c.Label, label) {
			return true
		}
	}
	return false
}

func repoPaths(repo *confrepotest.Fake) []string {
	out := make([]string, 0, len(repo.Files))
	for path := range repo.Files {
		out = append(out, path)
	}
	return out
}
