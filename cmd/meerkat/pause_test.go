package main

import (
	"net/http/httptest"
	"testing"
)

// During a pause the control plane turns writes away, but not the ones the
// move is made of: resuming, the test of the target, the copy, and dismissing
// the copy remembered.
func TestThePauseLetsTheMoveThrough(t *testing.T) {
	for _, c := range []struct {
		method, path string
		refused      bool
	}{
		{"GET", "/api/routes", false},
		{"PUT", "/api/backup/pause", false},
		{"POST", "/api/backup/copy", false},
		{"DELETE", "/api/backup/last-copy", false},
		{"POST", "/api/backup/check", false},
		{"PUT", "/api/routes/x", true},
		{"DELETE", "/api/backup/pause-not", true},
	} {
		if got := refusesWhilePaused(httptest.NewRequest(c.method, c.path, nil)); got != c.refused {
			t.Errorf("%s %s: refused %v, want %v", c.method, c.path, got, c.refused)
		}
	}
}
