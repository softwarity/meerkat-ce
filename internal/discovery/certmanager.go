package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// The internal plane's certificate, asked of cert-manager by the gateway
// itself (SSL-13).
//
// The usual way stays the deployment's: a Certificate declared beside the
// chart, its Secret mounted, picked from the file system (SSL-11). This one is
// for an installation that has cert-manager and nobody to write that
// manifest. The console only says "ask": the names are the ones the runtime
// resolves to the internal plane (InternalOrigin), the namespace is the
// gateway's own, the Certificate and its Secret carry the one name the chart
// granted (MEERKAT_CERT_MANAGER_SECRET) - nothing typed on a screen reaches
// the request.
//
// The Secret is read through the API rather than mounted: no Deployment to
// change, no restart. It is WATCHED, so a renewal - cert-manager rewriting the
// same Secret before the end - reaches the internal door when it happens, on
// every node, each holding the pair in memory: the key never enters the
// database.

// CertManagerEnv names the Certificate and its Secret. The chart sets it with
// the rights to manage them (internalTls.certManager); unset, the gateway asks
// nothing of cert-manager.
const CertManagerEnv = "MEERKAT_CERT_MANAGER_SECRET"

// Issuer is one issuer cert-manager offers this namespace.
type Issuer struct {
	// Kind is ClusterIssuer or Issuer (this namespace's own).
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Type is what signs: ca, acme, selfSigned, vault, venafi.
	Type  string `json:"type,omitempty"`
	Ready bool   `json:"ready"`
	// Why is the issuer's own message when it is not ready.
	Why string `json:"why,omitempty"`
}

// IssuedStatus is where the Certificate stands, as cert-manager says.
type IssuedStatus struct {
	Exists bool   `json:"exists"`
	Ready  bool   `json:"ready"`
	Reason string `json:"reason,omitempty"`
	// Message is the condition's own sentence: why it is not ready.
	Message     string `json:"message,omitempty"`
	RenewalTime int64  `json:"renewalTime,omitempty"`
	IssuerKind  string `json:"issuerKind,omitempty"`
	IssuerName  string `json:"issuerName,omitempty"`
}

// CertManager talks to cert-manager for the one Certificate it may manage.
type CertManager struct {
	api  *k8sAPI
	root string // https://host:port
	name string

	mu      sync.Mutex
	certPEM string
	keyPEM  string
}

// NewCertManager is nil, with the reason, when this gateway cannot ask:
// another runtime, or a chart that did not grant it.
func NewCertManager() (*CertManager, string) {
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return nil, "this gateway does not run on Kubernetes"
	}
	name := strings.TrimSpace(os.Getenv(CertManagerEnv))
	if name == "" {
		return nil, "the chart does not let it ask cert-manager: install it with internalTls.certManager=true"
	}
	api, err := newK8sAPI()
	if err != nil {
		return nil, "it cannot talk to Kubernetes: " + err.Error()
	}
	return newCertManager(api, name), ""
}

func newCertManager(api *k8sAPI, name string) *CertManager {
	return &CertManager{api: api, name: name, root: strings.TrimSuffix(api.base, "/api/v1/namespaces/"+api.ns)}
}

// SecretName is the Certificate's name, and its Secret's.
func (m *CertManager) SecretName() string { return m.name }

func (m *CertManager) group() string { return m.root + "/apis/cert-manager.io/v1" }

func (m *CertManager) certificate() string {
	return m.group() + "/namespaces/" + m.api.ns + "/certificates"
}

// do sends one request to the API server.
func (m *CertManager) do(ctx context.Context, method, u, contentType string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+m.api.token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return m.api.client.Do(req)
}

// answer reads a reply into out, or says what the API server refused.
func answer(res *http.Response, what string, out any) error {
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		var st struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(res.Body).Decode(&st)
		return &apiError{code: res.StatusCode, what: what, message: st.Message}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

type apiError struct {
	code    int
	what    string
	message string
}

func (e *apiError) Error() string {
	switch e.code {
	case http.StatusForbidden:
		return e.what + ": its service account may not - the chart grants it with internalTls.certManager=true"
	case http.StatusNotFound:
		return e.what + ": not found"
	}
	if e.message != "" {
		return fmt.Sprintf("%s: %d %s", e.what, e.code, e.message)
	}
	return fmt.Sprintf("%s: answered %d", e.what, e.code)
}

func isCode(err error, code int) bool {
	e, ok := err.(*apiError)
	return ok && e.code == code
}

type k8sIssuer struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec   map[string]json.RawMessage `json:"spec"`
	Status struct {
		Conditions []k8sCondition `json:"conditions"`
	} `json:"status"`
}

type k8sCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func ready(cs []k8sCondition) (k8sCondition, bool) {
	for _, c := range cs {
		if c.Type == "Ready" {
			return c, c.Status == "True"
		}
	}
	return k8sCondition{}, false
}

// signsForInside are the issuer types fit for the internal plane: each signs
// with an authority that stays the same, the one the services trust once. A
// selfSigned issuer makes every certificate its own authority, a new one at
// each renewal; an ACME one is a public authority, which cannot see the
// cluster's names.
var signsForInside = map[string]bool{"ca": true, "vault": true, "venafi": true}

