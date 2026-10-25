package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// A refused settings save leaves NOTHING saved. The rate limits, the pages
// scheme and the portal were checked after the hours, the session, MFA and
// the policies had been written, so a 422 answered for a screen half applied.
func TestARefusedSettingsSaveWritesNothing(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	with := func(t *testing.T, edit func(map[string]any)) string {
		t.Helper()
		_, body := f.call(t, "GET", "/api/settings", "", f.rootC)
		var s map[string]any
		if err := json.Unmarshal([]byte(body), &s); err != nil {
			t.Fatal(err)
		}
		edit(s)
		out, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	mfa := func() bool {
		var on bool
		_ = f.api.st.GetSetting(ctx, store.SettingMFARequired, &on)
		return on
	}

	refusals := map[string]func(map[string]any){
		"a rate limit out of range": func(s map[string]any) {
			s["rateLimit"].(map[string]any)["loginAttempts"] = 5000
		},
		"an unknown pages scheme": func(s map[string]any) { s["pagesScheme"] = "sepia" },
		"an unknown portal mode": func(s map[string]any) {
			s["portal"].(map[string]any)["mode"] = "sideways"
		},
	}
	for name, edit := range refusals {
		t.Run(name, func(t *testing.T) {
			body := with(t, func(s map[string]any) {
				s["mfaRequired"] = !mfa()
				edit(s)
			})
			was := mfa()
			if code, out := f.call(t, "PUT", "/api/settings", body, f.rootC); code != http.StatusUnprocessableEntity {
				t.Fatalf("%s: %d %s", name, code, out)
			}
			if mfa() != was {
				t.Fatalf("%s was refused, and the MFA switch beside it was saved anyway", name)
			}
		})
	}

	// The global hours meet the time-zone check every other level meets.
	t.Run("an unknown time zone", func(t *testing.T) {
		body := with(t, func(s map[string]any) {
			s["businessAccess"] = map[string]any{"inherited": false, "timezone": "Mars/Olympus",
				"days": []any{map[string]any{"day": 1, "from": "08:00", "to": "18:00"}}}
		})
		want := http.StatusUnprocessableEntity
		if !edition.Enterprise {
			want = http.StatusForbidden // hours of its own are not sold there at all
		}
		if code, out := f.call(t, "PUT", "/api/settings", body, f.rootC); code != want {
			t.Fatalf("an unknown time zone: %d %s, want %d", code, out, want)
		}
	})
}
