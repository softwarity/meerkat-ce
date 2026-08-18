package idp

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// configKeyRe finds the config keys a driver actually reads.
var configKeyRe = regexp.MustCompile(`cfg(?:String|Strings|Bool)\(p\.Config, "([^"]+)"`)

// looksSensitive is deliberately WIDER than SecretFields: it is not what
// declares a secret, it is what notices one has not been declared.
func looksSensitive(key string) bool {
	k := strings.ToLower(key)
	for _, hint := range []string{"secret", "password", "passwd", "apikey", "token", "credential", "privatekey"} {
		if strings.Contains(k, hint) {
			return true
		}
	}
	return false
}

// TestSecretFieldsCoverEveryDriver reads the drivers' own source and fails when
// one of them takes a key that looks like a secret without SecretFields saying
// so. The declaration is what four other behaviours trust (redaction on the way
// out, carry-forward on save, audit, the vault offer), so an undeclared field
// leaks quietly in all four at once - exactly the kind of thing a review
// misses and a build should not.
func TestSecretFieldsCoverEveryDriver(t *testing.T) {
	// Which file drives which kind, BY PATH: the directories moved out to ee/
	// (they are sold, and a licence gate written in the FSL tree is a lock
	// whose key the licence hands over) and this check has to follow them.
	// A glob of this directory alone would simply have stopped seeing the
	// biggest driver, silently, which is the failure this test exists against.
	byFile := map[string]string{
		"oidc.go":                      store.ProviderOIDC,
		"oauth2.go":                    store.ProviderGitHub,
		"../../ee/directories/ldap.go": store.ProviderLDAP,
	}
	seen := map[string]bool{}
	absent := map[string]bool{} // kinds whose source is not in THIS tree
	for path, kind := range byFile {
		// The community tree has no ee/ at all - that is what the mirror
		// publishes. A driver that lives there is simply not part of that
		// product, so it is skipped rather than failed; anything else missing
		// is a real problem and falls through to ReadFile below.
		if strings.HasPrefix(path, "../../ee/") {
			if _, err := os.Stat(path); err != nil {
				t.Logf("%s is not in this tree (community build): skipped", path)
				absent[kind] = true
				continue
			}
		}
		seen[kind] = true
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		declared := SecretFields(kind)
		for _, m := range configKeyRe.FindAllStringSubmatch(string(src), -1) {
			key := m[1]
			if looksSensitive(key) && !slices.Contains(declared, key) {
				t.Errorf("%s reads %q, which looks like a secret, but SecretFields(%q) = %v: "+
					"add it there or it will be returned in clear by the admin API",
					path, key, kind, declared)
			}
		}
	}
	// The reverse: a declared field nobody reads is a typo that silently
	// protects nothing.
	for file, kind := range byFile {
		src, err := os.ReadFile(file)
		if err != nil {
			if strings.HasPrefix(file, "../../ee/") {
				continue // not part of the community tree, said once above
			}
			t.Fatal(err)
		}
		for _, field := range SecretFields(kind) {
			if !strings.Contains(string(src), `"`+field+`"`) {
				t.Errorf("SecretFields(%q) declares %q, but %s never reads it", kind, field, file)
			}
		}
	}
	// And a NEW driver has to be listed above, or it would be scanned by
	// nobody. SAML is the one kind with no driver on purpose.
	for _, kind := range []string{store.ProviderOIDC, store.ProviderLDAP, store.ProviderGitHub, store.ProviderSAML} {
		if _, wired := New(store.AuthProvider{ID: "x", Kind: kind, Name: "x"}); wired == nil {
			continue // a driver that builds with an empty config: nothing to read
		}
		if !seen[kind] && !absent[kind] && kind != store.ProviderSAML {
			t.Errorf("kind %q has a driver but no entry in byFile: its secret fields are checked by nobody", kind)
		}
	}
}
