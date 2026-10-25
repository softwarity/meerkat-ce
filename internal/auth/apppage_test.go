package auth

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// An application page (AppPage) keeps the frame's theme and drops what dresses
// a sign-in: the brand block, the picture and the arrangement.
func TestAnAppPageDropsTheBrandAndTheArrangement(t *testing.T) {
	page := flowPage("app-test", `<p class="body-mark">content</p>`)
	base := flowChrome{
		Brand:     brandView{AppName: "Acme Wordmark", Tagline: "Acme tagline"},
		Layout:    store.PageLayout{Name: store.LayoutSplit},
		LayoutCSS: "body.mk-split .watch { color: red; }",
		Scheme:    "auto",
		T:         map[string]string{},
	}
	render := func(c flowChrome) string {
		rec := httptest.NewRecorder()
		writeFlow(rec, page, c, 200)
		b, _ := io.ReadAll(rec.Result().Body)
		return string(b)
	}
	signIn, app := render(base), render(AppPage(base))
	if !strings.Contains(signIn, "Acme Wordmark") || strings.Contains(signIn, " mk-app") {
		t.Fatal("a flow page lost its brand without being asked to")
	}
	for _, gone := range []string{"Acme Wordmark", "Acme tagline", "body.mk-split .watch", "mk-split"} {
		if strings.Contains(app, gone) {
			t.Errorf("the application page still carries %q", gone)
		}
	}
	if !strings.Contains(app, " mk-app") || !strings.Contains(app, "body-mark") {
		t.Error("the application page is not marked as one, or lost its own content")
	}
}
