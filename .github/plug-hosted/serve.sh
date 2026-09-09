#!/usr/bin/env bash
# The takeover, which is the whole feature: a cluster name stops being answered
# by the cluster and starts being answered from HERE.
#
# Three things are asserted, and the third is the one that has no other way of
# being checked: the deployed service is PARKED (plug scaled it to zero), a
# workload inside the cluster reaching `checkout` lands on this runner, and the
# gateway names the developer who did it - which is what the Enterprise half of
# the tunnel adds and what the labels on a signpost could never say.
set -euo pipefail

admin=http://localhost:19090
password=tunnel-Root-Password-1

before=$(docker service inspect tunnel_checkout --format '{{.Spec.Mode.Replicated.Replicas}}')
echo "the deployed service runs $before replica(s)"

# Something that says who answered. The port is plug's to choose - a pinned one
# on a shared runner is a race with whatever else is bound.
cat > /tmp/mine.py <<'PY'
import http.server, sys
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200); self.end_headers()
        self.wfile.write(b"SERVED-FROM-THE-RUNNER")
    def log_message(self, *a): pass
http.server.HTTPServer(("0.0.0.0", int(sys.argv[1])), H).serve_forever()
PY

# The profile carries the host and the port (deposit-key.sh wrote it), and
# {PORT} is plug's own placeholder: it picks a free one and writes it into the
# command, so a shared runner has nothing to collide on.
"$HOME/.local/bin/plug" -p ci -s checkout:8080:PORT \
  python3 /tmp/mine.py '{PORT}' > plug.log 2>&1 &
plug_pid=$!
trap 'kill $plug_pid 2>/dev/null || true' EXIT

ok=0
for i in $(seq 1 40); do
  # Asked from INSIDE the cluster, by the name, on the network the services
  # share - which is the only place the takeover means anything.
  out=$(docker run --rm --network tunnel-net curlimages/curl:latest \
        -s --max-time 5 http://checkout:8080/ 2>/dev/null || true)
  case "$out" in *SERVED-FROM-THE-RUNNER*) ok=1; break ;; esac
  sleep 3
done
if [ "$ok" != "1" ]; then
  echo "the name never reached this runner"; cat plug.log; exit 1
fi
echo "the cluster reaches this runner by name"

# Parked, not merely shadowed: leaving the real one up would mean two answers
# to one name and a coin toss between them.
during=$(docker service inspect tunnel_checkout --format '{{.Spec.Mode.Replicated.Replicas}}')
[ "$during" = "0" ] || { echo "the deployed service was left running ($during)"; exit 1; }
echo "the deployed service is parked"

# And the gateway says WHO. This is the Enterprise half: plug's own labels name
# the agent that holds a session, never the person.
jar=$(mktemp)
curl -s --max-time 15 -c "$jar" -o /dev/null -X POST "$admin/login" -d "username=admin&password=$password"
served=$(curl -s --max-time 15 -b "$jar" "http://localhost:18080/meerkat/user-button.json" || true)
echo "$served" | grep -q checkout || { echo "the gateway does not say the name is served: $served"; exit 1; }
echo "the gateway names what is served"

# Ending the session puts the cluster back. A takeover that does not give the
# name back is an outage with a nice name.
kill $plug_pid; trap - EXIT
for i in $(seq 1 40); do
  after=$(docker service inspect tunnel_checkout --format '{{.Spec.Mode.Replicated.Replicas}}' 2>/dev/null || echo "?")
  [ "$after" = "$before" ] && { echo "the cluster has its service back"; exit 0; }
  sleep 3
done
echo "the deployed service was never scaled back (still $after, was $before)"; exit 1
