package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/softwarity/meerkat/internal/gateway"
	"github.com/softwarity/meerkat/internal/session"
	"github.com/softwarity/meerkat/internal/store"
)

func body(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, rec.Body.String())
	}
	return out
}

// Liveness decides whether to KILL the process, so it must not fail for a
// dependency: a probe that went down with the database would restart every
// node at once, each killed for a fault none of them can fix by dying.
func TestLivenessDoesNotAskAboutTheStore(t *testing.T) {
	rec := httptest.NewRecorder()
	healthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("liveness answered %d", rec.Code)
	}
	if got := body(t, rec)["status"]; got != "UP" {
		t.Errorf("status %v, want UP", got)
	}
}

func TestReadinessNeedsTheStoreAndTheTable(t *testing.T) {
	st, err := store.OpenAt(t.TempDir(), "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	router := gateway.New(st, session.NewManager(st))

	// Before the first reload: the process is up and answers 404 to
	// everything it is about to route. An orchestrator must not send it
	// traffic, and the old handler said UP here.
	rec := httptest.NewRecorder()
	readyz(st, router)(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("before any reload: %d, want 503", rec.Code)
	}
	if got, _ := body(t, rec)["reason"].(string); got == "" {
		t.Error("a probe failure has to say which of the two it is")
	}

	if err := router.Reload(t.Context()); err != nil {
		t.Fatalf("reload: %v", err)
	}
	rec = httptest.NewRecorder()
	readyz(st, router)(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("after a reload: %d, want 200 - an empty table is a table", rec.Code)
	}

	// And with the store gone. It is the state that used to read as ready:
	// the compiled table still answers while no session, setting or reload
	// can be resolved any more.
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	rec = httptest.NewRecorder()
	readyz(st, router)(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("store closed: %d, want 503", rec.Code)
	}
}
