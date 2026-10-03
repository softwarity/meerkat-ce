package admin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// TLS (SSL-01 to SSL-03, SSL-05, SSL-08).
//
// A pool, then a placement. A certificate is made or imported ONCE - its names
// are its own, read from the material - and placed on the console, the
// application, or both. A wildcard or a certificate carrying several names
// serves every one of them; material both planes use is one entry.
//
// There is no "switch HTTPS on": a certificate placed on a plane is what opens
// its door, and taking it off is what closes it. A switch that can be on with
// nothing behind it is a switch that lies.
func (a *API) registerCertificates(mux Mux) {
	mux.Handle("GET /api/certificates", a.infraAdmin(a.listCertificates))
	mux.Handle("POST /api/certificates/import", a.infraAdmin(a.importCertificate))
	mux.Handle("POST /api/certificates/self-signed", a.infraAdmin(a.selfSignedCertificate))
	mux.Handle("POST /api/certificates/signing-request", a.infraAdmin(a.newSigningRequest))
	mux.Handle("POST /api/certificates/acme", a.infraAdmin(a.newACMEOrder))
	mux.Handle("POST /api/certificates/{id}/adopt", a.infraAdmin(a.adoptCertificate))
	mux.Handle("PUT /api/certificates/{id}/placement", a.infraAdmin(a.placeCertificate))
	mux.Handle("GET /api/certificates/{id}/signing-request", a.infraAdmin(a.downloadCSR))
	mux.Handle("GET /api/certificates/{id}/pem", a.infraAdmin(a.downloadCertificate))
	mux.Handle("DELETE /api/certificates/{id}", a.infraAdmin(a.deleteCertificate))
	mux.Handle("GET /api/settings/tls", a.infraAdmin(a.getTLS))
	mux.Handle("PUT /api/settings/tls", a.infraAdmin(a.putTLS))
}

