package admin

import (
	"net/http"
	"time"

	"github.com/softwarity/meerkat/internal/devtunnel"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
	"golang.org/x/crypto/ssh"
)

// The developer tunnel's page (DEV-11): Infra, Plug.
//
// INFRASTRUCTURE, beside developer mode rather than under it. Developer mode
// (Application, General) is a configuration - the installation offers its
// developers the tooling, the API docs, the simulated sign-ins. This opens a
// port into the cluster and exercises the deployment's rights over it, which
// is whoever runs the routing plane's decision. The tunnel runs only when
// both say so.
func (a *API) registerPlug(mux Mux) {
	mux.Handle("GET /api/settings/plug", a.gw(a.getPlug))
	mux.Handle("PUT /api/settings/plug", a.infraAdmin(a.putPlug))
}

// plugAnswer is the setting with what the page needs around it.
type plugAnswer struct {
	store.PlugSetting
	// Enterprise: the tunnel is in the Enterprise image only.
	Enterprise bool `json:"enterprise"`
	// DevMode is the other switch, read-only here: it belongs to the
	// application administrator, and the page says so rather than offering it.
	DevMode bool `json:"devMode"`
	// Production says MEERKAT_PRODUCTION closes the developer surface, so the
	// page explains a switch that cannot open instead of offering one.
	Production bool `json:"production"`
	// Status is what the tunnel is doing on the node that answered.
	Status devtunnel.Status `json:"status"`
	// Version is the plug client this image hands out.
	Version string `json:"version,omitempty"`
	// DefaultPort is the tunnel's own port, the default for the published one.
	DefaultPort int `json:"defaultPort"`
	// DataOrigin is where the applications answer: the developer's profile
	// page, where the key is deposited, lives there.
	DataOrigin string `json:"dataOrigin"`
	// Developers holds the dev capability, with their key if deposited: who
	// could plug in, and who still has a key to deposit.
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
		DevMode: a.st.DevMode(ctx), Production: store.Production(),
		Status: devtunnel.CurrentStatus(), Version: devtunnel.ClientVersion(),
		DefaultPort: store.DefaultPlugPort, DataOrigin: a.dataOrigin(r), Developers: []plugDeveloper{},
	}
	devs, err := a.st.Developers(ctx)
	if err != nil {
		return out, err
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
	return out, nil
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
	// what the tunnel does now; the others re-read both switches on their own
	// tick, which is how developer mode has always reached them.
	devtunnel.WakeAndWait(2 * time.Second)
	a.auditUpdate(r.Context(), actor, "plug.configure", "settings", "", "", "", before, body)
	out, err := a.plugState(r)
	if err != nil {
		a.internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
