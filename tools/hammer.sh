#!/usr/bin/env bash
#
# Hammer the data plane, to see what the metrics screen does under load.
#
# Two identities, because the interesting part of this fixture is that
# httpbin-acme and httpbin-globex BOTH match /**: which one takes a request is
# decided by the access rule, so it is the token that chooses the route. A
# request with no token, or with one whose user does not belong to acme, falls
# through to globex.
#
# Mint the tokens in the console, under Infra, Access tokens - PLANE: data.
# One for a user acme admits (a member of its organisation, or one named in
# the route's users list), one for anybody else.
#
#   ACME_TOKEN=mk_... GLOBEX_TOKEN=mk_... tools/hammer.sh
#
# Pasting them below works too. If you do, do not commit the file.
#
# What it sends is deliberately mixed: fast paths, failures, the occasional
# slow one, and paths carrying identifiers so the endpoint templates get
# exercised. CARDINALITY=1 turns a share of the traffic into paths that do NOT
# fold - the case the per-route budget exists for.

set -uo pipefail

# sort and awk read these files byte by byte, and a UTF-8 locale makes macOS
# sort refuse a line it cannot decode - which it will, since what comes back
# from a hammered service is not always clean. C is the right collation here
# anyway: these are numbers and status codes.
export LC_ALL=C

# ── the two tokens ────────────────────────────────────────────────────────
#
# Kept in tools/hammer.env when there is one, which .gitignore covers: a token
# pasted into a tracked file is a token in the history the day somebody runs
# git add. Anything on the command line still wins over what that file holds.
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=/dev/null
[ -f "$here/hammer.env" ] && . "$here/hammer.env"

ACME_TOKEN="${ACME_TOKEN:-}"
GLOBEX_TOKEN="${GLOBEX_TOKEN:-}"

# ── what to hit, and how hard ─────────────────────────────────────────────
BASE="${BASE:-http://localhost:8082}"
WORKERS="${WORKERS:-24}"      # parallel curl processes
DURATION="${DURATION:-30}"    # seconds
BATCH="${BATCH:-40}"          # urls per curl invocation, so connections are reused
CARDINALITY="${CARDINALITY:-0}" # 1 to send unfoldable paths and press the budget
CURL_OPTS="${CURL_OPTS:-}"

out=$(mktemp -d "${TMPDIR:-/tmp}/hammer.XXXXXX")
trap 'rm -rf "$out"' EXIT

say() { printf '%s\n' "$*"; }

# ── who the gateway thinks you are ────────────────────────────────────────
#
# The fixture serves /user through a respond filter that echoes the session's
# username and roles, which is the cheapest proof a token resolves to the user
# you meant. A blank answer means the token is not being read.
identity() {
  local label=$1 token=$2
  if [ -z "$token" ]; then
    say "  $label: no token, so its traffic lands on whichever route admits anonymous callers"
    return
  fi
  local body
  body=$(curl -s --max-time 5 -H "Authorization: Bearer $token" $CURL_OPTS "$BASE/user" \
    | tr -d '\n' | tr -s ' ')
  # An empty name is the shape a REFUSED token takes here, and it looks like a
  # success unless somebody says so: the route answers for whoever is there,
  # and with a token the gateway did not read, nobody is.
  case "$body" in
    ''|*'"name": ""'*|*'"name":""'*)
      say "  $label: nobody. This token is not being read - wrong plane (it must be data,"
      say "          not admin), revoked, expired, or a typo. Its traffic will land on the"
      say "          route that admits anonymous callers." ;;
    *) say "  $label: $body" ;;
  esac
}

# ── the path mix ──────────────────────────────────────────────────────────
#
# Weighted by repetition rather than by arithmetic: the common paths appear
# many times, /delay once, so a slow endpoint shows up in the ranking without
# holding every worker.
mix_path() {
  local n=$(( RANDOM % 100 ))
  case $n in
    0|1|2)      printf '/delay/1' ;;                       # slow, 3%
    3|4|5|6)    printf '/status/500' ;;
    7|8|9)      printf '/status/503' ;;
    10|11|12|13) printf '/status/404' ;;
    14|15|16|17|18) printf '/bytes/4096' ;;
    19|20|21|22) printf '/gzip' ;;
    23|24|25|26) printf '/headers' ;;
    27|28|29)   printf '/ip' ;;
    30|31|32)   printf '/uuid' ;;
    # Identifiers: these fold to one template each, which is the whole point.
    33|34|35|36|37|38|39|40|41|42)
      printf '/anything/%s' "$(( RANDOM * 32768 + RANDOM ))" ;;
    43|44|45|46|47)
      # 8-4-4-4-12. RANDOM tops out at 32767, so each group is built from
      # %04x pieces: an %08x of one draw pads to eight and made this
      # forty characters long, which is not the shape it claims to be.
      printf '/anything/%04x%04x-%04x-%04x-%04x-%04x%04x%04x' \
        $RANDOM $RANDOM $RANDOM $RANDOM $RANDOM $RANDOM $RANDOM $RANDOM ;;
    48|49|50|51|52)
      printf '/anything/orders/%s/items' "$(( RANDOM % 100000 ))" ;;
    *)          printf '/get' ;;
  esac
}

