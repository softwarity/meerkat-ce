package admin

import (
	"encoding/json"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The rights matrix is READ from the code, never written.
//
// Who may call what on the control plane is decided in one place per endpoint:
// the guard its handler is wrapped in at registration. This test walks those
// registrations, writes what it found to rights.json, and the documentation
// renders that file as a matrix. A matrix somebody maintains by hand drifts the
// week an endpoint is added; this one cannot, because changing a guard without
// regenerating the file fails here.
//
//	go test ./internal/admin -run TestTheRightsMatrixIsReadFromTheCode -update

var updateRights = flag.Bool("update", false, "rewrite rights.json from the guards in the code")

// right is one registration: the pattern it answers and the guard in front of
// it. The ROLES a guard admits live in the documentation generator, beside the
// words that explain them - this file records facts, not their reading.
type right struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Guard  string `json:"guard"`
}

// knownGuards is every guard the matrix knows how to read. A registration
// behind a guard missing from here fails the test: a new guard is a new line
// in the matrix, and somebody has to say what it admits.
var knownGuards = map[string]bool{
	"rootOnly": true, "infraAdmin": true, "gw": true, "appAdmin": true,
	"domainAdmin": true, "tenantScoped": true, "schedules": true,
	"tenantCreator": true, "devOrInfra": true,
	"authed": true, "agentDoor": true,
}

func TestTheRightsMatrixIsReadFromTheCode(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var found []right
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Handle" {
				return true
			}
			if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "mux" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			pattern, _ := strconv.Unquote(lit.Value)
			// Only the API and the agent: the console, the docs and the static
			// assets are pages, not rights.
			if !strings.Contains(pattern, "/api/") && pattern != "/mcp" {
				return true
			}
			guard := outerGuard(call.Args[1])
			if guard == "" {
				t.Errorf("%s: %q is registered behind no guard this test can read", name, pattern)
				return true
			}
			if !knownGuards[guard] {
				t.Errorf("%s: %q is behind a guard the matrix does not know: %s - add it to knownGuards and say what it admits in docs/scripts/gen-rights.mjs",
					name, pattern, guard)
			}
			method, path := "*", pattern
			if sp := strings.IndexByte(pattern, ' '); sp > 0 {
				method, path = pattern[:sp], pattern[sp+1:]
			}
			found = append(found, right{Method: method, Path: path, Guard: guard})
			return true
		})
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].Path != found[j].Path {
			return found[i].Path < found[j].Path
		}
		return found[i].Method < found[j].Method
	})
	got, err := json.MarshalIndent(found, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')

	const golden = "rights.json"
	if *updateRights {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s: %d endpoints", golden, len(found))
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read %s: %v - generate it with: go test ./internal/admin -run TestTheRightsMatrixIsReadFromTheCode -update", golden, err)
	}
	if string(want) != string(got) {
		t.Errorf("a guard changed and the rights matrix does not say so yet. Regenerate it, and read the diff - it is the documentation of who may do what:\n\n  go test ./internal/admin -run TestTheRightsMatrixIsReadFromTheCode -update")
	}
}

// outerGuard names the guard a handler is wrapped in: a.infraAdmin(...) gives
// infraAdmin. The agent is wrapped twice - its door, then authentication - and
// the door is what decides whether it answers at all.
func outerGuard(e ast.Expr) string {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if x, ok := sel.X.(*ast.Ident); !ok || x.Name != "a" {
		return ""
	}
	return sel.Sel.Name
}
