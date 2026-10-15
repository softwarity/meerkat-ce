package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// The authorities the platform publishes, offered as trusted authorities
// (SSL-09): what signs the services an upstream in HTTPS points at.
//
// Found, never named in advance: each source is asked or read where every
// installation of that platform keeps it, whatever the authority is called.
// Only PUBLIC certificates are read. The one place a platform keeps an
// authority's certificate beside its key - a cert-manager CA issuer's Secret -
// is never read: cert-manager is asked for a throwaway Certificate instead,
// and the Secret it writes for it carries the issuer's certificate in ca.crt,
// alone (probeIssuer).
//
// Sources, in the order they are offered:
//   - openshift     service-ca.crt, mounted by OpenShift in every pod: the
//                   authority of the Services' serving certificates
//   - cert-manager  the ClusterIssuers and this namespace's Issuers that sign
//                   with an authority of their own (ca, vault, venafi)
//   - trust-manager the Bundles that reach this namespace, as the ConfigMap
//                   trust-manager writes in it
//   - cluster       ca.crt, mounted in every pod: the API server's authority,
//                   rarely the services' - offered last, and said so
//
// Swarm, compose and a bare binary publish no authority: nothing to find, and
// the console keeps to the certificate imported by hand.

// TrustProbeEnv names the throwaway Certificate (and its Secret) the gateway
// may ask cert-manager for, to learn an issuer's certificate. The chart sets
// it with the rights on that one name; without it, the issuers are listed but
// cannot be taken.
const TrustProbeEnv = "MEERKAT_TRUST_PROBE"

// FoundAuthority is one authority the platform offers.
type FoundAuthority struct {
	// Ref is what the console sends back to take it: source and name.
	Ref    string `json:"ref"`
	Source string `json:"source"`
	// Name is the authority's own name where the platform gives one (the
	// issuer, the bundle), for the entry it becomes.
	Name string `json:"name"`
	// PEM is known at once for a file or a ConfigMap; an issuer's is asked of
	// cert-manager only when it is taken.
	PEM string `json:"pem,omitempty"`
	// Ready is false for an issuer cert-manager does not call ready; Why is
	// its message, or what keeps the authority from being taken.
	Ready bool   `json:"ready"`
	Why   string `json:"why,omitempty"`
}

// SourceNote says why a source offers nothing: absent from this cluster, or
// present but out of this gateway's rights - with what grants them.
type SourceNote struct {
	Source string `json:"source"`
	Why    string `json:"why"`
	// Absent is true when the platform simply does not have it.
	Absent bool `json:"absent,omitempty"`
}

// PlatformAuthorities is what the search found.
type PlatformAuthorities struct {
	// Kubernetes is false where there is nothing to search: the console then
	// offers no search at all.
	Kubernetes bool             `json:"kubernetes"`
	Found      []FoundAuthority `json:"found"`
	Notes      []SourceNote     `json:"notes"`
}

// FindAuthorities searches every source. One failing does not stop the
// others: each says why in Notes.
func FindAuthorities(ctx context.Context) PlatformAuthorities {
	out := PlatformAuthorities{Found: []FoundAuthority{}, Notes: []SourceNote{}}
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return out
	}
	out.Kubernetes = true
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()

	if pem, err := os.ReadFile(saDir + "/service-ca.crt"); err == nil {
		out.Found = append(out.Found, FoundAuthority{Ref: "openshift/service-ca", Source: "openshift",
			Name: "openshift-service-ca", PEM: string(pem), Ready: true})
	}
	api, err := newK8sAPI()
	if err != nil {
		out.Notes = append(out.Notes, SourceNote{Source: "cert-manager", Why: "the Kubernetes API cannot be reached: " + err.Error()})
	} else {
		issuers, note := listIssuers(ctx, api)
		out.Found = append(out.Found, issuers...)
		if note != nil {
			out.Notes = append(out.Notes, *note)
		}
		bundles, note := listBundles(ctx, api)
		out.Found = append(out.Found, bundles...)
		if note != nil {
			out.Notes = append(out.Notes, *note)
		}
	}
	if pem, err := os.ReadFile(saDir + "/ca.crt"); err == nil {
		out.Found = append(out.Found, FoundAuthority{Ref: "cluster/ca", Source: "cluster",
			Name: "kubernetes-cluster-ca", PEM: string(pem), Ready: true})
	}
	return out
}

