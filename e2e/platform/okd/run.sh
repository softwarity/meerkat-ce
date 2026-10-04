#!/usr/bin/env bash
# Meerkat on a real OpenShift, small enough for a CI runner: MicroShift built
# from OKD, one node, in a container. What kind cannot imitate is here for
# real - the restricted-v2 SCC that ADMITS the pod and hands it a uid, and the
# router behind a Route.
#
#   e2e/platform/okd/run.sh <image> <ce|ee>
#
# <image> is a local Docker image (the one under test, built beforehand). It is
# copied into the cluster's own storage: nothing is pulled from a registry, so
# what runs is what was built.
#
# The chart is the published one with the published one-node values, rewritten
# for the image only. One thing is turned off: the volume claim. MicroShift's
# storage driver wants an LVM volume group on the host, which a container on a
# shared runner has no business creating; /data is an emptyDir here, and the
# claim is what the kind targets cover.
#
# KEEP=1 leaves the cluster up to look at.
set -euo pipefail

IMAGE=${1:?usage: run.sh <image> <ce|ee>}
EDITION=${2:?usage: run.sh <image> <ce|ee>}
case "$EDITION" in ce|ee) ;; *) echo "edition must be ce or ee, not '$EDITION'" >&2; exit 2 ;; esac

# Pinned: `latest` there moves with every upstream build.
MICROSHIFT_IMAGE=${MICROSHIFT_IMAGE:-ghcr.io/microshift-io/microshift:4.21.0_g29f429c21_4.21.0_okd_scos.ec.15}
NODE=${NODE:-meerkat-okd}
NS=mk
ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
KC=/var/lib/microshift/resources/kubeadmin/kubeconfig
APPS=apps.127.0.0.1.nip.io
CONSOLE=console.127.0.0.1.nip.io

passed=0
skipped=0
say() { printf '\n== %s\n' "$*"; }
ok() { passed=$((passed + 1)); printf '  ok    %s\n' "$*"; }
skip() { skipped=$((skipped + 1)); printf '  SKIP  %s\n' "$*"; }
die() { printf '  FAIL  %s\n' "$*" >&2; exit 1; }
# In the node, as the cluster's admin.
node() { docker exec -i "$NODE" env KUBECONFIG=$KC "$@"; }
oc() { node oc "$@"; }
# expect <what> <wanted> <got>
expect() { [ "$2" = "$3" ] && ok "$1: $3" || die "$1: wanted $2, got $3"; }
# until_ok <seconds> <what> <command...>: a wait that says what it waited for.
until_ok() {
  local limit=$1 what=$2 t0=$SECONDS
  shift 2
  until "$@" >/dev/null 2>&1; do
    [ $((SECONDS - t0)) -lt "$limit" ] || die "$what: still not true after ${limit}s"
    sleep 5
  done
  ok "$what (${SECONDS}s in)"
}

finish() {
  local code=$?
  if [ $code -ne 0 ]; then
    say "what the cluster looked like"
    oc get pods -A -o wide 2>&1 | head -40 || true
    oc -n $NS describe pods 2>&1 | tail -60 || true
    oc -n $NS logs deploy/meerkat-meerkat --tail=40 2>&1 || true
  fi
  [ "${KEEP:-}" = 1 ] || docker rm -f "$NODE" >/dev/null 2>&1 || true
  exit $code
}
trap finish EXIT

say "MicroShift ($MICROSHIFT_IMAGE)"
docker rm -f "$NODE" >/dev/null 2>&1 || true
docker run --privileged -d --tty --name "$NODE" --hostname 127.0.0.1.nip.io "$MICROSHIFT_IMAGE" >/dev/null
until_ok 300 "the API answers" node test -f $KC
node_ready() { oc get nodes --no-headers | grep -q ' Ready'; }
until_ok 1200 "the node is Ready" node_ready
router_ready() { oc -n openshift-ingress get pods --no-headers | grep -q '1/1'; }
until_ok 600 "the router runs" router_ready

say "the image under test, copied into the cluster"
docker save "$IMAGE" | node sh -c 'cat > /tmp/image.tar && skopeo copy -q docker-archive:/tmp/image.tar containers-storage:localhost/meerkat:under-test && rm /tmp/image.tar'
ok "$IMAGE is localhost/meerkat:under-test in the node"

say "the published chart, values-$EDITION-one-node"
password="Okd-$(openssl rand -hex 8)-A1"
rendered=$(helm template meerkat "$ROOT/deploy/helm/meerkat" -n $NS \
  -f "$ROOT/deploy/helm/values-$EDITION-one-node.yaml" \
  --set image.repository=localhost/meerkat --set image.tag=under-test \
  --set image.pullPolicy=Never --set-json 'image.pullSecrets=[]' \
  --set persistence.enabled=false \
  --set admin.password="$password" --set vault.key="$(openssl rand -base64 32)")
# The rewrite is asserted: a chart that kept its own image would be pulled
# from the registry, and this would be a test of the last release.
echo "$rendered" | grep -q 'image: "localhost/meerkat:under-test"' || die "the rendered chart does not run the image under test"
# What OpenShift refuses, checked on what we publish rather than found there.
if echo "$rendered" | grep -Eq '^\s*(runAsUser|runAsGroup|fsGroup):'; then die "the chart pins a uid or a group: the restricted SCC rejects such a pod"; fi
ok "the chart runs the image under test and pins no uid"
oc create namespace $NS >/dev/null
echo "$rendered" | oc apply -n $NS -f - >/dev/null
oc -n $NS rollout status deploy/meerkat-meerkat --timeout=300s >/dev/null || die "the deployment did not roll out"
ok "the deployment rolled out"

