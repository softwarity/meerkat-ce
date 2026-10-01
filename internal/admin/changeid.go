package admin

import (
	"context"
	"net/http"
	"strings"
	"sync"
)

// Meerkat-Change-Id: the identifiers of the writes a request just made, handed
// back on its own response (CONSOLE-13).
//
// WHY A SCREEN NEEDS THEM. Every write is published on the console's live
// channel, and a screen that edits something warns when that something moved
// under it - "admin changed this somewhere else". Without this it warned about
// its OWN save: the event arrives on the socket like anybody else's, and the
// account it names is no answer, because two tabs of one operator are the very
// case the warning exists for. Routes solved it with their revision; a setting
// has none. The event identifier is better than either: it names THIS write,
// exactly, and a screen that made it knows it.
//
// Only writes carry it, and only on the way out of an authenticated handler.
// GETs are left alone on purpose: the live channel itself is a GET that
// upgrades to a websocket, and wrapping its writer would break the upgrade.
const changeIDHeader = "Meerkat-Change-Id"

type changeIDs struct {
	mu  sync.Mutex
	ids []string
}

type changeIDsKey struct{}

func withChangeIDs(ctx context.Context) (context.Context, *changeIDs) {
	c := &changeIDs{}
	return context.WithValue(ctx, changeIDsKey{}, c), c
}

// noteChange records one event against the request that caused it. Called
// from the audit funnel, so every write that is published is also returned -
// and a write that is not audited is not published either, so there is nothing
// for a screen to recognise.
func noteChange(ctx context.Context, id string) {
	if c, ok := ctx.Value(changeIDsKey{}).(*changeIDs); ok && c != nil {
		c.mu.Lock()
		c.ids = append(c.ids, id)
		c.mu.Unlock()
	}
}

func (c *changeIDs) header() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.ids, ",")
}

// changeIDWriter sets the header at the last moment it can: when the handler
// starts its answer. The audit is written before the answer in every handler
// that writes, which is what makes the identifiers known by then.
type changeIDWriter struct {
	http.ResponseWriter
	ids     *changeIDs
	written bool
}

func (w *changeIDWriter) stamp() {
	if w.written {
		return
	}
	w.written = true
	if h := w.ids.header(); h != "" {
		w.ResponseWriter.Header().Set(changeIDHeader, h)
	}
}

func (w *changeIDWriter) WriteHeader(code int) {
	w.stamp()
	w.ResponseWriter.WriteHeader(code)
}

func (w *changeIDWriter) Write(b []byte) (int, error) {
	w.stamp()
	return w.ResponseWriter.Write(b)
}

// Unwrap keeps http.ResponseController reaching the real writer, as the
// statusWriter around this one does.
func (w *changeIDWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// isWrite says whether a request can change anything - the only ones worth
// telling a screen about.
func isWrite(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}