// TakeAuthority answers the PEM of one authority FindAuthorities offered,
// read again from its source: the console sends a reference, never a
// certificate it says came from the cluster.
func TakeAuthority(ctx context.Context, ref string) (FoundAuthority, error) {
	source, name, _ := strings.Cut(ref, "/")
	switch source {
	case "openshift", "cluster":
		for _, f := range FindAuthorities(ctx).Found {
			if f.Ref == ref {
				return f, nil
			}
		}
		return FoundAuthority{}, fmt.Errorf("%s is not offered by this platform", ref)
	case "issuer", "clusterissuer":
		api, err := newK8sAPI()
		if err != nil {
			return FoundAuthority{}, fmt.Errorf("the Kubernetes API cannot be reached: %w", err)
		}
		kind := "Issuer"
		if source == "clusterissuer" {
			kind = "ClusterIssuer"
		}
		pem, err := probeIssuer(ctx, api, kind, name)
		if err != nil {
			return FoundAuthority{}, err
		}
		return FoundAuthority{Ref: ref, Source: "cert-manager", Name: name, PEM: pem, Ready: true}, nil
	case "bundle":
		api, err := newK8sAPI()
		if err != nil {
			return FoundAuthority{}, fmt.Errorf("the Kubernetes API cannot be reached: %w", err)
		}
		bundles, note := listBundles(ctx, api)
		for _, b := range bundles {
			if b.Ref == ref && b.PEM != "" {
				return b, nil
			}
		}
		if note != nil {
			return FoundAuthority{}, errors.New(note.Why)
		}
		return FoundAuthority{}, fmt.Errorf("the Bundle %s publishes nothing in this namespace", name)
	}
	return FoundAuthority{}, fmt.Errorf("unknown source %q: allowed are openshift, cluster, issuer, clusterissuer, bundle", source)
}

// k8sDo sends one request to the API server and reads the answer into out.
func k8sDo(ctx context.Context, api *k8sAPI, method, u string, body, out any) (int, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+api.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := api.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		var st struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(res.Body).Decode(&st)
		return res.StatusCode, fmt.Errorf("%s %s answered %d %s", method, u, res.StatusCode, st.Message)
	}
	if out != nil {
		return res.StatusCode, json.NewDecoder(res.Body).Decode(out)
	}
	return res.StatusCode, nil
}

func apiRoot(api *k8sAPI) string {
	return strings.TrimSuffix(api.base, "/api/v1/namespaces/"+api.ns)
}

// listIssuers offers the issuers that sign with an authority of their own.
// Their certificate is not read here: it is asked of cert-manager when one is
// taken.
func listIssuers(ctx context.Context, api *k8sAPI) ([]FoundAuthority, *SourceNote) {
	group := apiRoot(api) + "/apis/cert-manager.io/v1"
	probe := strings.TrimSpace(os.Getenv(TrustProbeEnv))
	var out []FoundAuthority
	for _, src := range []struct{ kind, ref, url string }{
		{"ClusterIssuer", "clusterissuer", group + "/clusterissuers"},
		{"Issuer", "issuer", group + "/namespaces/" + api.ns + "/issuers"},
	} {
		var list struct {
			Items []k8sIssuer `json:"items"`
		}
		code, err := k8sDo(ctx, api, http.MethodGet, src.url, nil, &list)
		switch {
		case code == http.StatusNotFound && src.kind == "ClusterIssuer":
			return nil, &SourceNote{Source: "cert-manager", Absent: true, Why: "cert-manager is not installed in this cluster"}
		case code == http.StatusForbidden:
			return out, &SourceNote{Source: "cert-manager", Why: "listing the " + src.kind + "s is not granted: install the chart with trustDiscovery.certManager=true"}
		case err != nil:
			return out, &SourceNote{Source: "cert-manager", Why: err.Error()}
		}
		for _, it := range list.Items {
			typ := ""
			for _, t := range []string{"ca", "vault", "venafi"} {
				if _, ok := it.Spec[t]; ok {
					typ = t
				}
			}
			if typ == "" {
				continue // ACME is public, self-signed has no authority to share
			}
			f := FoundAuthority{Ref: src.ref + "/" + it.Metadata.Name, Source: "cert-manager", Name: it.Metadata.Name}
			c, ok := ready(it.Status.Conditions)
			f.Ready = ok
			switch {
			case !ok:
				f.Why = c.Message
			case probe == "":
				f.Ready = false
				f.Why = "its certificate is asked of cert-manager, which the chart does not grant: install it with trustDiscovery.certManager=true"
			}
			out = append(out, f)
		}
	}
	return out, nil
}

