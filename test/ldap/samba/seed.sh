#!/bin/sh
# Mirror of ../openldap/init.ldif inside the domain: the same nine people, the
# same groups, the same nesting - told the way Active Directory tells it.
# Re-running is harmless.
#
# Read the LDIF for what each person is FOR. Two differences are the dialect's
# own and matter when comparing the two sides:
#
#   - dan is really DISABLED here (AD has a flag for it); on the OpenLDAP side
#     he simply has no password. Same outcome, a refusal, by two roads.
#   - gina lives in her own OU here too, so a search base that stops at the
#     default Users container never finds her.
set -e
PASS='Passw0rd!2026'
BASE='DC=ad,DC=example,DC=com'

# Every step goes through this. The previous version ended each command with
# "|| echo already there", which turned a REAL failure into a line of reassuring
# output: gina was never created for a week and the seed said "seeded". Only
# "already exists" is tolerated; anything else stops the script.
try() {
  out=$("$@" 2>&1) && return 0
  case "$out" in
    *"already exists"*|*"already a member"*|*"Already a member"*)
      echo "  already there: $*" ;;
    *)
      echo "FAILED: $*"
      echo "  $out"
      return 1 ;;
  esac
}

add_user() {  # login, given, sur, mail, [ou relative to the domain]
  if [ -n "$5" ]; then
    try samba-tool user create "$1" "$PASS" \
      --given-name="$2" --surname="$3" --mail-address="$4" \
      --userou="$5" --use-username-as-cn
  else
    try samba-tool user create "$1" "$PASS" \
      --given-name="$2" --surname="$3" --mail-address="$4" \
      --use-username-as-cn
  fi
}

# The partners subtree, before the person who lives in it. `ou create` wants
# the FULL dn; `user --userou` wants it RELATIVE to the domain and glues the
# base on itself - give it the full one and it builds
# OU=partners,DC=ad,DC=example,DC=com,DC=ad,DC=example,DC=com and says the
# parent does not exist.
try samba-tool ou create "OU=partners,$BASE"

add_user johndoe John Doe johndoe@ad.example.com
add_user janedoe Jane Doe janedoe@ad.example.com
add_user alice Alice Martin alice@ad.example.com
add_user bob Bob Nguyen bob@ad.example.com
add_user carla Carla Rossi carla@ad.example.com
add_user dan Dan Fisher dan@ad.example.com
add_user evec Ève Chevalier eve.chevalier@ad.example.com
add_user frank Frank Weber frank@ad.example.com
add_user gina Gina Lopez gina@partner.example.com "OU=partners"

for g in devops frontend backend developer operator "Brest Agents"; do
  try samba-tool group add "$g"
done

for pair in \
  "devops janedoe" \
  "frontend johndoe" "frontend janedoe" "frontend alice" "frontend evec" \
  "backend johndoe" "backend carla" \
  "operator johndoe" "operator janedoe" "operator gina" \
  "Brest_Agents frank"
do
  g=${pair% *}; m=${pair#* }
  try samba-tool group addmembers "$(echo "$g" | tr '_' ' ')" "$m"
done

# Nested: developer holds the three team groups, not the people. Carla reaches
# it through backend alone, which is what makes her the one who proves it.
for g in frontend backend devops; do
  try samba-tool group addmembers developer "$g"
done

# Known to the directory, refused at the door.
try samba-tool user disable dan

echo "seeded"
