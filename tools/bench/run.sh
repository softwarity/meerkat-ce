#!/usr/bin/env bash
#
# The benchmark: Meerkat and three open-source gateways, in front of the same
# upstream, under the same limits, driven by the same load - one after another,
# so none of them shares the machine with another while it is measured.
#
#   tools/bench/run.sh                         everything, into tools/bench/out/
#   ONLY="meerkat kong" FIXED=5s MAX=5s tools/bench/run.sh
#
# What makes the figures comparable, and therefore what must not drift:
#
#   - Placement. The gateway is pinned to ONE CPU, the upstream to another, the
#     load generator gets the rest. Every gateway is told it has one core (one
#     nginx worker, GOMAXPROCS=1): the defaults would size themselves on the
#     HOST and fight over the single core they are given.
#   - The upstream is the fastest thing that can answer (upstream/main.go), and
#     a direct call to it in the same run is the floor. An overhead is a gateway
#     figure minus that floor, which is what survives a slower machine.
#   - Access logs are off everywhere: Meerkat writes none by default, and a line
#     per request is a cost the others would carry alone.
#   - Each scenario is checked before it is measured: the route answers 200 with
#     the credential, and the protected one REFUSES without it. A plugin that
#     silently did not load would otherwise be measured as a fast gateway.
#
# Two latency figures, because they answer two questions. At a FIXED rate
# (--latency-correction, so a stall is not hidden by requests that were never
# sent) the percentiles say what a user waits. At MAX they say how much one core
# carries.
#
# One more entrant, and it is not a product: gostd (goproxy/main.go) is the Go
# standard library's reverse proxy with nothing around it, on /proxy only. It is
# the ceiling of the language Meerkat is written in, measured on the same core
# in the same run - which is what says how much of a gap to nginx-based
# gateways is Meerkat's own code.
#
# Scenarios. /proxy and /limit are the same on all four products. /auth checks a
# credential in the gateway and forwards the caller as headers: Meerkat resolves
# its token, Kong and APISIX check a key. Traefik's free edition has no token
# check, so it DELEGATES to an external service on every request - the hop an
# assembled stack pays, measured against the fastest service there is. Meerkat
# alone also runs /auth-jwt, the same call forwarded as a signed JWT with roles,
# which none of the others does here: shown apart, never in their comparison.

set -euo pipefail
export LC_ALL=C

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../.." && pwd)
out="${OUT:-$here/out}"

ONLY="${ONLY:-meerkat gostd kong apisix traefik}"
RATE="${RATE:-1000}"          # requests per second, fixed-rate runs
FIXED="${FIXED:-10s}"
MAX="${MAX:-10s}"
WARM="${WARM:-3s}"
CONN_FIXED="${CONN_FIXED:-32}"
CONN_MAX="${CONN_MAX:-64}"
MEMORY="${MEMORY:-1g}"
GOBENCH="${GOBENCH:-1}"       # 0 to skip the Go micro-benchmarks

OHA_IMAGE=ghcr.io/hatoo/oha:1.16.0
BASE_IMAGE=alpine:3.24
KONG_IMAGE=kong:3.9.3
APISIX_IMAGE=apache/apisix:3.18.0-debian
TRAEFIK_IMAGE=traefik:v3.7.13

KEY=bench-key-0123456789abcdef
ADMIN_PASSWORD=bench-Admin-Password-1
net=meerkat-bench

say() { printf '%s\n' "$*"; }
fail() { printf 'bench: %s\n' "$*" >&2; exit 1; }

