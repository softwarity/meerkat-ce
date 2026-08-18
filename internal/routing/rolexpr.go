package routing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// Shaping the roles a route forwards (ROUTE-18).
//
// A route sends a list of role names to its service, and every estate wants it
// differently: comma-separated, a JSON array, without the ROLE_ prefix, with a
// prefix of its own, and above all NOT ALL OF THEM - 320 roles is 9 KB of
// header on every request, sent to a service that uses three.
//
// Rather than one switch per wish (a JSON toggle, a prefix to trim, a tag
// list), the route carries ONE expression that says what to send. It reads as
// a pipeline, in the order it happens:
//
//	{{.Roles | .Keep "tag:BILLING" "ROLE_USER" | cutHead "ROLE_" | join ","}}
//
// Deliberately text/template rather than a small JavaScript: this runs on
// every proxied request, and text/template has no unbounded loop, no way out
// of its data, and no dependency. An expression cannot fail to terminate.

// RoleExprDefault is what a route sends when nobody wrote an expression: the
// list, comma-separated, which is what an upstream expects unless it says
// otherwise. It is also what the editor opens with, so the plain case is read
// rather than guessed.
const RoleExprDefault = `{{join "," .Roles}}`

// roleExprFuncs are the pipeline's verbs. Each takes the list LAST so it can be
// piped into: {{.Roles | .Keep "tag:X" | cutHead "ROLE_" | join ","}}.
// keep and drop are METHODS on the input rather than functions here: they need
// the caller's catalogue, and a package-level lookup would be shared by every
// request the gateway serves at once.
var roleExprFuncs = template.FuncMap{
	// The four ways to reshape a name, named symmetrically so the opposite of
	// one is obvious. A name that does not carry what cutHead/cutTail removes
	// is left alone rather than mangled: a catalogue is rarely uniform.
	"cutHead": func(head string, roles []string) []string {
		return mapEach(roles, func(r string) string { return strings.TrimPrefix(r, head) })
	},
	"cutTail": func(tail string, roles []string) []string {
		return mapEach(roles, func(r string) string { return strings.TrimSuffix(r, tail) })
	},
	"addHead": func(head string, roles []string) []string {
		return mapEach(roles, func(r string) string { return head + r })
	},
	"addTail": func(tail string, roles []string) []string {
		return mapEach(roles, func(r string) string { return r + tail })
	},
	"lower": func(roles []string) []string { return mapEach(roles, strings.ToLower) },
	"upper": func(roles []string) []string { return mapEach(roles, strings.ToUpper) },
	"join":  func(sep string, roles []string) string { return strings.Join(roles, sep) },
	"json": func(v any) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	},
	"count": func(roles []string) int { return len(roles) },
}

func mapEach(roles []string, f func(string) string) []string {
	out := make([]string, len(roles))
	for i, r := range roles {
		out[i] = f(r)
	}
	return out
}

// RoleExprInput is what an expression sees: the caller's roles, and the two
// verbs that narrow them.
type RoleExprInput struct {
	Roles  []string
	tagsOf func(string) []string
}

// Keep narrows to the roles a selector names: "tag:BILLING" is a catalogue tag,
// anything else is a role name. Several selectors are ORed - a service wanting
// a whole tag plus one specific role writes both.
func (in RoleExprInput) Keep(args ...any) ([]string, error) { return in.selectRoles(true, args...) }

// Drop is Keep's opposite, for "everything except the noisy ones".
func (in RoleExprInput) Drop(args ...any) ([]string, error) { return in.selectRoles(false, args...) }

// selectRoles implements Keep and Drop. The trailing argument is the list; the
// ones before it are selectors.
func (in RoleExprInput) selectRoles(keep bool, args ...any) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("keep/drop needs the roles to work on")
	}
	roles, ok := args[len(args)-1].([]string)
	if !ok {
		return nil, fmt.Errorf("keep/drop works on the role list: pipe .Roles into it")
	}
	selectors := make([]string, 0, len(args)-1)
	for _, a := range args[:len(args)-1] {
		s, ok := a.(string)
		if !ok {
			return nil, fmt.Errorf(`a selector is text: "tag:NAME" for a catalogue tag, or a role name`)
		}
		selectors = append(selectors, s)
	}
	if len(selectors) == 0 {
		return roles, nil
	}
	wantTags, wantNames := map[string]bool{}, map[string]bool{}
	for _, sel := range selectors {
		if tag, ok := strings.CutPrefix(sel, "tag:"); ok {
			wantTags[tag] = true
			continue
		}
		wantNames[sel] = true
	}
	out := roles[:0:0]
	for _, r := range roles {
		match := wantNames[r]
		if !match && in.tagsOf != nil {
			for _, tag := range in.tagsOf(r) {
				if wantTags[tag] {
					match = true
					break
				}
			}
		}
		if match == keep {
			out = append(out, r)
		}
	}
	return out, nil
}

// CompiledRoleExpr renders one route's role expression.
type CompiledRoleExpr struct{ tmpl *template.Template }

// CompileRoleExpr parses an expression and proves it runs, against a witness
// catalogue - a field that does not exist parses fine and would otherwise fail
// on a request, which is the worst place to learn it.
func CompileRoleExpr(expr string) (*CompiledRoleExpr, error) {
	if strings.TrimSpace(expr) == "" {
		expr = RoleExprDefault
	}
	tmpl, err := template.New("roles").Funcs(roleExprFuncs).Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("roles: %s", readableTemplateError(err))
	}
	c := &CompiledRoleExpr{tmpl: tmpl}
	// TWO witnesses, not one: a separator only shows up between elements, so a
	// single role would hide `join "\n"` - the very thing the render checks for.
	if _, err := c.Render([]string{"ROLE_SAMPLE", "ROLE_OTHER"},
		func(string) []string { return []string{"SAMPLE"} }); err != nil {
		return nil, fmt.Errorf("roles: %s", readableTemplateError(err))
	}
	return c, nil
}

// Render produces the header value for one caller. tagsOf resolves a role name
// to its catalogue tags.
func (c *CompiledRoleExpr) Render(roles []string, tagsOf func(string) []string) (string, error) {
	if c == nil || c.tmpl == nil {
		return strings.Join(roles, ","), nil
	}
	var buf bytes.Buffer
	if err := c.tmpl.Execute(&buf, RoleExprInput{Roles: roles, tagsOf: tagsOf}); err != nil {
		return "", err
	}
	out := buf.String()
	// The result becomes a header VALUE: a line break in it would let an
	// expression forge a second header.
	if strings.ContainsAny(out, "\r\n") {
		return "", fmt.Errorf("the result contains a line break, which a header value cannot carry")
	}
	return out, nil
}
