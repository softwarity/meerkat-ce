package admin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/vault"
)

// TLS (SSL-01 to SSL-03, SSL-05). Four doors a certificate can come through,
// and they exist because the rooms Meerkat runs in are not alike - see the
// certs package for the argument. What this file adds is the console's side:
// nothing here ever hands a private key back, and every mutation re-applies
// immediately, because saving IS applying.
func (a *API) registerCertificates(mux *http.ServeMux) {
	mux.Handle("GET /api/certificates", a.infraAdmin(a.listCertificates))
	mux.Handle("POST /api/certificates/import", a.infraAdmin(a.importCertificate))
	mux.Handle("POST /api/certificates/self-signed", a.infraAdmin(a.selfSignedCertificate))
	mux.Handle("POST /api/certificates/signing-request", a.infraAdmin(a.newSigningRequest))
	mux.Handle("POST /api/certificates/{id}/adopt", a.infraAdmin(a.adoptCertificate))
	mux.Handle("GET /api/certificates/{id}/signing-request", a.infraAdmin(a.downloadCSR))
	mux.Handle("GET /api/certificates/{id}/pem", a.infraAdmin(a.downloadCertificate))
	mux.Handle("PUT /api/certificates/{id}", a.infraAdmin(a.updateCertificate))
	mux.Handle("DELETE /api/certificates/{id}", a.infraAdmin(a.deleteCertificate))
	mux.Handle("GET /api/settings/tls", a.infraAdmin(a.getTLS))
	mux.Handle("PUT /api/settings/tls", a.infraAdmin(a.putTLS))
}

// certView is one row as the console sees it. The embedded store row already
// drops the key material (json:"-"); what is added is what the console would
// otherwise have to work out for itself.
type certView struct {
	store.Certificate
	// Pending is a signing request with no answer yet: it holds a key, serves
	// nothing, and its row exists so the request can be downloaded again.
	Pending bool `json:"pending"`
	// CSR describes what a pending request asked for.
	CSR *certs.CSRInfo `json:"csr,omitempty"`
}

func viewOf(c store.Certificate) certView {
	v := certView{Certificate: c, Pending: c.Pending()}
	if v.Pending {
		if info, err := certs.ParseCSR(c.CSRPEM); err == nil {
			v.CSR = &info
		}
	}
	return v
}

func (a *API) listCertificates(w http.ResponseWriter, r *http.Request, _ store.User) {
	rows, err := a.st.ListCertificates(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	out := make([]certView, 0, len(rows))
	for _, c := range rows {
		out = append(out, viewOf(c))
	}
	writeJSON(w, http.StatusOK, out)
}

// importPayload is material that already exists somewhere else. Two shapes,
// one endpoint: a PEM pair (what a CA e-mails back, what certbot writes) or a
// PKCS#12 keystore (what the Java and Windows worlds hand out).
type importPayload struct {
	Name    string `json:"name"`
	CertPEM string `json:"certPem,omitempty"`
	KeyPEM  string `json:"keyPem,omitempty"`
	// Keystore is a .p12/.pfx, base64. Its presence selects the mode.
	Keystore string `json:"keystore,omitempty"`
	Password string `json:"password,omitempty"`
	Default  bool   `json:"default,omitempty"`
}

func (a *API) importCertificate(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p importPayload
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed certificate: "+err.Error())
		return
	}
	var (
		m   *certs.Material
		err error
	)
	switch {
	case strings.TrimSpace(p.Keystore) != "":
		raw, derr := base64.StdEncoding.DecodeString(strings.TrimSpace(p.Keystore))
		if derr != nil {
			writeErr(w, http.StatusBadRequest, "the keystore is not valid base64")
			return
		}
		m, err = certs.ParsePKCS12(raw, p.Password)
	case strings.TrimSpace(p.CertPEM) != "" || strings.TrimSpace(p.KeyPEM) != "":
		m, err = certs.ParsePEM(p.CertPEM, p.KeyPEM)
	default:
		writeErr(w, http.StatusUnprocessableEntity,
			"nothing to import: send a PEM pair (certPem and keyPem), or a PKCS#12 keystore")
		return
	}
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.saveMaterial(w, r, actor, p.Name, store.CertSourceImport, m, p.Default, "certificate.import")
}

