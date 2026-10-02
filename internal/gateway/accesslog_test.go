package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/logging"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/store/dbtest"
)

// A machine calling with an API token is named twice on its access line: the
// account the token belongs to, and the token itself (OBS-03).
func TestTheAccessLineNamesTheToken(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	if err := st.CreateUser(ctx, store.User{ID: "u1", Username: "neo", Enabled: true, PasswordHash: "x"}); err != nil {
		t.Fatal(err)
	}
	const secret = "mk_access-token-0123456789abcdef"
	sum := sha256.Sum256([]byte(secret))
	if err := st.AddAPIToken(ctx, store.NewToken{ID: "t1", UserID: "u1", Name: "nightly-export",
		TokenHash: hex.EncodeToString(sum[:]), Prefix: secret[:10],
		Plane: store.PlaneData, Scope: store.ScopeFull}); err != nil {
		t.Fatal(err)
	}
	up := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(up.Close)
	route := pathRoute("r1", "orders", 1, "/**", up.URL)
	route.Access = store.Access{Level: store.AccessAuth}
	if err := st.SaveRoute(ctx, route); err != nil {
		t.Fatal(err)
	}
	rt := New(st, session.NewManager(st))
	if err := rt.Reload(ctx); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	logging.EnableAccessLogTo(&out, logging.FormatJSON)
	t.Cleanup(logging.DisableAccessLog)

	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &line); err != nil {
		t.Fatalf("not one JSON line: %q", out.String())
	}
	if line["user"] != "neo" || line["token"] != "nightly-export" {
		t.Fatalf("want user neo and token nightly-export, got %v", line)
	}
}
