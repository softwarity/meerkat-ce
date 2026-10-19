package admin

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// The rule that keeps the agent from knowing less than the console (MCP-04,
// MCP-09).
//
// Every endpoint of the control plane is in an agent's reach through
// call_admin_api, unless agentClosedSections or agentClosedEndpoints
// (mcp_api.go) close it with a reason. That was not always so: the tools were
// written section by section, and a section counted as covered while forty of
// its actions had no tool. So the unit here is the endpoint: each one either
// resolves for an agent's call, or is closed by name.
//
// The dedicated tools stay, because an agent tool answers a question ("who may
// reach this route") better than an endpoint does. agentCovers says which
// sections have them; a section without one is still open, through the
// generic tools.

// agentCovers maps a control-plane section to the tools that speak for it.
var agentCovers = map[string][]string{
	"routes":   {"list_routes", "get_route", "test_routing", "save_route", "delete_route", "list_operations", "call_route"},
	"catalog":  {"list_route_bricks"},
	"services": {"list_services"},
	"metrics":  {"read_traffic"},
	"logs":     {"read_logs"},
	// The granting chain (mcp_grants.go). A group in an organisation names
	// roles, a person is a member of that organisation, that member holds
	// groups - three links, none of which was reachable, so an application
	// could arrive with its roles and nobody could be given them. "Who gets
	// what is a decision" confused deciding with typing it in four screens.
	"users":    {"list_users"},
	"tenants":  {"list_tenants", "list_groups", "save_group", "delete_group", "list_members", "save_member"},
	"audit":    {"read_audit"},
	"edition":  {"describe_gateway"},
	"config":   {"export_configuration", "import_configuration"},
	"branding": {"get_branding", "save_branding"},
	"themes":   {"list_themes"},
	// The TLS settings live under this section too - the redirect, HSTS, the
	// names reached through a proxy in front - and no tool writes them: like
	// the certificates beside them, which names this gateway vouches for in
	// a mailed link is decided on the screen that holds the certificates.
	"settings": {"get_settings", "save_portal"},
	// Correcting ONE wording is a judgement about a sentence, made against the
	// screen it appears on - which is what the console's editor is for, and why
	// this was out of an agent's reach at first. A WHOLE LANGUAGE is the
	// opposite kind of work: nobody writes two hundred and seventy strings by
	// hand, and the editor is then where a person who speaks it reviews what
	// was written. So the tools are coarse on purpose - a language at a time,
	// never a string.
	"locales": {"list_languages", "read_language", "write_language"},
	"configurations": {"list_configurations", "save_configuration", "activate_configuration",
		"pull_configuration", "push_configuration"},
	// The git locations themselves (CFG-07): an agent lists them, because
	// pulling needs to name one, and creates or changes one, root's like the
	// screen. The credential stays a vault reference, typed by a person: an
	// agent names ${token}, it never holds it.
	"config-remotes": {"list_git_locations", "save_git_location"},
	"schedules":      {"list_schedules", "list_schedule_runs", "pause_schedule", "run_schedule"},
	// The catalogue was out of reach on the grounds that renaming a role
	// silently changes who reaches what - every access rule names roles by
	// name. The observation was right and the answer was wrong: the console
	// renames with the same consequence, so what was missing was not a closed
	// door but a rename that FOLLOWS its references (roleref.go) and a deletion
	// that says what still names the role. Both paths share that code now.
	"roles": {"list_roles", "list_role_references", "save_role", "delete_role"},
	// The applications signing their people in against this gateway: the
	// other half of a route bound with the oidc mechanism, which an agent
	// writes with save_route. The secret is shown once, in the answer of the
	// call that drew it, and is never readable - by an agent or a person.
	"oidc": {"list_oidc_clients", "save_oidc_client", "regenerate_oidc_client_secret", "delete_oidc_client", "try_oidc_sign_in"},
	// The public keys, the issuer and a draft's preview (mcp_identity.go).
	// The private halves never leave the server - not to a person either.
	"identity": {"get_signing_keys", "set_issuer", "renew_signing_keys", "preview_identity"},
}

// patternRecorder collects what the API registers. See admin.Mux for why the
// registration takes an interface at all.
type patternRecorder struct{ patterns []string }

func (p *patternRecorder) Handle(pattern string, _ http.Handler) {
	p.patterns = append(p.patterns, pattern)
}

func (p *patternRecorder) HandleFunc(pattern string, _ func(http.ResponseWriter, *http.Request)) {
	p.patterns = append(p.patterns, pattern)
}

// controlPlaneSurface is every pattern the API mounts.
func controlPlaneSurface() []string {
	rec := &patternRecorder{}
	(&API{}).Register(rec)
	return rec.patterns
}