// certView is one entry as the console sees it. The embedded row already drops
// the key material (json:"-").
type certView struct {
	store.Certificate
	// Pending is a signing request with no answer yet: it holds a key, serves
	// nothing, and its row exists so the request can be downloaded again.
	Pending bool           `json:"pending"`
	CSR     *certs.CSRInfo `json:"csr,omitempty"`
	// Waiting is an order to an authority with a name not issued yet.
	Waiting bool `json:"waiting,omitempty"`
	// Issued are the names of an order the authority already answered.
	Issued []string `json:"issued,omitempty"`
	// AuthorityName is what the pool calls the authority of an order.
	AuthorityName string `json:"authorityName,omitempty"`
	// Status is where the request for a waiting name stands on this node -
	// requesting, or failed with the authority's own Error.
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
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

// orderRow is an order to the authority as a member of the pool: the same
// names, the same placement, so the conflicts and the placement treat both
// kinds alike.
func orderRow(o certs.ACMEOrder) store.Certificate {
	return store.Certificate{
		ID: o.ID, Console: o.Console, App: o.App, Source: store.CertSourceACME, Authority: o.Authority,
		Info: certs.Info{DNSNames: append([]string{}, o.Names...)}, CreatedAt: o.CreatedAt,
	}
}

// pool is everything the screen lists: the rows of the table, then the orders.
func (a *API) pool(ctx context.Context) ([]store.Certificate, certs.Settings, error) {
	rows, err := a.st.ListCertificates(ctx)
	if err != nil {
		return nil, certs.Settings{}, err
	}
	cfg := a.st.RawTLS(ctx)
	for _, o := range cfg.ACME.Orders {
		rows = append(rows, orderRow(o))
	}
	return rows, cfg, nil
}

// orderView fills an order from the cache: what the authority issued for each
// name, the end shown being the earliest - the one that matters.
func (a *API) orderView(ctx context.Context, c store.Certificate) certView {
	v := certView{Certificate: c}
	auth, _ := a.st.RawTLS(ctx).ACME.AuthorityByID(c.Authority)
	v.AuthorityName = auth.Name
	cache := certs.PrefixedCache{
		C:      certs.StoreCache{S: a.st, Missing: func(err error) bool { return errors.Is(err, store.ErrNoRows) }},
		Prefix: auth.CachePrefix,
	}
	for _, n := range c.Info.DNSNames {
		info, ok := certs.CachedInfo(ctx, cache, strings.ToLower(n))
		if !ok {
			v.Waiting = true
			// What this node knows of the request: the failure, with the
			// authority's reason, wins over a request still running.
			if a.TLS != nil {
				if st, ok := a.TLS.Issue(c.Authority, n); ok && v.Status != certs.IssueFailed {
					v.Status, v.Error = st.State, st.Error
				}
			}
			continue
		}
		v.Issued = append(v.Issued, n)
		if v.Info.NotAfter == 0 || info.NotAfter < v.Info.NotAfter {
			names := v.Info.DNSNames
			v.Info = info
			v.Info.DNSNames = names
		}
	}
	return v
}

func (a *API) listCertificates(w http.ResponseWriter, r *http.Request, _ store.User) {
	all, _, err := a.pool(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	out := make([]certView, 0, len(all))
	for _, c := range all {
		if c.Source == store.CertSourceACME {
			out = append(out, a.orderView(r.Context(), c))
			continue
		}
		out = append(out, viewOf(c))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	writeJSON(w, http.StatusOK, out)
}

// find reads one member of the pool, row or order.
func (a *API) find(ctx context.Context, id string) (store.Certificate, bool) {
	if row, err := a.st.FindCertificate(ctx, id); err == nil {
		return row, true
	}
	for _, o := range a.st.RawTLS(ctx).ACME.Orders {
		if o.ID == id {
			return orderRow(o), true
		}
	}
	return store.Certificate{}, false
}

// placement is where a certificate is served. Optional on the creation doors -
// an agent or a script can make and place in one call - and the whole body of
// the placement endpoint. Replace takes the certificates already answering
// for one of the same names off that plane, instead of refusing.
type placement struct {
	Console bool `json:"console,omitempty"`
	App     bool `json:"app,omitempty"`
	Replace bool `json:"replace,omitempty"`
}

// placeConflict is a certificate already placed on a plane and answering for one of
// the names of the one being placed there.
type placeConflict struct {
	ID    string   `json:"id"`
	Plane string   `json:"plane"`
	Names []string `json:"names"`
}

// conflicts lists, for the planes p asks for, the OTHER certificates placed
// there that answer for a name c answers for too.
//
// Two of them on one door is not a fault the handshake cannot survive - it
// presents the one that lasts longest - but it is a question nobody was
// asked: which of the two did the operator mean? So the placement says who is
// in the way, and replacing is a word the caller says, not a guess.
func conflicts(all []store.Certificate, c store.Certificate, p placement) []placeConflict {
	mine := map[string]bool{}
	for _, n := range c.Names() {
		mine[strings.ToLower(n)] = true
	}
	var out []placeConflict
	for _, plane := range []string{store.PlaneConsole, store.PlaneApp} {
		if (plane == store.PlaneConsole && !p.Console) || (plane == store.PlaneApp && !p.App) {
			continue
		}
		for _, o := range all {
			if o.ID == c.ID || !o.On(plane) {
				continue
			}
			var shared []string
			for _, n := range o.Names() {
				if mine[strings.ToLower(n)] {
					shared = append(shared, n)
				}
			}
			if len(shared) > 0 {
				out = append(out, placeConflict{ID: o.ID, Plane: plane, Names: shared})
			}
		}
	}
	return out
}

// place applies p to c, after the conflict check. It writes c (a creation
// door hands a row not yet saved) and takes whatever was replaced off its
// plane. A refusal answers 422 with the certificates in the way, and writes
// nothing - 422 and not 409, which this API keeps for a save built on a
// revision somebody else replaced.
func (a *API) place(w http.ResponseWriter, r *http.Request, c *store.Certificate, p placement) bool {
	// An order placed on the community image would be asked of nobody: it is
	// refused there, rather than shown as placed and never served.
	if (p.Console || p.App) && c.Source == store.CertSourceACME {
		if err := edition.Require(acmeEdition); err != nil {
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
			return false
		}
	}
	if (p.Console || p.App) && c.Pending() {
		writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
			"%s is a signing request: it serves nothing until the signed certificate is adopted", c.Name()))
		return false
	}
	all, cfg, err := a.pool(r.Context())
	if err != nil {
		a.internal(w, err)
		return false
	}
	in := conflicts(all, *c, p)
	if len(in) > 0 && !p.Replace {
		var parts []string
		for _, x := range in {
			parts = append(parts, fmt.Sprintf("%s on the %s", strings.Join(x.Names, ", "), planeWord(x.Plane)))
		}
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error": fmt.Sprintf("another certificate already answers for %s: replace it, or take it off first",
				strings.Join(parts, "; ")),
			"conflicts": in,
		})
		return false
	}
	// The orders change in the settings, the rows in the table: one write of
	// the settings at the end, whatever number of orders moved.
	orders := cfg.ACME.Orders
	setOrder := func(id string, console, app bool) bool {
		for i := range orders {
			if orders[i].ID == id {
				orders[i].Console, orders[i].App = console, app
				return true
			}
		}
		return false
	}
	byID := map[string]store.Certificate{}
	for _, o := range all {
		byID[o.ID] = o
	}
	for _, x := range in {
		o := byID[x.ID]
		if x.Plane == store.PlaneConsole {
			o.Console = false
		} else {
			o.App = false
		}
		byID[x.ID] = o
		if o.Source == store.CertSourceACME {
			setOrder(o.ID, o.Console, o.App)
			continue
		}
		if err := a.st.PlaceCertificate(r.Context(), o.ID, o.Console, o.App); err != nil {
			a.internal(w, err)
			return false
		}
	}
	c.Console, c.App = p.Console, p.App
	if c.Source == store.CertSourceACME {
		if !setOrder(c.ID, c.Console, c.App) {
			orders = append(orders, certs.ACMEOrder{
				ID: c.ID, Authority: c.Authority, Names: c.Info.DNSNames,
				Console: c.Console, App: c.App, CreatedAt: c.CreatedAt,
			})
		}
	} else if err := a.st.SaveCertificate(r.Context(), *c); err != nil {
		a.internal(w, err)
		return false
	}
	if c.Source == store.CertSourceACME || len(orders) != len(cfg.ACME.Orders) || ordersMoved(in, byID) {
		cfg.ACME.Orders = orders
		if err := a.st.SetSetting(r.Context(), store.SettingTLS, cfg); err != nil {
			a.internal(w, err)
			return false
		}
	}
	return true
}

