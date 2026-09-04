MODULE  := github.com/softwarity/meerkat
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)

.PHONY: build build-ee ui dev dev-ce test test-ee test-pg lint fmt vet clean ldap-up ldap-down ldap-test pg-up pg-down

# Hot-reload dev loop: rebuilds and restarts the gateway on every .go save.
# Requires air (once): go install github.com/air-verse/air@latest
# Resolved from PATH or GOPATH/bin, so it works even when ~/go/bin is not in PATH.
AIR := $(shell command -v air 2>/dev/null || echo "$$(go env GOPATH)/bin/air")

# Two shapes, because there are two images and nothing in between:
#
#   make dev     the Enterprise product - what .air.toml builds (`-tags ee`)
#   make dev-ce  the community one, where ee/ is not compiled in at all
#
# The tag lives in .air.toml, so typing `air` by hand (the way DEV.md passes
# -addr/-console-url) builds the same binary as `make dev`. Anything after `--`
# goes to MEERKAT, never to the build.
dev:
	$(AIR)

dev-ce:
	$(AIR) --build.cmd "go build -o ./tmp/meerkat ./cmd/meerkat"

# Two artifacts from one commit. The community one carries no Enterprise code
# at all: ee/ is only linked by internal/edition's `ee`-tagged file.
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/meerkat ./cmd/meerkat

build-ee:
	CGO_ENABLED=0 go build -trimpath -tags ee -ldflags "$(LDFLAGS)" -o bin/meerkat-ee ./cmd/meerkat

# Build the console (all locales) and stage it for go:embed. Run before
# `make build` to get a binary that ships its own console; skip it and the
# binary builds console-less (admin port answers a JSON status page).
# Requires console/node_modules (once: cd console && npm install).
ui:
	cd console && npm run build
	rm -rf internal/admin/ui/dist
	mkdir -p internal/admin/ui/dist
	cp -R console/dist/console/browser/. internal/admin/ui/dist/
	touch internal/admin/ui/dist/.gitkeep

test:
	go test -race ./...

# The Enterprise suite: the same tests plus everything under ee/. Both have to
# be green - a change that only compiles with the tag breaks the community
# image silently, and the publication is what would find out.
test-ee:
	go test -race -tags ee ./...

lint:
	golangci-lint run

fmt:
	gofmt -l -w .

vet:
	go vet ./...

clean:
	rm -rf bin dist

# ── authorities to test against ──────────────────────────────────────────────
# Dex (a real OIDC provider, 46 MB of static Go), an OpenLDAP, and a REAL
# Active Directory domain controller (Samba 4) seeded with the same people and
# the same nested groups. No Keycloak: a gateway that exists so an installation
# need not run an identity server should not need half a gigabyte of one to
# test itself. The idp tests skip when these are down, so `make test` never
# depends on Docker.
ldap-up:
	cd test/ldap && docker compose up -d
	@echo "waiting for the domain controller to provision (about a minute on a cold start)..."
	@cd test/ldap && for i in $$(seq 1 60); do \
		[ "$$(docker inspect meerkat-samba-ad --format '{{.State.Health.Status}}' 2>/dev/null)" = healthy ] && break; \
		sleep 5; \
	done
	docker exec meerkat-samba-ad sh /seed.sh
	@echo "dex http://localhost:5556/dex - openldap ldap://localhost:3389 - active directory ldaps://localhost:3636"

# The last metre: register the seeded directory as an authority on a RUNNING
# gateway, so trying an LDAP sign-in is a click instead of five fields typed
# from memory. Needs `make ldap-up` and a gateway (see DEV.md).
ldap-demo:
	@go run ./test/ldap/demo

ldap-down:
	cd test/ldap && docker compose down -v

# The directory tests live with the driver, under ee/ - it is sold, so it is
# not in the FSL tree. Dex stays in internal/idp: OIDC is community.
ldap-test:
	go test ./ee/directories/ ./internal/idp/ -run 'LDAP|OIDCAgainstDex' -count=1 -v

# The same suite on the other database (QUAL-01). Every test asks
# internal/store/dbtest which database to open, so this needs no second suite
# and no tag - just a server, and one variable naming it.
#
# What it buys is the only thing the embedded database cannot say: that the
# queries in internal/store, written once with `?` placeholders, actually run
# on PostgreSQL. The first run of it found a 32-bit column holding a Unix
# second and two INSERT OR IGNORE nobody else speaks.
pg-up:
	cd test/postgres && docker compose up -d --wait
	@echo "postgres://meerkat:meerkat@localhost:55432/meerkat?sslmode=disable"

pg-down:
	cd test/postgres && docker compose down -v

test-pg:
	MEERKAT_TEST_DATABASE_URL='postgres://meerkat:meerkat@localhost:55432/meerkat?sslmode=disable' \
		go test -tags ee ./... -count=1