// probeMu serializes this node's probes: there is one Certificate name.
var probeMu sync.Mutex

// probeIssuer learns an issuer's certificate without ever reading its key: a
// throwaway Certificate is asked of it, cert-manager writes the issuer's
// certificate in that Certificate's Secret (ca.crt), and both are deleted.
func probeIssuer(ctx context.Context, api *k8sAPI, kind, name string) (string, error) {
	probe := strings.TrimSpace(os.Getenv(TrustProbeEnv))
	if probe == "" {
		return "", errors.New("asking cert-manager for an issuer's certificate is not granted: install the chart with trustDiscovery.certManager=true")
	}
	probeMu.Lock()
	defer probeMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	certs := apiRoot(api) + "/apis/cert-manager.io/v1/namespaces/" + api.ns + "/certificates"
	secret := api.base + "/secrets/" + probe
	cleanup := func() {
		c, done := context.WithTimeout(context.WithoutCancel(ctx), lookupTimeout)
		defer done()
		_, _ = k8sDo(c, api, http.MethodDelete, certs+"/"+probe, nil, nil)
		_, _ = k8sDo(c, api, http.MethodDelete, secret, nil, nil)
	}
	cleanup() // a previous probe a crash left behind
	defer cleanup()

	cert := map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": map[string]any{
			"name": probe, "namespace": api.ns,
			"labels": map[string]string{"app.kubernetes.io/managed-by": "meerkat"},
		},
		"spec": map[string]any{
			"secretName": probe,
			// A name nothing resolves: the certificate is never served.
			"dnsNames":   []string{probe + ".invalid"},
			"duration":   "1h",
			"issuerRef":  map[string]string{"name": name, "kind": kind, "group": "cert-manager.io"},
			"privateKey": map[string]any{"algorithm": "ECDSA", "size": 256},
		},
	}
	if code, err := k8sDo(ctx, api, http.MethodPost, certs, cert, nil); err != nil {
		if code == http.StatusForbidden {
			return "", errors.New("creating a Certificate is not granted: install the chart with trustDiscovery.certManager=true")
		}
		return "", fmt.Errorf("asking %s %s for a certificate: %w", kind, name, err)
	}
	for {
		var s struct {
			Data map[string][]byte `json:"data"` // base64 in JSON, bytes here
		}
		code, err := k8sDo(ctx, api, http.MethodGet, secret, nil, &s)
		if err == nil {
			if ca := strings.TrimSpace(string(s.Data["ca.crt"])); ca != "" {
				return ca + "\n", nil
			}
			if len(s.Data["tls.crt"]) > 0 {
				return "", fmt.Errorf("%s %s signed the certificate but gives no ca.crt: its authority is not published this way", kind, name)
			}
		} else if code != http.StatusNotFound {
			return "", fmt.Errorf("reading the Secret %s: %w", probe, err)
		}
		select {
		case <-ctx.Done():
			var c k8sCertificate
			_, _ = k8sDo(context.WithoutCancel(ctx), api, http.MethodGet, certs+"/"+probe, nil, &c)
			if cond, ok := ready(c.Status.Conditions); !ok && cond.Message != "" {
				return "", fmt.Errorf("%s %s did not sign in 30s: %s", kind, name, cond.Message)
			}
			return "", fmt.Errorf("%s %s did not sign in 30s", kind, name)
		case <-time.After(time.Second):
		}
	}
}

