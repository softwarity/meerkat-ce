#!/bin/sh
# A signing certificate for the test identity provider, made where it runs.
#
# The image ships one that expired in 2020, and Meerkat refuses an assertion
# signed under an expired certificate - rightly. A key is made here rather than
# committed: a private key has no business in the repository, even a test one.
set -e
dir=$(dirname "$0")/cert
[ -s "$dir/server.crt" ] && exit 0
mkdir -p "$dir"
openssl req -x509 -newkey rsa:2048 -nodes -days 3650 -subj "/CN=simplesamlphp.test" \
  -keyout "$dir/server.pem" -out "$dir/server.crt" 2>/dev/null
chmod 644 "$dir/server.pem" "$dir/server.crt"
echo "test identity provider certificate made in $dir"
