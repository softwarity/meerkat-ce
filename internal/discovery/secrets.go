package discovery

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
)

// The Kubernetes Secrets an OIDC client's secret may be read from: the one the
// application's own chart generates (SAUTH-05).
//
// What is OFFERED is what the applications behind the routes use: each
// route's upstream is a Service, the Service selects pods, and the pods name
// the Secrets they read - a key in an environment variable (secretKeyRef), a
// whole Secret as variables (envFrom) or as files (a volume). DbGate reading
// OAUTH_CLIENT_SECRET from dbgate-oidc/client-secret offers exactly that key.
// Nothing is listed: the offer is built from Services and pods the gateway
// reads already, and a whole Secret is read only for the names of its keys.
//
// What is READ is any Secret of the cluster, by decision: `get` on secrets
// cluster-wide (rbac.readSecrets, on unless false). A client's reference typed
// by hand works too, wherever the Secret lives.

// SecretKey is one key of one Secret: where a value is read from.
type SecretKey struct {
	Namespace string
	Name      string
	Key       string
}

// Ref is the key as it is written: namespace/name/key.
func (k SecretKey) Ref() string { return k.Namespace + "/" + k.Name + "/" + k.Key }

// ParseSecretRef reads namespace/name/key.
func ParseSecretRef(ref string) (SecretKey, error) {
	parts := strings.Split(strings.TrimSpace(ref), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return SecretKey{}, fmt.Errorf("%q is not a Secret key: write namespace/name/key, as canopy/my-app-oidc/client-secret", ref)
	}
	return SecretKey{Namespace: parts[0], Name: parts[1], Key: parts[2]}, nil
}

// SecretOffer is one Secret as the console offers it: the keys the
// applications read in it, and the routes those applications are behind.
type SecretOffer struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	Keys      []string `json:"keys"`
	Routes    []string `json:"routes"`
}

// RouteTarget is a route's upstream host, by the route's name.
type RouteTarget struct {
	Route string
	Host  string
}

// errNotKubernetes is what reading a Secret says off Kubernetes.
var errNotKubernetes = errors.New("this gateway does not run in Kubernetes: there is no Secret to read")

// secretsNotGranted is what a refused read says, with the way to grant it.
const secretsNotGranted = "reading Secrets is not granted: install the chart with rbac.readSecrets left on"

// OfferSecrets says which Secrets the applications behind these routes read,
// and which keys. Never a value. Empty outside Kubernetes. A route whose host
// is not a Service, or whose pods cannot be read here (a namespace the chart
// does not watch), offers nothing; the reasons come back as notes.
func OfferSecrets(ctx context.Context, routes []RouteTarget) ([]SecretOffer, []string) {
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return []SecretOffer{}, nil
	}
	api, err := newK8sAPI()
	if err != nil {
		return []SecretOffer{}, []string{"the Kubernetes API cannot be reached: " + err.Error()}
	}
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	return offerSecrets(ctx, api, routes)
}

// podSecrets is what a pod's spec says of Secrets.
type podSecrets struct {
	Spec struct {
		Containers     []containerSecrets `json:"containers"`
		InitContainers []containerSecrets `json:"initContainers"`
		Volumes        []struct {
			Secret *struct {
				SecretName string `json:"secretName"`
				Items      []struct {
					Key string `json:"key"`
				} `json:"items"`
			} `json:"secret"`
			Projected *struct {
				Sources []struct {
					Secret *struct {
						Name  string `json:"name"`
						Items []struct {
							Key string `json:"key"`
						} `json:"items"`
					} `json:"secret"`
				} `json:"sources"`
			} `json:"projected"`
		} `json:"volumes"`
	} `json:"spec"`
}

type containerSecrets struct {
	Env []struct {
		ValueFrom *struct {
			SecretKeyRef *struct {
				Name string `json:"name"`
				Key  string `json:"key"`
			} `json:"secretKeyRef"`
		} `json:"valueFrom"`
	} `json:"env"`
	EnvFrom []struct {
		SecretRef *struct {
			Name string `json:"name"`
		} `json:"secretRef"`
	} `json:"envFrom"`
}

// serviceOfHost reads a route's host as a Service: name, name.ns, name.ns.svc
// or name.ns.svc.cluster.local; a bare name is this namespace's.
func serviceOfHost(host, own string) (name, ns string, ok bool) {
	parts := strings.Split(strings.TrimSuffix(host, "."), ".")
	switch {
	case len(parts) == 1:
		return parts[0], own, parts[0] != ""
	case len(parts) == 2, len(parts) == 3 && parts[2] == "svc", len(parts) == 5 && parts[2] == "svc":
		return parts[0], parts[1], true
	}
	return "", "", false
}

