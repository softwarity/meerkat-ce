package admin

import (
	"errors"
	"net/http"

	"github.com/softwarity/meerkat/internal/certs"
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
