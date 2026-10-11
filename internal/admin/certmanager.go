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

func (a *API) certManagerView(r *http.Request) (certManagerView, []discovery.Issuer) {
	v := certManagerView{Issuers: []discovery.Issuer{}, Names: []string{}}
	if a.CertManager == nil {
		v.Why = a.CertManagerWhy
		if v.Why == "" {
			v.Why = "this gateway does not run on Kubernetes"
		}
		return v, nil
	}
	v.Secret = a.CertManager.SecretName()
	if names := a.internalNames(r.Context()); len(names) > 0 {
		v.Names = names
	} else {
		v.Why = "the runtime does not say what the internal plane is called, so there is no name to ask a certificate for"
		return v, nil
	}
	issuers, err := a.CertManager.Issuers(r.Context())
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
	v, _ := a.certManagerView(r)
	writeJSON(w, http.StatusOK, v)
}

type certManagerPayload struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
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
	v, issuers := a.certManagerView(r)
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
	if err := a.CertManager.Ensure(r.Context(), p.Kind, p.Name, v.Names); err != nil {
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
			// Asked again - another issuer: the same row follows the same
			// Secret, which cert-manager rewrites.
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
		Internal: !internalTaken,
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
	if a.CertManager == nil {
		v.Error = a.CertManagerWhy
		return v
	}
	st, err := a.CertManager.Status(r.Context())
	switch {
	case err != nil:
		v.Error = err.Error()
	case !st.Exists:
		v.Error = "the Certificate " + a.CertManager.SecretName() + " is gone from the cluster: ask cert-manager again"
	default:
		v.AuthorityName = st.IssuerName
		v.RenewsAt = st.RenewalTime
		if !st.Ready {
			v.Error = st.Message
		}
	}
	return v
}