// listBundles offers what trust-manager publishes in this namespace: each
// Bundle writes a ConfigMap of its own name, under the key it names.
func listBundles(ctx context.Context, api *k8sAPI) ([]FoundAuthority, *SourceNote) {
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Target struct {
					ConfigMap *struct {
						Key string `json:"key"`
					} `json:"configMap"`
				} `json:"target"`
			} `json:"spec"`
		} `json:"items"`
	}
	code, err := k8sDo(ctx, api, http.MethodGet, apiRoot(api)+"/apis/trust.cert-manager.io/v1alpha1/bundles", nil, &list)
	switch {
	case code == http.StatusNotFound:
		return nil, &SourceNote{Source: "trust-manager", Absent: true, Why: "trust-manager is not installed in this cluster"}
	case code == http.StatusForbidden:
		return nil, &SourceNote{Source: "trust-manager", Why: "listing the Bundles is not granted: install the chart with trustDiscovery.trustManager=true"}
	case err != nil:
		return nil, &SourceNote{Source: "trust-manager", Why: err.Error()}
	}
	var out []FoundAuthority
	for _, b := range list.Items {
		if b.Spec.Target.ConfigMap == nil {
			continue // published as a Secret: not read, a Secret is not where public things go
		}
		f := FoundAuthority{Ref: "bundle/" + b.Metadata.Name, Source: "trust-manager", Name: b.Metadata.Name}
		var cm struct {
			Data map[string]string `json:"data"`
		}
		code, err := k8sDo(ctx, api, http.MethodGet, api.base+"/configmaps/"+b.Metadata.Name, nil, &cm)
		switch {
		case code == http.StatusNotFound:
			f.Why = "it does not reach this namespace: its target namespaceSelector leaves it out"
		case code == http.StatusForbidden:
			f.Why = "reading its ConfigMap is not granted: install the chart with trustDiscovery.trustManager=true"
		case err != nil:
			f.Why = err.Error()
		default:
			f.PEM = cm.Data[b.Spec.Target.ConfigMap.Key]
			f.Ready = f.PEM != ""
			if !f.Ready {
				f.Why = "its ConfigMap holds no " + b.Spec.Target.ConfigMap.Key
			}
		}
		out = append(out, f)
	}
	return out, nil
}

// IssuedFor is one service's certificate as the platform records it: which
// authority signs it, for which names. cert-manager keeps a Certificate per
// request (its issuer and dnsNames); OpenShift marks a Service it gives a
// serving certificate with an annotation. Read in this namespace and the ones
// the chart lets it watch - no key is in either.
type IssuedFor struct {
	Namespace string `json:"namespace"`
	// Name is the Certificate's, or the Service's for OpenShift.
	Name string `json:"name"`
	// Authority is the FoundAuthority.Ref that signs it: clusterissuer/x,
	// issuer/x (this namespace's own), openshift/service-ca.
	Authority string   `json:"authority"`
	Names     []string `json:"names"`
}

// OpenShiftServingAnnotation marks a Service OpenShift's service CA issues a
// serving certificate for.
const OpenShiftServingAnnotation = "service.beta.openshift.io/serving-cert-secret-name"

