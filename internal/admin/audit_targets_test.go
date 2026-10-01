package admin

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// A kind of object nobody classed is a kind nobody hears about.
//
// The audit funnel names the kind of object every administrative write touched,
// and store.AuditTargets says who administers each one. That table decides two
// things: which capability reads the event in the trail (RBAC-05), and which
// console screens are told their list moved (CONSOLE-13). A kind written
// somewhere and absent from the table is therefore silent twice over - invisible
// to everyone but root, and pushed to nobody - and silently so, which is why
// this reads the source rather than trusting anyone to remember.
//
// It parses instead of using reflection because the kind is a literal at the
// call site: there is nothing to inspect at run time until somebody exercises
// that exact endpoint, and the endpoint added next month is exactly the one no
// test covers yet.
func TestEveryAuditedKindIsClassed(t *testing.T) {
	found := kindsWrittenIn(t, ".")
	if len(found) < 15 {
		t.Fatalf("only %d kinds found in the sources - the scan stopped working, not the code: %v",
			len(found), found)
	}
	for _, kind := range found {
		if _, ok := store.AuditTargetOf(kind); !ok {
			t.Errorf("the audit trail writes the kind %q and store.AuditTargets does not class it: "+
				"add it there with the capabilities that administer it, or nobody but root will ever "+
				"see it in the trail and no console screen will be told it moved", kind)
		}
	}
	// And the other way round: a kind classed but never written is a line to
	// delete, or an endpoint that forgot to record what it changed.
	written := map[string]bool{}
	for _, kind := range found {
		written[kind] = true
	}
	// The security half is written by the sign-in pages (internal/auth), not
	// by this package: counted as written only while that code still names
	// them, so the day it stops, this says so.
	security, err := os.ReadFile(filepath.Join("..", "auth", "security.go"))
	if err != nil {
		t.Fatal(err)
	}
	for kind, name := range map[string]string{
		store.AuditTargetAccount: "store.AuditTargetAccount",
		store.AuditTargetConsole: "store.AuditTargetConsole",
	} {
		if strings.Contains(string(security), name) {
			written[kind] = true
		}
	}
	for _, kind := range store.AuditTargetKinds() {
		if !written[kind] {
			t.Errorf("store.AuditTargets classes the kind %q and nothing writes it: "+
				"either an endpoint records the wrong kind, or this line outlived its endpoint", kind)
		}
	}
}

// kindsWrittenIn collects every literal kind the audit helpers are called with,
// in the package rooted at dir.
func kindsWrittenIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	kinds := map[string]bool{}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			// auditEvent(ctx, actor, action, TARGET, ...) and
			// auditUpdate(ctx, actor, action, TARGET, ...): the kind is the
			// fourth argument.
			case "auditEvent", "auditUpdate":
				if len(call.Args) > 3 {
					if kind, ok := literal(call.Args[3]); ok {
						kinds[kind] = true
					}
				}
			// audit(ctx, store.AuditEvent{... Target: TARGET ...}): the two call
			// sites that build the event themselves.
			case "audit":
				for _, arg := range call.Args {
					lit, ok := arg.(*ast.CompositeLit)
					if !ok {
						continue
					}
					for _, elt := range lit.Elts {
						kv, ok := elt.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Target" {
							if kind, ok := literal(kv.Value); ok {
								kinds[kind] = true
							}
						}
					}
				}
			}
			return true
		})
	}
	out := make([]string, 0, len(kinds))
	for kind := range kinds {
		out = append(out, kind)
	}
	sort.Strings(out)
	return out
}

// literal reads a string constant, and answers false for anything computed. A
// kind assembled at run time is not something a table can class, and failing on
// it here would fail on the wrong line - every call site writes it as a literal,
// which is what makes this readable at all.
func literal(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil || s == "" {
		return "", false
	}
	return s, true
}
