package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type goFile struct{ name, body string }

// goFilesOf reads this package's sources, for the tests that assert something
// about the way the SQL is written rather than about what it returns.
func goFilesOf(t *testing.T, dir string) []goFile {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []goFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, goFile{name: e.Name(), body: string(b)})
	}
	return out
}