// IssuedCertificates says what each authority signs here. A source it cannot
// read is said in the notes, and the others still count.
func IssuedCertificates(ctx context.Context) ([]IssuedFor, []SourceNote) {
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return nil, nil
	}
	api, err := newK8sAPI()
	if err != nil {
		return nil, []SourceNote{{Source: "cert-manager", Why: "the Kubernetes API cannot be reached: " + err.Error()}}
	}
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	var out []IssuedFor
	var notes []SourceNote
	openshift := false
	if _, err := os.Stat(saDir + "/service-ca.crt"); err == nil {
		openshift = true
	}
	for _, ns := range watchedNamespaces(api.ns) {
		var certs struct {
			Items []struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
				Spec struct {
					DNSNames  []string `json:"dnsNames"`
					IssuerRef struct {
						Name string `json:"name"`
						Kind string `json:"kind"`
					} `json:"issuerRef"`
				} `json:"spec"`
			} `json:"items"`
		}
		code, err := k8sDo(ctx, api, http.MethodGet, apiRoot(api)+"/apis/cert-manager.io/v1/namespaces/"+ns+"/certificates", nil, &certs)
		switch {
		case code == http.StatusNotFound:
			// no cert-manager: nothing to say per namespace
		case code == http.StatusForbidden:
			notes = append(notes, SourceNote{Source: "cert-manager", Why: "listing the Certificates of " + ns + " is not granted: install the chart with trustDiscovery.certManager=true"})
		case err != nil:
			notes = append(notes, SourceNote{Source: "cert-manager", Why: err.Error()})
		default:
			for _, c := range certs.Items {
				ref := "issuer/" + c.Spec.IssuerRef.Name
				if c.Spec.IssuerRef.Kind == "ClusterIssuer" {
					ref = "clusterissuer/" + c.Spec.IssuerRef.Name
				} else if ns != api.ns {
					ref = "issuer/" + ns + "/" + c.Spec.IssuerRef.Name // not offered: another namespace's Issuer
				}
				out = append(out, IssuedFor{Namespace: ns, Name: c.Metadata.Name, Authority: ref, Names: c.Spec.DNSNames})
			}
		}
		if !openshift {
			continue
		}
		var svcs struct {
			Items []struct {
				Metadata struct {
					Name        string            `json:"name"`
					Annotations map[string]string `json:"annotations"`
				} `json:"metadata"`
			} `json:"items"`
		}
		if _, err := k8sDo(ctx, api, http.MethodGet, api.in(ns).base+"/services", nil, &svcs); err != nil {
			continue
		}
		for _, s := range svcs.Items {
			if s.Metadata.Annotations[OpenShiftServingAnnotation] == "" {
				continue
			}
			n := s.Metadata.Name
			out = append(out, IssuedFor{Namespace: ns, Name: n, Authority: "openshift/service-ca",
				Names: []string{n + "." + ns + ".svc", n + "." + ns + ".svc.cluster.local"}})
		}
	}
	return out, notes
}

// ServiceTarget is the Kubernetes Service an upstream host names, and the
// names a certificate for it may carry.
type ServiceTarget struct {
	Name      string
	Namespace string
	Ports     []int
}

// ServiceOf resolves an upstream host to a Service of a namespace this
// gateway reads - "geoserver", "geoserver.canopy", "geoserver.canopy.svc",
// "geoserver.canopy.svc.cluster.local" - and asks its ports. false when the
// host is not one, or the Service is not there.
func ServiceOf(ctx context.Context, host string) (ServiceTarget, bool) {
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return ServiceTarget{}, false
	}
	api, err := newK8sAPI()
	if err != nil {
		return ServiceTarget{}, false
	}
	parts := strings.Split(strings.TrimSuffix(host, "."), ".")
	name, ns := parts[0], api.ns
	switch {
	case len(parts) == 1:
	case len(parts) == 2, len(parts) == 3 && parts[2] == "svc", len(parts) == 5 && parts[2] == "svc":
		ns = parts[1]
	default:
		return ServiceTarget{}, false
	}
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	var svc struct {
		Spec struct {
			Ports []struct {
				Port int `json:"port"`
			} `json:"ports"`
		} `json:"spec"`
	}
	if _, err := k8sDo(ctx, api, http.MethodGet, api.in(ns).base+"/services/"+name, nil, &svc); err != nil {
		return ServiceTarget{}, false
	}
	t := ServiceTarget{Name: name, Namespace: ns}
	for _, p := range svc.Spec.Ports {
		t.Ports = append(t.Ports, p.Port)
	}
	return t, true
}
