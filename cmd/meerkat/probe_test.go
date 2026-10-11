package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestTheHealthVerbSaysWhatTheProbeAnswered: 0 on a 2xx, 1 on anything else
// or nothing at all, 2 on an address that is not one - what a Swarm or
// compose healthcheck reads, in an image with no wget to ask.
func TestTheHealthVerbSaysWhatTheProbeAnswered(t *testing.T) {
	ready := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !ready {
			http.Error(w, `{"status":"starting"}`, http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"status":"ready"}`)
	}))
	defer srv.Close()
	var out, errs strings.Builder
	if code := runHealth([]string{srv.URL + "/readyz"}, &out, &errs); code != 0 || !strings.Contains(out.String(), "ready") {
		t.Fatalf("ready: %d %q %q", code, out.String(), errs.String())
	}
	ready = false
	errs.Reset()
	if code := runHealth([]string{srv.URL + "/readyz"}, io.Discard, &errs); code != 1 || !strings.Contains(errs.String(), "503") {
		t.Fatalf("not ready: %d %q", code, errs.String())
	}
	srv.Close()
	if code := runHealth([]string{srv.URL + "/readyz"}, io.Discard, io.Discard); code != 1 {
		t.Fatalf("nobody there: %d", code)
	}
	if code := runHealth([]string{"127.0.0.1:8080/readyz"}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("not an address: %d", code)
	}
}
