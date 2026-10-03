package admin

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/vault"
)

// ACME authorities (SSL-05, Enterprise): the accounts certificates are asked
// of, set up in the TLS screen's drawer - a provider picked, its form filled,
// saved - and listed below the form. Each becomes a way in of the Add
// certificate menu. On the community image they can be read and deleted - a
// configuration may have brought some over - but not set up or asked.

// acmeEdition is the refusal of the community image, said once.
const acmeEdition = "ACME, which has authorities issue and renew certificates on their own,"

func (a *API) registerAuthorities(mux Mux) {
	mux.Handle("GET /api/acme/authorities", a.infraAdmin(a.listAuthorities))
	mux.Handle("POST /api/acme/authorities", a.infraAdmin(a.createAuthority))
	mux.Handle("PUT /api/acme/authorities/{id}", a.infraAdmin(a.updateAuthority))
	mux.Handle("DELETE /api/acme/authorities/{id}", a.infraAdmin(a.deleteAuthority))
	mux.Handle("POST /api/certificates/{id}/retry", a.infraAdmin(a.retryOrder))
}

// authorityView is one authority as the console sees it: a literal HMAC key
// never travels back (VAULT-05), the console is only told one is held.
type authorityView struct {
	certs.Authority
	EABSecretSet bool `json:"eabSecretSet"`
	// Uses are the names its orders ask for: what deleting it would orphan.
	Uses []string `json:"uses,omitempty"`
}

func authorityViewOf(x certs.Authority, orders []certs.ACMEOrder) authorityView {
	v := authorityView{Authority: x}
	if !vault.IsRef(x.EABHMACKey) {
		v.EABHMACKey = ""
		v.EABSecretSet = x.EABHMACKey != ""
	}
	v.CachePrefix = ""
	for _, o := range orders {
		if o.Authority == x.ID {
			v.Uses = append(v.Uses, o.Names...)
		}
	}
	return v
}

func (a *API) listAuthorities(w http.ResponseWriter, r *http.Request, _ store.User) {
	cfg := a.st.RawTLS(r.Context())
	out := make([]authorityView, 0, len(cfg.ACME.Authorities))
	for _, x := range cfg.ACME.Authorities {
		out = append(out, authorityViewOf(x, cfg.ACME.Orders))
	}
	writeJSON(w, http.StatusOK, map[string]any{"authorities": out, "providers": certs.Providers})
}

// checkAuthority refuses at save time rather than at renewal time: what is
// missing, then whether the authority answers at all.
func (a *API) checkAuthority(ctx context.Context, x certs.Authority) error {
	if !x.AcceptTOS {
		return fmt.Errorf("the authority's terms of service have not been accepted: an account cannot be opened without that")
	}
	if err := x.Check(a.st.ExpandInfra(ctx, x.EABHMACKey)); err != nil {
		return err
	}
	discover := a.DiscoverACME
	if discover == nil {
		discover = func(ctx context.Context, x certs.Authority) error { return x.Discover(ctx) }
	}
	return discover(ctx, x)
}

func (a *API) createAuthority(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := edition.Require(acmeEdition); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	var x certs.Authority
	if err := decodeStrict(r, &x); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed authority: "+err.Error())
		return
	}
	x = x.Normalize()
	x.ID = certs.NewAuthorityID()
	// Its own corner of the cache: its account key and what it issues are
	// never mistaken for another authority's.
	x.CachePrefix = x.ID + "/"
	x.CreatedAt = time.Now().Unix()
	if err := a.checkAuthority(r.Context(), x); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	cfg := a.st.RawTLS(r.Context())
	cfg.ACME.Authorities = append(cfg.ACME.Authorities, x)
	if err := a.st.SetSetting(r.Context(), store.SettingTLS, cfg); err != nil {
		a.internal(w, err)
		return
	}
	a.applyTLS(r.Context())
	a.auditEvent(r.Context(), actor, "acme.authority.create", "acme-authority", x.ID, x.Name, "", x.Directory())
	writeJSON(w, http.StatusCreated, authorityViewOf(x, cfg.ACME.Orders))
}

