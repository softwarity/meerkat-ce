package auth

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// A preview speaks the language it was asked for. Every one of these pages is
// translated, and a preview stuck on English would show the palette and never
// the TEXT - which is half of what a page looks like, and all of what somebody
// correcting a translation needs to see.
func TestAPagePreviewSpeaksTheAskedLanguage(t *testing.T) {
	for _, tc := range []struct{ locale, want, dir string }{
		{"fr", messages["fr"]["signIn"], "ltr"},
		{"ja", messages["ja"]["signIn"], "ltr"},
		{"ar", messages["ar"]["signIn"], "rtl"},
		{"he", messages["he"]["signIn"], "rtl"},
	} {
		rec := httptest.NewRecorder()
		if !WritePagePreview(rec, "login", store.DefaultTheme(), store.DefaultBranding(),
			"dark", store.DefaultPageLayout(), tc.locale, true) {
			t.Fatalf("%s: the login page is not in the catalogue", tc.locale)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		page := string(body)
		if tc.want != "" && !strings.Contains(page, tc.want) {
			t.Errorf("%s: the page does not carry %q", tc.locale, tc.want)
		}
		if !strings.Contains(page, `lang="`+tc.locale+`"`) {
			t.Errorf("%s: <html lang> does not say so", tc.locale)
		}
		// Arabic and Hebrew read the other way, and a page that does not say
		// so lays every form out backwards.
		if !strings.Contains(page, `dir="`+tc.dir+`"`) {
			t.Errorf("%s: <html dir> is not %s", tc.locale, tc.dir)
		}
	}
}

// A language we do not embed falls back to English rather than rendering a
// page of empty strings: every label resolves through the catalogue, so a
// missing one blanks the whole page at once.
func TestAnUnknownLocaleFallsBackToEnglish(t *testing.T) {
	for _, locale := range []string{"", "xx", "klingon"} {
		rec := httptest.NewRecorder()
		WritePagePreview(rec, "login", store.DefaultTheme(), store.DefaultBranding(),
			"dark", store.DefaultPageLayout(), locale, true)
		body, _ := io.ReadAll(rec.Result().Body)
		if page := string(body); !strings.Contains(page, messages["en"]["signIn"]) {
			t.Errorf("locale %q did not fall back to English", locale)
		}
	}
}

// Every catalogue key a fixture reads must EXIST. A missing key resolves to an
// empty string, and an empty string is invisible: the sign-in page and the
// refused-sign-in page rendered identically for a while, because the error the
// second was supposed to carry was spelled wrong.
func TestFixturesOnlyUseKeysThatExist(t *testing.T) {
	en := messages["en"]
	for _, key := range []string{"errInvalidCreds", "refusedRoles", "mfaOn"} {
		if en[key] == "" {
			t.Errorf("a fixture reads %q, which the catalogue does not define", key)
		}
	}
	// The sign-in page carries its refusal. There used to be two entries, one
	// with and one without, and they rendered identically because the key was
	// misspelled - so the one that is kept is the one that shows something.
	rec := httptest.NewRecorder()
	WritePagePreview(rec, "login", store.DefaultTheme(), store.DefaultBranding(),
		"dark", store.DefaultPageLayout(), "en", true)
	b, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(b), en["errInvalidCreds"]) {
		t.Error("the sign-in page does not carry the refusal it is meant to show")
	}
}

// A page's title key is one the catalogue HAS.
//
// Five of them were not: titleSelectTenant where the catalogue says
// titleChooseTenant, titleSignInCode against titleSigninCode, and three more.
// Nothing failed - c.T[missing] is the empty string, so the preview quietly
// wore the fallback title and four real wordings sat on no screen at all.
// A typo in a key is invisible by construction; this is what makes it loud.
func TestEveryPageNamesATitleThatExists(t *testing.T) {
	en := messages["en"]
	for _, p := range PreviewPages {
		if p.title == "" {
			continue // an English-only screen titles itself in its fixture
		}
		if _, ok := en[p.title]; !ok {
			t.Errorf("%s names the title key %q, which the catalogue does not have",
				p.Key, p.title)
		}
	}
}
