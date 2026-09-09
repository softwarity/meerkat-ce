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
#
# TWO SESSIONS, and that is not an oversight. The capability is an
# administration act, on the control plane; depositing the key is something a
# person does to their OWN profile, on the data plane. They are separate
# origins with separate cookies, and using one jar for both quietly signed the
# deposit out - which answered 303 to /login and looked like success.
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
printf '%s\n' "$pub" > /tmp/plug.pub
fingerprint=$(ssh-keygen -lf /tmp/plug.pub | awk '{print $2}')
echo "the profile's key: $fingerprint"

# --- the control plane: grant the capability -------------------------------
ajar=$(mktemp)
code=$(curl -s --max-time 15 -c "$ajar" -o /dev/null -w '%{http_code}' \
  -X POST "$admin/login" -d "username=admin&password=$password")
[ "$code" = "303" ] || { echo "signing in on the control plane answered $code"; exit 1; }

# /api/me answers {"user": {...}, "tenants": [...], ...} in one payload, and
# the account under "user" is the whole object - there is no GET
# /api/users/{id} to read it back from, and none is needed: this IS the read
# half of the read-modify-write.
curl -s --max-time 15 -b "$ajar" "$admin/api/me" > /tmp/me.json
if ! python3 -c 'import json;me=json.load(open("/tmp/me.json"));u=me["user"];u["dev"]=True;json.dump(u,open("/tmp/user.json","w"));print("dev capability set on",u["username"])'; then
  echo "--- what /api/me answered ---"; head -c 400 /tmp/me.json; exit 1
fi
id=$(python3 -c 'import json;print(json.load(open("/tmp/user.json"))["id"])')

# Written back WHOLE: a PUT built from the fields this script happens to know
# would clear every field it does not.
code=$(curl -s --max-time 15 -b "$ajar" -o /dev/null -w '%{http_code}' \
  -X PUT "$admin/api/users/$id" -H 'Content-Type: application/json' \
  --data-binary @/tmp/user.json)
[ "$code" = "200" ] || { echo "granting the capability answered $code"; exit 1; }

# --- the data plane: deposit the key on one's own profile ------------------
djar=$(mktemp)
code=$(curl -s --max-time 15 -c "$djar" -o /dev/null -w '%{http_code}' \
  -X POST "$data/login" -d "username=admin&password=$password")
[ "$code" = "303" ] || { echo "signing in on the data plane answered $code"; exit 1; }

# The answer is a redirect either way - to the profile page when it worked, to
# /login when the session was not there - so the STATUS says nothing and the
# destination says everything.
where=$(curl -s --max-time 15 -b "$djar" -o /dev/null -w '%{redirect_url}' \
  -X POST "$data/profile/dev-key" --data-urlencode "key=$pub")
case "$where" in
  */profile/dev/key) echo "deposited, and the page says so" ;;
  *) echo "the deposit went to $where - not the profile page"; exit 1 ;;
esac

# And read back, because a redirect is a promise and this is the evidence: the
# profile page prints the FINGERPRINT of what it holds.
if ! curl -s --max-time 15 -b "$djar" "$data/profile/dev/key" | grep -qF "$fingerprint"; then
  echo "the profile page does not show $fingerprint - the key was not stored"
  exit 1
fi
echo "the gateway holds $fingerprint"
