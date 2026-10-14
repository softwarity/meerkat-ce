package admin

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/softwarity/meerkat/internal/discovery"
	"github.com/softwarity/meerkat/internal/store"
)

// The internal plane's certificate asked of cert-manager (SSL-13).
//
// A spare way, for an installation with cert-manager and nobody to write the
// Certificate beside the chart; the usual one is that manifest, its Secret
// mounted and picked from the file system. The screen only says "ask", and
// which of the issuers offered here signs: the names are the ones the runtime
// resolves to the internal plane, the namespace the gateway's own, the
// Certificate and its Secret the one name the chart granted.

// certManagerView is whether the gateway can ask, what it would ask for, and
// of whom.
type certManagerView struct {
	Available bool               `json:"available"`
	Why       string             `json:"why,omitempty"`
	Issuers   []discovery.Issuer `json:"issuers"`
	Names     []string           `json:"names"`
	Secret    string             `json:"secret,omitempty"`
}

// certManagerOn is the client of one plane's Certificate: the internal
// plane's is CertManager, the console's and the application's are in
// CertManagers (SSL-14). Nil when there is none.
func (a *API) certManagerOn(plane string) CertManager {
	if m := a.CertManagers[plane]; m != nil {
		return m
	}
	if plane == store.PlaneInternal {
		return a.CertManager
	}
	return nil
}

// certManagerBySecret is the client following one Secret, nil when none does.
func (a *API) certManagerBySecret(secret string) CertManager {
	for _, plane := range store.CertPlanes {
		if m := a.certManagerOn(plane); m != nil && m.SecretName() == secret {
			return m
		}
	}
	return nil
}

// requestedPlane reads ?plane= (or the body's), the internal plane when none.
func requestedPlane(raw string) (string, error) {
	switch raw {
	case "", store.PlaneInternal:
		return store.PlaneInternal, nil
	case store.PlaneConsole, store.PlaneApp:
		return raw, nil
	}
	return "", fmt.Errorf("plane %q: expected %s, %s or %s", raw, store.PlaneInternal, store.PlaneConsole, store.PlaneApp)
}

func (a *API) certManagerView(r *http.Request, plane string) (certManagerView, []discovery.Issuer) {
	v := certManagerView{Issuers: []discovery.Issuer{}, Names: []string{}}
	m := a.certManagerOn(plane)
	if m == nil {
		v.Why = a.CertManagerWhy
		if v.Why == "" {
			v.Why = "this gateway does not run on Kubernetes"
		}
		return v, nil
	}
	// The console's and the application's public names are a public
	// authority's business; what cert-manager signs here is the hop from a
	// proxy in front, which reaches the plane by its runtime names.
	if plane != store.PlaneInternal && !a.st.RawTLS(r.Context()).DoorOn(plane).Proxied {
		v.Why = "the " + planeWord(plane) + " serves its public names itself: a certificate for them comes from an authority (ACME), an import or the file system. cert-manager signs the hop from a proxy in front, once one is declared"
		return v, nil
	}
	v.Secret = m.SecretName()
	if names := a.PlaneNames(r.Context(), plane); len(names) > 0 {
		v.Names = names
	} else {
		v.Why = "the runtime does not say what the " + planeWord(plane) + " is called, so there is no name to ask a certificate for"
		return v, nil
	}
	issuers, err := m.Issuers(r.Context())
	if err != nil {
		v.Why = err.Error()
		return v, nil
	}
	v.Issuers = append(v.Issuers, issuers...)
	for _, i := range issuers {
		if i.Ready {
			v.Available = true
		}
	}
	if !v.Available {
		v.Why = "no CA issuer of cert-manager is ready in this cluster (a ca, vault or venafi one: a selfSigned or ACME issuer cannot sign for the internal plane)"
	}
	return v, issuers
}

func (a *API) getCertManager(w http.ResponseWriter, r *http.Request, _ store.User) {
	plane, err := requestedPlane(r.URL.Query().Get("plane"))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	v, _ := a.certManagerView(r, plane)
	writeJSON(w, http.StatusOK, v)
}

type certManagerPayload struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Plane is whose Certificate: the internal plane when absent, the console
	// or the application behind a proxy (SSL-14).
	Plane string `json:"plane,omitempty"`
}

