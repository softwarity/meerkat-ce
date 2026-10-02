package logging

import (
	"log/slog"
	"testing"
	"time"
)

func TestBufferKeepsTheLinesStructuredWhateverTheFormat(t *testing.T) {
	Setup(Options{Format: FormatText, Level: "info"})
	before := LastSeq()
	slog.Info("upstream down", "route", "orders", "trace_id", "abc")
	slog.Debug("not at info")
	got := Recent(before, 0)
	if len(got) != 1 {
		t.Fatalf("want one line after %d, got %d: %+v", before, len(got), got)
	}
	e := got[0]
	if e.Message != "upstream down" || e.Level != "info" || e.TraceID != "abc" || e.Attrs["route"] != "orders" {
		t.Fatalf("line not kept as written: %+v", e)
	}
}

func TestRecentReturnsTheNewestWithinTheLimit(t *testing.T) {
	Setup(Options{Format: FormatText, Level: "info"})
	before := LastSeq()
	for i := 0; i < 10; i++ {
		slog.Info("line", "i", i)
	}
	got := Recent(before, 3)
	if len(got) != 3 || got[2].Seq != before+10 || got[0].Seq != before+8 {
		t.Fatalf("want the last three lines, got %+v", got)
	}
}

func TestBufferWrapsAround(t *testing.T) {
	Setup(Options{Format: FormatText, Level: "info"})
	for i := 0; i < BufferSize+5; i++ {
		slog.Info("fill")
	}
	all := Recent(0, 0)
	if len(all) != BufferSize {
		t.Fatalf("want %d lines kept, got %d", BufferSize, len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i].Seq != all[i-1].Seq+1 {
			t.Fatalf("lines out of order at %d: %d after %d", i, all[i].Seq, all[i-1].Seq)
		}
	}
	if all[len(all)-1].Seq != LastSeq() {
		t.Fatalf("newest line is not the last one written")
	}
}

func TestATalkativeLevelGoesBackOnItsOwn(t *testing.T) {
	Setup(Options{Format: FormatText, Level: "info"})
	SetLevelUntil("debug", time.Now().Add(50*time.Millisecond))
	if now, start, until := LevelState(); now != slog.LevelDebug || start != slog.LevelInfo || until.IsZero() {
		t.Fatalf("debug not set with a deadline: %v %v %v", now, start, until)
	}
	time.Sleep(150 * time.Millisecond)
	if now, _, until := LevelState(); now != slog.LevelInfo || !until.IsZero() {
		t.Fatalf("level did not come back: %v until %v", now, until)
	}
	// Back to the startup level by hand: nothing left to undo.
	SetLevelUntil("debug", time.Now().Add(time.Hour))
	SetLevelUntil("info", time.Time{})
	if _, _, until := LevelState(); !until.IsZero() {
		t.Fatalf("a deadline survived going back by hand")
	}
}

func TestNothingNewIsAnEmptyListNotNil(t *testing.T) {
	if got := Recent(LastSeq(), 0); got == nil || len(got) != 0 {
		t.Fatalf("want an empty list, got %#v", got)
	}
}
