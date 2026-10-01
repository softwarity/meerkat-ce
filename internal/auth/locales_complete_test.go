package auth

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// A LANGUAGE WE SHIP IS A LANGUAGE WE SHIP WHOLE.
//
// For twelve days the three transactional mails and the second factor by mail
// existed in English and French only: fourteen strings, eighteen catalogues,
// and nothing said so. The loader stands English in for a missing wording so no
// page renders blank, which is right for the page and fatal for the author - it
// is exactly why nobody noticed. A German user got a German sign-in page and an
// English confirmation mail.
//
// So the guard is here rather than in a habit. Adding a string to en.json and
// stopping there now fails the build, naming the language and the keys: a hole
// costs one test run to find, and a release to discover otherwise.
//
// This reads the EMBEDDED FILES, not the messages map - the map is backfilled
// at load, so asking it would be asking the thing that hides the answer.
func TestWeShipNoHalfLanguage(t *testing.T) {
	cat := rawCatalogues(t)
	en, ok := cat["en"]
	if !ok {
		t.Fatal("no en.json: English is the reference every other catalogue is measured against")
	}

	for _, lang := range sortedLangs(cat) {
		if lang == "en" {
			continue
		}
		m := cat[lang]
		var missing, stale []string
		for key := range en {
			if _, there := m[key]; !there {
				missing = append(missing, key)
			}
		}
		for key := range m {
			if _, there := en[key]; !there {
				stale = append(stale, key)
			}
		}
		sort.Strings(missing)
		sort.Strings(stale)
		if len(missing) > 0 {
			t.Errorf("%s.json is missing %d of the %d strings en.json carries, so those pages speak English "+
				"to somebody who asked for %s. Translate them or take them out of en.json:\n  %s",
				lang, len(missing), len(en), lang, strings.Join(missing, "\n  "))
		}
		if len(stale) > 0 {
			t.Errorf("%s.json carries %d strings en.json does not: a key removed from English and left behind "+
				"here, which no page will ever read:\n  %s", lang, len(stale), strings.Join(stale, "\n  "))
		}
	}
}

// An empty wording is a hole a key cannot hide: the key is there, the page
// renders nothing, and the completeness check above sees a full catalogue.
func TestNoShippedWordingIsBlank(t *testing.T) {
	for _, lang := range sortedLangs(rawCatalogues(t)) {
		for key, text := range rawCatalogues(t)[lang] {
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s.json has %q set to an empty string, which renders as nothing. "+
					"Remove the key or write the wording", lang, key)
			}
		}
	}
}

// rawCatalogues reads the JSON as shipped, before the English backfill.
func rawCatalogues(t *testing.T) map[string]map[string]string {
	t.Helper()
	entries, err := localeFiles.ReadDir("locales")
	if err != nil {
		t.Fatalf("no locales embedded: %v", err)
	}
	out := make(map[string]map[string]string, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		raw, err := localeFiles.ReadFile("locales/" + name)
		if err != nil {
			t.Fatalf("locale %s: %v", name, err)
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("locale %s: %v", name, err)
		}
		out[strings.TrimSuffix(name, ".json")] = m
	}
	return out
}

func sortedLangs(cat map[string]map[string]string) []string {
	out := make([]string, 0, len(cat))
	for l := range cat {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}
