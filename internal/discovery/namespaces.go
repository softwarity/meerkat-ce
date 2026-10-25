package discovery

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"
)

// The other namespaces (SVC-02). A route very often points outside the
// gateway's own namespace - Grafana in monitoring, an application in its own -
// and the chart grants by default the right to list the services of the whole
// cluster (rbac.clusterServices, services only: no pod, no secret). Where the
// deployment narrowed that grant, the namespaces cannot be listed and the
// editor shows the gateway's own, which is the devops' decision and needs no
// sentence on screen.

// validNamespace is a Kubernetes namespace name (an RFC 1123 label).
var validNamespace = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

// ValidNamespace says whether ns could name a namespace, and why not.
func ValidNamespace(ns string) error {
	if !validNamespace.MatchString(ns) {
		return fmt.Errorf("namespace %q is not allowed: lowercase letters, digits and '-', at most 63, starting and ending with a letter or digit", ns)
	}
	return nil
}

// DiscoverIn is Discover for namespace ns ("" or the gateway's own: the
// default list), with the cluster's namespaces when this gateway may list
// them. Outside Kubernetes it is Discover: namespaces mean nothing there.
//
// Its services are a list even when no runtime answered: this is what the
// console and the agents read, and a null where a list is expected is what
// they trip on - an installation without a runtime is the ordinary case.
func DiscoverIn(ctx context.Context, ns string) Result {
	r := discoverIn(ctx, ns)
	if r.Services == nil {
		r.Services = []Service{}
	}
	return r
}

func discoverIn(ctx context.Context, ns string) Result {
	if os.Getenv("KUBERNETES_SERVICE_HOST") == "" {
		return Discover(ctx)
	}
	own := ""
	if b, err := os.ReadFile(saDir + "/namespace"); err == nil {
		own = strings.TrimSpace(string(b))
	}
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	var r Result
	if ns == "" || ns == own {
		r = Discover(ctx)
		r.Namespace = own
	} else {
		var err error
		if r, err = fromKubernetesIn(ctx, ns, false); err != nil {
			r = Result{Namespace: ns, Unavailable: "could not read the services of namespace " + ns + ": " + err.Error() +
				" - the service account needs list on services there (the chart's rbac.clusterServices grants it in every namespace)"}
		}
	}
	if names, err := k8sNamespaces(ctx, own); err == nil && len(names) > 1 {
		r.Namespaces = names
	}
	return r
}

// k8sNamespaces lists the namespaces that hold at least one Service, sorted;
// an error when this gateway may not, which is an answer and not a failure.
//
// From the Services, not from the namespaces: a namespace with no Service
// (kube-node-lease, a CSI driver's, a secrets store's) has nothing a route
// could point at, and offering it is offering an empty list. One request for
// the whole cluster, metadata only - the names, not the specs.
func k8sNamespaces(ctx context.Context, own string) ([]string, error) {
	token, err := os.ReadFile(saDir + "/token")
	if err != nil {
		return nil, err
	}
	client, err := k8sClient()
	if err != nil {
		return nil, err
	}
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if port == "" {
		port = "443"
	}
	endpoint := fmt.Sprintf("https://%s/api/v1/services", net.JoinHostPort(os.Getenv("KUBERNETES_SERVICE_HOST"), port))
	var list k8sMetaList
	if err := getJSON(ctx, client, endpoint, &list,
		"Authorization", "Bearer "+strings.TrimSpace(string(token)),
		"Accept", "application/json;as=PartialObjectMetadataList;g=meta.k8s.io;v=v1, application/json"); err != nil {
		return nil, err
	}
	return servedNamespaces(list, own), nil
}

// k8sMetaList is a list read for its items' namespaces only.
type k8sMetaList struct {
	Items []struct {
		Metadata struct {
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	} `json:"items"`
}

// servedNamespaces is the distinct namespaces of the listed Services, sorted,
// the gateway's own always among them.
func servedNamespaces(list k8sMetaList, own string) []string {
	seen := map[string]bool{}
	if own != "" {
		seen[own] = true
	}
	for _, it := range list.Items {
		if ns := it.Metadata.Namespace; ns != "" {
			seen[ns] = true
		}
	}
	out := make([]string, 0, len(seen))
	for ns := range seen {
		out = append(out, ns)
	}
	sort.Strings(out)
	return out
}
