package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/config"
	"github.com/softwarity/meerkat/internal/confrepo"
	"github.com/softwarity/meerkat/internal/confrepo/confrepotest"
	"github.com/softwarity/meerkat/internal/store"
)

// A saved configuration with a route that names a role the gateway does not
// have, and a role of its own: the right side of a composition.
func composeFixture(t *testing.T) fixture {
	t.Helper()
	f := setupBare(t)
	ctx := context.Background()
	c := store.Configuration{ID: "acme", Name: "acme", Document: strings.TrimSpace(`
version: 1
roles:
  - name: ROLE_BILLING
    description: invoices
routes:
  - id: r-billing
    name: billing
    enabled: true
    order: 45
    upstream: http://billing:8080
    predicates:
      - type: path
        args:
          pattern: /billing/**
    access:
      level: auth
      roles: [ROLE_BILLING, ROLE_SUPPORT]
`) + "\n"}
	if err := f.api.st.SaveConfiguration(ctx, &c); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestComposePreviewChangesNothingAndSaysWhatItWould(t *testing.T) {
	f := composeFixture(t)
	body := `{"left":{"source":"running"},"right":{"source":"configuration","id":"acme"},` +
		`"picks":[{"path":"route:r-billing","from":"right"}]}`
	code, out := f.call(t, "POST", "/api/configurations/compose", body, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("preview: %d %s", code, out)
	}
	var answer struct {
		RightName string          `json:"rightName"`
		Result    config.Document `json:"result"`
		Report    config.Report   `json:"report"`
		Plan      *config.Plan    `json:"plan"`
	}
	if err := json.Unmarshal([]byte(out), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.RightName != "acme" {
		t.Errorf("rightName = %q", answer.RightName)
	}
	if len(answer.Result.Routes) != 1 || answer.Result.Routes[0].ID != "r-billing" {
		t.Errorf("result routes = %+v, want billing taken", answer.Result.Routes)
	}
	// ROLE_BILLING pulled along, ROLE_SUPPORT said missing, soft.
	if len(answer.Report.Pulled) != 1 || answer.Report.Pulled[0].ID != "ROLE_BILLING" {
		t.Errorf("pulled = %+v", answer.Report.Pulled)
	}
	if len(answer.Report.Missing) != 1 || answer.Report.Missing[0].Key != "ROLE_SUPPORT" || answer.Report.Missing[0].Hard {
		t.Errorf("missing = %+v", answer.Report.Missing)
	}
	if answer.Plan == nil || !answer.Plan.Touches() {
		t.Errorf("the plan of applying must say the route is added: %+v", answer.Plan)
	}
	routes, err := f.api.st.ListRoutes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Errorf("a preview applies nothing, the gateway has %d routes", len(routes))
	}
}

func TestComposeApplyMakesTheResultCurrent(t *testing.T) {
	f := composeFixture(t)
	body := `{"left":{"source":"running"},"right":{"source":"configuration","id":"acme"},` +
		`"picks":[{"path":"route:r-billing","from":"right"},{"path":"route:r-billing/access.roles[ROLE_SUPPORT]","op":"drop"}]}`
	code, out := f.call(t, "POST", "/api/configurations/compose/apply", body, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("apply: %d %s", code, out)
	}
	ctx := context.Background()
	route, err := f.api.st.GetRoute(ctx, "r-billing")
	if err != nil {
		t.Fatalf("billing was not applied: %v", err)
	}
	if strings.Join(route.Access.Roles, ",") != "ROLE_BILLING" {
		t.Errorf("roles = %v, want ROLE_SUPPORT dropped", route.Access.Roles)
	}
	roles, err := f.api.st.ListRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range roles {
		names = append(names, r.Name)
	}
	if !strings.Contains(strings.Join(names, ","), "ROLE_BILLING") {
		t.Errorf("roles = %v, want ROLE_BILLING pulled and saved", names)
	}
}

func TestComposeApplyRefusesAHardMiss(t *testing.T) {
	f := composeFixture(t)
	ctx := context.Background()
	// A right side whose role has a parent; the parent left out blocks.
	c := store.Configuration{ID: "parented", Name: "parented", Document: strings.TrimSpace(`
version: 1
roles:
  - name: ROLE_TOP
  - name: ROLE_CHILD
    parent: ROLE_TOP
`) + "\n"}
	if err := f.api.st.SaveConfiguration(ctx, &c); err != nil {
		t.Fatal(err)
	}
	body := `{"left":{"source":"running"},"right":{"source":"configuration","id":"parented"},` +
		`"picks":[{"path":"role:ROLE_CHILD","from":"right"},{"path":"role:ROLE_TOP","from":"none"}]}`
	code, out := f.call(t, "POST", "/api/configurations/compose/apply", body, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "ROLE_TOP") {
		t.Fatalf("apply with a hard miss: %d %s, want 422 naming ROLE_TOP", code, out)
	}
}

func TestComposeSavePutsTheResultOnTheShelf(t *testing.T) {
	f := composeFixture(t)
	body := `{"left":{"source":"running"},"right":{"source":"configuration","id":"acme"},` +
		`"picks":[{"path":"route:r-billing","from":"right"}],"name":"acme-lite"}`
	code, out := f.call(t, "POST", "/api/configurations/compose/save", body, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("save: %d %s", code, out)
	}
	var saved store.Configuration
	if err := json.Unmarshal([]byte(out), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Name != "acme-lite" || !strings.Contains(saved.Document, "r-billing") {
		t.Errorf("saved = %+v", saved)
	}
	routes, _ := f.api.st.ListRoutes(context.Background())
	if len(routes) != 0 {
		t.Errorf("saving applies nothing, the gateway has %d routes", len(routes))
	}
}

func TestComposeRefusesAnUnknownSource(t *testing.T) {
	f := composeFixture(t)
	code, out := f.call(t, "POST", "/api/configurations/compose",
		`{"left":{"source":"running"},"right":{"source":"configuration","id":"nope"}}`, f.rootC)
	if code != http.StatusNotFound || !strings.Contains(out, "right side") {
		t.Fatalf("unknown configuration: %d %s", code, out)
	}
	code, out = f.call(t, "POST", "/api/configurations/compose",
		`{"left":{"source":"running"},"right":{"source":"file"}}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "carries no document") {
		t.Fatalf("file without document: %d %s", code, out)
	}
}

// A push goes to the git location read on the right, and nowhere else: with
// a saved configuration on the right there is nothing to push to, whatever
// remoteId says, and the refusal names the cure.
func TestComposePushGoesToTheRightSideOnly(t *testing.T) {
	f := composeFixture(t)
	body := `{"left":{"source":"running"},"right":{"source":"configuration","id":"acme"},"remoteId":"somewhere"}`
	code, out := f.call(t, "POST", "/api/configurations/compose/push", body, f.rootC)
	// 403 on the community binary (no git driver), 422 on the Enterprise one.
	if code != http.StatusUnprocessableEntity && code != http.StatusForbidden {
		t.Fatalf("push with a configuration on the right: %d %s", code, out)
	}
	if code == http.StatusUnprocessableEntity && !strings.Contains(out, "right side") {
		t.Fatalf("the refusal must say where a push goes: %s", out)
	}
}

// A push writes the location and nothing else: no row appears on the shelf,
// and a location that moved since the right side was read is refused.
func TestComposePushSavesNothingHere(t *testing.T) {
	f := setupEE(t)
	repo := confrepotest.New()
	repo.Register(t)
	repo.Put("platforms/acme/"+confrepo.DocumentName, []byte("version: 1\nroles:\n  - name: from-git\n"))
	code, out := f.call(t, "POST", "/api/config-remotes",
		`{"name":"acme","url":"https://example.invalid/acme.git","branch":"main","dir":"platforms/acme"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create location: %d %s", code, out)
	}
	id := remoteID(t, out)
	sides := `"left":{"source":"running"},"right":{"source":"remote","id":"` + id + `"}`
	code, out = f.call(t, "POST", "/api/configurations/compose", `{`+sides+`}`, f.rootC)
	if code != http.StatusOK {
		t.Fatalf("preview: %d %s", code, out)
	}
	var seen struct {
		RightRev string `json:"rightRev"`
	}
	if err := json.Unmarshal([]byte(out), &seen); err != nil || seen.RightRev == "" {
		t.Fatalf("no rightRev: %v %s", err, out)
	}

	code, out = f.call(t, "POST", "/api/configurations/compose/push",
		`{`+sides+`,"remoteId":"`+id+`","remoteRev":"another"}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "moved") || repo.Commits != 0 {
		t.Fatalf("a stale revision must be refused: %d %s, %d commits", code, out, repo.Commits)
	}

	code, out = f.call(t, "POST", "/api/configurations/compose/push",
		`{`+sides+`,"remoteId":"`+id+`","remoteRev":"`+seen.RightRev+`"}`, f.rootC)
	if code != http.StatusOK || repo.Commits != 1 {
		t.Fatalf("push: %d %s, %d commits", code, out, repo.Commits)
	}
	var pushed struct {
		Name, Rev string
		Files     int
	}
	if err := json.Unmarshal([]byte(out), &pushed); err != nil || pushed.Name != "acme" || pushed.Rev == "" || pushed.Files == 0 {
		t.Fatalf("what a push answers: %v %s", err, out)
	}
	rows, err := f.api.st.ListConfigurations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("a push left %d rows on the shelf: %+v", len(rows), rows)
	}
}

// A package reads on a side like a YAML file does, and nothing is saved for
// it: a file looked at is not a file kept.
func TestComposeReadsAPackageOnASide(t *testing.T) {
	f := composeFixture(t)
	source := `{"left":{"source":"configuration","id":"pictured"},"right":{"source":"running"},"format":"zip"}`

	// Nothing to extract: said in words, the way the other exports say it.
	code, out := f.call(t, "POST", "/api/configurations/compose/export",
		`{"left":{"source":"configuration","id":"acme"},"right":{"source":"running"},"format":"zip"}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "plain file") {
		t.Fatalf("a package of a configuration without a picture: %d %s", code, out)
	}

	c := store.Configuration{ID: "pictured", Name: "pictured", Document: `version: 1
settings:
  branding:
    appName: Acme
    logo: data:image/png;base64,iVBORw0KGgo=
roles:
  - name: ROLE_PACKED
`}
	if err := f.api.st.SaveConfiguration(context.Background(), &c); err != nil {
		t.Fatal(err)
	}
	code, out = f.call(t, "POST", "/api/configurations/compose/export", source, f.rootC)
	if code != http.StatusOK || !config.IsBundle([]byte(out)) {
		t.Fatalf("export a package: %d, %s", code, out)
	}
	before, err := f.api.st.ListConfigurations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	pkg := base64.StdEncoding.EncodeToString([]byte(out))
	code, out = f.call(t, "POST", "/api/configurations/compose",
		`{"left":{"source":"running"},"right":{"source":"file","package":"`+pkg+`"}}`, f.rootC)
	if code != http.StatusOK || !strings.Contains(out, "ROLE_PACKED") {
		t.Fatalf("a package on the right: %d %s", code, out)
	}
	after, err := f.api.st.ListConfigurations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("reading a package saved something: %d rows, was %d", len(after), len(before))
	}
	code, out = f.call(t, "POST", "/api/configurations/compose",
		`{"left":{"source":"running"},"right":{"source":"file","package":"not base64!"}}`, f.rootC)
	if code != http.StatusUnprocessableEntity || !strings.Contains(out, "base64") {
		t.Fatalf("a broken package: %d %s", code, out)
	}
}
