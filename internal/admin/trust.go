package admin

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/store"
)

// The authorities the gateway trusts when it calls an upstream in HTTPS
// (SSL-09), under Infra, TLS. Apart from the certificate pool on purpose: the
// pool is what the gateway PRESENTS, with private keys; this is what it
// ACCEPTS, public certificates only - a PEM pasted, or read from a file chosen
// or dropped in the browser.
func (a *API) registerTrust(mux Mux) {
	mux.Handle("GET /api/trusted-authorities", a.infraAdmin(a.listTrust))
	mux.Handle("POST /api/trusted-authorities", a.infraAdmin(a.createTrust))
	mux.Handle("PUT /api/trusted-authorities/{id}", a.infraAdmin(a.updateTrust))
	mux.Handle("DELETE /api/trusted-authorities/{id}", a.infraAdmin(a.deleteTrust))
	mux.Handle("GET /api/trusted-authorities/platform", a.infraAdmin(a.listPlatformTrust))
	mux.Handle("POST /api/trusted-authorities/platform", a.infraAdmin(a.takePlatformTrust))
	mux.Handle("GET /api/trusted-authorities/https-switch", a.infraAdmin(a.listHTTPSSwitch))
}

// The platform's own authorities (discovery.FindAuthorities): what OpenShift,
// cert-manager, trust-manager or the cluster publish, found rather than
// named, so the person picks instead of fetching a file. Swapped in tests.
var (
	findAuthorities = discovery.FindAuthorities
	takeAuthority   = discovery.TakeAuthority
)

// platformAuthority is one found authority as the console lists it: what its
// certificates say when they are known already - an issuer's are asked of
// cert-manager only when it is taken.
type platformAuthority struct {
	Ref          string                `json:"ref"`
	Source       string                `json:"source"`
	Name         string                `json:"name"`
	Ready        bool                  `json:"ready"`
	Why          string                `json:"why,omitempty"`
	Certificates []certs.AuthorityCert `json:"certificates"`
	// Signs is what it signs here (trustsigns.go), indicative; Preselect says
	// it signs a route the gateway refuses today.
	Signs     []signedItem `json:"signs"`
	Preselect bool         `json:"preselect,omitempty"`
}

type platformAnswer struct {
	Kubernetes bool                   `json:"kubernetes"`
	Found      []platformAuthority    `json:"found"`
	Notes      []discovery.SourceNote `json:"notes"`
	// Unmatched are the routes in https no offered authority accounts for.
	Unmatched []unmatchedRoute `json:"unmatched"`
}