func (a *API) updateAuthority(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := edition.Require(acmeEdition); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	id := r.PathValue("id")
	cfg := a.st.RawTLS(r.Context())
	i := slices.IndexFunc(cfg.ACME.Authorities, func(x certs.Authority) bool { return x.ID == id })
	if i < 0 {
		writeErr(w, http.StatusNotFound, "authority not found")
		return
	}
	var x certs.Authority
	if err := decodeStrict(r, &x); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed authority: "+err.Error())
		return
	}
	old := cfg.ACME.Authorities[i]
	// A blank secret means "leave it alone": the console never received the
	// literal, so it cannot resend it (VAULT-05).
	if strings.TrimSpace(x.EABHMACKey) == "" {
		x.EABHMACKey = old.EABHMACKey
	}
	x = x.Normalize()
	x.ID, x.CreatedAt, x.CachePrefix = old.ID, old.CreatedAt, old.CachePrefix
	// Another directory is another authority: the account registered at the
	// old one is worthless there, and so is what it issued. A fresh corner of
	// the cache, rather than a deletion - nothing of the other is touched.
	if x.Directory() != old.Directory() {
		x.CachePrefix = old.ID + "-" + fmt.Sprint(time.Now().Unix()) + "/"
	}
	if err := a.checkAuthority(r.Context(), x); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	cfg.ACME.Authorities[i] = x
	if err := a.st.SetSetting(r.Context(), store.SettingTLS, cfg); err != nil {
		a.internal(w, err)
		return
	}
	a.applyTLS(r.Context())
	a.auditUpdate(r.Context(), actor, "acme.authority.update", "acme-authority", x.ID, x.Name, "",
		authorityViewOf(old, nil), authorityViewOf(x, nil))
	writeJSON(w, http.StatusOK, authorityViewOf(x, cfg.ACME.Orders))
}

// deleteAuthority refuses while orders ask it: deleting it would leave them
// asking nobody. They are named, so the way out is in the refusal.
func (a *API) deleteAuthority(w http.ResponseWriter, r *http.Request, actor store.User) {
	id := r.PathValue("id")
	cfg := a.st.RawTLS(r.Context())
	i := slices.IndexFunc(cfg.ACME.Authorities, func(x certs.Authority) bool { return x.ID == id })
	if i < 0 {
		writeErr(w, http.StatusNotFound, "authority not found")
		return
	}
	x := cfg.ACME.Authorities[i]
	if v := authorityViewOf(x, cfg.ACME.Orders); len(v.Uses) > 0 {
		writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
			"%s is still asked for %s: delete those certificates from the pool first", x.Name, strings.Join(v.Uses, ", ")))
		return
	}
	cfg.ACME.Authorities = slices.Delete(cfg.ACME.Authorities, i, i+1)
	if err := a.st.SetSetting(r.Context(), store.SettingTLS, cfg); err != nil {
		a.internal(w, err)
		return
	}
	a.applyTLS(r.Context())
	a.auditEvent(r.Context(), actor, "acme.authority.delete", "acme-authority", x.ID, x.Name, x.Directory(), "")
	w.WriteHeader(http.StatusNoContent)
}

// retryOrder asks again for the names of an order the authority refused -
// once whatever it refused for (a DNS record, a closed port) is fixed.
func (a *API) retryOrder(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := edition.Require(acmeEdition); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row, ok := a.find(r.Context(), r.PathValue("id"))
	if !ok || row.Source != store.CertSourceACME {
		writeErr(w, http.StatusNotFound, "no order to an authority with this id")
		return
	}
	if !row.Console && !row.App {
		writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
			"%s is not placed on a door: place it, and it is asked at once", row.Name()))
		return
	}
	if a.TLS != nil {
		a.TLS.Retry(r.Context(), row.Authority, row.Info.DNSNames)
	}
	a.auditEvent(r.Context(), actor, "certificate.retry", "certificate", row.ID, row.Name(), "", "")
	writeJSON(w, http.StatusAccepted, a.orderView(r.Context(), row))
}
