#!/usr/bin/env bash
# Builds the images from this tree and installs them side by side in one
# namespace of the cluster kubectl points at - the loop for looking at a change
# in the three shapes it ships in:
#
#   ee     the Enterprise image          release meerkat
#   ce     the community image           release meerkat-ce
#   eval   the evaluation image          release meerkat-eval
#
#   deploy/local/deploy.sh             all three
#   deploy/local/deploy.sh ee eval     only those
#
# NAMESPACE says where; it is read from deploy/local/env.local when that file
# exists (untracked: it names your cluster, not the project).
#
# A release that exists keeps its values and takes the new image. A release
# that does not is installed on ports of its own, with an admin password drawn
# here and written to deploy/local/<release>.password.local - never printed.
#
# The images are not pushed anywhere. Where the cluster's nodes are containers
# of this Docker (kind, Docker Desktop), each image is copied into them before
# the release is touched: relying on the node to fetch it is how a gateway
# ended up down, waiting for an image that was sitting next to it.
#
# A rollout that does not finish is rolled back: this replaces a running
# gateway, and a failed look at a change must not leave nothing running.
set -euo pipefail

cd "$(dirname "$0")/../.."
[ -f deploy/local/env.local ] && . deploy/local/env.local
NAMESPACE=${NAMESPACE:?set NAMESPACE, or write NAMESPACE=... in deploy/local/env.local}

editions=("$@")
[ ${#editions[@]} -gt 0 ] || editions=(ee ce eval)

stamp=$(date +%m%d-%H%M%S)
rev=$(git rev-parse --short HEAD)

for edition in "${editions[@]}"; do
  case "$edition" in
    ee)   release=meerkat;      tags="ee";      flavour=ee; ports=(8080 8443 19090 19443); says="Enterprise edition" ;;
    ce)   release=meerkat-ce;   tags="";        flavour=ce; ports=(8081 8444 19091 19444); says="Community edition" ;;
    eval) release=meerkat-eval; tags="ee eval"; flavour=ee; ports=(8083 8446 19093 19446); says="Enterprise edition" ;;
    *) echo "unknown edition '$edition': ee, ce or eval" >&2; exit 2 ;;
  esac
  tag="$edition-$stamp"
  echo "== $edition: building meerkat:$tag"
  docker build -q -f docker/Dockerfile \
    --build-arg GO_TAGS="$tags" --build-arg EDITION="$flavour" \
    --build-arg VERSION="$tag" --build-arg GIT_REV="$rev" \
    --build-arg BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -t "meerkat:$tag" . >/dev/null

  for node in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do
    if docker inspect "$node" >/dev/null 2>&1; then
      docker save "meerkat:$tag" | docker exec -i "$node" ctr -n k8s.io images import - >/dev/null
    fi
  done

  existed=false
  if helm status "$release" -n "$NAMESPACE" >/dev/null 2>&1; then
    existed=true
    # The chart's new defaults, then what this release was given.
    helm upgrade "$release" deploy/helm/meerkat -n "$NAMESPACE" --reset-then-reuse-values \
      --set image.repository=meerkat --set image.tag="$tag" >/dev/null
  else
    secret="deploy/local/$release.password.local"
    [ -f "$secret" ] || (umask 077; echo "Local-$(openssl rand -hex 9)-A1" > "$secret")
    helm install "$release" deploy/helm/meerkat -n "$NAMESPACE" \
      --set image.repository=meerkat --set image.tag="$tag" --set image.pullPolicy=IfNotPresent \
      --set-json 'image.pullSecrets=[]' \
      --set admin.password="$(cat "$secret")" --set vault.key="$(openssl rand -base64 32)" \
      --set persistence.enabled=true --set persistence.size=1Gi --set rbac.read=true \
      --set plug.enabled=$([ "$edition" = ce ] && echo false || echo true) \
      --set service.type=LoadBalancer --set service.adminType=LoadBalancer \
      --set service.appPort="${ports[0]}" --set service.appTlsPort="${ports[1]}" \
      --set service.adminPort="${ports[2]}" --set service.adminTlsPort="${ports[3]}" >/dev/null
    echo "   first install: admin password in $secret"
  fi
  deploy=$(kubectl -n "$NAMESPACE" get deploy -l "app.kubernetes.io/instance=$release" -o name | head -1)
  if ! kubectl -n "$NAMESPACE" rollout status "$deploy" --timeout=240s | tail -1; then
    kubectl -n "$NAMESPACE" get pods -l "app.kubernetes.io/instance=$release" >&2
    if $existed; then
      echo "   $release did not come up: back to what was running" >&2
      helm rollback "$release" -n "$NAMESPACE" >/dev/null
    fi
    exit 1
  fi
  # The image that runs is the one just built, and it is the edition asked for.
  # A few tries: the new pod is Ready before its first lines can be read.
  said=false
  for _ in 1 2 3 4 5 6; do
    if kubectl -n "$NAMESPACE" logs "$deploy" 2>/dev/null | grep -q "$says"; then said=true; break; fi
    sleep 2
  done
  $said || { echo "   $release does not say '$says'" >&2; exit 1; }
  echo "   $release runs meerkat:$tag ($says), app :${ports[0]}/:${ports[1]}, console :${ports[2]}/:${ports[3]} on a first install"
done
