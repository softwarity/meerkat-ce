package auth

import (
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// TestMessageCatalogueComplete: every locale carries exactly the same keys -
// a missing key renders as an empty string, which no test UI would catch.
func TestMessageCatalogueComplete(t *testing.T) {
	ref := messages["en"]
	for lang, m := range messages {
		if lang == "en" {
			continue
		}
		for k := range ref {
			if _, ok := m[k]; !ok {
				t.Errorf("locale %q is missing key %q", lang, k)
			}
		}
		for k := range m {
			if _, ok := ref[k]; !ok {
				t.Errorf("locale %q has extra key %q (missing from en)", lang, k)
			}
		}
	}
}

// TestSupportedLanguagesSpeakTheCatalogue: the store's advertised languages
// must all exist in the catalogue (and have an endonym) - the two lists are
// maintained by hand in different packages.
func TestSupportedLanguagesSpeakTheCatalogue(t *testing.T) {
	for _, code := range store.SupportedLanguages {
		if _, ok := messages[code]; !ok {
			t.Errorf("store.SupportedLanguages advertises %q but the auth catalogue does not speak it", code)
		}
		if _, ok := langNames[code]; !ok {
			t.Errorf("language %q has no endonym in langNames", code)
		}
	}
}

// TestMatchAcceptLanguage: resolution stays within the offered list and falls
// back to English when it is offered - the list is sorted, so its first entry
// says nothing about anybody's choice.
func TestMatchAcceptLanguage(t *testing.T) {
	offered := []string{"fr", "en"}
	cases := map[string]string{
		"fr-FR,fr;q=0.9,en;q=0.8": "fr",
		"en-US,en;q=0.9":          "en",
		"de-DE,de;q=0.9":          "en", // nothing offered matches -> English
		"":                        "en",
	}
	for header, want := range cases {
		if got := matchAcceptLanguage(header, offered); got != want {
			t.Errorf("matchAcceptLanguage(%q) = %q, want %q", header, got, want)
		}
	}
	// No English on offer: the first language there is.
	if got := matchAcceptLanguage("", []string{"de", "fr"}); got != "de" {
		t.Errorf("with no English offered, got %q, want de", got)
	}
}

// The checklist labels carry the number the policy asks for, so each one must
// hold exactly one %d - in every language. A translation that drops it renders
// "%!d(MISSING)" on a password page, and only for the people who read that
// language, which is precisely who would not report it.
func TestPasswordRuleLabelsKeepTheirNumber(t *testing.T) {
	for lang, m := range messages {
		for _, key := range pwRuleLabels {
			v, ok := m[key]
			if !ok {
				t.Errorf("locale %q is missing %q", lang, key)
				continue
			}
			if n := strings.Count(v, "%d"); n != 1 {
				t.Errorf("locale %q, key %q: %d occurrences of %%d, want 1 (%q)", lang, key, n, v)
			}
			if strings.Contains(strings.ReplaceAll(v, "%d", ""), "%") {
				t.Errorf("locale %q, key %q has a stray verb: %q", lang, key, v)
			}
		}
	}
}