# Paths that do NOT fold: a fresh word every time, so each one is a template of
# its own. This is what the per-route budget is there to survive - past it
# everything shares one bucket, and the gateway stops growing.
wild_path() {
  printf '/anything/w%s%s' "$RANDOM" "$RANDOM"
}

worker() {
  local id=$1 deadline=$2
  local file="$out/w$id"
  : > "$file"
  while [ "$(date +%s)" -lt "$deadline" ]; do
    # One token per batch, so a curl invocation reuses its connection.
    local token label
    if [ $(( RANDOM % 2 )) -eq 0 ] && [ -n "$ACME_TOKEN" ]; then
      token=$ACME_TOKEN label=acme
    else
      token=$GLOBEX_TOKEN label=globex
    fi
    # One -o per url, and that is not decoration: with several urls curl
    # applies a single -o to the FIRST one and writes the rest to stdout, so
    # the bodies came back mixed into the measurements - a /bytes/4096 answer
    # turned the file binary and the count into nonsense.
    local args=()
    local i=0
    while [ $i -lt "$BATCH" ]; do
      local p
      if [ "$CARDINALITY" = "1" ] && [ $(( RANDOM % 4 )) -eq 0 ]; then
        p=$(wild_path)
      else
        p=$(mix_path)
      fi
      args+=(-o /dev/null "$BASE$p")
      i=$(( i + 1 ))
    done
    local auth=()
    [ -n "$token" ] && auth=(-H "Authorization: Bearer $token")
    curl -s --max-time 20 \
      "${auth[@]}" $CURL_OPTS \
      -w "$label %{http_code} %{time_total}\n" \
      "${args[@]}" >> "$file" 2>/dev/null
  done
}

# ── go ────────────────────────────────────────────────────────────────────
say "Hammering $BASE"
say "  $WORKERS workers, $BATCH urls a batch, for ${DURATION}s$([ "$CARDINALITY" = "1" ] && printf ', unfoldable paths ON')"
say ""
say "Identities, as the gateway resolves them:"
identity "acme  " "$ACME_TOKEN"
identity "globex" "$GLOBEX_TOKEN"
[ -z "$ACME_TOKEN" ] && say "" && say "  No ACME_TOKEN: every request will fall through to the route that admits anonymous callers."
say ""

started=$(date +%s)
deadline=$(( started + DURATION ))
w=0
while [ $w -lt "$WORKERS" ]; do
  worker "$w" "$deadline" &
  w=$(( w + 1 ))
done
wait
elapsed=$(( $(date +%s) - started ))
[ "$elapsed" -lt 1 ] && elapsed=1

# Only well formed lines: a curl killed mid-write, or a service answering
# something curl could not measure, must not take the summary down with it.
cat "$out"/w* 2>/dev/null | grep -E '^[a-z]+ [0-9]+ [0-9.]+$' > "$out/all"
total=$(wc -l < "$out/all" | tr -d ' ')
if [ "$total" = "0" ]; then
  say "Nothing got through. Is the gateway up on $BASE?"
  exit 1
fi

say "Sent   $total requests in ${elapsed}s   $(( total / elapsed )) req/s"
say ""
say "By route (as the token asked for it, not as the gateway decided):"
awk '{n[$1]++} END {for (r in n) printf "  %-8s %d\n", r, n[r]}' "$out/all" | sort
say ""
say "By status:"
awk '{n[$2]++} END {for (s in n) printf "  %-6s %d\n", s, n[s]}' "$out/all" | sort
say ""
# Percentiles from the sorted times: a mean alone hides the tail, and the tail
# is the whole reason anybody looks at a latency figure.
say "Latency:"
awk '{print $3}' "$out/all" | sort -n > "$out/times"
awk '
  { t[NR] = $1; sum += $1 }
  END {
    p = NR
    printf "  mean %7.1f ms\n", sum / p * 1000
    printf "  p50  %7.1f ms\n", t[int(p * 0.50) + (p > 1)] * 1000
    printf "  p95  %7.1f ms\n", t[int(p * 0.95)] * 1000
    printf "  p99  %7.1f ms\n", t[int(p * 0.99)] * 1000
    printf "  max  %7.1f ms\n", t[p] * 1000
  }' "$out/times"
say ""
say "Now open the console, Metrics: the routes should add up to the numbers above,"
say "and opening one should name the endpoints that took the time."
