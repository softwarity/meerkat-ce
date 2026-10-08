#!/bin/sh
# OpenLDAP and phpLDAPadmin on the local cluster (openldap.yaml).
#
# The admin password is drawn once and kept beside this script, in
# openldap.password.local (git ignores *.local): the Secret is made from it,
# never from an argument on a command line.
set -e
here=$(dirname "$0")
[ -f "$here/env.local" ] && . "$here/env.local"
ns=${NAMESPACE:-default}
secret="$here/openldap.password.local"
# No trailing newline: it would travel into the Secret, and the directory
# would take it as part of the password.
[ -s "$secret" ] || (umask 077; printf '%s' "$(openssl rand -hex 12)" > "$secret")

kubectl -n "$ns" create secret generic openldap-admin --from-file=password="$secret" \
  --dry-run=client -o yaml | kubectl -n "$ns" apply -f - >/dev/null
kubectl -n "$ns" create configmap openldap-seed \
  --from-file=init.ldif="$here/../../test/ldap/openldap/init.ldif" \
  --dry-run=client -o yaml | kubectl -n "$ns" apply -f - >/dev/null
kubectl -n "$ns" apply -f "$here/openldap.yaml" >/dev/null
kubectl -n "$ns" rollout status deploy/openldap --timeout=180s | tail -1
kubectl -n "$ns" rollout status deploy/phpldapadmin --timeout=180s | tail -1
echo "OpenLDAP: ldap://openldap.$ns.svc:389 (base dc=example,dc=com, admin cn=admin,dc=example,dc=com)"
echo "phpLDAPadmin: http://phpldapadmin.$ns.svc:8091, in the cluster only - publish it through a Meerkat route"
echo "The admin password is in $secret"
