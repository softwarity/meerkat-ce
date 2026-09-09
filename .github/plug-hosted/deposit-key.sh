#!/usr/bin/env bash
# A developer's key, made and deposited the way a developer does it.
#
# The pair is plug's own: `plug keygen` writes it under ~/.plug/keys, and
# `plug pubkey` prints the public half "ready to paste where the operator
# enrols it". Those two verbs only exist in the hosted flavour - the one this
# image hands out - so running them here also proves which client was served.
#
# The profile is written rather than created with `plug init`: init is a wizard
# and wants a terminal. The file is three keys, and writing it is the honest way
# to be non-interactive instead of feeding a prompt.
set -euo pipefail

admin=http://localhost:19090
data=http://localhost:18080
password=tunnel-Root-Password-1
profile=ci
plug="$HOME/.local/bin/plug"

mkdir -p "$HOME/.plug"
printf 'host = localhost\nport = 2222\nupdate = none\n' > "$HOME/.plug/$profile.conf"
"$plug" ls

"$plug" keygen -p "$profile"
pub=$("$plug" pubkey -p "$profile")
test -n "$pub" || { echo "the client printed no public key"; exit 1; }
echo "the profile's key: ${pub:0:44}..."

jar=$(mktemp)
code=$(curl -s --max-time 15 -c "$jar" -o /dev/null -w '%{http_code}' \
  -X POST "$admin/login" -d "username=admin&password=$password")
[ "$code" = "303" ] || { echo "signing in answered $code"; exit 1; }

# The developer capability, read-modify-write: the account object is PUT whole,
# so building one from the fields this script happens to know would quietly
# clear every field it does not.
id=$(curl -s --max-time 15 -b "$jar" "$admin/api/me" \
     | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')
curl -s --max-time 15 -b "$jar" "$admin/api/users/$id" > /tmp/user.json
python3 -c "
import json
u = json.load(open('/tmp/user.json'))
u['dev'] = True
json.dump(u, open('/tmp/user.json','w'))
print('dev capability set on', u['username'])
"
code=$(curl -s --max-time 15 -b "$jar" -o /dev/null -w '%{http_code}' \
  -X PUT "$admin/api/users/$id" -H 'Content-Type: application/json' \
  --data-binary @/tmp/user.json)
[ "$code" = "200" ] || { echo "granting the capability answered $code"; exit 1; }

# Deposited on the DATA plane, where the profile pages live. Without the
# capability above this is a 403 - which is the rule being relied on rather
# than worked around.
code=$(curl -s --max-time 15 -b "$jar" -o /dev/null -w '%{http_code}' \
  -X POST "$data/profile/dev-key" --data-urlencode "key=$pub")
case "$code" in
  200|204|303) echo "key deposited" ;;
  *) echo "depositing the key answered $code"; exit 1 ;;
esac
