package admin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/store"
	"github.com/softwarity/meerkat/internal/vault"
)

// TLS (SSL-01 to SSL-03, SSL-05, SSL-08).
//
// A certificate belongs to a NAME. The console has one name and one
// certificate; the application has one per host it serves. Material used by
// both is added twice - two entries and two keys are cheaper to understand
// than one shared object plus the rule that works out which name it covers.
//
// There is no "switch HTTPS on": having a certificate is what opens the door,
// and deleting it is what closes it. A switch that can be on with nothing
// behind it is a switch that lies.
func (a *API) registerCertificates(mux *http.ServeMux) {
	mux.Handle("GET /api/certificates", a.infraAdmin(a.listCertificates))
	mux.Handle("POST /api/certificates/import", a.infraAdmin(a.importCertificate))
	mux.Handle("POST /api/certificates/self-signed", a.infraAdmin(a.selfSignedCertificate))
	mux.Handle("POST /api/certificates/signing-request", a.infraAdmin(a.newSigningRequest))
	mux.Handle("POST /api/certificates/{id}/adopt", a.infraAdmin(a.adoptCertificate))
	mux.Handle("GET /api/certificates/{id}/signing-request", a.infraAdmin(a.downloadCSR))
	mux.Handle("GET /api/certificates/{id}/pem", a.infraAdmin(a.downloadCertificate))
	mux.Handle("DELETE /api/certificates/{id}", a.infraAdmin(a.deleteCertificate))
	mux.Handle("GET /api/settings/tls", a.infraAdmin(a.getTLS))
	mux.Handle("PUT /api/settings/tls", a.infraAdmin(a.putTLS))
}

// certView is one entry as the console sees it. The embedded row already drops
// the key material (json:"-"); what is added is what the console would
// otherwise have to work out.
type certView struct {
	store.Certificate
	// Pending is a signing request with no answer yet: it holds a key, serves
	// nothing, and its row exists so the request can be downloaded again.
	Pending bool `json:"pending"`
	// Mismatch says the material does not actually answer for the host it was
	// filed under - an import of the wrong file, or a name changed afterwards.
	// It is the one check worth making continuously: a certificate that covers
	// nothing fails at handshake time, where nobody connects it to this screen.
	Mismatch bool           `json:"mismatch"`
	CSR      *certs.CSRInfo `json:"csr,omitempty"`
}

func viewOf(c store.Certificate) certView {
	v := certView{Certificate: c, Pending: c.Pending()}
	if v.Pending {
		if info, err := certs.ParseCSR(c.CSRPEM); err == nil {
			v.CSR = &info
		}
		return v
	}
	v.Mismatch = !certs.Covers(append(append([]string{}, c.Info.DNSNames...), c.Info.IPAddresses...), c.Host)
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

// entry is where a new certificate is filed: which plane, and for which host.
// Nothing else - the host is the name of the thing.
type entry struct {
	Plane string `json:"plane"`
	Host  string `json:"host"`
}

func (e entry) check() error {
	if !store.ValidCertPlane(e.Plane) {
		return fmt.Errorf("unknown plane %q (allowed: %s, %s)", e.Plane, store.PlaneConsole, store.PlaneApp)
	}
	if strings.TrimSpace(e.Host) == "" {
		return fmt.Errorf("no host: a certificate belongs to the name it answers for")
	}
	return nil
}

// request builds the generation request from the host alone. The name was
// declared once, on the screen; asking for it again in the form is exactly
// where the two drift apart.
func (e entry) request(keyType string, days int, subject certs.Subject) certs.Request {
	host := strings.ToLower(strings.TrimSpace(e.Host))
	req := certs.Request{Subject: subject, KeyType: keyType, Days: days}
	if req.CommonName == "" {
		req.CommonName = host
	}
	if ip := net.ParseIP(host); ip != nil {
		req.IPAddresses = []string{host}
		return req
	}
	req.DNSNames = []string{host}
	// localhost is reached by its name AND by its address, depending on who is
	// typing. A certificate that carries only one of the two fails half the
	// time, for a reason nobody enjoys tracking down.
	if host == "localhost" {
		req.IPAddresses = []string{"127.0.0.1", "::1"}
	}
	return req
}

// importPayload is material that already exists somewhere else. Two shapes,
// one endpoint: a PEM pair (what a CA e-mails back, what certbot writes) or a
// PKCS#12 keystore (what the Java and Windows worlds hand out).
type importPayload struct {
	entry
	CertPEM string `json:"certPem,omitempty"`
	KeyPEM  string `json:"keyPem,omitempty"`
	// Keystore is a .p12/.pfx, base64. Its presence selects the mode.
	Keystore string `json:"keystore,omitempty"`
	Password string `json:"password,omitempty"`
}

func (a *API) importCertificate(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p importPayload
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed certificate: "+err.Error())
		return
	}
	if err := p.check(); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
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
	// Refuse a certificate that does not answer for the host it is being filed
	// under, HERE. It would otherwise fail at the first handshake with a name
	// mismatch nobody traces back to this screen - and this is the moment the
	// operator has the right file in their hand.
	if !certs.Covers(m.Names(), p.Host) {
		writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
			"this certificate does not answer for %s: it covers %s",
			p.Host, strings.Join(m.Names(), ", ")))
		return
	}
	a.saveMaterial(w, r, actor, p.entry, store.CertSourceImport, m, "certificate.import")
}

