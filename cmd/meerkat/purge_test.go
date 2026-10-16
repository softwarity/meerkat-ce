package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A purge nobody calls is a table that only grows.
//
// The store had three of them: the agents' OAuth codes and refresh tokens,
// the one-shot challenges and the trusted browsers were each written with an
// expiry and a method to sweep them, and the upkeep never named the method.
// Nothing failed - the rows simply stayed. So this reads the sources: every
// Purge method of the store is called by purge() below, or is named here with
// who calls it instead.
var purgedElsewhere = map[string]string{
	// The session manager owns its table, and purge() calls the manager.
	"PurgeExpiredSessions": "session.Manager.PurgeExpired",
}

func TestEveryStorePurgeIsCalledByTheUpkeep(t *testing.T) {
	fset := token.NewFileSet()
	declared := map[string]bool{}
	files, err := filepath.Glob(filepath.Join("..", "..", "internal", "store", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !strings.HasPrefix(fn.Name.Name, "Purge") {
				continue
			}
			if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
				if id, ok := star.X.(*ast.Ident); ok && id.Name == "Store" {
					declared[fn.Name.Name] = true
				}
			}
		}
	}
	if len(declared) < 5 {
		t.Fatalf("only %d purge methods found - the scan stopped working, not the code", len(declared))
	}

	called := map[string]bool{}
	main, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range main.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "purge" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "st" {
					called[sel.Sel.Name] = true
				}
			}
			return true
		})
	}
	for name := range declared {
		if !called[name] && purgedElsewhere[name] == "" {
			t.Errorf("store.%s is never called by purge(): call it there, or say in purgedElsewhere who does", name)
		}
	}
	for name := range purgedElsewhere {
		if !declared[name] {
			t.Errorf("purgedElsewhere names store.%s, which no longer exists", name)
		}
	}
}

// And the sweep does what it says on rows that expired without being spent.
func TestThePurgeSweepsAbandonedGrants(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	past := time.Now().Add(-time.Hour).Unix()

	if err := st.SaveOIDCCode(ctx, store.OIDCCode{Hash: "code-1", ClientID: "app", ExpiresAt: past}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveOIDCRefresh(ctx, store.OIDCRefresh{Hash: "refresh-1", ClientID: "app", ExpiresAt: past}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutChallenge(ctx, "challenge-1", "data", past); err != nil {
		t.Fatal(err)
	}

	purge(ctx, session.NewManager(st), st)

	// Gone, not merely expired: a row the purge missed answers "expired".
	if _, err := st.TakeOIDCCode(ctx, "code-1"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("an expired code survived the purge: %v", err)
	}
	if _, err := st.TakeOIDCRefresh(ctx, "refresh-1"); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("an expired refresh token survived the purge: %v", err)
	}
	if n, err := st.PurgeExpiredChallenges(ctx, time.Now().Unix()); err != nil || n != 0 {
		t.Errorf("an expired challenge survived the purge: %d left (%v)", n, err)
	}
}