// ordersMoved reports whether a replacement took an order off a door.
func ordersMoved(in []placeConflict, byID map[string]store.Certificate) bool {
	for _, x := range in {
		if byID[x.ID].Source == store.CertSourceACME {
			return true
		}
	}
	return false
}

func planeWord(plane string) string {
	if plane == store.PlaneConsole {
		return "console"
	}
	return "application"
}

func where(c store.Certificate) string {
	switch {
	case c.Console && c.App:
		return "console, application"
	case c.Console:
		return "console"
	case c.App:
		return "application"
	}
	return "reserve"
}

// placeCertificate moves one certificate onto or off the planes. Saving IS
// applying: the door opens, or closes, before the answer comes back.
func (a *API) placeCertificate(w http.ResponseWriter, r *http.Request, actor store.User) {
	row, ok := a.find(r.Context(), r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "certificate not found")
		return
	}
	var p placement
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed placement: "+err.Error())
		return
	}
	before := where(row)
	if !a.place(w, r, &row, p) {
		return
	}
	a.applyTLS(r.Context())
	a.auditEvent(r.Context(), actor, "certificate.place", "certificate", row.ID, row.Name(), before, where(row))
	if row.Source == store.CertSourceACME {
		writeJSON(w, http.StatusOK, a.orderView(r.Context(), row))
		return
	}
	writeJSON(w, http.StatusOK, viewOf(row))
}

