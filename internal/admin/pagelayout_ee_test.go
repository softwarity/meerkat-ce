package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
)

// Arranging the built-in pages is part of white-label - the same purchase as
// removing the mark: making these pages look like the integrator's product
// rather than ours.
//
// Four behaviours, and each one is a decision rather than a detail. They are
// pinned here because a settings PUT carries the WHOLE payload, so the obvious
// implementation (refuse any non-default arrangement) breaks saving a locale
// on a community instance, and nothing in the console would have said why.
func TestArrangingThePagesIsSoldWithTheMark(t *testing.T) {
	settingsBody := func(t *testing.T, f fixture, layout string) string {
		t.Helper()
		code, body := f.call(t, "GET", "/api/settings", "", f.rootC)
		if code != http.StatusOK {
			t.Fatalf("read settings = %d", code)
		}
		var s map[string]any
		if err := json.Unmarshal([]byte(body), &s); err != nil {
			t.Fatal(err)
		}
		s["pageLayout"] = json.RawMessage(layout)
		out, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}

	t.Run("a community instance cannot change it", func(t *testing.T) {
		f := communityFixture(t)
		code, body := f.call(t, "PUT", "/api/settings",
			settingsBody(t, f, `{"name":"split","side":"left"}`), f.rootC)
		if edition.Enterprise {
			if code != http.StatusOK {
				t.Fatalf("the Enterprise image must allow it: %d %s", code, body)
			}
			return
		}
		if code != http.StatusUnprocessableEntity {
			t.Fatalf("change = %d %s, want 422", code, body)
		}
		if !strings.Contains(body, "Enterprise edition") {
			t.Fatalf("the refusal must say which edition it needs: %s", body)
		}
	})

	// The gate is on the CHANGE. Every other screen sends the current layout
	// back untouched, so refusing on the VALUE would make a community instance
	// unable to save its languages - a screen breaking for a reason written on
	// another screen.
	t.Run("the rest of the settings still save", func(t *testing.T) {
		f := communityFixture(t)
		code, body := f.call(t, "PUT", "/api/settings",
			settingsBody(t, f, `{"name":"centered"}`), f.rootC)
		if code != http.StatusOK {
			t.Fatalf("saving with the layout untouched = %d %s, want 200", code, body)
		}
	})

	// And the way out is never barred: an instance must not be stranded on an
	// arrangement it cannot leave.
	t.Run("going back to the default needs nothing", func(t *testing.T) {
		f := communityFixture(t)
		if code, body := f.call(t, "PUT", "/api/settings",
			settingsBody(t, f, `{"name":"centered"}`), f.rootC); code != http.StatusOK {
			t.Fatalf("returning to the default = %d %s, want 200", code, body)
		}
		want := http.StatusUnprocessableEntity
		if edition.Enterprise {
			want = http.StatusOK
		}
		if code, _ := f.call(t, "PUT", "/api/settings",
			settingsBody(t, f, `{"name":"bare"}`), f.rootC); code != want {
			t.Fatalf("moving on to a paid arrangement = %d, want %d", code, want)
		}
	})
}