func offerSecrets(ctx context.Context, api *k8sAPI, routes []RouteTarget) ([]SecretOffer, []string) {
	type found struct {
		keys   map[string]bool
		whole  bool
		routes map[string]bool
	}
	secrets := map[[2]string]*found{}
	var notes []string
	add := func(ns, name, key, route string) {
		f := secrets[[2]string{ns, name}]
		if f == nil {
			f = &found{keys: map[string]bool{}, routes: map[string]bool{}}
			secrets[[2]string{ns, name}] = f
		}
		if key == "" {
			f.whole = true
		} else {
			f.keys[key] = true
		}
		f.routes[route] = true
	}
	for _, rt := range routes {
		name, ns, ok := serviceOfHost(rt.Host, api.ns)
		if !ok {
			continue
		}
		var svc struct {
			Spec struct {
				Selector map[string]string `json:"selector"`
			} `json:"spec"`
		}
		if _, err := k8sDo(ctx, api, http.MethodGet, api.in(ns).base+"/services/"+name, nil, &svc); err != nil || len(svc.Spec.Selector) == 0 {
			continue // not a Service, or one with no pods of its own to read
		}
		sel := make([]string, 0, len(svc.Spec.Selector))
		for k, v := range svc.Spec.Selector {
			sel = append(sel, k+"="+v)
		}
		sort.Strings(sel)
		var pods struct {
			Items []podSecrets `json:"items"`
		}
		code, err := k8sDo(ctx, api, http.MethodGet, api.in(ns).base+"/pods?labelSelector="+url.QueryEscape(strings.Join(sel, ",")), nil, &pods)
		if err != nil {
			if code == http.StatusForbidden {
				notes = append(notes, fmt.Sprintf("route %s: the pods of %s cannot be read: add %s to the chart's rbac.watchNamespaces", rt.Route, ns, ns))
			} else {
				notes = append(notes, fmt.Sprintf("route %s: %v", rt.Route, err))
			}
			continue
		}
		for _, p := range pods.Items {
			for _, c := range append(p.Spec.Containers, p.Spec.InitContainers...) {
				for _, e := range c.Env {
					if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
						add(ns, e.ValueFrom.SecretKeyRef.Name, e.ValueFrom.SecretKeyRef.Key, rt.Route)
					}
				}
				for _, e := range c.EnvFrom {
					if e.SecretRef != nil {
						add(ns, e.SecretRef.Name, "", rt.Route)
					}
				}
			}
			for _, v := range p.Spec.Volumes {
				if v.Secret != nil {
					if len(v.Secret.Items) == 0 {
						add(ns, v.Secret.SecretName, "", rt.Route)
					}
					for _, it := range v.Secret.Items {
						add(ns, v.Secret.SecretName, it.Key, rt.Route)
					}
				}
				if v.Projected != nil {
					for _, src := range v.Projected.Sources {
						if src.Secret == nil {
							continue
						}
						if len(src.Secret.Items) == 0 {
							add(ns, src.Secret.Name, "", rt.Route)
						}
						for _, it := range src.Secret.Items {
							add(ns, src.Secret.Name, it.Key, rt.Route)
						}
					}
				}
			}
		}
	}
	out := make([]SecretOffer, 0, len(secrets))
	for id, f := range secrets {
		if f.whole {
			// A whole Secret: its keys, read for their names.
			data, err := secretData(ctx, api, id[0], id[1])
			if err != nil {
				notes = append(notes, err.Error())
			}
			for k := range data {
				f.keys[k] = true
			}
		}
		if len(f.keys) == 0 {
			continue
		}
		o := SecretOffer{Namespace: id[0], Name: id[1], Keys: make([]string, 0, len(f.keys)), Routes: make([]string, 0, len(f.routes))}
		for k := range f.keys {
			o.Keys = append(o.Keys, k)
		}
		for r := range f.routes {
			o.Routes = append(o.Routes, r)
		}
		sort.Strings(o.Keys)
		sort.Strings(o.Routes)
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, notes
}

// ReadSecretKey reads one key of a Secret. The errors name what failed and
// what to change, since they reach the console and the log of a token
// request that was refused for it.
func ReadSecretKey(ctx context.Context, ref string) (string, error) {
	k, err := ParseSecretRef(ref)
	if err != nil {
		return "", err
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return "", errNotKubernetes
	}
	api, err := newK8sAPI()
	if err != nil {
		return "", fmt.Errorf("the Kubernetes API cannot be reached: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	return readSecretKey(ctx, api, k)
}

func readSecretKey(ctx context.Context, api *k8sAPI, k SecretKey) (string, error) {
	data, err := secretData(ctx, api, k.Namespace, k.Name)
	if err != nil {
		return "", err
	}
	v, ok := data[k.Key]
	if !ok {
		keys := make([]string, 0, len(data))
		for key := range data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return "", fmt.Errorf("the Secret %s/%s holds no key %q: it holds %s", k.Namespace, k.Name, k.Key, strings.Join(keys, ", "))
	}
	return v, nil
}

// secretData reads a Secret's data, decoded.
func secretData(ctx context.Context, api *k8sAPI, ns, name string) (map[string]string, error) {
	var s struct {
		Data map[string]string `json:"data"`
	}
	code, err := k8sDo(ctx, api, http.MethodGet, api.in(ns).base+"/secrets/"+name, nil, &s)
	switch {
	case code == http.StatusNotFound:
		return nil, fmt.Errorf("the Secret %s/%s does not exist", ns, name)
	case code == http.StatusForbidden:
		return nil, fmt.Errorf("the Secret %s/%s cannot be read: %s", ns, name, secretsNotGranted)
	case err != nil:
		return nil, fmt.Errorf("reading the Secret %s/%s: %w", ns, name, err)
	}
	out := make(map[string]string, len(s.Data))
	for k, v := range s.Data {
		b, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("the key %q of the Secret %s/%s is not base64: %w", k, ns, name, err)
		}
		out[k] = string(b)
	}
	return out, nil
}
