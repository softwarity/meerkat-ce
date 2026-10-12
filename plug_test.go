package meerkat

import (
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// The plug release this gateway hands out is written ONCE, in docker/plug.json,
// and read where it is needed: the image's payload (docker/Dockerfile), the
// agent linked into the binary (go.mod), and the site, which publishes it as
// assets/plug.json for the README's badge and writes it as {{plug.version}}. The image and the module must move together - an agent
// older than the image's payload answers the download account with a script
// the distroless image has no shell to run - so nothing may drift from the
// file. `make plug V=x.y.z` moves all of them.
func TestThePlugReleaseIsTheOneOfPlugJSON(t *testing.T) {
	raw, err := os.ReadFile("docker/plug.json")
	if err != nil {
		t.Fatal(err)
	}
	var pin struct{ Version, Commit string }
	if err := json.Unmarshal(raw, &pin); err != nil || pin.Version == "" || len(pin.Commit) != 40 {
		t.Fatalf("docker/plug.json needs a version and the tag's 40-character commit: %s %v", raw, err)
	}
	read := func(path string) string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if want := "ARG PLUG_VERSION=" + pin.Version + "-hosted\n"; !strings.Contains(read("docker/Dockerfile"), want) {
		t.Errorf("docker/Dockerfile does not say %q: run make plug V=%s", strings.TrimSpace(want), pin.Version)
	}
	m := regexp.MustCompile(`github\.com/softwarity/plug/agent v0\.0\.0-\d{14}-([0-9a-f]{12})`).FindStringSubmatch(read("go.mod"))
	if m == nil || m[1] != pin.Commit[:12] {
		t.Errorf("go.mod links the plug agent at %v, not at %s (v%s): run make plug V=%s", m, pin.Commit[:12], pin.Version, pin.Version)
	}
	if !strings.Contains(read("README.md"), "www.softwarity.io%2Fassets%2Fplug.json") {
		t.Error("README.md has lost the badge that reads the site's assets/plug.json")
	}
}

// The version reaches the bump from outside - plug's dispatch, or the tag of
// its latest release - and the bump runs with the right to push to main. So
// it is a VERSION or it is nothing: every attempt below must be refused before
// a single command runs - by the script, and through make, which pastes a
// $(V) into the recipe's text before the shell ever reads it.
//
// Not through make with a `$` in it: make expands `$(shell ...)` in a
// variable given on its command line by itself, before any recipe, and no
// Makefile can refuse that. Which is why the workflow calls the script, not
// make, and checks the version before either.
func TestThePlugBumpRunsNothingButAVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("runs make and the bump script")
	}
	for _, v := range []string{
		`1.2.3;touch PWNED`,
		`1.2.3$(touch PWNED)`,
		"1.2.3`touch PWNED`",
		`1.2.3" ; touch PWNED ; echo "`,
		`1.2.3' ; touch PWNED ; echo '`,
		`1.2.3 PWNED`,
		`v1.2`,
		``,
	} {
		cmds := map[string]*exec.Cmd{"script": exec.Command("sh", "tools/plug/bump.sh", v)}
		if !strings.Contains(v, "$") {
			cmds["make"] = exec.Command("make", "--no-print-directory", "plug", "V="+v)
		}
		for name, cmd := range cmds {
			out, err := cmd.CombinedOutput()
			if _, statErr := os.Stat("PWNED"); statErr == nil {
				_ = os.Remove("PWNED")
				t.Fatalf("%s ran a command out of the version %q:\n%s", name, v, out)
			}
			if err == nil {
				t.Fatalf("%s accepted %q as a plug release:\n%s", name, v, out)
			}
			if !strings.Contains(string(out), "is not a plug release") {
				t.Fatalf("%s refused %q for another reason than the version itself:\n%s", name, v, out)
			}
		}
	}
}
