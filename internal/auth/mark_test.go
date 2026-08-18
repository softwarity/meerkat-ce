package auth

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// The mark rides on every served page unless TWO things are true: the
// integrator asked for it to go, and they hold the right to ask. Either alone
// is wrong - a licence that strips it from someone who never requested it, or
// a switch that works without the licence behind it.
func TestTheMarkNeedsBothTheAskAndTheRight(t *testing.T) {
	mux, st := setupWithStore(t)
	ctx := context.Background()
	page := func() string {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login", nil))
		b, _ := io.ReadAll(rec.Result().Body)
		return string(b)
	}
	set := func(hide bool) {
		var b store.Branding
		_ = st.GetSetting(ctx, store.SettingBranding, &b)
		b.HideMark = hide
		if err := st.SetSetting(ctx, store.SettingBranding, b); err != nil {
			t.Fatal(err)
		}
	}

	// Out of the box: the mark is there, whichever image this is.
	if !strings.Contains(page(), "powered by softwarity/meerkat") {
		t.Fatal("a plain installation served no mark")
	}

	// Asked for. Removing it is what the Enterprise image sells, so the answer
	// is the image's: gone there, still there here. The community case is the
	// one that matters - an installation that wrote HideMark by hand, or kept
	// it from a database that once ran the other image, must not win it.
	set(true)
	if edition.Enterprise {
		if strings.Contains(page(), "powered by softwarity/meerkat") {
			t.Error("the Enterprise image kept a mark that was asked to go")
		}
		return
	}
	if !strings.Contains(page(), "powered by softwarity/meerkat") {
		t.Error("the community image dropped the mark")
	}

	// And putting the flag back restores it, which is the way out for anyone
	// who set it while running the other image.
	set(false)
	if !strings.Contains(page(), "powered by softwarity/meerkat") {
		t.Error("the mark did not come back")
	}
}
