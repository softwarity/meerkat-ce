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
