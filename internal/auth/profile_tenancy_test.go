package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// A single-organisation instance never names the organisation: the console
// hides the notion, the identity headers freeze it, and the profile printing
// "Default" would introduce a word the user meets nowhere else and can act on
// in no way.
func TestProfileNamesTheOrganisationOnlyInMulti(t *testing.T) {
	profile := func(mode string) string {
		ctx := context.Background()
		st, err := store.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.Close() })
		if err := st.SetSetting(ctx, store.SettingTenancy, mode); err != nil {
			t.Fatal(err)
		}
		if err := st.SaveTenant(ctx, store.Tenant{ID: "acme", Name: "Acme Corp", Enabled: true}); err != nil {
			t.Fatal(err)
		}
		if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "jane", Enabled: true, PasswordHash: "x"}); err != nil {
			t.Fatal(err)
		}

		sm := session.NewManager(st)
		mux := http.NewServeMux()
		New(st, sm).Register(mux)

		rec := httptest.NewRecorder()
		if _, err := sm.Issue(ctx, rec, httptest.NewRequest("POST", "/login", nil), "u1"); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/profile", nil)
		for _, c := range rec.Result().Cookies() {
			req.AddCookie(c)
		}
		if err := sm.SetTenant(ctx, req, "acme"); err != nil {
			t.Fatalf("could not put the session in an organisation: %v", err)
		}
		out := httptest.NewRecorder()
		mux.ServeHTTP(out, req)
		return out.Body.String()
	}

	if body := profile(store.TenancySingle); strings.Contains(body, "Acme Corp") {
		t.Error("a single-organisation profile named the organisation")
	}
	// The other half, so the first is not passing for the wrong reason: where
	// there IS a choice, the profile says which one is active.
	if body := profile(store.TenancyMulti); !strings.Contains(body, "Acme Corp") {
		t.Error("a multi-organisation profile did not name the active organisation")
	}
}
