# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26
ARG NODE_VERSION=24
# The plug client this image hands out, and which edition carries it. Declared
# here because a FROM can only read an ARG from the global scope.
ARG PLUG_VERSION=latest
ARG EDITION=ce

# The console first: its production build is embedded in the Go binary, so the
# image needs nothing but the binary itself.
FROM node:${NODE_VERSION}-alpine AS console
WORKDIR /console
COPY console/package.json console/package-lock.json ./
RUN npm ci
COPY console/ ./
RUN npm run build

FROM golang:${GO_VERSION} AS build
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
RUN CGO_ENABLED=0 go build -trimpath -tags "${GO_TAGS}" -ldflags "-s -w \
      -X github.com/softwarity/meerkat/internal/version.Version=${VERSION} \
      -X github.com/softwarity/meerkat/internal/version.Commit=${GIT_REV} \
      -X github.com/softwarity/meerkat/internal/version.Date=${BUILD_DATE}" \
      -o /meerkat ./cmd/meerkat

# An empty /data owned by the runtime user. Distroless has no shell, so the
# directory is built here and copied in — and Docker gives a fresh volume
# mounted over it the SAME ownership, which is what lets a nonroot process
# write to it.
FROM busybox:stable AS datadir
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
# Empty, but a real filesystem: a COPY from scratch has nothing to match.
FROM alpine:3.24 AS payload-ce
RUN mkdir -p /opt/plug /usr/local/bin
FROM payload-${EDITION} AS payload

# Alpine, like plug's own image, and for the same reason: what answers the
# anonymous download account is a shell script. distroless has no shell, so the
# script could not run there - and carrying a static busybox into a distroless
# image to make it run was building a contraption around a base that had
# already said no.
FROM alpine:3.24
RUN adduser -D -u 65532 nonroot && mkdir -p /data && chown nonroot:nonroot /data
COPY --from=build /meerkat /meerkat
# The plug client and the script that hands it out, from plug's own image.
# Empty in the community one, where nothing runs an agent.
COPY --from=payload /opt/plug /opt/plug
COPY --from=payload /usr/local/bin/ /usr/local/bin/
USER nonroot
# ABSOLUTE, and that is the whole point. The flag's default is the RELATIVE
# path "data", which resolves against the working directory. The database
# therefore landed in the container's own layer while a volume sat unused on
# /data: every restart came back to a blank gateway, re-seeded from the
# configuration file, with the console's work gone. Nothing in the logs said
# so, because nothing was wrong from the binary's side.
ENV MEERKAT_DATA=/data
VOLUME ["/data"]
ENTRYPOINT ["/meerkat"]