// Issuers lists the ClusterIssuers, then this namespace's Issuers, of the
// types fit for the internal plane. An error says why there is none to offer:
// cert-manager absent, or no right to look.
func (m *CertManager) Issuers(ctx context.Context) ([]Issuer, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	var out []Issuer
	for _, src := range []struct{ kind, url string }{
		{"ClusterIssuer", m.group() + "/clusterissuers"},
		{"Issuer", m.group() + "/namespaces/" + m.api.ns + "/issuers"},
	} {
		res, err := m.do(ctx, http.MethodGet, src.url, "", nil)
		if err != nil {
			return nil, fmt.Errorf("listing the %ss: %w", src.kind, err)
		}
		var list struct {
			Items []k8sIssuer `json:"items"`
		}
		if err := answer(res, "listing the "+src.kind+"s", &list); err != nil {
			if src.kind == "ClusterIssuer" && isCode(err, http.StatusNotFound) {
				return nil, fmt.Errorf("cert-manager is not installed in this cluster: the API server serves no cert-manager.io")
			}
			if src.kind == "Issuer" {
				continue // the namespace's own: a nice-to-have
			}
			return nil, err
		}
		for _, it := range list.Items {
			i := Issuer{Kind: src.kind, Name: it.Metadata.Name}
			for _, t := range []string{"ca", "acme", "selfSigned", "vault", "venafi"} {
				if _, ok := it.Spec[t]; ok {
					i.Type = t
				}
			}
			if !signsForInside[i.Type] {
				continue
			}
			c, ok := ready(it.Status.Conditions)
			i.Ready = ok
			if !ok {
				i.Why = c.Message
			}
			out = append(out, i)
		}
	}
	return out, nil
}

// Ensure creates the Certificate for names, signed by the issuer, or brings
// the one already there to them.
func (m *CertManager) Ensure(ctx context.Context, issuerKind, issuerName string, names []string) error {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	spec := map[string]any{
		"secretName": m.name,
		"dnsNames":   names,
		"issuerRef":  map[string]string{"name": issuerName, "kind": issuerKind, "group": "cert-manager.io"},
		// A new key at each renewal: a key that never changes is the one a
		// leak keeps working with.
		"privateKey": map[string]any{"algorithm": "ECDSA", "size": 256, "rotationPolicy": "Always"},
	}
	cert := map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": map[string]any{
			"name":      m.name,
			"namespace": m.api.ns,
			"labels":    map[string]string{"app.kubernetes.io/managed-by": "meerkat"},
		},
		"spec": spec,
	}
	res, err := m.do(ctx, http.MethodPost, m.certificate(), "application/json", cert)
	if err != nil {
		return fmt.Errorf("creating the Certificate %s: %w", m.name, err)
	}
	err = answer(res, "creating the Certificate "+m.name, nil)
	if !isCode(err, http.StatusConflict) {
		return err
	}
	res, err = m.do(ctx, http.MethodPatch, m.certificate()+"/"+m.name, "application/merge-patch+json", map[string]any{"spec": spec})
	if err != nil {
		return fmt.Errorf("updating the Certificate %s: %w", m.name, err)
	}
	return answer(res, "updating the Certificate "+m.name, nil)
}