// generatePayload is a generation form. It carries no host list: the host is
// the entry it is being created for.
type generatePayload struct {
	entry
	certs.Subject
	KeyType string `json:"keyType,omitempty"`
	Days    int    `json:"days,omitempty"`
}

// selfSignedCertificate is HTTPS in one click. The browser will warn, and it is
// right to: nobody vouched for anything. What it buys is a real handshake, a
// Secure cookie that can be marked secure, and passkeys that work today.
func (a *API) selfSignedCertificate(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p generatePayload
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	if err := p.check(); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	m, err := certs.SelfSigned(p.request(p.KeyType, p.Days, p.Subject), time.Now())
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.saveMaterial(w, r, actor, p.entry, store.CertSourceSelfSigned, m, "certificate.self-signed")
}

// newSigningRequest is the offline door. The request leaves, the key does not.
// Whoever runs the company authority signs it, and the answer comes back
// through adopt - no network, no authority Meerkat has to know how to talk to.
func (a *API) newSigningRequest(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p generatePayload
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	if err := p.check(); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	csrPEM, keyPEM, err := certs.NewCSR(p.request(p.KeyType, 0, p.Subject))
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
		ID: newID(), Plane: p.Plane, Host: p.Host,
		Source: store.CertSourceCSR, CSRPEM: csrPEM, KeyPEM: keyPEM,
		Info: certs.Info{
			Subject: info.Subject, DNSNames: info.DNSNames,
			IPAddresses: info.IPAddresses, KeyType: info.KeyType,
		},
	}
	if err := a.st.SaveCertificate(r.Context(), row); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "certificate.signing-request", "certificate", row.ID, row.Host, "", row.Plane)
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
			fmt.Sprintf("%s already holds a certificate: adopting applies to a signing request waiting for its answer", row.Host))
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
	a.auditEvent(r.Context(), actor, "certificate.adopt", "certificate", row.ID, row.Host, "", m.Info.Issuer)
	writeJSON(w, http.StatusOK, viewOf(row))
}

// saveMaterial is the tail every creation door shares.
func (a *API) saveMaterial(w http.ResponseWriter, r *http.Request, actor store.User,
	e entry, source string, m *certs.Material, action string) {
	row := store.Certificate{
		ID: newID(), Plane: e.Plane, Host: e.Host, Source: source,
		CertPEM: m.CertPEM, KeyPEM: m.KeyPEM, Info: m.Info,
	}
	if err := a.st.SaveCertificate(r.Context(), row); err != nil {
		a.internal(w, err)
		return
	}
	a.applyTLS(r.Context())
	a.auditEvent(r.Context(), actor, action, "certificate", row.ID, row.Host, "", row.Plane)
	writeJSON(w, http.StatusCreated, viewOf(row))
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
	a.auditEvent(r.Context(), actor, "certificate.delete", "certificate", row.ID, row.Host, "", row.Plane)
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
	writePEM(w, row.Host+".csr", row.CSRPEM)
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
	writePEM(w, row.Host+".pem", row.CertPEM)
}

func writePEM(w http.ResponseWriter, filename, body string) {
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = w.Write([]byte(body))
}

// tlsPayload is the TLS screen: the declared names, what is actually running,
// and what an authority already holds.
type tlsPayload struct {
	certs.Settings
	// EABSecretSet says a LITERAL external-account key is stored: something is
	// configured that this payload does not carry (VAULT-05).
	EABSecretSet bool `json:"eabSecretSet"`
	// State is what the supervisor last managed to do.
	State certs.State `json:"state"`
	// Issued is what the authority already holds, per domain, so the screen
	// shows a real expiry rather than a form that says nothing.
	Issued map[string]certs.Info `json:"issued,omitempty"`
}

func (a *API) tlsView(ctx context.Context) tlsPayload {
	cfg := a.st.RawTLS(ctx)
	if cfg.ConsoleName == "" {
		cfg.ConsoleName = store.DefaultConsoleName
	}
	v := tlsPayload{Settings: cfg}
	if !vault.IsRef(cfg.ACME.EABHMACKey) {
		v.ACME.EABHMACKey = ""
		v.EABSecretSet = cfg.ACME.EABHMACKey != ""
	}
	if a.TLS != nil {
		v.State = a.TLS.State()
	}
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
	cfg.ConsoleName = strings.ToLower(strings.TrimSpace(cfg.ConsoleName))
	cfg.AppNames = trimAll(cfg.AppNames)
	cfg.ACME.Domains = trimAll(cfg.ACME.Domains)
	// Every automatic domain must have a declared name, or it would sit in the
	// authority's closed list with no row on screen to switch it off again.
	// The invariant is kept by ADOPTING, never by dropping: a caller that sets
	// domains directly - the API, an imported configuration - asked for
	// something, and silently discarding it is the worst of the three possible
	// answers.
	declared := map[string]bool{cfg.ConsoleName: true}
	for _, n := range cfg.AppNames {
		declared[n] = true
	}
	for _, d := range cfg.ACME.Domains {
		if !declared[d] {
			cfg.AppNames = append(cfg.AppNames, d)
			declared[d] = true
		}
	}
	// Refuse at save time rather than at renewal time: a misconfigured
	// authority only discovered three months later, when a certificate
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
