package discovery

import "testing"

// A Service picks this pod when every label of its selector is on the pod.
func TestASelectorPicksThePodItMatches(t *testing.T) {
	pod := map[string]string{"app.kubernetes.io/name": "meerkat", "app.kubernetes.io/instance": "ce", "pod-template-hash": "x"}
	if !selects(map[string]string{"app.kubernetes.io/name": "meerkat", "app.kubernetes.io/instance": "ce"}, pod) {
		t.Error("a matching selector did not pick the pod")
	}
	if selects(map[string]string{"app.kubernetes.io/name": "meerkat", "app.kubernetes.io/instance": "ee"}, pod) {
		t.Error("the other release's Service picked this pod")
	}
}
