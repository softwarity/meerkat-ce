package admin

import (
	"net/http"
	"testing"

	"github.com/softwarity/meerkat/internal/edition"
)

// An account CREATED with working hours of its own meets the rule an update
// meets: the Enterprise edition, and a time zone the gateway knows. Skipping
// them at creation stored a window the community edition does not sell, and a
// zone nobody knows, which fails open at every sign-in.
func TestAnAccountsOwnHoursAreCheckedAtCreation(t *testing.T) {
	f := setup(t)
	unknown := `{"username":"carol","enabled":true,"businessAccess":{"inherited":false,"timezone":"Mars/Olympus","days":[{"day":1,"from":"08:00","to":"18:00"}]}}`
	code, out := f.call(t, "POST", "/api/users", unknown, f.rootC)
	if !edition.Enterprise {
		if code != http.StatusForbidden {
			t.Fatalf("hours of its own on the Community edition: %d %s", code, out)
		}
		// Saying nothing about hours is not a window of its own.
		if code, out := f.call(t, "POST", "/api/users", `{"username":"dave","enabled":true}`, f.rootC); code != http.StatusCreated {
			t.Fatalf("an account without hours: %d %s", code, out)
		}
		return
	}
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("an unknown time zone: %d %s", code, out)
	}
	known := `{"username":"carol","enabled":true,"businessAccess":{"inherited":false,"timezone":"Europe/Paris","days":[{"day":1,"from":"08:00","to":"18:00"}]}}`
	if code, out := f.call(t, "POST", "/api/users", known, f.rootC); code != http.StatusCreated {
		t.Fatalf("a known time zone: %d %s", code, out)
	}
}