say "admitted by the SCC, as the uid the cluster chose"
pod=$(oc -n $NS get pod -l app.kubernetes.io/name=meerkat -o name | head -1)
[ -n "$pod" ] || pod=$(oc -n $NS get pod -o name | head -1)
expect "SCC" restricted-v2 "$(oc -n $NS get "$pod" -o jsonpath='{.metadata.annotations.openshift\.io/scc}')"
uid=$(oc -n $NS exec "$pod" -- id -u)
gid=$(oc -n $NS exec "$pod" -- id -g)
[ "$uid" -ge 1000000000 ] || die "the pod runs as uid $uid: the SCC did not allocate it"
ok "runs as uid $uid, which is not the image's 65532"
expect "group" 0 "$gid"
oc -n $NS exec "$pod" -- test -s /data/meerkat.db || die "no database in /data: the store could not write"
ok "the store wrote /data/meerkat.db"
want=$([ "$EDITION" = ee ] && echo "Enterprise edition" || echo "Community edition")
oc -n $NS logs "$pod" | grep -q "$want" || die "the log does not say '$want'"
ok "the log says $want"

say "behind Routes"
oc -n $NS apply -f - >/dev/null <<ROUTES
apiVersion: route.openshift.io/v1
kind: Route
metadata: {name: meerkat}
spec:
  host: $APPS
  to: {kind: Service, name: meerkat-meerkat}
  port: {targetPort: app}
  tls: {termination: edge, insecureEdgeTerminationPolicy: Redirect}
---
apiVersion: route.openshift.io/v1
kind: Route
metadata: {name: meerkat-admin}
spec:
  host: $CONSOLE
  to: {kind: Service, name: meerkat-meerkat-admin}
  port: {targetPort: admin}
  tls: {termination: edge}
ROUTES
router=$(node sh -c "hostname -I | cut -d' ' -f1")
status() { node curl -sk -m 10 -o /dev/null -w '%{http_code}' --resolve "$APPS:443:$router" --resolve "$APPS:80:$router" --resolve "$CONSOLE:443:$router" "$@"; }
ready() { [ "$(status "https://$APPS/readyz")" = 200 ]; }
until_ok 120 "the data plane answers through its Route" ready
expect "plain HTTP is sent to HTTPS" 302 "$(status "http://$APPS/readyz")"
# Its own wait: the router takes each Route in when it gets to it, and answers
# 503 for a host it has not loaded yet.
console_up() { [ "$(status "https://$CONSOLE/login")" = 200 ]; }
until_ok 120 "the console's sign-in page answers through its own Route" console_up
expect "the admin API without a session" 401 "$(status "https://$CONSOLE/api/routes")"
expect "the admin API on the data plane's Route" 404 "$(status "https://$APPS/api/routes")"

say "the developer tunnel's mount helper"
if [ "$EDITION" = ce ]; then
  skip "the community image carries no agent: nothing to mount with"
else
  # The pod plug's agent creates (agent/mount.go, k8sMountPod), with what it
  # reads on the workload: its uid and gid. A volume is an emptyDir here.
  oc -n $NS apply -f - >/dev/null <<HELPER
apiVersion: v1
kind: Pod
metadata: {name: plug-mnt-test, labels: {plug.mount.helper: plug-mnt-test}}
spec:
  securityContext: {runAsUser: $uid, runAsNonRoot: true, runAsGroup: $gid, fsGroup: $uid}
  volumes: [{name: vol, emptyDir: {}}]
  containers:
  - name: mount
    image: localhost/meerkat:under-test
    imagePullPolicy: Never
    command: [/usr/local/bin/plug-agent, mount-serve]
    env: [{name: PLUG_SMB_USER, value: dev}, {name: PLUG_SMB_PASS, value: "$password"}]
    ports: [{containerPort: 1445}]
    volumeMounts: [{name: vol, mountPath: /mnt/vol}]
    securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}, seccompProfile: {type: RuntimeDefault}}
---
apiVersion: v1
kind: Service
metadata: {name: plug-mnt-test}
spec:
  selector: {plug.mount.helper: plug-mnt-test}
  ports: [{port: 445, targetPort: 1445}]
HELPER
  oc -n $NS wait --for=condition=Ready pod/plug-mnt-test --timeout=180s >/dev/null || die "the helper pod did not start"
  expect "the helper's SCC" restricted-v2 "$(oc -n $NS get pod plug-mnt-test -o jsonpath='{.metadata.annotations.openshift\.io/scc}')"
  share=$(oc -n $NS get svc plug-mnt-test -o jsonpath='{.spec.clusterIP}')
  # A real SMB client, on 445 as a developer's machine would ask.
  # Bounded and retried: a package mirror that stalls must not hang the job,
  # and must not read as a verdict on the helper.
  node podman run -q --rm --network host docker.io/library/alpine sh -c \
    "for i in 1 2 3 4 5; do timeout 90 apk add -q --no-cache samba-client && break; sleep 3; done >/dev/null 2>&1; echo from-the-developer > /tmp/f && timeout 60 smbclient //$share/vol -p 445 -U 'dev%$password' -c 'put /tmp/f saved.txt'" >/dev/null 2>&1 ||
    die "the SMB client could not write through the helper"
  expect "a file the developer saved belongs to the workload's uid" "$uid" "$(oc -n $NS exec plug-mnt-test -- stat -c %u /mnt/vol/saved.txt)"
fi

say "$passed checks passed, $skipped skipped ($EDITION)"
# A skip is said, never counted as a pass (and seen in the job's annotations).
[ "$skipped" -eq 0 ] || [ -z "${GITHUB_ACTIONS:-}" ] || echo "::warning::OKD ($EDITION): $skipped check(s) skipped"
