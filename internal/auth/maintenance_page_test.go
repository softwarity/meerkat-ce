package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// The unavailable page is a page this gateway SERVES, so it wears what every
// other served page wears. It used to be a block of HTML with none of it, and
// it is the page every visitor meets during an outage - the one they judge the
// installation on.
func TestTheUnavailablePageIsADressedPage(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := t.Context()
	if err := st.SetSetting(ctx, store.SettingBranding, store.Branding{
		AppName: "Acme Portal", Logo: "data:image/svg+xml;base64,PHN2Zy8+",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, store.SettingLanguages, []string{"en", "fr"}); err != nil {
		t.Fatal(err)
	}
	h := New(st, session.NewManager(st))

	rec := httptest.NewRecorder()
	h.ServeMaintenance(rec, httptest.NewRequest(http.MethodGet, "/anything", nil), store.ReasonUpgrade, 0, "/anything?x=1")
	res := rec.Result()
	body := rec.Body.String()

	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", res.StatusCode)
	}
	if res.Header.Get("Retry-After") == "" || res.Header.Get("Cache-Control") != "no-store" {
		t.Error("the headers a 503 owes a client are missing")
	}
	for what, want := range map[string]string{
		"the chosen reason, translated": "An update is being rolled out.",
		"when service is expected":      "Service will be back shortly.",
		"the application name":          "Acme Portal",
		// Not the raw data URI: html/template escapes the base64's "+" to
		// &#43; in an attribute, which is correct and is what a browser
		// decodes back.
		"the logo":              `img class="applogo" src="data:image/svg`,
		"the language switcher": "lang-menu",
		"the colour scheme":     "scheme-cycle",
		"the way through":       "/anything?x=1",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("%s is not on the page", what)
		}
	}
}

// A visitor is shown no way through, because there is none for them.
func TestAVisitorIsOfferedNothing(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	h := New(st, session.NewManager(st))

	rec := httptest.NewRecorder()
	h.ServeMaintenance(rec, httptest.NewRequest(http.MethodGet, "/", nil), store.ReasonNone, 0, "")
	// The class name alone also appears in the shared stylesheet, so the
	// element is what gets looked for.
	if strings.Contains(rec.Body.String(), `class="maint-go"`) {
		t.Error("a visitor was shown a way through")
	}
}

// The stripe lands in a document this product did not write, whose encoding is
// often undeclared - an upstream answering "text/html" with no charset leaves
// the browser on its own default, and a UTF-8 accent written straight in comes
// out as two wrong letters. Which is exactly what it did.
func TestTheStripeCarriesNoBytesABrowserCouldMisread(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.SetSetting(t.Context(), store.SettingLanguages, []string{"en", "fr"}); err != nil {
		t.Fatal(err)
	}
	h := New(st, session.NewManager(st))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: "MEERKAT_LANG", Value: "fr"})
	stripe := h.MaintenanceStripe(req)

	for i := range len(stripe) {
		if stripe[i] > 127 {
			t.Fatalf("byte %d is 0x%x: the stripe must be ASCII, whatever the page it lands in", i, stripe[i])
		}
	}
	// And it is really the French one, entities and all.
	if !strings.Contains(stripe, "&#233;") {
		t.Error("the accented text did not survive as a numeric reference")
	}
}

// How long away is a rounded DURATION, never a moment: the operator typed an
// estimate in hours, and a page answering to the minute turns it into a
// promise a maintenance window famously breaks. It is also recomputed at every
// render, so it shrinks on its own.
func TestHowLongAwayIsRoundedAndNeverAMoment(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	h := New(st, session.NewManager(st))

	// The thresholds are set against the durations the console OFFERS, and
	// they have to hold on both sides: eight hours must not inflate into "a
	// day or two", and a day must not deflate into "a few hours" after one
	// hour has passed, when twenty-three are left.
	cases := map[time.Duration]string{
		20 * time.Minute:   "within the hour",
		5 * time.Hour:      "within a few hours",
		8 * time.Hour:      "within a few hours", // what the console offers as hours reads as hours
		11 * time.Hour:     "within a few hours",
		23 * time.Hour:     "within a day or so",
		4 * 24 * time.Hour: "within a few days",
		-2 * time.Hour:     "back shortly", // already past: nothing anybody can act on
	}
	for left, want := range cases {
		rec := httptest.NewRecorder()
		h.ServeMaintenance(rec, httptest.NewRequest(http.MethodGet, "/", nil),
			store.ReasonMaintenance, time.Now().Add(left).Unix(), "")
		body := rec.Body.String()
		if !strings.Contains(body, want) {
			t.Errorf("%v away should say %q", left, want)
		}
		// Never a clock reading, whatever the distance.
		for _, precise := range []string{":0", ":1", ":2", ":3", ":4", ":5"} {
			if strings.Contains(bodyOf(body), precise) {
				t.Errorf("%v away put a time on the page", left)
				break
			}
		}
	}
}

// bodyOf keeps the assertion above off the stylesheet and the scripts, which
// are full of colons.
func bodyOf(page string) string {
	i := strings.Index(page, `class="maint-when"`)
	if i < 0 {
		return ""
	}
	j := strings.Index(page[i:], "</p>")
	return page[i : i+j]
}