func TestEverySectionIsEitherCoveredOrKnowinglyNot(t *testing.T) {
	seen := map[string]bool{}
	patterns := map[string]bool{}
	for _, pattern := range controlPlaneSurface() {
		seen[section(pattern)] = true
		patterns[pattern] = true
	}
	tools := map[string]bool{}
	for _, tool := range (&API{}).tools() {
		tools[tool.Name] = true
	}
	// A tool named as covering a section must exist, a section named must
	// exist, and so must what is closed: every half rots in silence otherwise.
	for s, named := range agentCovers {
		if !seen[s] {
			t.Errorf("agentCovers names section %q, which the control plane no longer serves", s)
		}
		if _, closed := agentClosedSections[s]; closed {
			t.Errorf("section %q has tools and is closed to agents: decide", s)
		}
		for _, name := range named {
			if !tools[name] {
				t.Errorf("section %q claims the tool %q, which no longer exists", s, name)
			}
		}
	}
	for s, why := range agentClosedSections {
		if !seen[s] {
			t.Errorf("agentClosedSections names section %q, which the control plane no longer serves", s)
		}
		if strings.TrimSpace(why) == "" {
			t.Errorf("section %q is closed with no reason", s)
		}
	}
	for p, why := range agentClosedEndpoints {
		if !patterns[p] {
			t.Errorf("agentClosedEndpoints names %q, which the control plane no longer serves", p)
		}
		if strings.TrimSpace(why) == "" {
			t.Errorf("%q is closed with no reason", p)
		}
	}
}

// Every endpoint resolves for an agent's call to the endpoint it is, or is
// closed by name: nothing is out of reach by accident.
func TestEveryEndpointIsInAnAgentsReachOrClosedByName(t *testing.T) {
	mux := http.NewServeMux()
	a := &API{}
	a.Register(mux)
	if a.selfMux != mux || len(a.surface) == 0 {
		t.Fatal("Register did not keep what it mounted")
	}
	fill := strings.NewReplacer("{name...}", "a/b", "...}", "}")
	open, closed := 0, 0
	for _, pattern := range a.surface {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok || !strings.HasPrefix(path, "/api/") {
			continue
		}
		if agentClosed(pattern) != "" {
			closed++
			continue
		}
		concrete := fill.Replace(path)
		for strings.Contains(concrete, "{") {
			i := strings.Index(concrete, "{")
			j := strings.Index(concrete[i:], "}")
			concrete = concrete[:i] + "x" + concrete[i+j+1:]
		}
		req := httptest.NewRequest(method, concrete, nil)
		if _, got := mux.Handler(req); got != pattern {
			t.Errorf("%s: an agent's %s %s reaches %q", pattern, method, concrete, got)
		}
		open++
	}
	if open < 150 || closed == 0 {
		t.Fatalf("open %d, closed %d: the surface is not what it was", open, closed)
	}
}

