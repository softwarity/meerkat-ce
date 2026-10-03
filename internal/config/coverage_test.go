package config

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every setting answers one question: does it TRAVEL?
//
// A configuration export is how an installation is reproduced - staging from
// production, a new region from an old one, a customer's environment from a
// reference one. A setting that nobody classified is a setting that silently
// does not travel, and the way that is discovered is an environment that
// behaves differently for a reason nobody can find.
//
// So this test refuses silence, the way internal/admin/mcp_coverage_test.go
// refuses it for endpoints. A new setting is either in ExportedSettings, or
// named below with the reason it stays home. There is no third answer.

// staysHome is every setting that deliberately does not travel, and why. The
// reason is the point: it is what the next person reads before moving one.
var staysHome = map[string]string{
	"SettingInstallationID": "names THIS installation for its cookies: generated at install, and a document " +
		"moved to another gateway must not make that gateway's cookies the same as this one's",
	// Secrets and key material. A configuration document is not a vault, and
	// these have their own artifact.
	"SettingSigningKeys":   "private key material: it belongs to the vault, never to a document that gets mailed around",
	"SettingSimulationKey": "the key that signs a simulated identity: one per installation, and a shared one would let two gateways forge for each other",
	"SettingPlugHostKey":   "the developer tunnel's host key: an identity, not a configuration",
	"SettingSMTP":          "carries a mail credential, and the host is the one THIS environment sends through",

	// Decisions about this machine, this cluster, this moment.
	"SettingMaintenance":       "whether this gateway is down right now: a state, and importing it would take an installation offline",
	"SettingTenancy":           "single or multi is decided when an installation is born, and changing it under existing data is not an import",
	"SettingTenancyChosen":     "records that the question above was answered here",
	"SettingScheduleRetention": "how long a finished delayed action is kept before the sweep: housekeeping for THIS installation's own rows, like the trail's retention, and a number an imported configuration has no business shortening",
	"SettingAuditRetention":    "how long THIS installation keeps its trail: a compliance decision of this place, root's alone, and not something a configuration imported by someone else should be able to shorten",
	"SettingPlug":              "the tunnel's switch and the address developers type: a port into THIS cluster, published by THIS platform - a reproduced installation decides its own",
	"SettingAgentEnabled":      "whether the agent endpoint is open on THIS gateway, which is a decision about this deployment's exposure",
	"SettingEmailSignin":       "depends on SMTP being configured here, and SMTP does not travel",

	// Markers: something already happened, and it happened HERE.
	"SettingConfigSeed":         "records that this installation was seeded",
	"SettingVaultSeed":          "records that this installation's vault was seeded",
	"SettingThemePresetsSeeded": "records that the theme presets were installed here",
	"SettingExpiryDigestSent":   "when the last digest went out, from this node",
}

// TestEverySettingSaysWhetherItTravels reads the store's own source for the
// settings it declares, and fails on any that neither list names.
func TestEverySettingSaysWhetherItTravels(t *testing.T) {
	declared := settingsDeclaredInStore(t)
	if len(declared) < 20 {
		t.Fatalf("only %d settings found in internal/store: the reader is broken, not the code", len(declared))
	}

	exported := map[string]bool{}
	for _, name := range exportedConstNames(t) {
		exported[name] = true
	}

	var unclassified []string
	for _, name := range declared {
		if exported[name] || staysHome[name] != "" {
			continue
		}
		unclassified = append(unclassified, name)
	}
	if len(unclassified) > 0 {
		sort.Strings(unclassified)
		t.Errorf("these settings say nothing about whether they travel: %s\n"+
			"Add each to ExportedSettings (internal/config/document.go) if an installation "+
			"reproduced from this document should carry it, or to staysHome in this file with "+
			"the reason it does not. A setting nobody classified is one that silently stays behind.",
			strings.Join(unclassified, ", "))
	}

	// And the other way: a reason written for a setting that also travels is a
	// contradiction somebody should resolve rather than leave for a reader.
	for name := range staysHome {
		if exported[name] {
			t.Errorf("%s is in ExportedSettings AND in staysHome: it cannot be both", name)
		}
	}
	// A reason for a setting that no longer exists is a reason nobody will
	// ever check again.
	known := map[string]bool{}
	for _, n := range declared {
		known[n] = true
	}
	for name := range staysHome {
		if !known[name] {
			t.Errorf("staysHome names %q, which internal/store no longer declares", name)
		}
	}
}

var settingConst = regexp.MustCompile(`\b(Setting[A-Z]\w*)\s*=\s*"`)

func settingsDeclaredInStore(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("..", "store")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for _, m := range settingConst.FindAllStringSubmatch(string(body), -1) {
			seen[m[1]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

var exportedRef = regexp.MustCompile(`store\.(Setting\w+)`)

// exportedConstNames reads the names AS WRITTEN in ExportedSettings rather
// than the values they hold: the question is whether somebody listed the
// constant, and a value comparison would not say which one is missing.
func exportedConstNames(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile("document.go")
	if err != nil {
		t.Fatalf("reading document.go: %v", err)
	}
	src := string(body)
	start := strings.Index(src, "var ExportedSettings")
	if start < 0 {
		t.Fatal("ExportedSettings is no longer declared in document.go")
	}
	end := strings.Index(src[start:], "\n}")
	if end < 0 {
		t.Fatal("ExportedSettings has no end: the reader is broken")
	}
	var out []string
	for _, m := range exportedRef.FindAllStringSubmatch(src[start:start+end], -1) {
		out = append(out, m[1])
	}
	return out
}
