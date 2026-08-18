package auth

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// The community build knows ONE arrangement, and must say so by falling back.
//
// This package cannot link ee/layouts (that package imports this one), which
// is exactly the community image's situation: the block is not there. A page
// stamped mk-split with no rule to dress it is a BROKEN page, not a centred
// one, so the fallback happens where the layout is READ - and this pins it.
//
// The whole catalogue is exercised next door, in ee/layouts, where the blocks
// live.
func TestABuildWithoutTheBlocksServesTheCentredOne(t *testing.T) {
	for _, name := range []string{store.LayoutSplit, store.LayoutDrawer, store.LayoutBanner, store.LayoutBare} {
		mux, st := setupWithStore(t)
		l := store.PageLayout{Name: name}
		if err := store.SanitizePageLayout(&l); err != nil {
			t.Fatal(err)
		}
		if err := st.SetSetting(context.Background(), store.SettingPageLayout, l); err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login", nil))
		body, _ := io.ReadAll(rec.Result().Body)
		page := string(body)
		if !strings.Contains(page, "mk-"+store.LayoutCentered) {
			t.Errorf("%s: the page did not fall back to centred", name)
		}
		if strings.Contains(page, "mk-"+name) {
			t.Errorf("%s: the page claims an arrangement this build cannot draw", name)
		}
	}
}

// A layout that shipped away - a downgrade, a hand-edited database - must not
// leave every page wearing a class no rule dresses.
func TestALayoutNobodyShipsFallsBackToTheOriginal(t *testing.T) {
	mux, st := setupWithStore(t)
	if err := st.SetSetting(context.Background(), store.SettingPageLayout,
		store.PageLayout{Name: "carousel"}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login", nil))
	body, _ := io.ReadAll(rec.Result().Body)
	if page := string(body); !strings.Contains(page, "mk-"+store.LayoutCentered) {
		t.Error("an unknown layout was served as-is instead of falling back")
	}
}

// A page in an iframe answers its context by itself, on the CLIENT, exactly as
// the user button does (UIF-03) - so the bytes served are the same for
// everyone and no cache can hand one context the other's page.
func TestTheFramedArrangementIsDecidedByTheBrowser(t *testing.T) {
	mux, _ := setup(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login", nil))
	body, _ := io.ReadAll(rec.Result().Body)
	page := string(body)
	if !strings.Contains(page, "window.self !== window.top") {
		t.Error("nothing on the page notices it is framed")
	}
	if !strings.Contains(page, "body.mk-framed") {
		t.Error("the framed class has no rules, so noticing changes nothing")
	}
	// The mark stays. Removing it is what the white-label licence buys, and a
	// frame must not be a way to buy it.
	if strings.Contains(page, "body.mk-framed .powered") {
		t.Error("the framed arrangement touches the mark: an iframe would strip it for free")
	}
}

// The console's preview is served INSIDE an iframe, and it is the one page
// that must not answer that as a context: its frame is a scaled viewport whose
// whole job is showing a full page. It shipped without this and the preview
// lost its background and drew all five layouts identically - the screen meant
// to show the difference was the only screen that could not.
func TestThePreviewIsNotSomebodyElsesFrame(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteThemePreview(rec, store.DefaultTheme(), store.DefaultBranding(), "dark",
		store.PageLayout{Name: store.LayoutSplit, Side: "left"})
	body, _ := io.ReadAll(rec.Result().Body)
	page := string(body)

	if strings.Contains(page, "window.self !== window.top") {
		t.Error("the preview answers its own frame: it will go compact inside the console")
	}
}