// The perimeter's classification has the same failure mode: a POST that reads
// gets refused to a read-only token forever, and nobody hears about it. Every
// non-GET endpoint therefore states which one it is.
func TestEveryWriteVerbIsClassified(t *testing.T) {
	// writesSomething is the other half of admin.readsNothing. Together they
	// must cover every non-GET endpoint exactly once.
	writesSomething := map[string]bool{
		"POST /api/admin-tokens": true, "POST /api/admin-tokens/{id}/toggle": true,
		"PUT /api/admin-tokens/{id}": true, "POST /api/admin-tokens/{id}/renew": true,
		"DELETE /api/admin-tokens/{id}": true, "POST /api/apidocs/token": true,
		"POST /api/certificates/import": true, "POST /api/certificates/self-signed": true,
		"POST /api/certificates/signing-request": true, "POST /api/certificates/{id}/adopt": true,
		"DELETE /api/certificates/{id}": true, "PUT /api/certificates/{id}/placement": true,
		"POST /api/certificates/acme": true,
		// A certificate taken into the pool by the paths of mounted files.
		"POST /api/certificates/file": true,
		// The internal plane's certificate asked of cert-manager: a
		// Certificate created in the cluster.
		"POST /api/certificates/cert-manager": true,
		"POST /api/acme/authorities":          true, "PUT /api/acme/authorities/{id}": true,
		"POST /api/trusted-authorities": true, "PUT /api/trusted-authorities/{id}": true,
		"DELETE /api/trusted-authorities/{id}": true,
		// An authority the platform publishes, taken: an entry created, and
		// for a cert-manager issuer a throwaway Certificate asked and deleted.
		"POST /api/trusted-authorities/platform": true,
		"DELETE /api/acme/authorities/{id}":      true, "POST /api/certificates/{id}/retry": true,
		"POST /api/config/import": true, "POST /api/config/history/{id}/restore": true,
		"POST /api/config/history/{id}/save": true,
		// The scheduled calls: pausing one stops it firing, running one brings
		// its turn forward, and deleting one is deleting one. None of them
		// computes an answer.
		"POST /api/schedules/{id}/pause": true, "POST /api/schedules/{id}/resume": true,
		"POST /api/schedules/{id}/run": true, "DELETE /api/schedules/{id}": true,
		// Written by the service that owns them: the schedule itself, and the
		// progress of the run it is working on.
		"POST /api/schedules": true, "PUT /api/schedules/{id}": true,
		"PATCH /api/schedules/{id}/run": true,
		// How long a finished delayed action is kept before the sweep: a
		// number stored, like the trail's own retention beside it.
		"PUT /api/settings/schedules": true,
		"POST /api/configurations":    true, "POST /api/configurations/import": true,
		// A pull SHELVES a document and applies nothing - but it writes the
		// row it shelved it in, which is what this list is about.
		"POST /api/configurations/pull": true, "POST /api/configurations/{id}/pull": true,
		"POST /api/configurations/{id}/push": true, "PUT /api/configurations/{id}/remote": true,
		"POST /api/config-remotes": true, "PUT /api/config-remotes/{id}": true,
		"DELETE /api/config-remotes/{id}":        true,
		"POST /api/configurations/{id}/activate": true, "POST /api/configurations/{id}/capture": true,
		"POST /api/configurations/{id}/duplicate": true, "PUT /api/configurations/{id}": true,
		"PUT /api/configurations/{id}/document": true, "DELETE /api/configurations/{id}": true,
		"PUT /api/auth-providers/{id}": true, "DELETE /api/auth-providers/{id}": true,
		"POST /api/identity/signing-keys/renew": true,
		// A stored issuer, and the router reloads on it: a write.
		"PUT /api/identity/issuer": true,
		// The OIDC clients: declared, changed, a secret drawn, deleted.
		"POST /api/oidc/clients": true, "PUT /api/oidc/clients/{id}": true,
		"POST /api/oidc/clients/{id}/secret": true, "DELETE /api/oidc/clients/{id}": true,
		"POST /api/plug/sessions/{id}/close": true,
		"POST /api/issues/{id}/comments":     true, "PUT /api/issues/{id}/status": true,
		"DELETE /api/issues/{id}": true,
		"POST /api/roles":         true, "PUT /api/roles/{name}": true, "DELETE /api/roles/{name}": true,
		"POST /api/routes/reorder": true, "PUT /api/routes/{id}": true,
		"DELETE /api/routes/{id}": true, "PUT /api/routes/{id}/security": true, "PUT /api/routes/{id}/audit": true,
		"PUT /api/routes/{id}/role-map": true,
		"PUT /api/routes/{id}/spec":     true, "DELETE /api/routes/{id}/spec": true,
		"PUT /api/routes/{id}/files/{name...}": true, "PATCH /api/routes/{id}/files/{name...}": true, "DELETE /api/routes/{id}/files/{name...}": true,
		"PUT /api/settings": true, "PUT /api/settings/agent": true, "PUT /api/settings/issues": true,
		"PUT /api/settings/telemetry": true,
		"PUT /api/settings/plug":      true,
		"PUT /api/settings/audit":     true,
		// Nothing is stored, but every write on every node stops: a write.
		"PUT /api/backup/pause": true,
		// Writes a whole other database: never this one, but a write.
		"POST /api/backup/copy": true,
		// Forgets the copy a migration was in the middle of: a write.
		"DELETE /api/backup/last-copy": true,
		"DELETE /api/sessions/{id}":    true,
		"DELETE /api/data-tokens/{id}": true,
		"PUT /api/settings/proxy":      true,
		// The level is a write even though nothing is stored: it changes what
		// every node writes from now on.
		"PUT /api/logs/level":           true,
		"PUT /api/settings/maintenance": true,
		"PUT /api/settings/mail-relay":  true, "PUT /api/settings/tenancy": true,
		"PUT /api/license":      true,
		"PUT /api/settings/tls": true,
		"POST /api/tenants":     true, "PUT /api/tenants/{id}": true, "DELETE /api/tenants/{id}": true,
		"POST /api/tenants/{id}/group-rules": true, "PUT /api/tenants/{id}/group-rules/{ruleId}": true,
		"DELETE /api/tenants/{id}/group-rules/{ruleId}": true,
		"POST /api/tenants/{id}/groups":                 true, "PUT /api/tenants/{id}/groups/{groupId}": true,
		"DELETE /api/tenants/{id}/groups/{groupId}":              true,
		"PUT /api/tenants/{id}/members/{userId}":                 true,
		"PUT /api/tenants/{id}/members/{userId}/groups":          true,
		"POST /api/tenants/{id}/members/{userId}/reset-password": true,
		"DELETE /api/tenants/{id}/members/{userId}":              true,
		"POST /api/tenants/{id}/owner":                           true,
		"PUT /api/locales/{code}":                                true, "DELETE /api/locales/{code}": true,
		"POST /api/themes": true, "PUT /api/themes/{id}": true,
		"POST /api/themes/{id}/activate": true, "DELETE /api/themes/{id}": true,
		"PUT /api/branding": true, "PUT /api/model/user-fields": true,
		"POST /api/users": true, "PUT /api/users/{id}": true, "DELETE /api/users/{id}": true,
		"POST /api/users/must-change-password":      true,
		"POST /api/users/{id}/must-change-password": true,
		"POST /api/users/{id}/reset-password":       true,
		"POST /api/vault/export":                    true, "POST /api/vault/import": true,
		"POST /api/vault/stash": true, "PUT /api/vault/{scope}/{name}": true,
		"DELETE /api/vault/{scope}/{name}": true,
		// The names a door is reached by through a proxy in front: stored,
		// and what a mailed link may point at from then on.
		"PUT /api/settings/tls/doors/{plane}": true,
		// Which of a plane's doors listen on the network (SSL-15).
		"PUT /api/settings/tls/ports/{plane}": true,
	}

	registered := map[string]bool{}
	var undecided []string
	for _, pattern := range controlPlaneSurface() {
		registered[pattern] = true
		if strings.HasPrefix(pattern, "GET ") {
			continue
		}
		read, write := readsNothing[pattern], writesSomething[pattern]
		if read && write {
			t.Errorf("%s is declared both a read and a write", pattern)
		}
		if !read && !write {
			undecided = append(undecided, pattern)
		}
	}
	sort.Strings(undecided)
	if len(undecided) > 0 {
		t.Errorf(`these endpoints change something, or they do not, and nobody said which: %s

Put each one in admin.readsNothing (it computes an answer and stores nothing)
or in writesSomething here. The perimeter of a read-only token is decided by
that list, not by the verb: half the testers in this API are POSTs.`,
			strings.Join(undecided, "\n  "))
	}
	// An entry pointing at nothing is worse than no entry: it reads as a
	// decision that is no longer enforced anywhere.
	for pattern := range readsNothing {
		if !registered[pattern] {
			t.Errorf("readsNothing lists %q, which is not registered", pattern)
		}
	}
	for pattern := range writesSomething {
		if !registered[pattern] {
			t.Errorf("writesSomething lists %q, which is not registered", pattern)
		}
	}
}