// names is the list a generation door is given: the names the certificate
// will answer for, checked here so the refusal names the one that is wrong.
func request(names []string, keyType string, days int, subject certs.Subject) (certs.Request, error) {
	req := certs.Request{Subject: subject, KeyType: keyType, Days: days}
	seen := map[string]bool{}
	for _, raw := range names {
		n := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		if ip := net.ParseIP(n); ip != nil {
			req.IPAddresses = append(req.IPAddresses, ip.String())
			continue
		}
		if !validHostname(n) {
			return req, fmt.Errorf("%q is not a host name: letters, digits and hyphens between dots, "+
				"a leading *. for a wildcard, or an IP address", raw)
		}
		req.DNSNames = append(req.DNSNames, n)
		// localhost is reached by its name AND by its address, depending on who
		// is typing. A certificate that carries only one of the two fails half
		// the time, for a reason nobody enjoys tracking down.
		if n == "localhost" {
			for _, ip := range []string{"127.0.0.1", "::1"} {
				if !seen[ip] {
					seen[ip] = true
					req.IPAddresses = append(req.IPAddresses, ip)
				}
			}
		}
	}
	if len(req.DNSNames) == 0 && len(req.IPAddresses) == 0 {
		return req, fmt.Errorf("no name: a certificate answers for at least one host name or IP address")
	}
	if req.CommonName == "" {
		if len(req.DNSNames) > 0 {
			req.CommonName = req.DNSNames[0]
		} else {
			req.CommonName = req.IPAddresses[0]
		}
	}
	return req, nil
}

func validHostname(n string) bool {
	n = strings.TrimPrefix(n, "*.")
	if n == "" || len(n) > 253 {
		return false
	}
	for _, label := range strings.Split(n, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
				return false
			}
		}
	}
	return true
}

// orderPayload asks the authority for a certificate: the names, and
// optionally where it is served at once.
type orderPayload struct {
	placement
	// Authority is the ID of the authority asked.
	Authority string   `json:"authority"`
	Names     []string `json:"names"`
}

// newACMEOrder puts an order to an authority in the pool. Placed on a door -
// here or later - it is asked at once, in the background (certs/request.go).
func (a *API) newACMEOrder(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := edition.Require(acmeEdition); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	var p orderPayload
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed order: "+err.Error())
		return
	}
	cfg := a.st.RawTLS(r.Context())
	auth, ok := cfg.ACME.AuthorityByID(p.Authority)
	if !ok {
		var known []string
		for _, x := range cfg.ACME.Authorities {
			known = append(known, x.ID+" ("+x.Name+")")
		}
		if len(known) == 0 {
			writeErr(w, http.StatusUnprocessableEntity, "no authority is set up: add one in the authorities drawer first")
			return
		}
		writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
			"unknown authority %q (set up: %s)", p.Authority, strings.Join(known, ", ")))
		return
	}
	var names []string
	for _, raw := range p.Names {
		n := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "."))
		if n == "" || slices.Contains(names, n) {
			continue
		}
		// What the TLS-ALPN challenge can prove: a host name the authority
		// connects to. A wildcard needs a DNS challenge, an address is not a
		// name an authority issues for.
		if strings.HasPrefix(n, "*.") || net.ParseIP(n) != nil || !validHostname(n) {
			writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
				"%q cannot be asked of the authority: host names only - no wildcard (it needs a DNS challenge) and no IP address", raw))
			return
		}
		names = append(names, n)
	}
	if len(names) == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "no name: an order asks for at least one host name")
		return
	}
	id := certs.OrderID(auth.ID, names)
	for _, o := range cfg.ACME.Orders {
		if o.ID == id {
			writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
				"%s is already asked of %s", strings.Join(names, ", "), auth.Name))
			return
		}
	}
	row := store.Certificate{
		ID: id, Source: store.CertSourceACME, Authority: auth.ID,
		Info: certs.Info{DNSNames: names}, CreatedAt: time.Now().Unix(),
	}
	if !a.place(w, r, &row, p.placement) {
		return
	}
	a.applyTLS(r.Context())
	a.auditEvent(r.Context(), actor, "certificate.acme", "certificate", row.ID, row.Name(), "", where(row))
	writeJSON(w, http.StatusCreated, a.orderView(r.Context(), row))
}

