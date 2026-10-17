package routing

import "testing"

// The shapes the console offers under "Start from". They are proposed with one
// click, so a shape the server would refuse is a trap with a button on it -
// keep this list in step with SHAPES in role-expr-dialog.component.ts.
func TestConsoleShapesCompile(t *testing.T) {
	pipes := []string{
		`.Keep "tag:TAGNAME" | cutHead "ROLE_"`,
		`.Keep "tag:TAGNAME" "tag:OTHERTAG" | cutHead "ROLE_"`,
		`.Keep "tag:TAGNAME" "ROLE_USER" "ROLE_ADMIN" | cutHead "ROLE_"`,
		`.Drop "tag:TAGNAME" | cutHead "ROLE_"`,
		`.Keep "tag:TAGNAME" | cutHead "ROLE_" | addHead "app:"`,
		``,
	}
	for _, tail := range []string{`join ","`, `json`} {
		for _, pipe := range pipes {
			expr := "{{.Roles | " + tail + "}}"
			if pipe != "" {
				expr = "{{.Roles | " + pipe + " | " + tail + "}}"
			}
			if _, err := CompileRoleExpr(expr); err != nil {
				t.Errorf("%s: %v", expr, err)
			}
		}
	}
}

// Several selectors are ORed, which is how a service asks for two tags, or for
// a tag plus a handful of roles named one by one.
func TestKeepORsItsSelectors(t *testing.T) {
	roles := []string{"ROLE_BILLING_ADMIN", "ROLE_REPORTING_USER", "ROLE_USER", "ROLE_OTHER"}
	tagsOf := func(r string) []string {
		switch r {
		case "ROLE_BILLING_ADMIN":
			return []string{"BILLING"}
		case "ROLE_REPORTING_USER":
			return []string{"REPORTING"}
		}
		return nil
	}
	for expr, want := range map[string]string{
		`{{.Roles | .Keep "tag:BILLING" "tag:REPORTING" | join ","}}`: "ROLE_BILLING_ADMIN,ROLE_REPORTING_USER",
		`{{.Roles | .Keep "tag:BILLING" "ROLE_USER" | join ","}}`:     "ROLE_BILLING_ADMIN,ROLE_USER",
		`{{.Roles | .Drop "tag:BILLING" "ROLE_OTHER" | join ","}}`:    "ROLE_REPORTING_USER,ROLE_USER",
	} {
		c, err := CompileRoleExpr(expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		got, err := c.Render(roles, tagsOf)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got != want {
			t.Errorf("%s\n got %q\nwant %q", expr, got, want)
		}
	}
}

// .Declared answers in the APPLICATION's names, and only for what the caller
// holds: the map's other entries, and the caller's other roles, stay home.
func TestDeclaredSendsTheApplicationsNames(t *testing.T) {
	c, err := CompileRoleExpr(RoleExprDeclared)
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]string{"ROLE_ADMIN": "demo-ROLE_ADMIN", "ROLE_USER": "opt-svc", "ROLE_AUDIT": "auditor"}
	got, err := c.RenderDeclared([]string{"opt-svc", "demo-ROLE_ADMIN", "billing"}, nil, declared)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ROLE_ADMIN,ROLE_USER" {
		t.Errorf("declared roles %q, want ROLE_ADMIN,ROLE_USER", got)
	}
	// Composes: the application's names, as a JSON array.
	c, err = CompileRoleExpr(`{{.Roles | .Declared | json}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := c.RenderDeclared([]string{"auditor"}, nil, declared); got != `["ROLE_AUDIT"]` {
		t.Errorf("as json %q, want [\"ROLE_AUDIT\"]", got)
	}
	// A route with no map sends nothing through .Declared, not everything.
	if got, _ := c.RenderDeclared([]string{"auditor"}, nil, nil); got != `[]` {
		t.Errorf("without a map %q, want []", got)
	}
}
