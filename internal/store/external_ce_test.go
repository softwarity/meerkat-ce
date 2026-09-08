//go:build !ee

package store

import (
	"errors"
	"strings"
	"testing"
)

// This image runs on the embedded database, and a URL naming another one has
// to be answered in words. What it must NOT be is a driver error:
// `sql: unknown driver "pgx"` tells a reader nothing about what to do next,
// and this is read at startup by somebody who has just set a variable.
func TestAnExternalDatabaseIsAnsweredInWords(t *testing.T) {
	if ExternalAvailable() {
		t.Fatal("a driver for an external database registered itself here")
	}
	_, err := OpenAt(t.TempDir(), "postgres://meerkat:meerkat@localhost:5432/meerkat")
	if !errors.Is(err, ErrExternalDatabase) {
		t.Fatalf("opening an external database answered %v, want the named refusal", err)
	}
	// It names the way forward, not only the wall.
	if !strings.Contains(err.Error(), "embedded") {
		t.Errorf("the answer does not say what IS available: %v", err)
	}
}

// And the embedded one still opens, which is the whole point of the default.
func TestTheEmbeddedDatabaseStillOpens(t *testing.T) {
	s, err := OpenAt(t.TempDir(), "")
	if err != nil {
		t.Fatalf("the embedded database did not open: %v", err)
	}
	_ = s.Close()
}
