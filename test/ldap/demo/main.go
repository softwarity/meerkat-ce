// Command demo wires the seeded directory (test/ldap) into a RUNNING gateway,
// so that exploring an LDAP sign-in is a click rather than five fields typed
// from memory.
//
// The bench had everything except the last metre: two directories answering,
// the integration tests passing against them - and then trying it by hand
// meant retyping a URL, a search base, a service account, a filter and a group
// base into the console, and inventing roles, groups and rules to have
// anything to look at.
//
// It registers the authority, asks the gateway to CHECK it, then lays down the
// chain a directory sign-in actually travels: roles, the tenant's groups that
// carry them, and the group rules (RBAC-10) that turn what the directory SAYS
// into what someone may do here. Without those rules every arrival waits in
// /account-pending, which is correct - an authority proves who you are, never
// what you may do - but it makes the interesting half invisible.
//
// Idempotent: everything is looked up by name and created only when missing,
// so running it twice changes nothing.
//
//	make ldap-demo
//	MEERKAT_ADMIN_URL=http://localhost:9092 MEERKAT_TENANT=acme make ldap-demo
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
)

// The authority, spelt out rather than left to the dialect defaults: this file
// doubles as the answer to "what do I put in those fields", which is the
// question the console screen actually raises.
const providerID = "demo-directory"

func providerConfig(ldapURL string) map[string]any {
	return map[string]any{
		"url":          ldapURL,
		"dialect":      "directory",
		"baseDn":       "dc=example,dc=com",
		"bindDn":       "cn=admin,dc=example,dc=com",
		"bindPassword": "adminpassword",
		"userFilter":   "(&(objectClass=inetOrgPerson)(uid=%s))",
		"usernameAttr": "uid",
		"emailAttr":    "mail",
		"nameAttr":     "cn",
		"groupBaseDn":  "ou=groups,dc=example,dc=com",
		"groupFilter":  "(uniqueMember=%s)",
		"groupIdAttr":  "cn",
		"nestedGroups": true,
	}
}

// What the demo lays down. Four roles, four groups carrying them, and four
// rules - deliberately NOT one per directory group: "developer" and "operator"
// are left unmapped so the screen shows what an unmapped group does, which is
// nothing at all. That is the case an installation meets far more often than
// the mapped one.
var (
	demoRoles = []struct{ Name, Description string }{
		{"demo-front", "Front-end work"},
		{"demo-back", "Back-end work"},
		{"demo-ops", "On-call and deployments"},
		{"demo-read", "Read only"},
	}
	demoGroups = []struct {
		Name, Description, Role string
	}{
		{"Front", "Granted by the directory group frontend", "demo-front"},
		{"Back", "Granted by the directory group backend", "demo-back"},
		{"Ops", "Granted by the directory group devops", "demo-ops"},
		{"Agents Brest", "Granted by the directory group Brest Agents", "demo-read"},
	}
	// External is the authority's own name, VERBATIM - including the space and
	// the capitals of "Brest Agents". A rule written from memory in another
	// case matches nothing, silently, which is why the console has a screen
	// listing what the authorities were actually heard to say.
	demoRules = []struct{ External, Group string }{
		{"frontend", "Front"},
		{"backend", "Back"},
		{"devops", "Ops"},
		{"Brest Agents", "Agents Brest"},
	}
)

type client struct {
	base string
	http *http.Client
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ldap-demo:", err)
		os.Exit(1)
	}
}

func run() error {
	admin := env("MEERKAT_ADMIN_URL", "http://localhost:9092")
	user := env("MEERKAT_ADMIN_USER", "admin")
	// The password DEV.md seeds a development store with. Not a secret: it is
	// printed in that file, and this program only ever talks to a gateway one
	// is already running on one's own machine.
	pass := env("MEERKAT_ADMIN_PASSWORD", "test1234")
	ldapURL := env("MEERKAT_LDAP_URL", "ldap://localhost:3389")

	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	c := &client{base: strings.TrimSuffix(admin, "/"), http: &http.Client{Jar: jar}}

	if err := c.signIn(user, pass); err != nil {
		return err
	}

	// ── the authority ─────────────────────────────────────────────────────
	provider := map[string]any{
		"id": providerID, "kind": "ldap", "name": "Demo directory",
		"enabled": true, "order": 10, "autoCreate": "yes",
		"config": providerConfig(ldapURL),
	}
	if err := c.do("PUT", "/api/auth-providers/"+providerID, provider, nil); err != nil {
		return fmt.Errorf("registering the authority: %w", err)
	}
	verdict := "the gateway reached it"
	if err := c.do("POST", "/api/auth-providers/"+providerID+"/check", nil, nil); err != nil {
		verdict = "the gateway could NOT use it: " + err.Error()
	}

	// ── the chain behind it ───────────────────────────────────────────────
	tenant, err := c.pickTenant(env("MEERKAT_TENANT", ""))
	if err != nil {
		return err
	}
	roles, err := c.ensureRoles()
	if err != nil {
		return err
	}
	groups, err := c.ensureGroups(tenant, roles)
	if err != nil {
		return err
	}
	if err := c.ensureRules(tenant, groups); err != nil {
		return err
	}

	report(ldapURL, admin, tenant, verdict)
	return nil
}

