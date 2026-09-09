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

# /api/me answers {"user": {...}, "tenants": [...], ...} in one payload - what
# the console needs to draw itself - and the account under "user" is the whole
# object. There is no GET /api/users/{id} to read it back from, and none is
# needed: this IS the read half of the read-modify-write.
curl -s --max-time 15 -b "$jar" "$admin/api/me" > /tmp/me.json
if ! python3 -c 'import json;me=json.load(open("/tmp/me.json"));u=me["user"];u["dev"]=True;json.dump(u,open("/tmp/user.json","w"));print("dev capability set on",u["username"])'; then
  echo "--- what /api/me answered ---"
  head -c 400 /tmp/me.json
  exit 1
fi
id=$(python3 -c 'import json;print(json.load(open("/tmp/user.json"))["id"])')

# Written back WHOLE: a PUT built from the fields this script happens to know
# would clear every field it does not.
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
