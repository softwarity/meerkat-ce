package live

import (
	"context"
	"encoding/json"
	"strconv"

	livewire "github.com/softwarity/livewire/go"
	"github.com/softwarity/meerkat/internal/logging"
)

// LogsTopic is what the Logs screen subscribes to, to hear that this node
// wrote a line (OBS-03).
const LogsTopic = "logs"

// Logs tells the Logs screen that lines were written.
//
// It carries the number of the newest line and nothing else, on the same
// argument as RouteHealth: the screen then asks GET /api/logs for what came
// after the last one it holds, which is the one place that decides who may
// read the log. A burst of lines is one read: the library gathers the wakes.
//
// Served to the routing plane's perimeter only, the readers that endpoint
// answers.
type Logs struct{}

// NewLogs returns the source.
func NewLogs() *Logs { return &Logs{} }

// ReadQuery has nothing to read: every reader watches the same counter.
func (l *Logs) ReadQuery(_ json.RawMessage) (any, error) { return struct{}{}, nil }

// Key is constant: one read however many screens watch.
func (l *Logs) Key(_ any) string { return LogsTopic }

// Wake fires after a line is written.
func (l *Logs) Wake() <-chan struct{} { return logging.Written() }

// Read is one row whose version is the newest line's number.
func (l *Logs) Read(_ context.Context, _ any) (livewire.Window, error) {
	v := strconv.FormatInt(logging.LastSeq(), 10)
	total := 1
	return livewire.Window{
		Rows:  []livewire.Row{{ID: "lines", UpdatedAt: v, Data: map[string]any{"seq": v}}},
		Total: &total,
	}, nil
}
