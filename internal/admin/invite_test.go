package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/mail"
	"github.com/softwarity/meerkat/internal/store"
)

// An administrator gives a person their way in by a password shown once or by
// a mailed link, never both (AUTH-25): the invitation shows no password, a
// refusal leaves no account behind, and the mailed reset keeps the current
// password until the link is followed.
func TestAnAccountIsInvitedByMail(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	var sent []mail.Message
	f.api.Mailer = func(_ context.Context, m mail.Message) error {
		sent = append(sent, m)
		return nil
	}

	// No relay yet: refused before anything exists.
	if code, out := f.call(t, "POST", "/api/users?invite=true", `{"username":"ann","email":"ann@example.com","enabled":true}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "mail relay") {
		t.Fatalf("no relay: %d %s", code, out)
	}
	if _, err := f.api.st.GetUserByUsername(ctx, "ann"); err == nil {
		t.Fatal("a refused invitation left an account behind")
	}

	if err := f.api.st.SetSetting(ctx, store.SettingSMTP, mail.Config{From: "gw@example.com", Host: "smtp.example.com", Port: 25}); err != nil {
		t.Fatal(err)
	}
	if code, out := f.call(t, "PUT", "/api/settings/tls/doors/app", `{"proxied":true,"names":["shop.example.org"],"proxyAddresses":["203.0.113.10"]}`, f.rootC); code != http.StatusOK {
		t.Fatalf("door: %d %s", code, out)
	}
	if code, out := f.call(t, "POST", "/api/users?invite=true", `{"username":"bob","enabled":true}`, f.rootC); code != http.StatusUnprocessableEntity || !strings.Contains(out, "e-mail address") {
		t.Fatalf("no address: %d %s", code, out)
	}

	code, out := f.call(t, "POST", "/api/users?invite=true", `{"username":"ann","email":"ann@example.com","enabled":true}`, f.rootC)
	if code != http.StatusCreated {
		t.Fatalf("invite: %d %s", code, out)
	}
	var created map[string]any
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatal(err)
	}
	if _, shown := created["password"]; shown || created["invited"] != true {
		t.Fatalf("an invitation shows no password: %s", out)
	}
	if len(sent) != 1 || !strings.Contains(sent[0].HTML, "https://shop.example.org/reset-password?token=") {
		t.Fatalf("the invitation mail with a link on the vouched name: %+v", sent)
	}

	// The reset link, mailed: the current password is left alone.
	u, err := f.api.st.GetUserByUsername(ctx, "ann")
	if err != nil {
		t.Fatal(err)
	}
	if code, out := f.call(t, "POST", "/api/users/"+u.ID+"/reset-password?send=mail", ``, f.rootC); code != http.StatusOK || strings.Contains(out, "password\"") {
		t.Fatalf("mailed reset: %d %s", code, out)
	}
	after, _ := f.api.st.GetUserByID(ctx, u.ID)
	if after.PasswordHash != u.PasswordHash || len(sent) != 2 {
		t.Fatalf("a mailed reset changed the password or sent nothing: %d sent", len(sent))
	}
}