// signIn posts the ordinary sign-in form of the admin plane and then proves the
// session can actually administer: a 200 on /login means very little, the page
// answers with a form to anyone.
func (c *client) signIn(user, pass string) error {
	form := url.Values{"username": {user}, "password": {pass}}
	resp, err := c.http.PostForm(c.base+"/login", form)
	if err != nil {
		return fmt.Errorf("no gateway answering on %s (start one, see DEV.md): %w", c.base, err)
	}
	_ = resp.Body.Close()
	if err := c.do("GET", "/api/auth-providers", nil, nil); err != nil {
		return fmt.Errorf("signed in as %s but the authorities are refused - is that account root? (%w)", user, err)
	}
	return nil
}

func (c *client) pickTenant(want string) (string, error) {
	var tenants []struct{ ID, Name string }
	if err := c.do("GET", "/api/tenants", nil, &tenants); err != nil {
		return "", fmt.Errorf("listing organisations: %w", err)
	}
	if len(tenants) == 0 {
		return "", fmt.Errorf("this gateway has no organisation to put anyone in")
	}
	for _, t := range tenants {
		if want != "" && (t.ID == want || t.Name == want) {
			return t.ID, nil
		}
		if want == "" && t.ID == "default" {
			return t.ID, nil
		}
	}
	if want != "" {
		return "", fmt.Errorf("no organisation named %q here", want)
	}
	return tenants[0].ID, nil
}

// ensureRoles returns name -> id, creating what is missing. Roles are global:
// the catalogue belongs to the application, not to an organisation.
func (c *client) ensureRoles() (map[string]string, error) {
	var existing []struct{ ID, Name string }
	if err := c.do("GET", "/api/roles", nil, &existing); err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}
	byName := map[string]string{}
	for _, r := range existing {
		byName[r.Name] = r.ID
	}
	for _, want := range demoRoles {
		if _, ok := byName[want.Name]; ok {
			continue
		}
		var created struct{ ID, Name string }
		body := map[string]any{"name": want.Name, "description": want.Description, "tags": []string{"demo"}}
		if err := c.do("POST", "/api/roles", body, &created); err != nil {
			return nil, fmt.Errorf("creating role %s: %w", want.Name, err)
		}
		byName[created.Name] = created.ID
	}
	return byName, nil
}

func (c *client) ensureGroups(tenant string, roles map[string]string) (map[string]string, error) {
	path := "/api/tenants/" + tenant + "/groups"
	var existing []struct{ ID, Name string }
	if err := c.do("GET", path, nil, &existing); err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}
	byName := map[string]string{}
	for _, g := range existing {
		byName[g.Name] = g.ID
	}
	for _, want := range demoGroups {
		if _, ok := byName[want.Name]; ok {
			continue
		}
		var created struct{ ID, Name string }
		body := map[string]any{
			"name": want.Name, "description": want.Description,
			"roleIds": []string{roles[want.Role]},
		}
		if err := c.do("POST", path, body, &created); err != nil {
			return nil, fmt.Errorf("creating group %s: %w", want.Name, err)
		}
		byName[created.Name] = created.ID
	}
	return byName, nil
}

func (c *client) ensureRules(tenant string, groups map[string]string) error {
	path := "/api/tenants/" + tenant + "/group-rules"
	var existing []struct{ External, ProviderID string }
	if err := c.do("GET", path, nil, &existing); err != nil {
		return fmt.Errorf("listing group rules: %w", err)
	}
	seen := map[string]bool{}
	for _, r := range existing {
		seen[r.ProviderID+"\x00"+r.External] = true
	}
	for _, want := range demoRules {
		if seen[providerID+"\x00"+want.External] {
			continue
		}
		body := map[string]any{
			"providerId": providerID,
			"external":   want.External,
			"groupId":    groups[want.Group],
		}
		if err := c.do("POST", path, body, nil); err != nil {
			return fmt.Errorf("creating the rule for %q: %w", want.External, err)
		}
	}
	return nil
}

// do sends one JSON request. A refusal carries the gateway's own words: this
// program should never paraphrase a server that already explained itself.
func (c *client) do(method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		var e struct{ Error string }
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			msg = e.Error
		}
		return fmt.Errorf("%s %s: %s (%s)", method, path, resp.Status, msg)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func report(ldapURL, admin, tenant, verdict string) {
	fmt.Printf(`
  authority "Demo directory" registered on %s
  %s - %s
  organisation: %s

  Front / Back / Ops / Agents Brest carry a role each, and four rules map the
  directory onto them. "developer" and "operator" are deliberately NOT mapped:
  they are collected, they grant nothing, and that is what most names do.

  sign in on the DATA plane - password "password" for everyone but dan

    alice    frontend                      -> Front           demo-front
    janedoe  devops frontend operator      -> Ops, Front      demo-ops, demo-front
    johndoe  frontend backend operator     -> Front, Back     demo-front, demo-back
    evec     frontend                      -> Front           login is not the mail, accented name
    carla    backend (+developer, nested)  -> Back            developer has no rule: collected, ignored
    frank    Brest Agents                  -> Agents Brest    a name with a space, mapped verbatim
    bob      no group                      -> nothing         waits in /account-pending
    gina     operator                      -> nothing         operator has no rule; also lives in ou=partners
    dan      -                             -> refused         known to the directory, no way in

  the search base is the whole tree, so gina is visible at all; narrow it to
  ou=users,dc=example,dc=com and she stops existing.
`, admin, ldapURL, verdict, tenant)
}
