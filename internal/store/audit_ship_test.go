package store

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/tracing"
)

// Two identical actions in the same second - two routes saved at once - must
// leave for the collector at two instants: Loki takes the same line at the
// same time for a duplicate and keeps one, and the trail it holds then misses
// an event the database has.
func TestTwoAuditEventsInOneSecondLeaveAtTwoInstants(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	sent := make(chan string, 4)
	tracing.RegisterAuditStarter(func(tracing.Config) (func([]byte) error, error) {
		return func(b []byte) error { sent <- string(b); return nil }, nil
	})
	tracing.SetAuditScope(true, true)
	if err := tracing.ApplyAudit(tracing.Config{Endpoint: "http://collector"}, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tracing.ApplyAudit(tracing.Config{}, false); tracing.RegisterAuditStarter(nil) })

	for range 2 {
		if err := s.AddAuditEvent(ctx, AuditEvent{Action: "route.update", Target: "route", TargetID: "r1"}); err != nil {
			t.Fatal(err)
		}
	}
	var stamps []string
	stamp := regexp.MustCompile(`"timeUnixNano":"(\d+)"`)
	deadline := time.After(5 * time.Second)
	for len(stamps) < 2 {
		select {
		case batch := <-sent:
			for _, m := range stamp.FindAllStringSubmatch(batch, -1) {
				stamps = append(stamps, m[1])
			}
		case <-deadline:
			t.Fatalf("got %d events, want 2", len(stamps))
		}
	}
	if stamps[0] == stamps[1] {
		t.Fatalf("both events left at %s: Loki would keep one", stamps[0])
	}
}
