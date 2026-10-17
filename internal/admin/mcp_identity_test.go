package admin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// An agent reads the public keys and the issuer, chooses the issuer among the
// public addresses, renews the keys and previews a draft identity - what the
// OIDC / JWT drawer and the route editor's preview do.
func TestTheIdentityTools(t *testing.T) {
	f := setupBare(t)
	declareAppNames(t, f, "a.example.com", "b.example.com")
	ctx := context.WithValue(context.Background(), mcpActorKey{}, store.User{ID: "root", Username: "root", Root: true})
	tool := func(name, args string) (any, error) {
		for _, tl := range f.api.identityTools() {
			if tl.Name == name {
				return tl.Call(ctx, json.RawMessage(args))
			}
		}
		t.Fatalf("no tool %s", name)
		return nil, nil
	}

	out, err := tool("get_signing_keys", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	view := out.(signingKeysView)
	if view.Issuer != "https://a.example.com" || len(view.Issuers) != 2 || len(view.Keys) != 3 {
		t.Fatalf("keys and issuer: %+v", view)
	}
	for _, k := range view.Keys {
		if strings.Contains(k.PublicPem, "PRIVATE") {
			t.Fatalf("a private key left the server: %s", k.Algorithm)
		}
	}

	if _, err := tool("set_issuer", `{"issuer": "https://elsewhere.example.com"}`); err == nil || !strings.Contains(err.Error(), "b.example.com") {
		t.Fatalf("an issuer that is no public address: %v", err)
	}
	out, err = tool("set_issuer", `{"issuer": "https://b.example.com"}`)
	if err != nil || out.(signingKeysView).Issuer != "https://b.example.com" {
		t.Fatalf("set issuer: %v %v", out, err)
	}

	out, err = tool("renew_signing_keys", `{}`)
	if err != nil || out.(signingKeysView).Keys[0].Kid == view.Keys[0].Kid {
		t.Fatalf("renew: the kid did not change: %v", err)
	}

	out, err = tool("preview_identity", `{"routeName": "billing", "identity": {"mechanism": "jwt", "attributes": [{"field": "username"}]}}`)
	if err != nil || out.(identityPreviewResponse).Claims["iss"] != "https://b.example.com" {
		t.Fatalf("preview: %v %v", out, err)
	}
	if _, err := tool("preview_identity", `{"identity": {"mechanism": "oidc", "client": "x"}}`); err == nil || !strings.Contains(err.Error(), "try_oidc_sign_in") {
		t.Fatalf("an oidc preview points at the trial: %v", err)
	}

	events, err := f.api.st.ListAuditEvents(context.Background(), store.AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range events {
		seen[e.Action] = true
	}
	if !seen["identity.issuer"] || !seen["identity.signing_renew"] {
		t.Fatalf("the agent's changes are not in the trail: %v", seen)
	}
}

// A saved configuration is brought up to date from the running state, by
// name: what the console's Capture does, and what a push needs first.
func TestSavingAConfigurationAgainReplacesIt(t *testing.T) {
	f := setupBare(t)
	ctx := context.WithValue(context.Background(), mcpActorKey{}, store.User{ID: "root", Username: "root", Root: true})
	if _, err := f.api.toolSaveConfiguration(ctx, json.RawMessage(`{"name": "base"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.api.toolSaveConfiguration(ctx, json.RawMessage(`{"name": "base"}`)); err == nil || !strings.Contains(err.Error(), "replace: true") {
		t.Fatalf("a taken name without replace: %v", err)
	}
	if err := f.api.st.SaveRole(context.Background(), store.Role{Name: "LATE_ROLE"}); err != nil {
		t.Fatal(err)
	}
	out, err := f.api.toolSaveConfiguration(ctx, json.RawMessage(`{"name": "base", "replace": true}`))
	if err != nil || out.(map[string]any)["replaced"] != true {
		t.Fatalf("replace: %v %v", out, err)
	}
	named, err := f.api.configurationByName(context.Background(), "base")
	if err != nil {
		t.Fatal(err)
	}
	c, err := f.api.st.GetConfiguration(context.Background(), named.ID)
	if err != nil || !strings.Contains(c.Document, "LATE_ROLE") || !c.Active {
		t.Fatalf("the configuration was not refreshed: %v", err)
	}
	list, _ := f.api.st.ListConfigurations(context.Background())
	if len(list) != 1 {
		t.Fatalf("replace made another configuration: %d", len(list))
	}
}