// The tool names are a public interface: people put them in prompts and in
// saved workflows, so a rename has to show up in review as a diff, not as a
// support question.
func TestTheToolSetIsWhatWeThinkItIs(t *testing.T) {
	want := []string{
		"activate_configuration",
		"call_admin_api",
		"call_route",
		"delete_group",
		"delete_oidc_client",
		"delete_role",
		"delete_route",
		"describe_gateway",
		"export_configuration",
		"get_branding",
		"get_route",
		"get_settings",
		"get_signing_keys",
		"import_configuration",
		"list_admin_api",
		"list_configurations",
		"list_git_locations",
		"list_groups",
		"list_languages",
		"list_members",
		"list_oidc_clients",
		"list_operations",
		"list_role_references",
		"list_roles",
		"list_route_bricks",
		"list_routes",
		"list_schedule_runs",
		"list_schedules",
		"list_services",
		"list_tenants",
		"list_themes",
		"list_users",
		"pause_schedule",
		"preview_identity",
		"pull_configuration",
		"push_configuration",
		"read_admin_api",
		"read_audit",
		"read_language",
		"read_logs",
		"read_traffic",
		"regenerate_oidc_client_secret",
		"renew_signing_keys",
		"run_schedule",
		"save_branding",
		"save_configuration",
		"save_git_location",
		"save_group",
		"save_member",
		"save_oidc_client",
		"save_portal",
		"save_role",
		"save_route",
		"save_user",
		"set_issuer",
		"test_routing",
		"try_oidc_sign_in",
		"write_language",
	}
	var got []string
	for _, tool := range (&API{}).tools() {
		got = append(got, tool.Name)
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the tool set changed:\n  have %v\n  want %v\n\nIf this is deliberate, update the list - and remember a renamed tool breaks the prompts people wrote around it.", got, want)
	}
}
