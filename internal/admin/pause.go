package admin

import (
	"net/http"

	"github.com/softwarity/meerkat/internal/store"
)

// The pause (Configuration > Snapshot): every write stops, on every node, so
// the database can be copied whole and the gateway restarted on the copy.
//
// Read by any console session - the console shows the paused state to whoever
// is looking, since it explains every refused change - and turned on or off by
// root alone, like the snapshot it is there for.
//
// Never stored: see store/pause.go. It travels to the other nodes as a signal,
// and a node that starts during a pause starts unpaused, which is the right
// answer for the node that starts on the new database.

func (a *API) registerPause(mux Mux) {
	mux.Handle("GET /api/backup/pause", a.authed(a.getPause))
	mux.Handle("PUT /api/backup/pause", a.rootOnly(a.putPause))
}

type pauseState struct {
	Paused bool `json:"paused"`
}

func (a *API) getPause(w http.ResponseWriter, _ *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, pauseState{Paused: a.st.Paused()})
}

func (a *API) putPause(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p pauseState
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed pause: "+err.Error())
		return
	}
	if p.Paused == a.st.Paused() {
		writeJSON(w, http.StatusOK, p)
		return
	}
	// The trail is written while it still can be: before pausing, after
	// resuming. A pause nobody can see in the audit is a pause nobody did.
	if p.Paused {
		a.auditEvent(r.Context(), actor, "gateway.pause", "backup", "", "", "", "")
	}
	a.setPaused(r, p.Paused)
	if !p.Paused {
		a.auditEvent(r.Context(), actor, "gateway.resume", "backup", "", "", "", "")
	}
	writeJSON(w, http.StatusOK, p)
}

// setPaused pauses or resumes this node, tells the others, and wakes the
// consoles.
func (a *API) setPaused(r *http.Request, on bool) {
	a.st.Pause(on)
	if a.Bus != nil {
		arg := "off"
		if on {
			arg = "on"
		}
		a.Bus.Signal(r.Context(), store.TopicPause, arg)
	}
	if a.PauseMoved != nil {
		a.PauseMoved()
	}
}