type k8sCertificate struct {
	Spec struct {
		IssuerRef struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"issuerRef"`
	} `json:"spec"`
	Status struct {
		Conditions  []k8sCondition `json:"conditions"`
		RenewalTime string         `json:"renewalTime"`
	} `json:"status"`
}

// Status says where the Certificate stands.
func (m *CertManager) Status(ctx context.Context) (IssuedStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	res, err := m.do(ctx, http.MethodGet, m.certificate()+"/"+m.name, "", nil)
	if err != nil {
		return IssuedStatus{}, fmt.Errorf("reading the Certificate %s: %w", m.name, err)
	}
	var c k8sCertificate
	if err := answer(res, "reading the Certificate "+m.name, &c); err != nil {
		if isCode(err, http.StatusNotFound) {
			return IssuedStatus{}, nil
		}
		return IssuedStatus{}, err
	}
	cond, ok := ready(c.Status.Conditions)
	s := IssuedStatus{Exists: true, Ready: ok, Reason: cond.Reason, IssuerKind: c.Spec.IssuerRef.Kind, IssuerName: c.Spec.IssuerRef.Name}
	if !ok {
		s.Message = cond.Message
	}
	if t, err := time.Parse(time.RFC3339, c.Status.RenewalTime); err == nil {
		s.RenewalTime = t.Unix()
	}
	return s, nil
}

// Delete removes the Certificate and the Secret it wrote, which cert-manager
// leaves behind on its own. Gone already is done.
func (m *CertManager) Delete(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	for _, u := range []struct{ what, url string }{
		{"the Certificate " + m.name, m.certificate() + "/" + m.name},
		{"the Secret " + m.name, m.api.base + "/secrets/" + m.name},
	} {
		res, err := m.do(ctx, http.MethodDelete, u.url, "", nil)
		if err != nil {
			return fmt.Errorf("deleting %s: %w", u.what, err)
		}
		if err := answer(res, "deleting "+u.what, nil); err != nil && !isCode(err, http.StatusNotFound) {
			return err
		}
	}
	m.mu.Lock()
	m.certPEM, m.keyPEM = "", ""
	m.mu.Unlock()
	return nil
}

// Pair is the certificate and key the Secret last held, on this node.
func (m *CertManager) Pair() (certPEM, keyPEM string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.certPEM, m.keyPEM, m.certPEM != "" && m.keyPEM != ""
}

// Watch keeps the pair as the Secret holds it, calling issued each time it
// changes, and moved each time the Certificate does - a request refused, a
// renewal planned - for the screen to read it again. It returns with ctx.
func (m *CertManager) Watch(ctx context.Context, issued, moved func()) {
	go m.watchOne(ctx, m.api.base+"/secrets", func(typ string, raw json.RawMessage) {
		var s struct {
			Data map[string][]byte `json:"data"`
		}
		_ = json.Unmarshal(raw, &s)
		cert, key := string(s.Data["tls.crt"]), string(s.Data["tls.key"])
		if typ == "DELETED" {
			cert, key = "", ""
		}
		m.mu.Lock()
		changed := cert != m.certPEM || key != m.keyPEM
		m.certPEM, m.keyPEM = cert, key
		m.mu.Unlock()
		if changed {
			issued()
		}
	})
	m.watchOne(ctx, m.certificate(), func(string, json.RawMessage) { moved() })
}

// watchOne follows the one object of collection named m.name: a LIST, then a
// WATCH from its version, both narrowed by name - which is what lets the
// chart grant them on that name alone. A stream that ends is resumed, one too
// old is listed again, an error waits longer each time.
func (m *CertManager) watchOne(ctx context.Context, collection string, on func(typ string, raw json.RawMessage)) {
	sel := "fieldSelector=" + url.QueryEscape("metadata.name="+m.name)
	wait := retryMin
	said := ""
	for ctx.Err() == nil {
		rv, err := m.listOne(ctx, collection+"?"+sel, on)
		if err != nil {
			if err.Error() != said {
				slog.Warn("cert-manager: the internal certificate is not followed", "err", err)
				said = err.Error()
			}
			if !backoff(ctx, &wait) {
				return
			}
			continue
		}
		wait, said = retryMin, ""
		for ctx.Err() == nil {
			next, gone, err := m.watchFrom(ctx, collection+"?"+sel+"&watch=1&allowWatchBookmarks=true&resourceVersion="+url.QueryEscape(rv), on)
			if gone {
				break
			}
			if err != nil {
				if !backoff(ctx, &wait) {
					return
				}
				continue
			}
			wait = retryMin
			rv = next
		}
	}
}

func (m *CertManager) listOne(ctx context.Context, u string, on func(string, json.RawMessage)) (string, error) {
	lctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	res, err := m.do(lctx, http.MethodGet, u, "", nil)
	if err != nil {
		return "", err
	}
	var list struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Items []json.RawMessage `json:"items"`
	}
	if err := answer(res, "reading "+m.name, &list); err != nil {
		return "", err
	}
	if len(list.Items) == 0 {
		on("DELETED", nil)
	}
	for _, it := range list.Items {
		on("ADDED", it)
	}
	return list.Metadata.ResourceVersion, nil
}

func (m *CertManager) watchFrom(ctx context.Context, u string, on func(string, json.RawMessage)) (string, bool, error) {
	rv := ""
	if i := strings.LastIndex(u, "resourceVersion="); i >= 0 {
		rv, _ = url.QueryUnescape(u[i+len("resourceVersion="):])
	}
	res, err := m.do(ctx, http.MethodGet, u, "", nil)
	if err != nil {
		return rv, false, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusGone {
		return rv, true, nil
	}
	if res.StatusCode != http.StatusOK {
		return rv, false, fmt.Errorf("watching %s answered %s", m.name, res.Status)
	}
	dec := json.NewDecoder(res.Body)
	for {
		var ev struct {
			Type   string          `json:"type"`
			Object json.RawMessage `json:"object"`
		}
		if err := dec.Decode(&ev); err != nil {
			if ctx.Err() != nil {
				return rv, false, ctx.Err()
			}
			return rv, false, nil
		}
		var meta struct {
			Metadata k8sMeta `json:"metadata"`
			Code     int     `json:"code"`
		}
		_ = json.Unmarshal(ev.Object, &meta)
		switch ev.Type {
		case "ERROR":
			if meta.Code == http.StatusGone {
				return rv, true, nil
			}
			return rv, false, fmt.Errorf("watching %s: error %d", m.name, meta.Code)
		case "BOOKMARK":
			rv = meta.Metadata.ResourceVersion
			continue
		}
		if meta.Metadata.ResourceVersion != "" {
			rv = meta.Metadata.ResourceVersion
		}
		on(ev.Type, ev.Object)
	}
}