now_ms() {
  if [ -n "${EPOCHREALTIME:-}" ]; then
    local t=${EPOCHREALTIME/./}
    echo $(( 10#$t / 1000 ))
  else
    python3 -c 'import time; print(int(time.time() * 1000))'
  fi
}

cleanup() {
  for c in $(docker ps -aq --filter label=meerkat-bench); do
    docker rm -f "$c" >/dev/null 2>&1 || true
  done
  docker network rm "$net" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# ── placement ─────────────────────────────────────────────────────────────
cpus=$(docker info --format '{{.NCPU}}')
[ "$cpus" -ge 4 ] || fail "needs at least 4 CPUs visible to Docker (gateway, upstream, load), found $cpus"
GW_CPU=0
UP_CPU=1
LOAD_CPUS="2-$(( cpus - 1 ))"
arch=$(docker version --format '{{.Server.Arch}}')

mkdir -p "$out/bin" "$out/raw"
rm -f "$out"/raw/* "$out/results.json" "$out/summary.md"

# ── what is being measured, and on what ───────────────────────────────────
{
  echo "date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "commit=$(git -C "$root" rev-parse HEAD 2>/dev/null || echo unknown)"
  echo "subject=$(git -C "$root" log -1 --format=%s 2>/dev/null | tr -d '\n' || true)"
  echo "runner=${RUNNER_LABEL:-local}"
  echo "host_os=$(uname -s)"
  echo "host_arch=$(uname -m)"
  if [ "$(uname -s)" = Linux ]; then
    echo "cpu_model=$(lscpu | sed -n 's/^Model name:[[:space:]]*//p' | head -1)"
    echo "threads_per_core=$(lscpu | sed -n 's/^Thread(s) per core:[[:space:]]*//p' | head -1)"
    echo "host_cpus=$(nproc)"
    echo "host_memory_kb=$(awk '/^MemTotal:/ {print $2}' /proc/meminfo)"
  else
    echo "cpu_model=$(sysctl -n machdep.cpu.brand_string 2>/dev/null || echo unknown)"
    echo "threads_per_core=1"
    echo "host_cpus=$(sysctl -n hw.ncpu)"
    echo "host_memory_kb=$(( $(sysctl -n hw.memsize) / 1024 ))"
  fi
  echo "docker_cpus=$cpus"
  echo "docker_memory=$(docker info --format '{{.MemTotal}}')"
  echo "docker_version=$(docker version --format '{{.Server.Version}}')"
  echo "docker_arch=$arch"
  echo "rate=$RATE"
  echo "fixed=$FIXED"
  echo "max=$MAX"
  echo "conn_fixed=$CONN_FIXED"
  echo "conn_max=$CONN_MAX"
  echo "memory=$MEMORY"
  echo "load_cpus=$LOAD_CPUS"
  echo "go_version=$(go env GOVERSION)"
  echo "image_oha=$OHA_IMAGE"
  echo "image_kong=$KONG_IMAGE"
  echo "image_apisix=$APISIX_IMAGE"
  echo "image_traefik=$TRAEFIK_IMAGE"
} > "$out/raw/meta.env"

say "Benchmark on $(sed -n 's/^cpu_model=//p' "$out/raw/meta.env"), $cpus CPUs for Docker ($arch)"
say "  gateway on CPU $GW_CPU, upstream on CPU $UP_CPU, load on CPUs $LOAD_CPUS"
say "  fixed: $RATE req/s for $FIXED over $CONN_FIXED connections; max: $MAX over $CONN_MAX"
say ""

# ── the Go micro-benchmarks, first, alone on the machine ──────────────────
if [ "$GOBENCH" = 1 ]; then
  say "Go micro-benchmarks (internal/gateway)..."
  ( cd "$root" && go test ./internal/gateway/ -run '^$' -bench . -benchmem -count 3 ) > "$out/raw/gobench.txt"
fi

# ── binaries: Meerkat from this tree, and the upstream ────────────────────
say "Building Meerkat, the upstream and the Go reference for linux/$arch..."
( cd "$root" \
  && CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$out/bin/meerkat" ./cmd/meerkat \
  && CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$out/bin/upstream" ./tools/bench/upstream \
  && CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$out/bin/goproxy" ./tools/bench/goproxy )

for image in "$OHA_IMAGE" "$BASE_IMAGE"; do
  docker image inspect "$image" >/dev/null 2>&1 || docker pull -q "$image" >/dev/null
done

cleanup
docker network create "$net" >/dev/null

docker run -d --name bench-upstream --label meerkat-bench --network "$net" --network-alias upstream \
  --cpuset-cpus "$UP_CPU" -e BENCH_TOKEN="$KEY" \
  -v "$out/bin:/b:ro" "$BASE_IMAGE" /b/upstream >/dev/null

# ── helpers ───────────────────────────────────────────────────────────────

# load <mode> <url> <output> [header]
load() {
  local mode=$1 url=$2 file=$3 header=${4:-}
  local args
  case $mode in
    warm)  args=(-z "$WARM" -c "$CONN_MAX") ;;
    fixed) args=(-z "$FIXED" -c "$CONN_FIXED" -q "$RATE" --latency-correction) ;;
    max)   args=(-z "$MAX" -c "$CONN_MAX") ;;
  esac
  if [ -n "$header" ]; then
    args+=(-H "$header")
  fi
  docker run --rm --label meerkat-bench --network "$net" --cpuset-cpus "$LOAD_CPUS" \
    "$OHA_IMAGE" "${args[@]}" --no-tui --output-format json "$url" > "$file"
}

