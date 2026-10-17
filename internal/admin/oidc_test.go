package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// oidcRoute is a route bound to a client with the oidc mechanism.
func oidcRoute(name, client string, order int) string {
	return fmt.Sprintf(`{
  "name": %q, "order": %d, "enabled": true, "upstream": "http://app.invalid",
  "predicates": [{"type": "path", "args": {"patterns": ["/app/**"]}}],
  "identity": {"mechanism": "oidc", "client": %q,
    "attributes": [{"field": "username"}],
    "claims": [{"name": "preferred_username", "field": "username"}, {"name": "tier", "value": "gold"}]}
}`, name, order, client)
}

// The whole life of a client on the Enterprise image: declared with its
// secret shown once, listed with its routes and never its hash, edited without
// losing its secret, given a new secret, and refused deletion while a route
// signs people into it - the refusal naming the route.
func TestOIDCClientsLifeCycle(t *testing.T) {
	f := setupEE(t)
	code, out := f.call(t, "POST", "/api/oidc/clients",
		`{"id": "billing", "tokenTtl": "PT10M"}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, out)
	}
	var created oidcClientIssued
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatal(err)
	}
	if !created.Client.Confidential || !strings.HasPrefix(created.Secret, store.OIDCSecretPrefix) ||
		!strings.HasPrefix(created.Secret, created.Client.SecretPrefix) {
		t.Fatalf("a new client is confidential and comes with its secret: %s", out)
	}
	if strings.Contains(out, store.HashOIDCSecret(created.Secret)) {
		t.Fatal("the secret's hash left the server")
	}
	if code, out := f.call(t, "POST", "/api/oidc/clients", `{"id": "billing"}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "already called") {
		t.Fatalf("a duplicate id: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/oidc/clients", `{"id": "spa", "confidential": false, "redirectUris": []}`, f.rootC); code != http.StatusCreated || strings.Contains(out, `"secret"`) {
		t.Fatalf("a public client comes without a secret: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/oidc/clients", `{"id": "x"}`, f.plainC); code != http.StatusForbidden {
		t.Fatalf("a plain account: %d %s", code, out)
	}

	// A route binds to it; one naming a client nobody declared is refused,
	// with the clients there are.
	if code, out := f.call(t, "PUT", "/api/routes/admins", oidcRoute("billing-admins", "billing", 1), f.rootC); code != http.StatusOK {
		t.Fatalf("route bound to billing: %d %s", code, out)
	}
	if code, out := f.call(t, "PUT", "/api/routes/ghost", oidcRoute("ghost", "nobody", 2), f.rootC); code != http.StatusUnprocessableEntity ||
		!strings.Contains(out, `OIDC client \"nobody\" is not declared`) || !strings.Contains(out, "billing, spa") {
		t.Fatalf("a route naming an undeclared client: %d %s", code, out)
	}
	if code, out := f.call(t, "PUT", "/api/routes/reserved", strings.Replace(oidcRoute("reserved", "billing", 3), `"tier"`, `"iss"`, 1), f.rootC); code != http.StatusUnprocessableEntity ||
		!strings.Contains(out, `oidc claim \"iss\" is not allowed`) {
		t.Fatalf("a reserved claim: %d %s", code, out)
	}

	code, out = f.call(t, "GET", "/api/oidc/clients", "", f.rootC)
	var list []oidcClientView
	if err := json.Unmarshal([]byte(out), &list); err != nil || code != http.StatusOK || len(list) != 2 {
		t.Fatalf("list: %d %s", code, out)
	}
	if list[0].ID != "billing" || len(list[0].Routes) != 1 || list[0].Routes[0].Name != "billing-admins" ||
		list[1].Routes == nil || len(list[1].Routes) != 0 {
		t.Fatalf("the routes bound to each client: %s", out)
	}

	// An edit sent back whole, as the console reads it, keeps the secret.
	edit := strings.Replace(out[1:strings.Index(out, `},{"id":"spa"`)+1], `"audience":""`, `"audience":"billing-api"`, 1)
	if code, out := f.call(t, "PUT", "/api/oidc/clients/billing", edit, f.rootC); code != http.StatusOK || !strings.Contains(out, `"audience":"billing-api"`) ||
		!strings.Contains(out, `"secretPrefix":"`+created.Client.SecretPrefix+`"`) {
		t.Fatalf("edit: %d %s\nsent %s", code, out, edit)
	}
	if code, out := f.call(t, "PUT", "/api/oidc/clients/billing", `{"id": "other"}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "cannot change") {
		t.Fatalf("an id changed: %d %s", code, out)
	}
	if code, out := f.call(t, "PUT", "/api/oidc/clients/nobody", `{"audience": "x"}`, f.rootC); code != http.StatusNotFound {
		t.Fatalf("edit of an unknown client: %d %s", code, out)
	}
	// A client declares no return address: one passed is refused, saying
	// where the addresses come from, on a write as on a creation.
	for _, w := range []struct{ method, path, body string }{
		{"PUT", "/api/oidc/clients/billing", `{"redirectUris": ["https://x.example.com/cb"]}`},
		{"POST", "/api/oidc/clients", `{"id": "elsewhere", "redirectUris": ["https://x.example.com/cb"]}`},
	} {
		if code, out := f.call(t, w.method, w.path, w.body, f.rootC); code != http.StatusUnprocessableEntity ||
			!strings.Contains(out, "a client declares no return address") || !strings.Contains(out, "bind the application's route") {
			t.Fatalf("%s %s with a redirect uri: %d %s", w.method, w.path, code, out)
		}
	}

	// A new secret: the old one stops working.
	code, out = f.call(t, "POST", "/api/oidc/clients/billing/secret", "", f.rootC)
	var renewed oidcClientIssued
	if err := json.Unmarshal([]byte(out), &renewed); err != nil || code != http.StatusOK || renewed.Secret == "" || renewed.Secret == created.Secret {
		t.Fatalf("new secret: %d %s", code, out)
	}
	c, err := f.api.st.GetOIDCClient(context.Background(), "billing")
	if err != nil || c.SecretMatches(created.Secret) || !c.SecretMatches(renewed.Secret) {
		t.Fatalf("the old secret still works, or the new one does not: %v", err)
	}
	if code, out := f.call(t, "POST", "/api/oidc/clients/spa/secret", "", f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "public") {
		t.Fatalf("a secret for a public client: %d %s", code, out)
	}

	// Deletion is refused while a route signs people into it.
	if code, out := f.call(t, "DELETE", "/api/oidc/clients/billing", "", f.rootC); code != http.StatusConflict || !strings.Contains(out, "billing-admins") {
		t.Fatalf("delete while used: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/routes/admins", "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("route delete: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/oidc/clients/billing", "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", code, out)
	}
	if code, _ := f.call(t, "DELETE", "/api/oidc/clients/billing", "", f.rootC); code != http.StatusNotFound {
		t.Fatalf("a second delete: %d", code)
	}

	// Every write is in the trail, and no secret is.
	events, err := f.api.st.ListAuditEvents(context.Background(), store.AuditFilter{Target: "oidc-client"})
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]bool{}
	for _, e := range events {
		actions[e.Action] = true
		raw, _ := json.Marshal(e)
		if strings.Contains(string(raw), created.Secret) || strings.Contains(string(raw), renewed.Secret) {
			t.Errorf("a secret reached the trail: %s", raw)
		}
	}
	for _, a := range []string{"oidc.client.create", "oidc.client.update", "oidc.client.secret", "oidc.client.delete"} {
		if !actions[a] {
			t.Errorf("no %s in the trail: %v", a, actions)
		}
	}
}

// A client declares no address, and the list says where the provider accepts
// one: under each enabled route's path, on the
// gateway's public address or the route's own host - and which route covers
// a whole host.
func TestAnOIDCClientShowsWhereItIsAccepted(t *testing.T) {
	f := setupEE(t)
	declareAppNames(t, f, "gw.example.com")
	code, out := f.call(t, "POST", "/api/oidc/clients", `{"id": "rabbitmq", "confidential": false}`, f.rootC)
	if code != http.StatusCreated || !strings.Contains(out, `"acceptedUnder":[]`) {
		t.Fatalf("a client: %d %s", code, out)
	}
	if code, out := f.call(t, "PUT", "/api/routes/rmq", strings.Replace(oidcRoute("rmq", "rabbitmq", 1), "/app/**", "/rabbitmq/**", 1), f.rootC); code != http.StatusOK {
		t.Fatalf("route: %d %s", code, out)
	}
	whole := strings.Replace(oidcRoute("rmq-host", "rabbitmq", 2), `[{"type": "path", "args": {"patterns": ["/app/**"]}}]`,
		`[{"type": "host", "args": {"hosts": ["rmq.example.com"]}}]`, 1)
	if code, out := f.call(t, "PUT", "/api/routes/rmq-host", whole, f.rootC); code != http.StatusOK {
		t.Fatalf("whole-host route: %d %s", code, out)
	}
	off := strings.Replace(strings.Replace(oidcRoute("rmq-off", "rabbitmq", 3), "/app/**", "/off/**", 1), `"enabled": true`, `"enabled": false`, 1)
	if code, out := f.call(t, "PUT", "/api/routes/rmq-off", off, f.rootC); code != http.StatusOK {
		t.Fatalf("disabled route: %d %s", code, out)
	}
	code, out = f.call(t, "GET", "/api/oidc/clients", "", f.rootC)
	var list []oidcClientView
	if err := json.Unmarshal([]byte(out), &list); err != nil || code != http.StatusOK || len(list) != 1 {
		t.Fatalf("list: %d %s", code, out)
	}
	got := list[0]
	if strings.Join(got.AcceptedUnder, " ") != "https://gw.example.com/rabbitmq/ https://rmq.example.com/" {
		t.Fatalf("accepted under: %q", got.AcceptedUnder)
	}
	if len(got.WholeHostRoutes) != 1 || got.WholeHostRoutes[0] != "rmq-host" {
		t.Fatalf("whole host routes: %q", got.WholeHostRoutes)
	}
	// What was read goes back as it is: the derived fields are ignored.
	if code, out := f.call(t, "PUT", "/api/oidc/clients/rabbitmq", out[1:len(out)-2], f.rootC); code != http.StatusOK {
		t.Fatalf("the view sent back: %d %s", code, out)
	}
}

// The community image reads, deletes, and refuses the rest with the edition's
// sentence - and a route cannot be switched to oidc there either.
func TestOIDCClientsAreEnterprise(t *testing.T) {
	if edition.Enterprise {
		t.Skip("Enterprise build: the community image's refusal is asserted on the community build")
	}
	f := setupBare(t)
	want := "The OIDC provider is part of the Enterprise edition"
	if code, out := f.call(t, "POST", "/api/oidc/clients", `{"id": "billing"}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, want) {
		t.Fatalf("create on the community image: %d %s", code, out)
	}
	if code, out := f.call(t, "PUT", "/api/routes/r", oidcRoute("r", "billing", 1), f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, want) {
		t.Fatalf("an oidc route on the community image: %d %s", code, out)
	}
	if code, out := f.call(t, "GET", "/api/oidc/clients", "", f.rootC); code != http.StatusOK || strings.TrimSpace(out) != "[]" {
		t.Fatalf("the list is readable, and a list: %d %s", code, out)
	}
	// What a database brought over can be removed.
	if _, err := f.api.st.CreateOIDCClient(context.Background(), store.OIDCClient{ID: "old"}); err != nil {
		t.Fatal(err)
	}
	if code, out := f.call(t, "PUT", "/api/oidc/clients/old", `{"audience": "o"}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, want) {
		t.Fatalf("edit on the community image: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/oidc/clients/old/secret", "", f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, want) {
		t.Fatalf("secret on the community image: %d %s", code, out)
	}
	if code, out := f.call(t, "DELETE", "/api/oidc/clients/old", "", f.rootC); code != http.StatusNoContent {
		t.Fatalf("delete on the community image: %d %s", code, out)
	}
}

// The agent's tools do what the console does, through the same code.
func TestOIDCClientTools(t *testing.T) {
	f := setupEE(t)
	ctx := context.WithValue(context.Background(), mcpActorKey{}, store.User{ID: "root", Username: "root", Root: true})
	out, err := f.api.toolSaveOIDCClient(ctx, json.RawMessage(`{"id": "billing", "tokenTtl": "PT10M"}`))
	if err != nil {
		t.Fatal(err)
	}
	res := out.(map[string]any)
	if res["created"] != true || !strings.HasPrefix(fmt.Sprint(res["secret"]), store.OIDCSecretPrefix) {
		t.Fatalf("a new confidential client comes with its secret: %v", res)
	}
	// What is passed is what changes: the token lifetime stays.
	out, err = f.api.toolSaveOIDCClient(ctx, json.RawMessage(`{"id": "billing", "audience": "billing-api"}`))
	if err != nil {
		t.Fatal(err)
	}
	view := out.(map[string]any)["client"].(oidcClientView)
	if view.Audience != "billing-api" || view.TokenTTL != "PT10M" || out.(map[string]any)["secret"] != nil {
		t.Fatalf("a partial edit: %+v", out)
	}
	if _, err := f.api.toolSaveOIDCClient(ctx, json.RawMessage(`{"id": "billing", "colour": "red"}`)); err == nil {
		t.Fatal("an invented field was accepted")
	}
	out, err = f.api.toolRegenerateOIDCSecret(ctx, json.RawMessage(`{"id": "billing"}`))
	if err != nil || out.(map[string]any)["secret"] == "" {
		t.Fatalf("regenerate: %v %v", out, err)
	}
	if _, err := f.api.toolRegenerateOIDCSecret(ctx, json.RawMessage(`{"id": "nobody"}`)); err == nil || !strings.Contains(err.Error(), "list_oidc_clients") {
		t.Fatalf("an unknown client: %v", err)
	}
	out, err = f.api.toolListOIDCClients(ctx, nil)
	if err != nil || len(out.(map[string]any)["clients"].([]oidcClientView)) != 1 {
		t.Fatalf("list: %v %v", out, err)
	}
	if _, err := f.api.toolDeleteOIDCClient(ctx, json.RawMessage(`{"id": "billing"}`)); err != nil {
		t.Fatal(err)
	}
}

// An application that imposes its secret (its chart generates it, its vendor
// fixed it) brings it: kept as a hash like a drawn one, never sent back, and
// held to a length a leaked export does not give away.
func TestAnOIDCClientKeepsTheApplicationsSecret(t *testing.T) {
	f := setupEE(t)
	const given = "chart-generated-0123456789"
	code, out := f.call(t, "POST", "/api/oidc/clients", `{"id": "dbui", "secret": "`+given+`"}`, f.rootC)
	if code != http.StatusCreated || strings.Contains(out, given) || strings.Contains(out, `"secret":"`) ||
		!strings.Contains(out, `"secretPrefix":"char"`) {
		t.Fatalf("a client created with the application's secret: %d %s", code, out)
	}
	c, err := f.api.st.GetOIDCClient(context.Background(), "dbui")
	if err != nil || !c.SecretMatches(given) {
		t.Fatalf("the application's secret does not sign in: %v", err)
	}

	for _, w := range []struct{ method, path, body, says string }{
		{"POST", "/api/oidc/clients", `{"id": "short", "secret": "too-short"}`, "from 16 to 256"},
		{"POST", "/api/oidc/clients", `{"id": "spaced", "secret": "a secret with spaces"}`, "a space"},
		{"POST", "/api/oidc/clients", `{"id": "spa", "confidential": false, "secret": "` + given + `"}`, "is public"},
		{"PUT", "/api/oidc/clients/dbui", `{"secret": "` + given + `"}`, "/secret"},
		{"POST", "/api/oidc/clients/dbui/secret", `{"secret": "short"}`, "from 16 to 256"},
	} {
		if code, out := f.call(t, w.method, w.path, w.body, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, w.says) {
			t.Fatalf("%s %s %s: %d %s", w.method, w.path, w.body, code, out)
		}
	}
	if code, out := f.call(t, "POST", "/api/oidc/clients/dbui/secret", `{"colour": "red"}`, f.rootC); code != http.StatusBadRequest {
		t.Fatalf("an invented field: %d %s", code, out)
	}

	// The application's secret changed: the old one stops at once.
	const next = "chart-rotated-abcdefghijklm"
	code, out = f.call(t, "POST", "/api/oidc/clients/dbui/secret", `{"secret": "`+next+`"}`, f.rootC)
	if code != http.StatusOK || strings.Contains(out, next) || strings.Contains(out, `"secret":"`) {
		t.Fatalf("the application's new secret: %d %s", code, out)
	}
	c, err = f.api.st.GetOIDCClient(context.Background(), "dbui")
	if err != nil || c.SecretMatches(given) || !c.SecretMatches(next) {
		t.Fatalf("the old secret still works, or the new one does not: %v", err)
	}
	// And drawing one is still what an empty body does.
	if code, out := f.call(t, "POST", "/api/oidc/clients/dbui/secret", "", f.rootC); code != http.StatusOK || !strings.Contains(out, `"secret":"`+store.OIDCSecretPrefix) {
		t.Fatalf("a drawn secret after a given one: %d %s", code, out)
	}

	events, err := f.api.st.ListAuditEvents(context.Background(), store.AuditFilter{Target: "oidc-client"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		raw, _ := json.Marshal(e)
		if strings.Contains(string(raw), given) || strings.Contains(string(raw), next) {
			t.Errorf("a secret reached the trail: %s", raw)
		}
	}

	// The agent passes it the same way.
	ctx := context.WithValue(context.Background(), mcpActorKey{}, store.User{ID: "root", Username: "root", Root: true})
	res, err := f.api.toolSaveOIDCClient(ctx, json.RawMessage(`{"id": "agent-made", "secret": "`+given+`"}`))
	if err != nil || res.(map[string]any)["secret"] != nil {
		t.Fatalf("an agent's client with the application's secret: %v %v", res, err)
	}
	res, err = f.api.toolRegenerateOIDCSecret(ctx, json.RawMessage(`{"id": "agent-made", "secret": "`+next+`"}`))
	if err != nil || res.(map[string]any)["secret"] != nil {
		t.Fatalf("an agent setting the application's secret: %v %v", res, err)
	}
	if c, err := f.api.st.GetOIDCClient(context.Background(), "agent-made"); err != nil || !c.SecretMatches(next) {
		t.Fatalf("the agent's secret does not sign in: %v", err)
	}
}

// The console draws a secret in the browser, in the shape the server draws
// it, and hands it over: the list shows it as a drawn one.
func TestASecretDrawnByTheConsoleKeepsTheDrawnPrefix(t *testing.T) {
	f := setupEE(t)
	drawn, _, _, err := store.NewOIDCSecret()
	if err != nil {
		t.Fatal(err)
	}
	code, out := f.call(t, "POST", "/api/oidc/clients", `{"id": "drawn", "secret": "`+drawn+`"}`, f.rootC)
	if code != http.StatusCreated || strings.Contains(out, drawn) || !strings.Contains(out, `"secretPrefix":"`+drawn[:12]+`"`) {
		t.Fatalf("a console-drawn secret: %d %s", code, out)
	}
}

// A trial names what it is missing before it asks the provider anything.
func TestATrialSaysWhoItCannotTry(t *testing.T) {
	f := setupEE(t)
	if code, out := f.call(t, "POST", "/api/oidc/clients", `{"id": "app"}`, f.rootC); code != http.StatusCreated {
		t.Fatalf("client: %d %s", code, out)
	}
	for _, c := range []struct{ path, body, says string }{
		{"/api/oidc/clients/app/try", `{}`, "whom to try it for"},
		{"/api/oidc/clients/app/try", `{"user": "nobody"}`, `no account \"nobody\"`},
		{"/api/oidc/clients/app/try", `{"user": "bob", "tenant": "nowhere"}`, `no organisation \"nowhere\"`},
	} {
		if code, out := f.call(t, "POST", c.path, c.body, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, c.says) {
			t.Fatalf("%s: %d %s", c.body, code, out)
		}
	}
	if code, _ := f.call(t, "POST", "/api/oidc/clients/ghost/try", `{"user": "bob"}`, f.rootC); code != http.StatusNotFound {
		t.Fatalf("an unknown client: %d", code)
	}
	if code, _ := f.call(t, "POST", "/api/oidc/clients/app/try", `{"user": "bob"}`, f.plainC); code != http.StatusForbidden {
		t.Fatalf("a plain account: %d", code)
	}
}