// importPayload is material that already exists somewhere else. Two shapes,
// one endpoint: a PEM pair (what a CA e-mails back, what certbot writes) or a
// PKCS#12 keystore (what the Java and Windows worlds hand out).
type importPayload struct {
	placement
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
	if len(m.Names()) == 0 {
		writeErr(w, http.StatusUnprocessableEntity,
			"this certificate names no host and no address (no subject alternative name): browsers accept none like it")
		return
	}
	a.saveMaterial(w, r, actor, p.placement, store.CertSourceImport, m, "certificate.import")
}

// generatePayload is a generation form: the names, who it is from, the key.
type generatePayload struct {
	placement
	Names []string `json:"names"`
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
	req, err := request(p.Names, p.KeyType, p.Days, p.Subject)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	m, err := certs.SelfSigned(req, time.Now())
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	a.saveMaterial(w, r, actor, p.placement, store.CertSourceSelfSigned, m, "certificate.self-signed")
}

// newSigningRequest is the offline door. The request leaves, the key does not.
// Whoever runs the company authority signs it, and the answer comes back
// through adopt - no network, no authority Meerkat has to know how to talk to.
// It lands in the pool unplaced: it serves nothing until then.
func (a *API) newSigningRequest(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p generatePayload
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	req, err := request(p.Names, p.KeyType, 0, p.Subject)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	csrPEM, keyPEM, err := certs.NewCSR(req)
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
		ID: newID(), Source: store.CertSourceCSR, CSRPEM: csrPEM, KeyPEM: keyPEM,
		Info: certs.Info{
			Subject: info.Subject, DNSNames: info.DNSNames,
			IPAddresses: info.IPAddresses, KeyType: info.KeyType,
		},
	}
	if err := a.st.SaveCertificate(r.Context(), row); err != nil {
		a.internal(w, err)
		return
	}
	a.auditEvent(r.Context(), actor, "certificate.signing-request", "certificate", row.ID, row.Name(), "", "")
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
		writeErr(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("%s already holds a certificate: adopting applies to a signing request waiting for its answer", row.Name()))
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
	a.auditEvent(r.Context(), actor, "certificate.adopt", "certificate", row.ID, row.Name(), "", m.Info.Issuer)
	writeJSON(w, http.StatusOK, viewOf(row))
}

// saveMaterial is the tail every creation door shares.
func (a *API) saveMaterial(w http.ResponseWriter, r *http.Request, actor store.User,
	p placement, source string, m *certs.Material, action string) {
	row := store.Certificate{
		ID: newID(), Source: source,
		CertPEM: m.CertPEM, KeyPEM: m.KeyPEM, Info: m.Info,
	}
	if !a.place(w, r, &row, p) {
		return
	}
	a.applyTLS(r.Context())
	a.auditEvent(r.Context(), actor, action, "certificate", row.ID, row.Name(), "", where(row))
	writeJSON(w, http.StatusCreated, viewOf(row))
}

