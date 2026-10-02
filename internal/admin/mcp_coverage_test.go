package admin

import (
	"net/http"
	"sort"
	"strings"
	"testing"
)

// The rule that keeps the agent's tools from rotting (MCP-04).
//
// A surface like this decays one feature at a time: someone adds a section to
// the console, nobody adds the tool, and six months later the agent knows a
// gateway that no longer exists. So the control plane's sections are the unit,
// and every one of them is either covered by tools or deliberately not - with
// the reason written down. A NEW SECTION fails this test until somebody
// decides, which is the whole point.
//
// The section, not the endpoint, is the unit on purpose: an agent tool answers
// a question ("who may reach this route"), and a question spans half a dozen
// endpoints. Requiring one tool per endpoint would give sixty tools and an
// agent recomposing REST calls, which is exactly what MCP-04 refuses.

// agentCovers maps a control-plane section to the tools that speak for it.
var agentCovers = map[string][]string{
	"routes":   {"list_routes", "get_route", "test_routing", "save_route", "delete_route"},
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
	// The git locations themselves (CFG-07): an agent LISTS them, because
	// pulling needs to name one, and that is where its business ends. Creating
	// one hands over a repository URL and a credential reference - a decision
	// about which repository this installation answers to, taken once, on a
	// screen, by the person who owns both.
	"config-remotes": {"list_git_locations"},
	"schedules":      {"list_schedules", "list_schedule_runs", "pause_schedule", "run_schedule"},
	// The catalogue was out of reach on the grounds that renaming a role
	// silently changes who reaches what - every access rule names roles by
	// name. The observation was right and the answer was wrong: the console
	// renames with the same consequence, so what was missing was not a closed
	// door but a rename that FOLLOWS its references (roleref.go) and a deletion
	// that says what still names the role. Both paths share that code now.
	"roles": {"list_roles", "list_role_references", "save_role", "delete_role"},
}