// requestPayload is a generation form: for whom, under which key, for how long.
type requestPayload struct {
	Name string `json:"name"`
	certs.Request
	Default bool `json:"default,omitempty"`
}

// selfSignedCertificate is HTTPS in one click. The browser will warn, and it is
// right to: nobody vouched for anything. What it buys is a real handshake, a
// Secure cookie that can be marked secure, and passkeys that work today.
func (a *API) selfSignedCertificate(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p requestPayload
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	m, err := certs.SelfSigned(p.Request, time.Now())
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.saveMaterial(w, r, actor, p.Name, store.CertSourceSelfSigned, m, p.Default, "certificate.self-signed")
}

// newSigningRequest is the offline door. The request leaves, the key does not.
// Whoever runs the company authority signs it, and the answer comes back
// through adopt - no network, no authority Meerkat has to know how to talk to.
func (a *API) newSigningRequest(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p requestPayload
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		writeErr(w, http.StatusUnprocessableEntity, "a certificate needs a name")
		return
	}
	csrPEM, keyPEM, err := certs.NewCSR(p.Request)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	info, err := certs.ParseCSR(csrPEM)
	if err != nil {
		a.internal(w, err)
		return
	}
	row := store.Certificate{
		ID: newID(), Name: name, Source: store.CertSourceCSR,
		CSRPEM: csrPEM, KeyPEM: keyPEM,
		Info: certs.Info{
			Subject: info.Subject, DNSNames: info.DNSNames,
			IPAddresses: info.IPAddresses, KeyType: info.KeyType,
		},
	}
	if err := a.st.SaveCertificate(r.Context(), row); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "certificate.signing-request", "certificate", row.ID, row.Name, "",
		strings.Join(append(info.DNSNames, info.IPAddresses...), ", "))
	writeJSON(w, http.StatusCreated, viewOf(row))
}

// adoptCertificate marries the answer of an authority to the key that stayed.
func (a *API) adoptCertificate(w http.ResponseWriter, r *http.Request, actor store.User) {
	row, err := a.st.FindCertificate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "certificate not found")
		return
	}
	if !row.Pending() {
		writeErr(w, http.StatusConflict,
			fmt.Sprintf("%q already holds a certificate: adopting applies to a signing request waiting for its answer", row.Name))
		return
	}
	var p struct {
		CertPEM string `json:"certPem"`
	}
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed certificate: "+err.Error())
		return
	}
	m, err := certs.Adopt(row.KeyPEM, p.CertPEM)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	row.CertPEM, row.KeyPEM, row.Info = m.CertPEM, m.KeyPEM, m.Info
	row.CSRPEM = ""
	if err := a.st.SaveCertificate(r.Context(), row); err != nil {
		a.internal(w, err)
		return
	}
	a.applyTLS(r.Context())
	a.auditEvent(r.Context(), actor, "certificate.adopt", "certificate", row.ID, row.Name, "", m.Info.Issuer)
	writeJSON(w, http.StatusOK, viewOf(row))
}

// saveMaterial is the tail every creation door shares.
func (a *API) saveMaterial(w http.ResponseWriter, r *http.Request, actor store.User,
	name, source string, m *certs.Material, asDefault bool, action string) {
	name = strings.TrimSpace(name)
	if name == "" {
		// Name it after what it serves rather than refusing: the operator has
		// already told us, in the only field that matters.
		name = firstName(m)
	}
	row := store.Certificate{
		ID: newID(), Name: name, Source: source,
		CertPEM: m.CertPEM, KeyPEM: m.KeyPEM, Info: m.Info, Default: asDefault,
	}
	if err := a.st.SaveCertificate(r.Context(), row); err != nil {
		a.internal(w, err)
		return
	}
	a.applyTLS(r.Context())
	a.auditEvent(r.Context(), actor, action, "certificate", row.ID, row.Name, "",
		strings.Join(m.Names(), ", "))
	writeJSON(w, http.StatusCreated, viewOf(row))
}

