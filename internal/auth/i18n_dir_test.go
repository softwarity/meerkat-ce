package auth

import (
	"regexp"
	"sort"
	"testing"
)

// A language is not only words: Arabic and Hebrew are read right to left, and
// the pages say so on <html dir> so the browser mirrors the whole layout. The
// flow CSS uses logical properties (padding-inline-start, text-align: start),
// so nothing else has to know.
func TestWritingDirection(t *testing.T) {
	for lang, want := range map[string]string{
		"ar": "rtl", "he": "rtl", "en": "ltr", "fr": "ltr", "ja": "ltr", "": "ltr",
	} {
		if got := Dir(lang); got != want {
			t.Errorf("Dir(%q) = %q, want %q", lang, got, want)
		}
	}
}

// An empty translation renders as nothing at all, on a page nobody is watching
// - worse than an untranslated one, which at least says something.
func TestNoEmptyTranslations(t *testing.T) {
	for lang, m := range messages {
		for k, v := range m {
			if v == "" {
				t.Errorf("locale %q: key %q is empty", lang, k)
			}
		}
	}
}

// A translation carries the same format verbs as its English source, in the
// same number: a message printed with fmt against a catalogue that lost its
// %s renders "%!s(MISSING)" to a user, in the one language nobody on the team
// reads.
func TestFormatVerbsSurviveTranslation(t *testing.T) {
	verb := regexp.MustCompile(`%[a-zA-Z]`)
	shape := func(s string) []string {
		v := verb.FindAllString(s, -1)
		sort.Strings(v)
		return v
	}
	ref := messages["en"]
	for lang, m := range messages {
		if lang == "en" {
			continue
		}
		for k, want := range ref {
			got := shape(m[k])
			expected := shape(want)
			if len(got) != len(expected) {
				t.Errorf("locale %q, key %q: format verbs %v, want %v", lang, k, got, expected)
				continue
			}
			for i := range got {
				if got[i] != expected[i] {
					t.Errorf("locale %q, key %q: format verbs %v, want %v", lang, k, got, expected)
					break
				}
			}
		}
	}
}
