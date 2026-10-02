package logging

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"
)

// The last lines this node wrote, for the console's Logs screen (OBS-03).
//
// NOT READ BACK FROM THE OUTPUT. A process cannot read what it already wrote to
// its standard error, and redirecting that into a pipe of our own would take it
// away from `docker logs` and from the collector reading the containers. Every
// line goes through slog anyway, so the buffer is one more handler beside the
// output and the push - and it keeps the line STRUCTURED, before any format:
// whether the output says text, JSON or OTel, the screen gets the same fields
// and lays them out itself.
//
// THE OPERATIONAL LOG ONLY. The access log is one line per request, and at four
// hundred a second it would push everything else out of the buffer in a few
// seconds; the traffic screen and the traces answer what it would.
//
// PER NODE. Each gateway holds its own lines, and the screen says which node
// answered; the whole cluster's log is the collector's business.

// BufferSize is how many lines are kept: a few hours of a quiet gateway, a few
// minutes of one at debug.
const BufferSize = 5000

// Entry is one line as the screen gets it.
type Entry struct {
	// Seq numbers the lines this process wrote, from 1. It is what a reader
	// asks "what came after" with, and it starts over on a restart.
	Seq     int64          `json:"seq"`
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	TraceID string         `json:"traceId,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

type ring struct {
	mu    sync.Mutex
	lines []Entry // circular once full
	next  int     // where the next line goes once full
	seq   int64

	wake chan struct{}
}

var buffer = &ring{wake: make(chan struct{}, 1)}

func (b *ring) add(e Entry) {
	b.mu.Lock()
	b.seq++
	e.Seq = b.seq
	if len(b.lines) < BufferSize {
		b.lines = append(b.lines, e)
	} else {
		b.lines[b.next] = e
		b.next = (b.next + 1) % BufferSize
	}
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// Recent returns the lines after seq, oldest first, at most limit of them -
// the newest ones when there are more.
func Recent(after int64, limit int) []Entry {
	b := buffer
	b.mu.Lock()
	defer b.mu.Unlock()
	ordered := make([]Entry, 0, len(b.lines))
	ordered = append(ordered, b.lines[b.next:]...)
	ordered = append(ordered, b.lines[:b.next]...)
	start := len(ordered)
	for start > 0 && ordered[start-1].Seq > after {
		start--
	}
	out := ordered[start:]
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	// An empty list rather than nil: a reader asking "what came after" is
	// told nothing came, not handed a null to trip on.
	return append([]Entry{}, out...)
}

// LastSeq is the number of the newest line.
func LastSeq() int64 {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.seq
}

// Written fires after a line lands. One channel for the life of the process:
// the live channel listens once.
func Written() <-chan struct{} { return buffer.wake }

// bufferHandler writes a record into the buffer, with the fields the OTel
// format already computes.
type bufferHandler struct{ *otelHandler }

func newBufferHandler(lv slog.Leveler) *bufferHandler {
	return &bufferHandler{newOTelHandler(io.Discard, &slog.HandlerOptions{Level: lv})}
}

func (h *bufferHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return &bufferHandler{h.otelHandler.WithAttrs(a).(*otelHandler)}
}

func (h *bufferHandler) WithGroup(n string) slog.Handler {
	return &bufferHandler{h.otelHandler.WithGroup(n).(*otelHandler)}
}

func (h *bufferHandler) Handle(ctx context.Context, r slog.Record) error {
	line := h.build(ctx, r)
	e := Entry{Time: r.Time, Level: LevelName(r.Level), Message: r.Message}
	e.TraceID, _ = line["trace_id"].(string)
	e.Attrs, _ = line["attributes"].(map[string]any)
	buffer.add(e)
	return nil
}

// The level an operator turned the log to, and when it goes back.
//
// Debug is what somebody turns on to look at something, and forgets: a
// gateway left at debug fills a disk. So a level more talkative than the one
// chosen at startup comes with a deadline, after which the startup level is
// back on its own.

var (
	startup  slog.Level
	revertMu sync.Mutex
	revertAt time.Time
	revertT  *time.Timer
)

// SetLevelUntil turns the level to name, and back to the startup level at
// until when that is not zero.
func SetLevelUntil(name string, until time.Time) slog.Level {
	l := SetLevel(name)
	revertMu.Lock()
	defer revertMu.Unlock()
	if revertT != nil {
		revertT.Stop()
		revertT = nil
	}
	revertAt = time.Time{}
	if until.IsZero() || l == startup {
		return l
	}
	revertAt = until
	revertT = time.AfterFunc(time.Until(until), func() {
		revertMu.Lock()
		revertAt, revertT = time.Time{}, nil
		revertMu.Unlock()
		level.Set(startup)
		slog.Info("log level back to its startup value", "level", LevelName(startup))
	})
	return l
}

// LevelState is the level now, the one chosen at startup, and when the first
// returns to the second (zero when it does not).
func LevelState() (now, start slog.Level, until time.Time) {
	revertMu.Lock()
	defer revertMu.Unlock()
	return level.Level(), startup, revertAt
}