func firstName(m *certs.Material) string {
	if names := m.Names(); len(names) > 0 {
		return names[0]
	}
	return "certificate"
}

// updateCertificate renames, and points the no-SNI fallback.
func (a *API) updateCertificate(w http.ResponseWriter, r *http.Request, actor store.User) {
	row, err := a.st.FindCertificate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "certificate not found")
		return
	}
	var p struct {
		Name    string `json:"name"`
		Default bool   `json:"default"`
	}
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed certificate: "+err.Error())
		return
	}
	if p.Default && row.Pending() {
		writeErr(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("%q is a signing request waiting for an answer: it has no certificate to present", row.Name))
		return
	}
	before := viewOf(row)
	if n := strings.TrimSpace(p.Name); n != "" {
		row.Name = n
	}
	row.Default = p.Default
	if err := a.st.SaveCertificate(r.Context(), row); err != nil {
		a.internal(w, err)
		return
	}
	if !p.Default && before.Default {
		if err := a.st.SetDefaultCertificate(r.Context(), ""); err != nil {
			a.internal(w, err)
			return
		}
	}
	a.applyTLS(r.Context())
	after := viewOf(row)
	a.auditUpdate(r.Context(), actor, "certificate.update", "certificate", row.ID, row.Name, "", before, after)
	writeJSON(w, http.StatusOK, after)
}

func (a *API) deleteCertificate(w http.ResponseWriter, r *http.Request, actor store.User) {
	row, err := a.st.FindCertificate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "certificate not found")
		return
	}
	if err := a.st.DeleteCertificate(r.Context(), row.ID); err != nil {
		if errors.Is(err, store.ErrNoRows) {
			writeErr(w, http.StatusNotFound, "certificate not found")
			return
		}
		a.internal(w, err)
		return
	}
	a.applyTLS(r.Context())
	a.auditEvent(r.Context(), actor, "certificate.delete", "certificate", row.ID, row.Name, "",
		strings.Join(row.Info.DNSNames, ", "))
	w.WriteHeader(http.StatusNoContent)
}

// downloadCSR hands back the signing request. This is the point of the whole
// flow: it is carried to an authority that Meerkat never talks to.
func (a *API) downloadCSR(w http.ResponseWriter, r *http.Request, _ store.User) {
	row, err := a.st.FindCertificate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "certificate not found")
		return
	}
	if row.CSRPEM == "" {
		writeErr(w, http.StatusNotFound, "this certificate has no pending signing request")
		return
	}
	writePEM(w, row.Name+".csr", row.CSRPEM)
}

// downloadCertificate hands back the chain. Public material by construction:
// it is what every visitor is given during the handshake.
func (a *API) downloadCertificate(w http.ResponseWriter, r *http.Request, _ store.User) {
	row, err := a.st.FindCertificate(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "certificate not found")
		return
	}
	if row.CertPEM == "" {
		writeErr(w, http.StatusNotFound, "this row is a signing request: there is no certificate yet")
		return
	}
	writePEM(w, row.Name+".pem", row.CertPEM)
}

func writePEM(w http.ResponseWriter, filename, body string) {
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = w.Write([]byte(body))
}

// Coverage answers, for ONE declared name, the only question an operator
// actually has about TLS: will this work?
//
// It is computed continuously rather than at the moment a certificate is
// added. A check that runs once says nothing on the day somebody deletes the
// wrong row, and that day is exactly when it would have been worth saying.
type Coverage struct {
	Name  string `json:"name"`
	Plane string `json:"plane"` // console | app
	// Status is what to show: covered, expiring (under 30 days), expired,
	// acme (no certificate yet, but an authority is allowed to fetch one), or
	// uncovered.
	Status   string `json:"status"`
	CertID   string `json:"certId,omitempty"`
	CertName string `json:"certName,omitempty"`
	NotAfter int64  `json:"notAfter,omitempty"`
	// Automatic says this name is in the authority's closed list.
	Automatic bool `json:"automatic"`
}