# hostport <container> <port> - where the host reaches a published port
hostport() {
  local mapped
  mapped=$(docker port "$1" "$2/tcp" | head -1)
  echo "127.0.0.1:${mapped##*:}"
}

# status <url> [header]
status() {
  if [ -n "${2:-}" ]; then
    curl -s -o /dev/null -w '%{http_code}' --max-time 2 -H "$2" "$1"
  else
    curl -s -o /dev/null -w '%{http_code}' --max-time 2 "$1"
  fi
}

# ready <container> <port> <started_ms> - milliseconds until /proxy answers 200
ready() {
  local name=$1 port=$2 started=$3 hp deadline
  hp=$(hostport "$name" "$port")
  deadline=$(( $(now_ms) + 180000 ))
  while [ "$(now_ms)" -lt "$deadline" ]; do
    if [ "$(status "http://$hp/proxy/ready")" = 200 ]; then
      echo $(( $(now_ms) - started ))
      return 0
    fi
    sleep 0.05
  done
  docker logs "$name" 2>&1 | tail -30 >&2
  fail "$name never answered 200 on /proxy"
}

mem() {
  docker stats --no-stream --format '{{.MemUsage}}' "$1" | awk '{print $1}'
}

# measure <gateway> <container> <port> <scenario> <path> <header>
measure() {
  local gw=$1 name=$2 port=$3 scenario=$4 path=$5 header=$6
  local hp url
  hp=$(hostport "$name" "$port")
  url="http://$name:$port$path"

  local code
  code=$(status "http://$hp$path" "$header")
  [ "$code" = 200 ] || fail "$gw $scenario: $path answered $code with the credential, 200 expected"
  if [ "$scenario" != proxy ] && [ "$scenario" != limit ]; then
    code=$(status "http://$hp$path")
    case $code in
      401|403) ;;
      *) fail "$gw $scenario: $path answered $code WITHOUT a credential - the check is not active" ;;
    esac
  fi

  say "  $scenario"
  load warm "$url" /dev/null "$header"
  load fixed "$url" "$out/raw/$gw-$scenario-fixed.json" "$header"

  local stop="$out/raw/.$gw-$scenario.stop" peak="$out/raw/$gw-$scenario-mem.txt"
  rm -f "$stop"
  : > "$peak"
  ( while [ ! -f "$stop" ]; do mem "$name" >> "$peak" 2>/dev/null || true; sleep 1; done ) &
  local sampler=$!
  load max "$url" "$out/raw/$gw-$scenario-max.json" "$header"
  touch "$stop"
  wait "$sampler" 2>/dev/null || true
  rm -f "$stop"
}

# ── the floor: straight to the upstream ───────────────────────────────────
say "Direct to the upstream (the floor)"
sleep 1
load warm http://upstream:9000/proxy/bench /dev/null
load fixed http://upstream:9000/proxy/bench "$out/raw/direct-proxy-fixed.json"
load max http://upstream:9000/proxy/bench "$out/raw/direct-proxy-max.json"

# ── the gateways ──────────────────────────────────────────────────────────
common=(--label meerkat-bench --network "$net" --cpuset-cpus "$GW_CPU" --memory "$MEMORY")

start_meerkat() {
  docker run -d --name bench-meerkat "${common[@]}" -e GOMAXPROCS=1 \
    -e MEERKAT_ADMIN_PASSWORD="$ADMIN_PASSWORD" -p 127.0.0.1::8080 \
    -v "$out/bin:/b:ro" -v "$here/gateways:/bench:ro" \
    "$BASE_IMAGE" /b/meerkat -data /data -addr :8080 -admin-addr :9090 -config /bench/meerkat.yaml >/dev/null
}

