package admin

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// consoleMonitoring is where the Prometheus drawer's files live.
func consoleMonitoring(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "console", "public", "monitoring")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("console sources not here: %v", err) // the CE mirror builds Go alone
	}
	return dir
}

func monitoringSource(t *testing.T) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("..", "..", "console", "src", "app", "metrics", "monitoring-files.ts"))
	if err != nil {
		t.Skipf("console sources not here: %v", err)
	}
	return string(text)
}

// The Prometheus drawer shows FILES now, fetched from this gateway by name
// rather than built in TypeScript. A renamed or moved one breaks no build: it
// turns into an empty panel that nothing failed to produce, on a screen read
// once and never looked at again.
func TestMonitoringFilesAreOnDisk(t *testing.T) {
	dir := consoleMonitoring(t)
	named := regexp.MustCompile(`'([a-z0-9/-]+\.(?:ya?ml|promql))'`).
		FindAllStringSubmatch(monitoringSource(t), -1)
	if len(named) < 5 {
		t.Fatalf("monitoring-files.ts names %d files, which cannot be right: the pattern "+
			"stopped matching and this test now checks nothing", len(named))
	}
	for _, m := range named {
		if _, err := os.Stat(filepath.Join(dir, m[1])); err != nil {
			t.Errorf("the console offers %s and it is not on disk: %v", m[1], err)
		}
	}
}

// The console rewrites a handful of literals in these files with what this
// installation knows - its port, its network. A literal that no longer appears
// in any of them is a substitution that silently does nothing, and what the
// reader copies is then the default dressed up as their own.
func TestStampedLiteralsAreInTheFiles(t *testing.T) {
	dir := consoleMonitoring(t)
	literals := regexp.MustCompile(`replaceAll\('([^']+)'`).
		FindAllStringSubmatch(monitoringSource(t), -1)
	if len(literals) < 3 {
		t.Fatalf("found %d stamped literals, which cannot be right: the pattern stopped matching",
			len(literals))
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			text, e := os.ReadFile(path)
			if e == nil {
				files = append(files, string(text))
			}
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range literals {
		found := false
		for _, text := range files {
			if strings.Contains(text, l[1]) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the console replaces %q and no file contains it any more", l[1])
		}
	}
}

// One scrape file carries every platform's discovery, each line prefixed with
// the platform it belongs to, and the drawer hands back the one asked for with
// its own block uncommented and the others gone.
//
// The rule is reimplemented here rather than imported, and that is the point:
// what this checks is the FILE - that each marked block is indented so that
// removing its marker leaves valid YAML underneath, and that no variant comes
// out with no way to find a target. An indentation off by one is invisible in
// a diff and fatal in a scrape.
func TestScrapeVariantsAreValidYAML(t *testing.T) {
	dir := consoleMonitoring(t)
	whole, err := os.ReadFile(filepath.Join(dir, "prometheus.yml"))
	if err != nil {
		t.Fatalf("prometheus.yml: %v", err)
	}
	// The platforms the console declares, so the file and the screen cannot
	// drift apart: a block marked for a platform nothing offers is dead, and a
	// platform offered with no block downloads the scrape with no target.
	declared := regexp.MustCompile(`\{ key: '([a-z0-9]+)', label: '[^']+' \}`).
		FindAllStringSubmatch(monitoringSource(t), -1)
	if len(declared) < 2 {
		t.Fatal("monitoring-files.ts declares fewer than two platforms: the pattern stopped matching")
	}

	for _, d := range declared {
		key := d[1]
		text := scrapeVariant(string(whole), key)
		if strings.Contains(text, "#"+key+" ") {
			t.Errorf("%s: a marker survived into the file it was meant to open", key)
		}
		var doc struct {
			Global map[string]any   `yaml:"global"`
			Jobs   []map[string]any `yaml:"scrape_configs"`
		}
		if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
			t.Errorf("%s is not valid YAML: %v\n%s", key, err, text)
			continue
		}
		if len(doc.Jobs) != 1 {
			t.Errorf("%s: %d scrape jobs, want 1", key, len(doc.Jobs))
			continue
		}
		if !findsATarget(doc.Jobs[0]) {
			t.Errorf("%s: the job says nothing about where the target is, so it scrapes "+
				"nothing:\n%s", key, text)
		}
	}
	// And the file as it stands finds nothing, which is what makes the choice
	// a choice: every discovery in it is commented out.
	var raw struct {
		Jobs []map[string]any `yaml:"scrape_configs"`
	}
	if err := yaml.Unmarshal(whole, &raw); err != nil {
		t.Fatalf("prometheus.yml is not valid YAML: %v", err)
	}
	if len(raw.Jobs) == 1 && findsATarget(raw.Jobs[0]) {
		t.Error("prometheus.yml already finds a target: one platform's block is not commented out, " +
			"and a reader taking the file as it stands would scrape on that one by accident")
	}
}

// findsATarget says whether a scrape job knows where to look. Discovery keys
// all end in _configs; the relabelling ones sort what discovery returned and
// find nothing themselves.
func findsATarget(job map[string]any) bool {
	for key := range job {
		if strings.HasSuffix(key, "_configs") && !strings.HasSuffix(key, "relabel_configs") {
			return true
		}
	}
	return false
}

// scrapeVariant is the console's rule, written again: keep the lines marked
// for this platform with their marker stripped, drop the ones marked for
// another, and collapse the blank line a removed block leaves behind.
func scrapeVariant(text, key string) string {
	marker := regexp.MustCompile(`^#([a-z0-9]+) `)
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		m := marker.FindStringSubmatch(line)
		switch {
		case m == nil:
			kept = append(kept, line)
		case m[1] == key:
			kept = append(kept, line[len(m[0]):])
		}
	}
	var out []string
	for i, line := range kept {
		if strings.TrimSpace(line) == "" && i > 0 && strings.TrimSpace(kept[i-1]) == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}