// tlsPayload is the TLS screen: the switches, what is actually running, and
// what an authority has already issued.
type tlsPayload struct {
	certs.Settings
	// EABSecretSet says a LITERAL external-account key is stored: something is
	// configured that this payload does not carry (VAULT-05).
	EABSecretSet bool `json:"eabSecretSet"`
	// State is what the supervisor last managed to do - which is not always
	// what was asked for, and the difference is the whole point of showing it.
	State certs.State `json:"state"`
	// Issued is what the authority already holds, per domain, so the screen
	// shows a real expiry rather than a form that says nothing.
	Issued map[string]certs.Info `json:"issued,omitempty"`
	// Coverage is one row per declared name, both planes together.
	Coverage []Coverage `json:"coverage"`
}

func (a *API) tlsView(ctx context.Context) tlsPayload {
	cfg := a.st.RawTLS(ctx)
	v := tlsPayload{Settings: cfg}
	if !vault.IsRef(cfg.ACME.EABHMACKey) {
		v.ACME.EABHMACKey = ""
		v.EABSecretSet = cfg.ACME.EABHMACKey != ""
	}
	if a.TLS != nil {
		v.State = a.TLS.State()
	}
	v.Coverage = a.coverage(ctx, cfg)
	if cfg.ACME.Enabled {
		cache := certs.StoreCache{S: a.st, Missing: func(err error) bool { return errors.Is(err, store.ErrNoRows) }}
		issued := map[string]certs.Info{}
		for _, d := range cfg.ACME.Domains {
			if info, ok := certs.CachedInfo(ctx, cache, strings.ToLower(strings.TrimSpace(d))); ok {
				issued[d] = info
			}
		}
		if len(issued) > 0 {
			v.Issued = issued
		}
	}
	return v
}

func (a *API) getTLS(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, a.tlsView(r.Context()))
}

func (a *API) putTLS(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p tlsPayload
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed TLS settings: "+err.Error())
		return
	}
	before := a.tlsView(r.Context())
	stored := a.st.RawTLS(r.Context())
	cfg := p.Settings
	// A blank secret means "leave it alone": the console never received the
	// literal, so it cannot resend it (VAULT-05).
	if strings.TrimSpace(cfg.ACME.EABHMACKey) == "" {
		cfg.ACME.EABHMACKey = stored.ACME.EABHMACKey
	}
	cfg.ACME.Domains = trimAll(cfg.ACME.Domains)
	cfg.ConsoleNames, cfg.AppNames = trimAll(cfg.ConsoleNames), trimAll(cfg.AppNames)
	// Every automatic domain must have a declared name, or it would sit in the
	// authority's closed list with no row on screen to switch it off again.
	// The invariant is kept by ADOPTING, never by dropping: a caller that sets
	// domains directly - the API, an imported configuration - asked for
	// something, and answering by silently discarding it is the worst of the
	// three possible answers.
	declared := map[string]bool{}
	for _, n := range append(append([]string{}, cfg.ConsoleNames...), cfg.AppNames...) {
		declared[n] = true
	}
	for _, d := range cfg.ACME.Domains {
		if !declared[d] {
			cfg.AppNames = append(cfg.AppNames, d)
			declared[d] = true
		}
	}
	// Refuse at save time rather than at renewal time: a misconfigured
	// authority that is only discovered three months later, when a certificate
	// silently fails to renew, is the worst way to learn about it.
	resolved := cfg
	resolved.ACME.EABHMACKey = a.st.ExpandInfra(r.Context(), cfg.ACME.EABHMACKey)
	if err := certs.Check(resolved.ACME, nil); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := a.st.SetSetting(r.Context(), store.SettingTLS, cfg); err != nil {
		a.internal(w, err)
		return
	}
	// If the authority CHANGED, the account registered at the old one is
	// worthless and keeping it is how a renewal fails months later with an
	// error about an unknown account.
	if directoryChanged(stored.ACME, cfg.ACME) {
		if err := a.st.ForgetACME(r.Context()); err != nil {
			a.internal(w, err)
			return
		}
	}
	a.applyTLS(r.Context())
	// The settings ARE saved even when a door refused to open - refusing to
	// store them would leave the screen and the gateway disagreeing. What
	// failed travels in State.Problems, which is on screen.
	after := a.tlsView(r.Context())
	a.auditUpdate(r.Context(), actor, "tls.update", "settings", "", "", "", before, after)
	writeJSON(w, http.StatusOK, after)
}

