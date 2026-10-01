package auth

import (
	"strings"
	"testing"
)

// The loader fills every catalogue with English so no page renders blank, which
// made the catalogues indistinguishable: asked what a language ships, all
// twenty answered "everything", and the editor's count of what was left to
// translate was zero for every language forever.
//
// This is that difference, kept apart. The catalogues are complete today (see
// TestWeShipNoHalfLanguage, which is what keeps them so), so the two questions
// happen to have the same answer - the test guards that they are still two
// questions, because the day a string is added they diverge again.
func TestALanguageIsAskedWhatItShipsItself(t *testing.T) {
	en := messages["en"]
	if len(en) == 0 {
		t.Fatal("no English catalogue")
	}
	total := 0
	for lang := range messages {
		if lang == "en" {
			continue
		}
		own, filled := 0, 0
		for key := range en {
			if EmbeddedString(lang, key) == "" {
				t.Errorf("%s renders nothing for %q: the backfill is meant to make that impossible", lang, key)
			}
			if OwnString(lang, key) == "" {
				own++
			} else {
				filled++
			}
		}
		if filled == 0 {
			t.Errorf("%s ships nothing of its own", lang)
		}
		total += own
	}
	if total == 0 {
		t.Log("every catalogue is complete - nothing left to translate")
	}
}

// A language added here stands on English: its pages read, in another language,
// rather than going blank. That is what makes a language one can create in the
// console usable the moment it exists, and it is the same fallback the data
// plane serves in production.
func TestAnAddedLanguageStandsOnEnglish(t *testing.T) {
	t.Cleanup(func() { SetLocaleOverrides(nil) })
	SetLocaleOverrides(map[string]map[string]string{
		"fr-CA": {"signIn": "Ouvrir la session"},
	})

	if !IsKnownLanguage("fr-CA") {
		t.Fatal("a language added here is not known")
	}
	c := catalogue("fr-CA")
	if got := c["signIn"]; got != "Ouvrir la session" {
		t.Errorf("its own wording is not used: %q", got)
	}
	// Everything it has not said yet is English, not empty.
	blank := 0
	for key := range messages["en"] {
		if c[key] == "" {
			blank++
		}
	}
	if blank > 0 {
		t.Errorf("%d strings render blank in an added language", blank)
	}
	if got := c["signOut"]; got != messages["en"]["signOut"] {
		t.Errorf("an untranslated string is %q, want the English %q", got, messages["en"]["signOut"])
	}
	// And it is counted as untranslated, or nobody would know to translate it.
	if OwnString("fr-CA", "signOut") != "" {
		t.Error("an added language claims to ship a wording it borrowed")
	}
	// A tag nobody added is English outright.
	if catalogue("xx-YZ")["signIn"] != messages["en"]["signIn"] {
		t.Error("an unknown tag is not English")
	}
	if IsKnownLanguage("xx-YZ") {
		t.Error("an unknown tag is offered as a language")
	}
	// Its name says which language it is a variant of.
	if n := LanguageName("fr-CA"); !strings.Contains(n, "CA") || n == "fr-CA" {
		t.Errorf("a variant is named %q", n)
	}
}
