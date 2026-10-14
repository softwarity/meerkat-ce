package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

// A passkey removed from the account and still on the device: the browser
// keeps offering it, and the refusal says so (X-Passkey-Unknown), for the
// page to tell the browser to forget it.
func TestARemovedPasskeyIsNamedUnknown(t *testing.T) {
	mux, _, st := setupAll(t)
	ctx := context.Background()
	kept, _ := json.Marshal(webauthn.Credential{ID: []byte("still-here")})
	if err := st.AddPasskey(ctx, "pk1", "u1", base64.RawURLEncoding.EncodeToString([]byte("still-here")), string(kept), "kept"); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/login/passkey/start", nil))
	var start struct {
		Challenge string `json:"challenge"`
		Options   struct {
			PublicKey struct {
				Challenge string `json:"challenge"`
			} `json:"publicKey"`
		} `json:"options"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatalf("start: %v %s", err, rec.Body.String())
	}

	b64 := base64.RawURLEncoding.EncodeToString
	rp := sha256.Sum256([]byte("example.com"))
	authData := append(rp[:], 0x05, 0, 0, 0, 1) // user present + verified, counter 1
	clientData, _ := json.Marshal(map[string]string{
		"type": "webauthn.get", "challenge": start.Options.PublicKey.Challenge, "origin": "http://example.com",
	})
	body, _ := json.Marshal(map[string]any{
		"id": b64([]byte("removed")), "rawId": b64([]byte("removed")), "type": "public-key",
		"response": map[string]any{
			"authenticatorData": b64(authData), "clientDataJSON": b64(clientData),
			"signature": b64([]byte("sig")), "userHandle": b64([]byte("u1")),
		},
	})
	req := httptest.NewRequest("POST", "/login/passkey/finish?challenge="+start.Challenge, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || rec.Header().Get("X-Passkey-Unknown") != "1" {
		t.Fatalf("a removed passkey: %d %q %s", rec.Code, rec.Header().Get("X-Passkey-Unknown"), rec.Body.String())
	}
}