func directoryChanged(before, after certs.ACMESettings) bool {
	norm := func(s string) string {
		if s = strings.TrimSpace(s); s == "" {
			return certs.LetsEncryptURL
		}
		return s
	}
	return norm(before.DirectoryURL) != norm(after.DirectoryURL)
}

func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// applyTLS re-reads everything and makes the running gateway match. A failure
// is not returned to the caller: the material IS saved, and what could not be
// applied is named in the TLS screen's State.Problems.
func (a *API) applyTLS(ctx context.Context) {
	if a.TLS == nil {
		return
	}
	_ = a.TLS.Reload(ctx)
}

// coverage matches every declared name against every installed certificate.
//
// It works from the names each row already stores, not from the DER: twenty
// certificates and twenty names would otherwise be four hundred parses to draw
// one table.
func (a *API) coverage(ctx context.Context, cfg certs.Settings) []Coverage {
	rows, err := a.st.ListCertificates(ctx)
	if err != nil {
		return []Coverage{}
	}
	automatic := map[string]bool{}
	for _, d := range cfg.ACME.Domains {
		automatic[strings.ToLower(strings.TrimSpace(d))] = true
	}
	now := time.Now()
	out := []Coverage{}
	seen := map[string]bool{}
	add := func(plane string, names []string) {
		for _, name := range names {
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" || seen[plane+"|"+name] {
				continue
			}
			seen[plane+"|"+name] = true
			out = append(out, coverOne(rows, name, plane, automatic[name], now))
		}
	}
	add("console", cfg.ConsoleNames)
	add("app", cfg.AppNames)
	return out
}

// coverOne picks the certificate that would actually answer for this name -
// the same choice the TLS manager makes at handshake time, so the screen and
// the gateway never disagree: a valid one beats an expired one, and among
// valid ones the one that lasts longest wins.
func coverOne(rows []store.Certificate, name, plane string, automatic bool, now time.Time) Coverage {
	c := Coverage{Name: name, Plane: plane, Automatic: automatic}
	var best *store.Certificate
	for i := range rows {
		row := &rows[i]
		if row.Pending() {
			continue
		}
		if !certs.Covers(append(append([]string{}, row.Info.DNSNames...), row.Info.IPAddresses...), name) {
			continue
		}
		if best == nil || better(row, best, now) {
			best = row
		}
	}
	if best == nil {
		c.Status = "uncovered"
		if automatic {
			// Not a fault: the authority fetches on demand, so a name it is
			// allowed to ask for is simply not fetched YET.
			c.Status = "acme"
		}
		return c
	}
	c.CertID, c.CertName, c.NotAfter = best.ID, best.Name, best.Info.NotAfter
	switch {
	case best.Info.NotAfter <= now.Unix():
		c.Status = "expired"
	case best.Info.NotAfter <= now.AddDate(0, 0, 30).Unix():
		c.Status = "expiring"
	default:
		c.Status = "covered"
	}
	return c
}

func better(a, b *store.Certificate, now time.Time) bool {
	t := now.Unix()
	aLive := a.Info.NotBefore <= t && t < a.Info.NotAfter
	bLive := b.Info.NotBefore <= t && t < b.Info.NotAfter
	if aLive != bLive {
		return aLive
	}
	return a.Info.NotAfter > b.Info.NotAfter
}
