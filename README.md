# Meerkat

> The sentinel at your application's door.

**Meerkat** is an **app-gateway**: one door in front of the applications your
teams build, which takes charge of everything that is not their core business -
authentication, access rules, organisations, routing, quotas, audit, and the
calls they want made on a timer or once, later. Your services receive requests that are
already authenticated, carrying a signed JWT with identity, roles and
organisation. One binary, zero dependency.

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat
```

8080 is what your users reach, 9090 is the admin console.

**[www.softwarity.io](https://www.softwarity.io/)** is where the product is
explained: what it does capability by capability, the documentation, what it
replaces and what that costs, the editions and the price. This file is for
people working ON it.

## Status

**Released, and still being built.** What the gateway does today, what is
half-built and what is not started are one table in
[FEATURES.md](./FEATURES.md) (French), with the state read from the code rather
than from a plan. Shipping something means ticking its box in the same commit.
What changed in each release is in [RELEASE_NOTES.md](./RELEASE_NOTES.md).

## Repository layout

| Path | What |
|---|---|
| `cmd/meerkat/` | the single binary: data plane on :8080, control plane on :9090 |
| `internal/` | the Go core - routing, gateway, store, sessions, sign-in, admin API |
| `ee/` | the Enterprise code, under a commercial license; absent from the community mirror |
| `console/` | the Angular admin console, served through the admin port |
| `docs/` | the documentation site |
| `deploy/` | the published Compose and Swarm files, and the Helm chart |
| `e2e/` | the Playwright integration suite, and the test platform |
| `tools/bench/` | the comparative benchmark the CI runs |

Two images, two products: the `ee` build tag decides what the linker puts in
(`internal/edition`). The community image does not carry the Enterprise code
switched off - it does not carry it at all.

## Development

Two terminals. Node is pinned by `.node-version` (fnm/nvm switch on their own);
the Go toolchain resolves from `go.mod`.

```bash
# terminal 1 - the console (:4200)
cd console && npm install && npm start

# terminal 2 - the gateway, rebuilt on every .go save
MEERKAT_ADDR=:8082 \
MEERKAT_ADMIN_ADDR=:9092 \
MEERKAT_CONSOLE_URL=http://localhost:4200 \
MEERKAT_ADMIN_PASSWORD=test1234 \
make dev               # needs air: go install github.com/air-verse/air@latest
```

Then browse **http://localhost:9092**, the admin port, and sign in as `admin` /
`test1234`: the gateway serves its API and its login there and proxies
everything else to the console dev server, HMR included. `MEERKAT_CONSOLE_URL`
is what makes that proxying happen - without it the admin port answers a JSON
status page. The API documentation is at http://localhost:9092/apidocs/.

`make dev` builds the **Enterprise** binary, `make dev-ce` the community one.
The tag lives in `.air.toml`, so `air` typed by hand builds the same binary as
`make dev`.

### Flags and environment

The binary takes `-addr`, `-admin-addr`, `-console-url`, `-data` and `-version`.
Each `MEERKAT_*` variable is the default of its flag, and the flag wins. The
admin password has no flag, on purpose: an argument shows in `ps` and in the
shell history, so it stays in `MEERKAT_ADMIN_PASSWORD`.

To pass flags under hot reload, call `air` directly - everything after `--` goes
to the **binary**, never to the build:

```bash
MEERKAT_ADMIN_PASSWORD=test1234 air -- -addr :8082 -admin-addr :9092 -console-url http://localhost:4200
```

### Other ways to run it

```bash
# A throwaway instance: other ports, its database in a temporary directory.
go build -o bin/meerkat ./cmd/meerkat && \
MEERKAT_ADMIN_PASSWORD=test1234 ./bin/meerkat \
  -addr :18082 -admin-addr :19092 \
  -console-url http://localhost:4200 -data "$(mktemp -d)"

# Demo content (routes, organisations, users) in the current database.
go run ./cmd/seed-demo

# One binary with the console EMBEDDED, no terminal 1.
make ui && make build && MEERKAT_ADMIN_ADDR=:9092 MEERKAT_ADMIN_PASSWORD=test1234 ./bin/meerkat

# The documentation site, on 0.0.0.0:8765 so a gateway route can reach it.
cd docs && npm start

# The OpenTelemetry browser bundle, staged for go:embed (OBS-04). Skip it and
# the Enterprise binary still builds: the injection stays off and says why.
make telemetry

# A trace end to end: a local Jaeger, then console > Infra > OpenTelemetry
# with the collector http://127.0.0.1:4318. Traces read on http://localhost:16686.
docker run -d --name jaeger -p 16686:16686 -p 4318:4318 jaegertracing/all-in-one:latest
```

### Traps worth knowing

- `MEERKAT_ADMIN_PASSWORD` seeds the admin on the **first** start of an empty
  data directory only. Password forgotten in development: delete `data/` and
  start again.
- **If one port cannot be bound, the whole process exits.** `bind :8082 in use`
  means a Meerkat is already running; pass `MEERKAT_ADMIN_ADDR` when :9090 is
  taken by something else.
- The console is **English only**. The pages of the data plane (sign-in, second
  factor, user button) are the translated ones: `internal/auth/i18n.go`.

## Before pushing

```bash
make fmt lint          # gofmt, golangci-lint
make test test-ee      # the Go suites, community and Enterprise, with -race
cd e2e && npx playwright test      # the integration suite
```

- **Language**: code, comments, commit messages and public documentation are in
  English. `FEATURES.md` is maintained in French.
- **Commits**: an imperative subject ("Add route matcher"); the body says *why*
  when it is not obvious.
- **Secrets**: never commit a credential, a key or a license file. No exception.
- **Releases** are cut by the CI after approval, from a green run; no image is
  published by a push alone.

## License

[FSL-1.1-Apache-2.0](./LICENSE.md) (Functional Source License): free to use,
copy, modify and redistribute for any purpose except building a competing
product or service - internal and production use in your company is explicitly
permitted. Each release automatically becomes **Apache 2.0 two years** after
its publication. The Enterprise code under `ee/` is source-visible and usable
only under a Softwarity commercial agreement; see
[the editions](https://www.softwarity.io/en/product/editions).

Edited by **[Softwarity](https://www.softwarity.io/)**.