// askCertManager creates (or re-aims) the Certificate, and the pool row that
// follows its Secret. The row is placed on the internal plane when that door
// has nothing yet; it serves from the moment cert-manager writes the Secret.
func (a *API) askCertManager(w http.ResponseWriter, r *http.Request, actor store.User) {
	var p certManagerPayload
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed request: "+err.Error())
		return
	}
	plane, err := requestedPlane(p.Plane)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	v, issuers := a.certManagerView(r, plane)
	if !v.Available {
		writeErr(w, http.StatusUnprocessableEntity, "cert-manager cannot be asked: "+v.Why)
		return
	}
	// The issuer is one offered here, ready - not a name the request makes up.
	var offered []string
	chosen := false
	for _, i := range issuers {
		if !i.Ready {
			continue
		}
		offered = append(offered, i.Kind+"/"+i.Name)
		if i.Kind == p.Kind && i.Name == p.Name {
			chosen = true
		}
	}
	if !chosen {
		writeErr(w, http.StatusUnprocessableEntity, fmt.Sprintf(
			"%s/%s is not a ready issuer of this cluster (allowed: %s)", p.Kind, p.Name, strings.Join(offered, ", ")))
		return
	}
	if err := a.certManagerOn(plane).Ensure(r.Context(), p.Kind, p.Name, v.Names); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	all, _, err := a.pool(r.Context())
	if err != nil {
		a.internal(w, err)
		return
	}
	internalTaken := false
	for _, c := range all {
		if c.Source == store.CertSourceCertManager && c.SecretName == v.Secret {
			// Asked again - another issuer, or after it was taken off: the
			// same row follows the same Secret, which cert-manager rewrites,
			// and it goes back on the plane it was asked for. Taken off and
			// asked again used to answer the row and leave it off, so the
			// screen showed nothing happening.
			if !c.On(plane) {
				c.Console, c.App, c.Internal = c.Console || plane == store.PlaneConsole,
					c.App || plane == store.PlaneApp, c.Internal || plane == store.PlaneInternal
				if err := a.st.PlaceCertificate(r.Context(), c.ID, c.Console, c.App, c.Internal); err != nil {
					a.internal(w, err)
					return
				}
				a.applyTLS(r.Context())
			}
			a.auditEvent(r.Context(), actor, "certificate.cert-manager", "certificate", c.ID, c.Name(), p.Kind+"/"+p.Name, where(c))
			writeJSON(w, http.StatusOK, a.issuedView(r, c))
			return
		}
		if c.Internal {
			internalTaken = true
		}
	}
	row := store.Certificate{
		ID: newID(), Source: store.CertSourceCertManager, SecretName: v.Secret,
		Internal: plane == store.PlaneInternal && !internalTaken,
		// Asked for the hop from the proxy in front: it is that plane's.
		Console: plane == store.PlaneConsole,
		App:     plane == store.PlaneApp,
	}
	// The names asked for, until the Secret says what was issued.
	row.Info.DNSNames = v.Names
	if err := a.st.SaveCertificate(r.Context(), row); err != nil {
		a.internal(w, err)
		return
	}
	a.applyTLS(r.Context())
	a.auditEvent(r.Context(), actor, "certificate.cert-manager", "certificate", row.ID, row.Name(), p.Kind+"/"+p.Name, where(row))
	writeJSON(w, http.StatusCreated, a.issuedView(r, row))
}

// issuedView is a cert-manager row as the pool shows it: waiting until the
// Secret holds a certificate, and what cert-manager says about it - who
// signs, when it renews, why it refused.
func (a *API) issuedView(r *http.Request, c store.Certificate) certView {
	v := viewOf(c)
	v.Waiting = c.CertPEM == ""
	m := a.certManagerBySecret(c.SecretName)
	if m == nil {
		v.Error = a.CertManagerWhy
		if v.Error == "" {
			v.Error = "nothing follows the Certificate " + c.SecretName + " on this gateway"
		}
		return v
	}
	st, err := m.Status(r.Context())
	switch {
	case err != nil:
		v.Error = err.Error()
	case !st.Exists:
		v.Error = "the Certificate " + m.SecretName() + " is gone from the cluster: ask cert-manager again"
	default:
		v.AuthorityName = st.IssuerName
		v.RenewsAt = st.RenewalTime
		if !st.Ready {
			v.Error = st.Message
		}
	}
	return v
}