start_gostd() {
  docker run -d --name bench-gostd "${common[@]}" -e GOMAXPROCS=1 -p 127.0.0.1::8000 \
    -v "$out/bin:/b:ro" "$BASE_IMAGE" /b/goproxy >/dev/null
}

start_kong() {
  docker run -d --name bench-kong "${common[@]}" \
    -e KONG_DATABASE=off -e KONG_DECLARATIVE_CONFIG=/bench/kong.yaml \
    -e KONG_PROXY_LISTEN=0.0.0.0:8000 -e KONG_ADMIN_LISTEN=off -e KONG_STATUS_LISTEN=off \
    -e KONG_NGINX_WORKER_PROCESSES=1 -e KONG_PROXY_ACCESS_LOG=off -e KONG_ANONYMOUS_REPORTS=off \
    -p 127.0.0.1::8000 -v "$here/gateways:/bench:ro" "$KONG_IMAGE" >/dev/null
}

start_apisix() {
  docker run -d --name bench-apisix "${common[@]}" \
    -v "$here/gateways/apisix-config.yaml:/usr/local/apisix/conf/config.yaml:ro" \
    -v "$here/gateways/apisix-routes.yaml:/usr/local/apisix/conf/apisix.yaml:ro" \
    -p 127.0.0.1::9080 "$APISIX_IMAGE" >/dev/null
}

start_traefik() {
  docker run -d --name bench-traefik "${common[@]}" -e GOMAXPROCS=1 \
    -p 127.0.0.1::8000 -v "$here/gateways:/bench:ro" \
    "$TRAEFIK_IMAGE" --configFile=/bench/traefik.yaml >/dev/null
}

for gw in $ONLY; do
  case $gw in
    meerkat) port=8080 ;;
    gostd) port=8000 ;;
    kong) port=8000; image=$KONG_IMAGE ;;
    apisix) port=9080; image=$APISIX_IMAGE ;;
    traefik) port=8000; image=$TRAEFIK_IMAGE ;;
    *) fail "unknown gateway '$gw': meerkat, gostd, kong, apisix or traefik" ;;
  esac
  name=bench-$gw
  if [ "$gw" != meerkat ] && [ "$gw" != gostd ]; then
    docker image inspect "$image" >/dev/null 2>&1 || { say "Pulling $image..."; docker pull -q "$image" >/dev/null; }
  fi

  say "$gw"
  started=$(now_ms)
  "start_$gw"
  startup=$(ready "$name" "$port" "$started")
  echo "$startup" > "$out/raw/$gw-startup.txt"
  say "  ready in ${startup} ms"

  case $gw in
    meerkat)
      jar=$(mktemp)
      hp=$(hostport "$name" "$port")
      curl -s -c "$jar" -o /dev/null -X POST "http://$hp/login" \
        --data-urlencode username=admin --data-urlencode "password=$ADMIN_PASSWORD"
      token=$(curl -s -b "$jar" -X POST "http://$hp/profile/tokens" -d action=create -d name=bench \
        | grep -o 'mk_[A-Za-z0-9_-]\{20,\}' | head -1 || true)
      rm -f "$jar"
      [ -n "$token" ] || fail "meerkat: could not mint a data-plane token"
      auth_scenario=auth
      auth_header="Authorization: Bearer $token"
      ;;
    gostd)
      auth_scenario=""
      ;;
    kong|apisix)
      auth_scenario=auth
      auth_header="apikey: $KEY"
      ;;
    traefik)
      auth_scenario=auth-delegated
      auth_header="Authorization: Bearer $KEY"
      ;;
  esac

  sleep 2
  mem "$name" > "$out/raw/$gw-mem-idle.txt"

  measure "$gw" "$name" "$port" proxy /proxy/bench ""
  # The reference proxies and nothing else: it has no credential to check and
  # no limit to count, which is the point of it.
  if [ -n "$auth_scenario" ]; then
    measure "$gw" "$name" "$port" "$auth_scenario" /auth/bench "$auth_header"
    if [ "$gw" = meerkat ]; then
      measure "$gw" "$name" "$port" auth-jwt /auth-jwt/bench "$auth_header"
    fi
    measure "$gw" "$name" "$port" limit /limit/bench ""
  fi

  docker rm -f "$name" >/dev/null
done

say ""
python3 "$here/report.py" "$out"
