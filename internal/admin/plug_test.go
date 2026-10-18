package admin

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
	"golang.org/x/crypto/ssh"
)

// The tunnel's switch and the address developers type are the routing
// plane's: an infra admin sets them, an application admin does not. Who holds
// the developer capability, and whether they deposited a key - by
// fingerprint, never the key - is the application's: its admin reads it, the
// infra admin does not.
func TestThePlugSettingBelongsToInfra(t *testing.T) {
	f := setupEE(t)
	ctx := context.Background()
	for _, u := range []store.User{
		{ID: "infra", Username: "infra", PasswordHash: "x", InfraAdmin: true, Enabled: true},
		{ID: "app", Username: "app", PasswordHash: "x", AppAdmin: true, Enabled: true},
		{ID: "devon", Username: "devon", Fullname: "Devon", PasswordHash: "x", Dev: true, Enabled: true},
		{ID: "dana", Username: "dana", PasswordHash: "x", Dev: true, Enabled: true},
	} {
		if err := f.api.st.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	sshPub, _ := ssh.NewPublicKey(pub)
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))) + " devon@laptop"
	if _, err := f.api.st.AddDevKey(ctx, "devon", line); err != nil {
		t.Fatal(err)
	}
	infraC, appC := issue(t, f.api.sm, "infra"), issue(t, f.api.sm, "app")

	if code, _ := f.call(t, "PUT", "/api/settings/plug", `{"enabled":true}`, appC); code != http.StatusForbidden {
		t.Errorf("an application admin opened the tunnel: %d", code)
	}
	if code, out := f.call(t, "PUT", "/api/settings/plug", `{"enabled":true,"host":"https://dev.example.com"}`, infraC); code != http.StatusUnprocessableEntity {
		t.Errorf("a URL was taken for a host: %d %s", code, out)
	}
	code, out := f.call(t, "PUT", "/api/settings/plug", `{"enabled":true,"host":"dev.example.com","port":30222}`, infraC)
	if code != http.StatusOK {
		t.Fatalf("an infra admin was refused: %d %s", code, out)
	}
	var got plugAnswer
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Enabled || got.Host != "dev.example.com" || got.Port != 30222 || got.DefaultPort != 22222 {
		t.Errorf("the answer does not say what was decided: %s", out)
	}
	if strings.Contains(out, `"developers"`) {
		t.Errorf("the tunnel's answer still lists the people: %s", out)
	}

	if code, _ := f.call(t, "GET", "/api/plug/developers", "", infraC); code != http.StatusForbidden {
		t.Errorf("an infra admin read the developers: %d", code)
	}
	if code, _ := f.call(t, "GET", "/api/plug/sessions", "", infraC); code != http.StatusForbidden {
		t.Errorf("an infra admin read who is plugged in: %d", code)
	}
	if code, _ := f.call(t, "POST", "/api/plug/sessions/x/close", "", infraC); code != http.StatusForbidden {
		t.Errorf("an infra admin ended a session: %d", code)
	}
	code, out = f.call(t, "GET", "/api/plug/developers", "", appC)
	if code != http.StatusOK {
		t.Fatalf("the application admin was refused the developers: %d %s", code, out)
	}
	var people plugPeople
	if err := json.Unmarshal([]byte(out), &people); err != nil {
		t.Fatal(err)
	}
	if !people.Enabled || len(people.Developers) != 2 {
		t.Fatalf("developers: %s", out)
	}
	for _, d := range people.Developers {
		switch d.Username {
		case "devon":
			if len(d.Keys) != 1 || !strings.HasPrefix(d.Keys[0].Fingerprint, "SHA256:") || d.Keys[0].Comment != "devon@laptop" {
				t.Errorf("devon's key is not shown by fingerprint: %+v", d)
			}
		case "dana":
			if len(d.Keys) != 0 {
				t.Errorf("dana has no key and one is shown: %+v", d)
			}
		}
	}
	if strings.Contains(out, "AAAA") {
		t.Errorf("a key travelled whole: %s", out)
	}
}
