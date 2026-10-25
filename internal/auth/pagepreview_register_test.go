package auth

import (
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// A page registered from outside (an Enterprise one: the notifications) joins
// the catalogue with its category's neighbours, is rendered like the others,
// and registering it twice keeps one entry. The community image registers
// nothing, which is the only way it does not offer the page.
func TestARegisteredPageJoinsItsCategory(t *testing.T) {
	saved := append([]PreviewPage(nil), PreviewPages...)
	t.Cleanup(func() { PreviewPages = saved })

	if _, ok := previewPageByKey("notifications"); ok {
		t.Fatal("the community catalogue offers the notifications page: it is Enterprise")
	}
	page := FlowPage("stand-in", `<p class="lead">stand-in {{.T.notifications}}</p>`)
	build := func(c FlowChrome) (*template.Template, any) { return page, struct{ FlowChrome }{AppPage(c)} }
	RegisterPreviewPage("stand-in", CategoryAccount, "Stand-in", "titleNotifications", build)
	RegisterPreviewPage("stand-in", CategoryAccount, "Stand-in", "titleNotifications", build)

	at, n, lastAccount := -1, 0, -1
	for i, p := range PreviewPages {
		if p.Key == "stand-in" {
			at, n = i, n+1
		} else if p.Category == CategoryAccount {
			lastAccount = i
		}
	}
	if n != 1 {
		t.Fatalf("registered twice, listed %d times", n)
	}
	if at != lastAccount+1 {
		t.Errorf("the page is at %d, not after the last account page (%d)", at, lastAccount)
	}
	if len(PreviewPages) != len(saved)+1 {
		t.Errorf("the catalogue went from %d to %d entries", len(saved), len(PreviewPages))
	}

	rec := httptest.NewRecorder()
	if !WritePagePreview(rec, "stand-in", store.DefaultTheme(), store.DefaultBranding(), "light",
		store.DefaultPageLayout(), "fr", false) {
		t.Fatal("a registered page does not render")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "stand-in "+EmbeddedString("fr", "notifications")) {
		t.Error("the page is not rendered in the language asked for")
	}
	if !strings.Contains(body, EmbeddedString("fr", "titleNotifications")) {
		t.Error("the page does not wear its own title")
	}
}
