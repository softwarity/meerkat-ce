package discovery

import (
	"reflect"
	"testing"
)

// The chart puts only its selector labels on the pod: the name and the
// release are enough to know Helm runs it, and under which release.
func TestAPodOfTheChartIsAHelmRelease(t *testing.T) {
	p := k8sPlatform(map[string]string{
		"app.kubernetes.io/name": "meerkat", "app.kubernetes.io/instance": "edge", "pod-template-hash": "7d9f8c6b5",
	}, "edge-meerkat-7d9f8c6b5-x2x9z", "shop", false)
	want := Platform{Name: "kubernetes", Helm: true, Release: "edge", Namespace: "shop", Workload: "edge-meerkat"}
	if !reflect.DeepEqual(p, want) {
		t.Fatalf("read as %+v, want %+v", p, want)
	}
	// Helm's own label says so too, on a chart that puts it on the pod.
	if p := k8sPlatform(map[string]string{"app.kubernetes.io/managed-by": "Helm", "app.kubernetes.io/instance": "gw"}, "gw-0", "x", true); !p.Helm || p.Release != "gw" || p.Name != "openshift" {
		t.Fatalf("managed-by Helm read as %+v", p)
	}
}

// A pod with none of the chart's labels is a manifest applied by hand; a pod
// that could not be read is assumed to be the chart's, and says it assumes.
func TestHelmIsSeenOrAssumedNeverGuessedSilently(t *testing.T) {
	if p := k8sPlatform(map[string]string{"app": "meerkat", "pod-template-hash": "abc"}, "meerkat-abc-12345", "x", false); p.Helm || p.Workload != "meerkat" {
		t.Fatalf("a hand-written Deployment read as %+v", p)
	}
	if p := k8sPlatform(nil, "", "", false); !p.Helm || !p.HelmAssumed || p.Release != "" {
		t.Fatalf("an unread pod read as %+v", p)
	}
	if w := workloadOf("db-2", map[string]string{"statefulset.kubernetes.io/pod-name": "db-2"}); w != "db" {
		t.Fatalf("a StatefulSet's pod gave %q", w)
	}
}

func TestAContainerSaysItsSwarmServiceOrItsComposeProject(t *testing.T) {
	if p := dockerPlatform(map[string]string{"com.docker.swarm.service.name": "edge_meerkat", "com.docker.stack.namespace": "edge"}, "/edge_meerkat.1.xyz"); p.Name != "swarm" || p.Workload != "edge_meerkat" || p.Stack != "edge" {
		t.Fatalf("a Swarm task read as %+v", p)
	}
	p := dockerPlatform(map[string]string{
		"com.docker.compose.service": "meerkat", "com.docker.compose.project": "gw",
		"com.docker.compose.project.working_dir":  "/srv/gw",
		"com.docker.compose.project.config_files": "/srv/gw/docker-compose.yml,/srv/gw/override.yml",
	}, "/gw-meerkat-1")
	if p.Name != "compose" || p.Workload != "meerkat" || p.Dir != "/srv/gw" || len(p.Files) != 2 || p.Files[1] != "/srv/gw/override.yml" || p.Container != "gw-meerkat-1" {
		t.Fatalf("a compose container read as %+v", p)
	}
	if p := dockerPlatform(nil, "/meerkat"); p.Name != "docker" || p.Workload != "meerkat" {
		t.Fatalf("a plain container read as %+v", p)
	}
}
