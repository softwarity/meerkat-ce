package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

// The page offers whatever zones the BROWSER knows (Intl.supportedValuesOf),
// so the server ships no table - it only has to agree the name exists, which
// time.LoadLocation answers from the platform's own database. That is also the
// validation: a name nobody can resolve is refused here rather than stored to
// fail later, at formatting time, on someone else's screen.
func TestProfileTimezoneIsSavedAndValidated(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
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
	cookies := rec.Result().Cookies()

	post := func(tz string) int {
		req := httptest.NewRequest(http.MethodPost, "/profile/timezone",
			strings.NewReader(url.Values{"timezone": {tz}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		out := httptest.NewRecorder()
		mux.ServeHTTP(out, req)
		return out.Code
	}

	if code := post("Asia/Ho_Chi_Minh"); code != http.StatusSeeOther {
		t.Fatalf("saving a real zone answered %d", code)
	}
	u, err := st.GetUserByID(ctx, "u1")
	if err != nil || u.Timezone != "Asia/Ho_Chi_Minh" {
		t.Fatalf("timezone not stored: %q (%v)", u.Timezone, err)
	}

	if code := post("Mars/Olympus_Mons"); code != http.StatusBadRequest {
		t.Errorf("an unknown zone answered %d, want 400", code)
	}
	u, _ = st.GetUserByID(ctx, "u1")
	if u.Timezone != "Asia/Ho_Chi_Minh" {
		t.Errorf("a refused zone overwrote the stored one: %q", u.Timezone)
	}

	// A button with no text is a button nobody can use: the catalogue key it
	// renders is the whole label, so an empty one leaves a blank pill on the
	// page and no test would notice. This one does.
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	page := httptest.NewRecorder()
	mux.ServeHTTP(page, req)
	if page.Code != http.StatusOK {
		t.Fatalf("profile answered %d", page.Code)
	}
	body := page.Body.String()
	for _, id := range []string{"tz-detect", "tz-save"} {
		m := regexp.MustCompile(`<button[^>]*id="` + id + `"[^>]*>([^<]*)</button>`).FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("button %q is not on the profile page", id)
		}
		if strings.TrimSpace(m[1]) == "" {
			t.Errorf("button %q renders without a label", id)
		}
	}
}
