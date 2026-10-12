#!/bin/sh
# make plug V=2.25.0 - hand out another plug release. docker/plug.json says which;
# this moves everything that must agree with it (plug_test.go checks they do):
# the agent module linked into the binary, and the image's payload.
set -eu
v="${1-}"
v="${v#v}"
# A version, or nothing runs: it comes from outside (plug's dispatch, the tag
# of its latest release) and this runs with the right to push to main.
if ! printf '%s' "$v" | grep -Eqx '[0-9]+\.[0-9]+\.[0-9]+'; then
  echo "'$v' is not a plug release (x.y.z): make plug V=x.y.z" >&2
  exit 1
fi
commit=$(git ls-remote https://github.com/softwarity/plug.git "refs/tags/v$v^{}" "refs/tags/v$v" | head -1 | cut -f1)
[ -n "$commit" ] || { echo "plug has no tag v$v" >&2; exit 1; }
printf '{\n  "version": "%s",\n  "commit": "%s"\n}\n' "$v" "$commit" > docker/plug.json
go get "github.com/softwarity/plug/agent@$commit"
go mod tidy
sed -i.bak "s/^ARG PLUG_VERSION=.*/ARG PLUG_VERSION=$v-hosted/" docker/Dockerfile && rm -f docker/Dockerfile.bak
# One line under NEXT RELEASE, and only there - the numbered sections are what
# was published. A bump before the release replaces the previous bump's line.
awk -v line="- **plug $v in the Enterprise image.** The client developers install with \`ssh get@<gateway> install\`, and the agent built into the gateway." '
  /^## NEXT RELEASE/ { print; inside = 1; getline; print; print line; next }
  inside && /^(## |---)/ { inside = 0 }
  inside && /^- \*\*plug [0-9.]+ in the Enterprise image\.\*\* The client developers install/ { next }
  { print }
' RELEASE_NOTES.md > RELEASE_NOTES.md.new && mv RELEASE_NOTES.md.new RELEASE_NOTES.md
echo "plug $v ($commit): docker/plug.json, go.mod, docker/Dockerfile and the release notes moved - deploy, check 'plug: ready (v$v-hosted)', commit"