// trustedFingerprints are the certificates already trusted, and the platform
// references they were taken from, to leave out what the platform offers
// twice: an issuer's certificate is not known before it is taken, its
// reference is.
func (a *API) trustedFingerprints(ctx context.Context) (map[string]bool, error) {
	list, err := a.st.ListTrustedAuthorities(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, t := range list {
		if t.Source != "" {
			seen["ref:"+t.Source] = true
		}
		if found, err := certs.ParseAuthorities([]byte(t.PEM)); err == nil {
			for _, c := range certs.DescribeAuthorities(found) {
				seen[c.Fingerprint] = true
			}
		}
	}
	return seen, nil
}

// allTrusted says every certificate of pem is trusted already.
func allTrusted(pem string, seen map[string]bool) bool {
	found, err := certs.ParseAuthorities([]byte(pem))
	if err != nil {
		return false
	}
	for _, c := range certs.DescribeAuthorities(found) {
		if !seen[c.Fingerprint] {
			return false
		}
	}
	return true
}

func (a *API) listPlatformTrust(w http.ResponseWriter, r *http.Request, _ store.User) {
	seen, err := a.trustedFingerprints(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	res := findAuthorities(r.Context())
	out := platformAnswer{Kubernetes: res.Kubernetes, Found: []platformAuthority{}, Notes: append([]discovery.SourceNote{}, res.Notes...), Unmatched: []unmatchedRoute{}}
	signs := map[string][]signedItem{}
	if res.Kubernetes {
		var unmatched []unmatchedRoute
		var notes []discovery.SourceNote
		signs, unmatched, notes = a.signsOf(r.Context(), res.Found)
		out.Unmatched = append(out.Unmatched, unmatched...)
		out.Notes = append(out.Notes, notes...)
	}
	for _, f := range res.Found {
		if seen["ref:"+f.Ref] {
			continue
		}
		p := platformAuthority{Ref: f.Ref, Source: f.Source, Name: f.Name, Ready: f.Ready, Why: f.Why, Certificates: []certs.AuthorityCert{}, Signs: []signedItem{}}
		p.Signs = append(p.Signs, signs[f.Ref]...)
		for _, s := range p.Signs {
			p.Preselect = p.Preselect || (s.Refused && f.Ready)
		}
		if f.PEM != "" {
			if allTrusted(f.PEM, seen) {
				continue
			}
			found, err := certs.ParseAuthorities([]byte(f.PEM))
			if err != nil {
				p.Ready, p.Why = false, err.Error()
			} else {
				p.Certificates = certs.DescribeAuthorities(found)
			}
		}
		out.Found = append(out.Found, p)
	}
	// What signs a refused route first, then what signs something here, then
	// what was seen signing nothing - each kept in the platform's order.
	rank := func(p platformAuthority) int {
		switch {
		case p.Preselect:
			return 0
		case len(p.Signs) > 0:
			return 1
		}
		return 2
	}
	sort.SliceStable(out.Found, func(i, j int) bool { return rank(out.Found[i]) < rank(out.Found[j]) })
	writeJSON(w, http.StatusOK, out)
}

// platformInput is the authority taken, by its reference, and the name its
// entry gets (empty keeps the platform's).
type platformInput struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

// takePlatformTrust reads the authority again from its source - never a
// certificate the browser says came from the cluster - and trusts it like an
// imported one.
func (a *API) takePlatformTrust(w http.ResponseWriter, r *http.Request, actor store.User) {
	var in platformInput
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed platform authority: "+err.Error())
		return
	}
	seen, err := a.trustedFingerprints(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	if seen["ref:"+in.Ref] {
		writeErr(w, http.StatusConflict, "this authority is trusted already")
		return
	}
	f, err := takeAuthority(r.Context(), in.Ref)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if allTrusted(f.PEM, seen) {
		writeErr(w, http.StatusConflict, "this authority is trusted already")
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = f.Name
	}
	row, err := checkedTrust(trustInput{Name: name, PEM: f.PEM}, store.TrustedAuthority{ID: newID(), Source: f.Ref})
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.SaveTrustedAuthority(r.Context(), &row); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "trusted-authority.create", "trusted-authority", row.ID, row.Name, "", in.Ref+" "+fingerprintOf(row))
	writeJSON(w, http.StatusCreated, trustViewOf(row))
}

// trustView is one entry as the console shows it: what its PEM holds.
type trustView struct {
	store.TrustedAuthority
	Certificates []certs.AuthorityCert `json:"certificates"`
}

func trustViewOf(t store.TrustedAuthority) trustView {
	v := trustView{TrustedAuthority: t, Certificates: []certs.AuthorityCert{}}
	if found, err := certs.ParseAuthorities([]byte(t.PEM)); err == nil {
		v.Certificates = certs.DescribeAuthorities(found)
	}
	return v
}

func (a *API) listTrust(w http.ResponseWriter, r *http.Request, _ store.User) {
	list, err := a.st.ListTrustedAuthorities(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	out := make([]trustView, 0, len(list))
	for _, t := range list {
		out = append(out, trustViewOf(t))
	}
	writeJSON(w, http.StatusOK, out)
}

// trustInput is what the console sends: a name and a PEM.
type trustInput struct {
	Name string `json:"name"`
	PEM  string `json:"pem"`
}

// checkedTrust builds the row and refuses, at save time, a PEM that holds no
// certificate or holds a private key.
func checkedTrust(in trustInput, row store.TrustedAuthority) (store.TrustedAuthority, error) {
	row.Name, row.PEM = in.Name, in.PEM
	if err := store.CheckTrustedAuthority(&row); err != nil {
		return row, err
	}
	if _, err := certs.ParseAuthorities([]byte(row.PEM)); err != nil {
		return row, err
	}
	return row, nil
}

func (a *API) createTrust(w http.ResponseWriter, r *http.Request, actor store.User) {
	var in trustInput
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed trusted authority: "+err.Error())
		return
	}
	row, err := checkedTrust(in, store.TrustedAuthority{ID: newID()})
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.SaveTrustedAuthority(r.Context(), &row); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "trusted-authority.create", "trusted-authority", row.ID, row.Name, "", fingerprintOf(row))
	writeJSON(w, http.StatusCreated, trustViewOf(row))
}

func (a *API) updateTrust(w http.ResponseWriter, r *http.Request, actor store.User) {
	old, err := a.st.GetTrustedAuthority(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrTrustedAuthorityNotFound) {
		writeErr(w, http.StatusNotFound, "trusted authority not found")
		return
	}
	if err != nil {
		a.internal(w, err)
		return
	}
	var in trustInput
	if err := decodeStrict(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed trusted authority: "+err.Error())
		return
	}
	row, err := checkedTrust(in, old)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.SaveTrustedAuthority(r.Context(), &row); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, err)
		return
	}
	// The PEM is public, but a diff of two certificates in base64 tells a
	// reader nothing: the trail says that the content moved, by fingerprint.
	type shown struct{ Name, Content string }
	a.auditUpdate(r.Context(), actor, "trusted-authority.update", "trusted-authority", row.ID, row.Name, "",
		shown{old.Name, fingerprintOf(old)}, shown{row.Name, fingerprintOf(row)})
	writeJSON(w, http.StatusOK, trustViewOf(row))
}

func (a *API) deleteTrust(w http.ResponseWriter, r *http.Request, actor store.User) {
	old, err := a.st.GetTrustedAuthority(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrTrustedAuthorityNotFound) {
		writeErr(w, http.StatusNotFound, "trusted authority not found")
		return
	}
	if err != nil {
		a.internal(w, err)
		return
	}
	if err := a.st.DeleteTrustedAuthority(r.Context(), old.ID); err != nil {
		a.internal(w, err)
		return
	}
	if err := a.reloadRouting(r.Context()); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "trusted-authority.delete", "trusted-authority", old.ID, old.Name, fingerprintOf(old), "")
	w.WriteHeader(http.StatusNoContent)
}

// fingerprintOf names a bundle by its certificates, so the trail can say the
// content moved without printing it.
func fingerprintOf(t store.TrustedAuthority) string {
	found, err := certs.ParseAuthorities([]byte(t.PEM))
	if err != nil {
		return ""
	}
	out := ""
	for i, c := range certs.DescribeAuthorities(found) {
		if i > 0 {
			out += ","
		}
		out += c.Fingerprint[:16]
	}
	return out
}
