package admin

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/logging"
	"github.com/softwarity/meerkat/internal/store"
)

// The gateway's own log, read and turned up from the console (OBS-03).
//
// What is read is THIS NODE's buffer (logging/buffer.go): the last lines it
// wrote, structured. The answer names the node, because in a cluster the
// console is talking to one of them and the others' lines are elsewhere - in
// the collector, which is where a whole cluster's log belongs.
//
// The level, on the other hand, is the INSTALLATION's: turning debug on to
// look at something must not depend on which node the load balancer picked,
// so the change is told to every node. It is not stored. A restart comes back
// at the level its manifest says, which is what an operator expects of a
// level turned up by hand.

// DebugFor is how long a level more talkative than the startup one lasts
// before the gateway goes back on its own.
const DebugFor = 30 * time.Minute

func (a *API) registerLogs(mux Mux) {
	mux.Handle("GET /api/logs", a.infraAdmin(a.readLogs))
	mux.Handle("PUT /api/logs/level", a.infraAdmin(a.putLogLevel))
}

type logLevelState struct {
	Level   string `json:"level"`
	Startup string `json:"startup"`
	// Until is when the level goes back to Startup, absent when it does not.
	Until *time.Time `json:"until,omitempty"`
}

func currentLogLevel() logLevelState {
	now, start, until := logging.LevelState()
	s := logLevelState{Level: logging.LevelName(now), Startup: logging.LevelName(start)}
	if !until.IsZero() {
		s.Until = &until
	}
	return s
}

type logsAnswer struct {
	logLevelState
	// Node is which gateway answered: the lines are its own.
	Node    string          `json:"node"`
	Last    int64           `json:"last"`
	Entries []logging.Entry `json:"entries"`
}

// readLogs answers the lines after ?after (a line number), at most ?limit of
// them, the newest when there are more.
func (a *API) readLogs(w http.ResponseWriter, r *http.Request, _ store.User) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > logging.BufferSize {
		limit = logging.BufferSize
	}
	writeJSON(w, http.StatusOK, logsAnswer{
		logLevelState: currentLogLevel(),
		Node:          nodeName(),
		Last:          logging.LastSeq(),
		Entries:       logging.Recent(after, limit),
	})
}

// nodeName is what the runtime calls this node: HOSTNAME, which Kubernetes
// sets to the pod's name and Docker to the container's id, and the OS's host
// name otherwise.
func nodeName() string {
	if h := os.Getenv("HOSTNAME"); h != "" {
		return h
	}
	h, _ := os.Hostname()
	return h
}

var logLevels = []string{"debug", "info", "warn", "error"}

func (a *API) putLogLevel(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body struct {
		Level string `json:"level"`
	}
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed level: "+err.Error())
		return
	}
	name := strings.ToLower(strings.TrimSpace(body.Level))
	known := false
	for _, l := range logLevels {
		known = known || l == name
	}
	if !known {
		writeErr(w, http.StatusUnprocessableEntity, "unknown log level "+strconv.Quote(body.Level)+": allowed are "+strings.Join(logLevels, ", "))
		return
	}
	before := currentLogLevel()
	until := time.Time{}
	if _, start, _ := logging.LevelState(); logging.ParseLevel(name) < start {
		until = time.Now().Add(DebugFor)
	}
	ApplyLogLevel(name, until)
	if a.Bus != nil {
		a.Bus.Signal(r.Context(), store.TopicLogLevel, EncodeLogLevel(name, until))
	}
	after := currentLogLevel()
	slog.Info("log level changed", "level", after.Level, "by", actor.Username)
	a.auditUpdate(r.Context(), actor, "logs.level", "settings", "", "", "", before, after)
	writeJSON(w, http.StatusOK, after)
}

// ApplyLogLevel sets this node's level, until a deadline when it has one.
func ApplyLogLevel(name string, until time.Time) { logging.SetLevelUntil(name, until) }

// EncodeLogLevel is the signal's argument: the level, a space, and the
// deadline in Unix seconds (0 for none).
func EncodeLogLevel(name string, until time.Time) string {
	var u int64
	if !until.IsZero() {
		u = until.Unix()
	}
	return name + " " + strconv.FormatInt(u, 10)
}

// ApplyLogLevelSignal is what another node does with that argument.
func ApplyLogLevelSignal(arg string) {
	name, unix, _ := strings.Cut(arg, " ")
	u, _ := strconv.ParseInt(unix, 10, 64)
	until := time.Time{}
	if u > 0 {
		until = time.Unix(u, 0)
	}
	ApplyLogLevel(name, until)
}
