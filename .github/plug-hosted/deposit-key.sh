#!/usr/bin/env bash
# The developer's public key, deposited the way a developer deposits one.
#
# The pair is NOT generated here: the installer that ran a step earlier
# pre-creates a profile and its key, which is the whole point of installing
# from the gateway rather than downloading a binary. What is checked is the
# chain that follows - a key the gateway never chose is accepted because
# somebody holding the developer capability put it there.
set -euo pipefail

admin=http://localhost:19090
data=http://localhost:18080
password=tunnel-Root-Password-1
jar=$(mktemp)

pub=$(cat "$HOME/.plug/profiles/default/id_ed25519.pub")
echo "the profile's key: ${pub:0:40}..."

code=$(curl -s -c "$jar" -o /dev/null -w '%{http_code}' \
  -X POST "$admin/login" -d "username=admin&password=$password")
[ "$code" = "303" ] || { echo "signing in answered $code"; exit 1; }

# The capability first. The endpoint refuses a key from an account that is not
# a developer, and that rule is being relied on rather than worked around: the
# deposit below would be a 403 without this line, which is the point of it.
id=$(curl -s -b "$jar" "$admin/api/me" | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')
curl -sf -b "$jar" -X PUT "$admin/api/users/$id/capabilities" \
  -H 'Content-Type: application/json' -d '{"dev":true}' >/dev/null

# Deposited on the DATA plane, where the profile pages live.
code=$(curl -s -b "$jar" -o /dev/null -w '%{http_code}' \
  -X POST "$data/profile/dev-key" --data-urlencode "key=$pub")
case "$code" in
  200|204|303) echo "key deposited" ;;
  *) echo "depositing the key answered $code"; exit 1 ;;
esac
