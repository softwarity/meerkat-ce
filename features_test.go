package meerkat

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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

// FEATURES.md says how far each feature is built, and e2e/platform/plan.json
// what each has to prove. The License screen and the site's Test plan page
// show the contract as it is written, so these hold it to what the rest of the
// tree says, naming the ID that drifted:
//
//   - every row has its entry in the plan, and the plan has no other;
//   - a row ticked built is not "blocked" in the plan, the plan's word for
//     a feature nothing exists of yet;
//   - a row still planned is named by no test. A test that names a feature
//     claims to check it: either the feature exists and its box is wrong, or
//     the test names the wrong one. Self-registration was cited under the
//     ID of Kerberos, from the numbering the table replaced (2026-10-16).
func TestTheContractAgreesWithThePlanAndTheTests(t *testing.T) {
	raw, err := os.ReadFile("e2e/platform/plan.json")
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Features []struct {
			ID          string `json:"id"`
			Feasibility string `json:"feasibility"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatalf("e2e/platform/plan.json: %v", err)
	}
	planned := map[string]string{}
	for _, f := range plan.Features {
		planned[f.ID] = f.Feasibility
	}
	rows := ParseFeatures(Features)
	inContract := map[string]bool{}
	for _, f := range rows {
		inContract[f.ID] = true
		feasibility, ok := planned[f.ID]
		switch {
		case !ok && f.Status != "retired":
			t.Errorf("%s is a row of FEATURES.md with no entry in e2e/platform/plan.json", f.ID)
		case f.Status == "done" && feasibility == "blocked":
			t.Errorf("%s is ticked built in FEATURES.md and \"blocked\" (not built) in e2e/platform/plan.json", f.ID)
		}
	}
	for id := range planned {
		if !inContract[id] {
			t.Errorf("e2e/platform/plan.json plans %s, which is no row of FEATURES.md", id)
		}
	}

	tests := testSources(t)
	for _, f := range rows {
		if f.Status != "planned" {
			continue
		}
		named := regexp.MustCompile(`\b` + regexp.QuoteMeta(f.ID) + `\b`)
		for path, src := range tests {
			if named.MatchString(src) {
				t.Errorf("%s names %s, which FEATURES.md says is not built: tick its box, or name the feature it checks", path, f.ID)
			}
		}
	}
}

// testSources reads every test of the tree: the Go tests, wherever they are,
// and the Playwright suite with its scenario matrix.
func testSources(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "node_modules" || (strings.HasPrefix(name, ".") && path != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		e2e := strings.HasPrefix(path, "e2e/tests/") && strings.HasSuffix(path, ".ts")
		if strings.HasSuffix(path, "_test.go") || e2e || path == "e2e/scenarios.json" {
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[path] = string(src)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("no test source found: the tree's shape changed")
	}
	return out
}