// agentIgnores says why a section is out of an agent's reach, and each reason
// has to be a reason - "not yet" is one, as long as it names what would change
// it.
var agentIgnores = map[string]string{
	"me":             "who the caller is, for the console's own chrome; an agent is told by describe_gateway",
	"release-notes":  "what the running release brought, prose for a person under the console's account button",
	"data-tokens":    "every account's application tokens: an agent holding one token has no business listing or revoking other people's",
	"sessions":       "who is signed in where, and ending a person's session: a decision about a human in front of a screen, taken by a human in front of the console",
	"apidocs":        "the developer documentation pages, served to a browser",
	"backup":         "a snapshot is a file to download; export_configuration is the readable half an agent can reason about",
	"certificates":   "certificates and private keys, one of the two places this product refuses to be clever",
	"vault":          "secrets: the vault answers references, never values, and an agent has no business asking",
	"admin-tokens":   "minting a control-plane token from an agent that holds one is how a perimeter stops meaning anything",
	"auth-providers": "external identity providers carry client secrets; the check endpoint reaches a third party under our credentials",
	"identity":       "JWT signing keys",
	"issues":         "user-filed reports, with screenshots",
	"mcp":            "the agent endpoint itself",
	"model": "the shape of an account, not its contents: a field added or removed changes " +
		"what every route may forward and what every form asks for, which is a decision " +
		"about this installation rather than a task. An agent that fills the values it " +
		"defines is save_user's business, not this",
	"live": "the console's websocket: an agent does not hold a socket open to watch a " +
		"list change, and what it carries is answered by read_traffic on demand",
	"portal": "the icon picker's search over the embedded Material Symbols catalogue, " +
		"for the console's portal editor; an agent picks no icons. The portal ITSELF " +
		"is a global setting, and it is read by get_settings",
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

// section is the part of the control plane a pattern belongs to:
// "PUT /api/tenants/{id}/groups/{groupId}" -> "tenants".
func section(pattern string) string {
	_, path, ok := strings.Cut(pattern, " ")
	if !ok {
		path = pattern
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if parts[0] == "api" && len(parts) > 1 {
		return parts[1]
	}
	return parts[0]
}

func TestEverySectionIsEitherCoveredOrKnowinglyNot(t *testing.T) {
	seen := map[string]bool{}
	for _, pattern := range controlPlaneSurface() {
		seen[section(pattern)] = true
	}
	tools := map[string]bool{}
	for _, tool := range (&API{}).tools() {
		tools[tool.Name] = true
	}

	var undecided []string
	for s := range seen {
		_, covered := agentCovers[s]
		_, ignored := agentIgnores[s]
		switch {
		case covered && ignored:
			t.Errorf("section %q is both covered and ignored: decide", s)
		case !covered && !ignored:
			undecided = append(undecided, s)
		}
	}
	sort.Strings(undecided)
	if len(undecided) > 0 {
		t.Errorf(`the control plane grew a section no one decided about: %s

Adding a section to the console means answering one question: can an agent do
this? Either add its tools to admin.tools() and list them in agentCovers, or
say in agentIgnores why an agent has no business there. Both answers are fine;
silence is not - it is how the agent quietly stops knowing this product.`,
			strings.Join(undecided, ", "))
	}

	// A tool named as covering a section must exist, and a section named here
	// must exist too: both halves rot in silence otherwise.
	for s, named := range agentCovers {
		if !seen[s] {
			t.Errorf("agentCovers names section %q, which the control plane no longer serves", s)
		}
		for _, name := range named {
			if !tools[name] {
				t.Errorf("section %q claims the tool %q, which no longer exists", s, name)
			}
		}
	}
	for s := range agentIgnores {
		if !seen[s] {
			t.Errorf("agentIgnores names section %q, which the control plane no longer serves", s)
		}
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
		"DELETE /api/certificates/{id}": true,
		"POST /api/config/import":       true, "POST /api/config/history/{id}/restore": true,
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
		"POST /api/issues/{id}/comments":        true, "PUT /api/issues/{id}/status": true,
		"DELETE /api/issues/{id}": true,
		"POST /api/roles":         true, "PUT /api/roles/{id}": true, "DELETE /api/roles/{id}": true,
		"POST /api/routes/reorder": true, "PUT /api/routes/{id}": true,
		"DELETE /api/routes/{id}": true, "PUT /api/routes/{id}/security": true, "PUT /api/routes/{id}/audit": true,
		"PUT /api/routes/{id}/spec": true, "DELETE /api/routes/{id}/spec": true,
		"PUT /api/settings": true, "PUT /api/settings/agent": true, "PUT /api/settings/issues": true,
		"PUT /api/settings/telemetry":  true,
		"PUT /api/settings/plug":       true,
		"PUT /api/settings/audit":      true,
		"DELETE /api/sessions/{id}":    true,
		"DELETE /api/data-tokens/{id}": true,
		"PUT /api/settings/proxy":      true,
		// The level is a write even though nothing is stored: it changes what
		// every node writes from now on.
		"PUT /api/logs/level":           true,
		"PUT /api/settings/maintenance": true,
		"PUT /api/settings/mail-relay":  true, "PUT /api/settings/tenancy": true,
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
		"delete_group",
		"delete_role",
		"delete_route",
		"describe_gateway",
		"export_configuration",
		"get_branding",
		"get_route",
		"get_settings",
		"import_configuration",
		"list_configurations",
		"list_git_locations",
		"list_groups",
		"list_languages",
		"list_members",
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
		"pull_configuration",
		"push_configuration",
		"read_audit",
		"read_language",
		"read_logs",
		"read_traffic",
		"run_schedule",
		"save_branding",
		"save_configuration",
		"save_group",
		"save_member",
		"save_portal",
		"save_role",
		"save_route",
		"save_user",
		"test_routing",
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