func (a *API) deleteCertificate(w http.ResponseWriter, r *http.Request, actor store.User) {
	row, ok := a.find(r.Context(), r.PathValue("id"))
	if !ok {
		writeErr(w, http.StatusNotFound, "certificate not found")
		return
	}
	// An order leaves the settings: the authority is no longer asked for its
	// names. What it issued stays in the cache, unserved, and is replaced by
	// a fresh issue if the same names are ever ordered again.
	if row.Source == store.CertSourceACME {
		cfg := a.st.RawTLS(r.Context())
		kept := cfg.ACME.Orders[:0]
		for _, o := range cfg.ACME.Orders {
			if o.ID != row.ID {
				kept = append(kept, o)
			}
		}
		cfg.ACME.Orders = kept
		if err := a.st.SetSetting(r.Context(), store.SettingTLS, cfg); err != nil {
			a.internal(w, err)
			return
		}
		a.applyTLS(r.Context())
		a.auditEvent(r.Context(), actor, "certificate.delete", "certificate", row.ID, row.Name(), where(row), "")
		w.WriteHeader(http.StatusNoContent)
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
	a.auditEvent(r.Context(), actor, "certificate.delete", "certificate", row.ID, row.Name(), where(row), "")
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
	writePEM(w, fileName(row)+".csr", row.CSRPEM)
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
	writePEM(w, fileName(row)+".pem", row.CertPEM)
}

// fileName is the certificate's first name, a wildcard spelled so a file
// system takes it.
func fileName(c store.Certificate) string {
	return strings.ReplaceAll(c.Name(), "*", "wildcard")
}

func writePEM(w http.ResponseWriter, filename, body string) {
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	_, _ = w.Write([]byte(body))
}

// tlsPayload is the TLS screen's settings: the redirect and its HSTS, and what
// is actually running. The authorities have their own endpoints, and the
// orders live in the pool.
type tlsPayload struct {
	Redirect   bool `json:"redirect"`
	HSTSMaxAge int  `json:"hstsMaxAge,omitempty"`
	// State is what the supervisor last managed to do.
	State certs.State `json:"state"`
	// Published is where the runtime publishes this gateway's ports, inside
	// port to outside one: the links are written with what the world reaches
	// (19443), not with what the container listens on (9443).
	Published discovery.Published `json:"published"`
}

func (a *API) tlsView(ctx context.Context) tlsPayload {
	cfg := a.st.RawTLS(ctx)
	v := tlsPayload{Redirect: cfg.Redirect, HSTSMaxAge: cfg.HSTSMaxAge}
	if a.TLS != nil {
		v.State = a.TLS.State()
	}
	v.Published = discovery.Self(ctx)
	return v
}

func (a *API) getTLS(w http.ResponseWriter, r *http.Request, _ store.User) {
	writeJSON(w, http.StatusOK, a.tlsView(r.Context()))
}

func (a *API) putTLS(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p struct {
		Redirect   bool `json:"redirect"`
		HSTSMaxAge int  `json:"hstsMaxAge,omitempty"`
	}
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed TLS settings: "+err.Error())
		return
	}
	if p.HSTSMaxAge < 0 || p.HSTSMaxAge > certs.MaxHSTS {
		writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
			"hstsMaxAge %d: expected 0 (off) up to %d seconds (two years)", p.HSTSMaxAge, certs.MaxHSTS))
		return
	}
	before := a.tlsView(r.Context())
	cfg := a.st.RawTLS(r.Context())
	cfg.Redirect, cfg.HSTSMaxAge = p.Redirect, p.HSTSMaxAge
	if err := a.st.SetSetting(r.Context(), store.SettingTLS, cfg); err != nil {
		a.internal(w, err)
		return
	}
	a.applyTLS(r.Context())
	after := a.tlsView(r.Context())
	a.auditUpdate(r.Context(), actor, "tls.update", "settings", "", "", "", before, after)
	writeJSON(w, http.StatusOK, after)
}

// applyTLS re-reads everything and makes the running gateway match. A failure
// is not returned to the caller: the material IS saved, and what could not be
// applied is named in the TLS screen's State.Problems.
func (a *API) applyTLS(ctx context.Context) {
	if a.TLS == nil {
		return
	}
	_ = a.TLS.Reload(ctx)
	a.announce(ctx, store.TopicCertificates)
}
