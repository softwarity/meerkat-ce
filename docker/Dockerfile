# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27
ARG NODE_VERSION=24
# The plug client this image hands out, and which edition carries it. Declared
# here because a FROM can only read an ARG from the global scope.
# The client flavour this image serves. `hosted` carries the verbs that make
# sense against an embedded agent and leaves out the ones the gateway decides -
# update and versions - because a gateway serves its own client and its own
# version. The flavour rides in the version string, so nothing here has to
# repeat it.
#
# A RELEASE, pinned. The bare `hosted` tag is moved by plug's CI on every push
# to its main branch, so an image built from it carries whatever plug was
# being worked on that morning. Bump this line when plug publishes a release.
ARG PLUG_VERSION=2.21.9-hosted
ARG EDITION=ce

# ONE build for every architecture: the console and the binary are made on the
# BUILD platform (Go cross-compiles for free, and a JavaScript bundle has no
# architecture), and only the last stage, the image that ships, is per TARGET
# platform. So `--platform linux/amd64,linux/arm64` runs no compiler under
# emulation - only the two shell lines of the final stage.
#
# The console first: its production build is embedded in the Go binary, so the
# image needs nothing but the binary itself.
FROM --platform=$BUILDPLATFORM node:${NODE_VERSION}-alpine AS console
WORKDIR /console
COPY console/package.json console/package-lock.json ./
RUN npm ci
COPY console/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
COPY --from=console /console/dist/console/browser/ internal/admin/ui/dist/
ARG VERSION=dev
ARG GIT_REV=none
ARG BUILD_DATE=unknown
# The edition, and the ONLY thing that separates the two images: empty builds
# the community one (ee/ is not compiled in at all), "ee" the Enterprise one.
ARG GO_TAGS=
# Where the binary will RUN, set by buildx from --platform; empty on a plain
# `docker build`, where Go then builds for the machine it is on.
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -tags "${GO_TAGS}" -ldflags "-s -w \
      -X github.com/softwarity/meerkat/internal/version.Version=${VERSION} \
      -X github.com/softwarity/meerkat/internal/version.Commit=${GIT_REV} \
      -X github.com/softwarity/meerkat/internal/version.Date=${BUILD_DATE}" \
      -o /meerkat ./cmd/meerkat

# An empty /data owned by the runtime user. Distroless has no shell, so the
# directory is built here and copied in — and Docker gives a fresh volume
# mounted over it the SAME ownership, which is what lets a nonroot process
# write to it.
FROM --platform=$BUILDPLATFORM busybox:stable AS datadir
RUN mkdir -p /seed/data

# The plug client, for the anonymous `get` account: `ssh get@gateway install | sh`
# is the first thing a developer types, before they have a client at all.
#
# It comes from plug's OWN image rather than being rebuilt or transcribed here.
# What answers that account is a 201-line shell script (agent/serve-binary) that
# emits an installer which reads the ssh command out of the process table to
# pre-create a profile, grants capabilities on Linux, installs a setuid helper on
# macOS, and carries wintun for Git Bash. Copying it is the only way that stays
# true to plug's; transcribing it would fork somebody else's installer and let it
# rot quietly at their next release.
#
# EE-only, selected by EDITION: an image with no agent has nobody to serve it,
# and the community one resolves to scratch - nothing copied, nothing carried.
FROM docker.io/softwarity/plug:${PLUG_VERSION} AS payload-ee
# Empty, but a real filesystem: a COPY from scratch has nothing to match. Two
# empty directories have no architecture either, hence the build platform.
FROM --platform=$BUILDPLATFORM alpine:3.24 AS payload-ce
RUN mkdir -p /opt/plug /usr/local/bin
FROM payload-${EDITION} AS payload

# Alpine, like plug's own image, and for the same reason: what answers the
# anonymous download account is a shell script. distroless has no shell, so the
# script could not run there - and carrying a static busybox into a distroless
# image to make it run was building a contraption around a base that had
# already said no.
FROM alpine:3.24
# /data belongs to the root GROUP, not to the user, and the group may write:
# OpenShift draws an arbitrary uid from the namespace range and adds group 0,
# so 65532:65532 mode 755 is read-only there and the store fails on its first
# write. A mounted volume hides that - the kubelet's fsGroup rewrites the
# ownership - so it only bites where /data is a plain directory. Everywhere
# else the process is still 65532 and still the owner: nothing changes.
RUN adduser -D -u 65532 nonroot && mkdir -p /data && chown 65532:0 /data && chmod g+rwX /data
COPY --from=build /meerkat /meerkat
# The plug client and the script that hands it out, from plug's own image.
# Empty in the community one, where nothing runs an agent.
COPY --from=payload /opt/plug /opt/plug
COPY --from=payload /usr/local/bin/ /usr/local/bin/
# The mount helper. When a developer mounts a volume, the agent starts a
# helper beside the workload running THIS image as `plug-agent mount-serve`
# (copied above). The SMB server is compiled into that binary, so there is no
# package to install for it - and one image to mirror into a disconnected
# cluster, not two.
# NUMERIC, and the number is the point: Pod Security Standards `restricted`
# set runAsNonRoot without allocating a uid, and the kubelet then refuses an
# image whose USER is a name - "image has non-numeric user (nonroot), cannot
# verify user is non-root". It is the uid adduser created above, so nothing
# else moves.
USER 65532
# ABSOLUTE, and that is the whole point. The flag's default is the RELATIVE
# path "data", which resolves against the working directory. The database
# therefore landed in the container's own layer while a volume sat unused on
# /data: every restart came back to a blank gateway, re-seeded from the
# configuration file, with the console's work gone. Nothing in the logs said
# so, because nothing was wrong from the binary's side.
ENV MEERKAT_DATA=/data
VOLUME ["/data"]
ENTRYPOINT ["/meerkat"]
