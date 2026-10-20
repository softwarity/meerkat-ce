package admin

import (
	"net/http"
	"time"

	"github.com/softwarity/meerkat/internal/devtunnel"
	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
	"golang.org/x/crypto/ssh"
)

// The developer tunnel's pages: Infra, Plug (DEV-11) and Application, Plug
// (DEV-13).
//
// The TUNNEL is infrastructure: it opens a port into the cluster and
// exercises the deployment's rights over it, which is whoever runs the
// routing plane's decision. Its own switch decides alone whether it runs; who
// may use it is the dev capability (DEV-01).
//
// The PEOPLE are the application's: who holds the developer capability and
// which key they deposited, who is plugged in now, and ending a run. The
// capability is granted on Application, Users, and withdrawing it already
// ends what the key holds, so the same administrator sees the runs and ends
// one.
func (a *API) registerPlug(mux Mux) {
	mux.Handle("GET /api/settings/plug", a.gw(a.getPlug))
	mux.Handle("PUT /api/settings/plug", a.infraAdmin(a.putPlug))
	mux.Handle("GET /api/plug/developers", a.appAdmin(a.listPlugDevelopers))
	mux.Handle("GET /api/plug/sessions", a.appAdmin(a.listPlugSessions))
	mux.Handle("POST /api/plug/sessions/{id}/close", a.appAdmin(a.closePlugSession))
}

// plugSession is one plug run as the page lists it (DEV-13): who, from which
// workstation, what it serves, and since when.
type plugSession struct {
	devtunnel.SessionView
	// Fullname and Workstation are read from the account and the deposited
	// key: the comment of its line, the name the developer gave the machine.
	Fullname    string `json:"fullname,omitempty"`
	Workstation string `json:"workstation,omitempty"`
}

// listPlugSessions answers "who is plugged into this cluster, now".
func (a *API) listPlugSessions(w http.ResponseWriter, r *http.Request, _ store.User) {
	out := []plugSession{}
	if a.Served == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	devs, err := a.st.Developers(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	names := map[string]string{}
	machines := map[string]string{}
	for _, d := range devs {
		names[d.Username] = d.Fullname
		for _, line := range d.Keys {
			if key, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(line)); err == nil {
				machines[ssh.FingerprintSHA256(key)] = comment
			}
		}
	}
	for _, s := range a.Served.Sessions() {
		out = append(out, plugSession{SessionView: s, Fullname: names[s.Who], Workstation: machines[s.Key]})
	}
	writeJSON(w, http.StatusOK, out)
}

// closeReason is what the developer reads in their terminal when an
// administrator ends their session.
const closeReason = "closed by an administrator of this gateway"

// closePlugSession ends one run on every node: the developer's command stops
// as on a Ctrl-C, with this reason, and what the run had set aside in the
// cluster is back at once. Not a ban - a new run is let in, as long as the key
// is: keeping somebody out is removing their key or their capability.
func (a *API) closePlugSession(w http.ResponseWriter, r *http.Request, actor store.User) {
	id := r.PathValue("id")
	if a.Served == nil || !a.Served.CloseSession(id, closeReason) {
		writeErr(w, http.StatusNotFound, "no plug session "+id+" is open: it may have ended already")
		return
	}
	a.auditEvent(r.Context(), actor, "plug.session.terminate", "plug", id, id, "", closeReason)
	w.WriteHeader(http.StatusNoContent)
}

// plugAnswer is the setting with what the page needs around it.
type plugAnswer struct {
	store.PlugSetting
	// Enterprise: the tunnel is in the Enterprise image only.
	Enterprise bool `json:"enterprise"`
	// Status is what the tunnel is doing on the node that answered.
	Status devtunnel.Status `json:"status"`
	// Version is the plug client this image hands out.
	Version string `json:"version,omitempty"`
	// DefaultPort is the tunnel's own port, the default for the published one.
	DefaultPort int `json:"defaultPort"`
	// DataOrigin is where the applications answer: the developer's profile
	// page, where the key is deposited, lives there.
	DataOrigin string `json:"dataOrigin"`
	// Exposure is how the runtime publishes the tunnel's port, and Address
	// the host and port the commands carry: what was recorded, else that,
	// else the defaults (devtunnel.Address).
	Exposure discovery.Exposure `json:"exposure"`
	Address  struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"address"`
	// Profile is the plug profile the commands name: the application's name,
	// cut down to what plug accepts (store.PlugProfile).
	Profile string `json:"profile"`
}

// plugPeople is Application, Plug: whether the tunnel is open at all, and who
// holds the developer capability, with their key if deposited - who could plug
// in, and who still has a key to deposit.
type plugPeople struct {
	Enabled    bool            `json:"enabled"`
	Developers []plugDeveloper `json:"developers"`
}

type plugDeveloper struct {
	Username string `json:"username"`
	Fullname string `json:"fullname,omitempty"`
	// Keys are the deposited keys, one per workstation, by fingerprint and
	// comment - never the key itself. Empty when none was deposited.
	Keys []plugKey `json:"keys"`
}

type plugKey struct {
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment,omitempty"`
}

func (a *API) plugState(r *http.Request) (plugAnswer, error) {
	ctx := r.Context()
	out := plugAnswer{
		PlugSetting: a.st.Plug(ctx), Enterprise: edition.Enterprise,
		Status: devtunnel.CurrentStatus(), Version: devtunnel.ClientVersion(),
		DefaultPort: store.DefaultPlugPort, DataOrigin: a.dataOrigin(r), Profile: a.st.PlugProfile(ctx),
	}
	out.Address.Host, out.Address.Port, out.Exposure = devtunnel.Address(ctx, out.PlugSetting)
	return out, nil
}

func (a *API) listPlugDevelopers(w http.ResponseWriter, r *http.Request, _ store.User) {
	ctx := r.Context()
	out := plugPeople{Enabled: edition.Enterprise && a.st.Plug(ctx).Enabled, Developers: []plugDeveloper{}}
	devs, err := a.st.Developers(ctx)
	if err != nil {
		a.internal(w, err)
		return
	}
	for _, d := range devs {
		p := plugDeveloper{Username: d.Username, Fullname: d.Fullname, Keys: []plugKey{}}
		for _, line := range d.Keys {
			if key, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(line)); err == nil {
				p.Keys = append(p.Keys, plugKey{Fingerprint: ssh.FingerprintSHA256(key), Comment: comment})
			}
		}
		out.Developers = append(out.Developers, p)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) getPlug(w http.ResponseWriter, r *http.Request) {
	out, err := a.plugState(r)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) putPlug(w http.ResponseWriter, r *http.Request, actor store.User) {
	var body store.PlugSetting
	if err := decodeStrict(r, &body); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed setting: "+err.Error())
		return
	}
	if err := store.SanitizePlug(&body); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if body.Enabled {
		if err := edition.Require("the developer tunnel (plug)"); err != nil {
			writeErr(w, http.StatusForbidden, err.Error())
			return
		}
	}
	before := a.st.Plug(r.Context())
	if err := a.st.SetSetting(r.Context(), store.SettingPlug, body); err != nil {
		a.internal(w, err)
		return
	}
	// This node looks at once, and the answer waits for it, so the page shows
	// what the tunnel does now; the others re-read the switch on their own
	// tick.
	devtunnel.WakeAndWait(2 * time.Second)
	a.auditUpdate(r.Context(), actor, "plug.configure", "settings", "", "", "", before, body)
	out, err := a.plugState(r)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
