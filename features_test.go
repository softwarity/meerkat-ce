package meerkat

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

// The License screen names every Enterprise row of the contract, and nothing
// else (CONSOLE-14). WHICH features are Enterprise is read from FEATURES.md at
// run time; the console only holds their names. If one side moves without the
// other, this says which ID is missing where.
func TestTheConsoleNamesEveryEnterpriseFeature(t *testing.T) {
	src, err := os.ReadFile("console/src/app/settings/ee-features.ts")
	if err != nil {
		t.Fatal(err)
	}
	named := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^  '([A-Z]+-\d+)': \{`).FindAllStringSubmatch(string(src), -1) {
		named[m[1]] = true
	}
	inContract := map[string]bool{}
	for _, f := range EnterpriseFeatures() {
		inContract[f.ID] = true
		if !named[f.ID] {
			t.Errorf("%s is an Enterprise row of FEATURES.md with no name in ee-features.ts", f.ID)
		}
	}
	for id := range named {
		if !inContract[id] {
			t.Errorf("ee-features.ts names %s, which FEATURES.md does not list as Enterprise", id)
		}
	}
	if len(inContract) == 0 {
		t.Fatal("no Enterprise row read from FEATURES.md: the table's shape changed")
	}
}

func TestParseFeatures(t *testing.T) {
	got := ParseFeatures("| [x] | A-1 | **T** | d | - | EE |\n| [~] | B-2 | **T** | d | m | CE/EE |\n| [-] | C-3 | **T** | d | - | EE |\nnot a row\n")
	want := []Feature{{"A-1", "done", "EE"}, {"B-2", "partial", "CE/EE"}, {"C-3", "retired", "EE"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v", got)
	}
}
