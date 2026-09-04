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
	"routes":         {"list_routes", "get_route", "test_routing", "save_route", "delete_route"},
	"catalog":        {"list_route_bricks"},
	"services":       {"list_services"},
	"users":          {"list_users"},
	"tenants":        {"list_tenants"},
	"audit":          {"read_audit"},
	"edition":        {"describe_gateway"},
	"config":         {"export_configuration"},
	"configurations": {"list_configurations", "save_configuration"},
}

// agentIgnores says why a section is out of an agent's reach, and each reason
// has to be a reason - "not yet" is one, as long as it names what would change
// it.
var agentIgnores = map[string]string{
	"me":             "who the caller is, for the console's own chrome; an agent is told by describe_gateway",
	"apidocs":        "the developer documentation pages, served to a browser",
	"branding":       "pictures and colours: an agent has no eyes, and a base64 image would flood the conversation",
	"themes":         "same as branding",
	"backup":         "a snapshot is a file to download; export_configuration is the readable half an agent can reason about",
	"certificates":   "certificates and private keys, one of the two places this product refuses to be clever",
	"vault":          "secrets: the vault answers references, never values, and an agent has no business asking",
	"admin-tokens":   "minting a control-plane token from an agent that holds one is how a perimeter stops meaning anything",
	"auth-providers": "external identity providers carry client secrets; the check endpoint reaches a third party under our credentials",
	"identity":       "JWT signing keys",
	"settings":       "a global setting changes every application at once and reads as one line in a diff: the console shows what it affects, an agent would not",
	"roles":          "the role catalogue is what every access rule points at; renaming one silently changes who reaches what, and no tool asked for it yet",
	"issues":         "user-filed reports, with screenshots",
	"mcp":            "the agent endpoint itself",
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
		"DELETE /api/admin-tokens/{id}": true, "POST /api/apidocs/token": true,
		"POST /api/certificates/import": true, "POST /api/certificates/self-signed": true,
		"POST /api/certificates/signing-request": true, "POST /api/certificates/{id}/adopt": true,
		"DELETE /api/certificates/{id}": true,
		"POST /api/config/import":       true, "POST /api/config/history/{id}/restore": true,
		"POST /api/config/history/{id}/save": true,
		"POST /api/configurations":           true, "POST /api/configurations/import": true,
		"POST /api/configurations/{id}/activate": true, "POST /api/configurations/{id}/capture": true,
		"POST /api/configurations/{id}/duplicate": true, "PUT /api/configurations/{id}": true,
		"PUT /api/configurations/{id}/document": true, "DELETE /api/configurations/{id}": true,
		"PUT /api/auth-providers/{id}": true, "DELETE /api/auth-providers/{id}": true,
		"POST /api/identity/signing-keys/renew": true,
		"POST /api/issues/{id}/comments":        true, "PUT /api/issues/{id}/status": true,
		"DELETE /api/issues/{id}": true,
		"POST /api/roles":         true, "PUT /api/roles/{id}": true, "DELETE /api/roles/{id}": true,
		"POST /api/routes/reorder": true, "PUT /api/routes/{id}": true,
		"DELETE /api/routes/{id}": true, "PUT /api/routes/{id}/security": true,
		"PUT /api/routes/{id}/spec": true, "DELETE /api/routes/{id}/spec": true,
		"PUT /api/settings": true, "PUT /api/settings/agent": true, "PUT /api/settings/issues": true,
		"PUT /api/settings/proxy":       true,
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
		"POST /api/themes":                                       true, "PUT /api/themes/{id}": true,
		"POST /api/themes/{id}/activate": true, "DELETE /api/themes/{id}": true,
		"PUT /api/branding": true,
		"POST /api/users":   true, "PUT /api/users/{id}": true, "DELETE /api/users/{id}": true,
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
		"delete_route",
		"describe_gateway",
		"export_configuration",
		"get_route",
		"list_configurations",
		"list_route_bricks",
		"list_routes",
		"list_services",
		"list_tenants",
		"list_users",
		"read_audit",
		"save_configuration",
		"save_route",
		"test_routing",
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
