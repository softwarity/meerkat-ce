# Meerkat

> The sentinel at your application's door.

**Meerkat** is an **app-gateway**: one door in front of the applications your
teams build, which takes charge of everything that is not their core business -
authentication, access rules, organisations, routing, quotas and audit. Your
services receive requests that are already authenticated, carrying a signed JWT
with identity, roles and organisation. One binary, zero dependency.

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

🚧 **Under construction, and running.** What the gateway does today, what is
half-built and what is not started are one table in
[FEATURES.md](./FEATURES.md) (French), with the state read from the code rather
than from a plan. Shipping something means ticking its box in the same commit.

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

Then browse **http://localhost:9092**, the admin port: the gateway serves its
API and its login there and proxies everything else to the console dev server,
HMR included. `MEERKAT_CONSOLE_URL` is what makes that proxying happen - without
it the admin port answers a JSON status page.

`make dev` builds the **Enterprise** binary, `make dev-ce` the community one -
and the difference is what the linker put in, not a flag. The ports, the
disposable instances, the demo data, the integration suite and the traps worth
knowing are in **[DEV.md](./DEV.md)**; the repository layout, the conventions
and what to run before pushing are in
**[CONTRIBUTING.md](./CONTRIBUTING.md)**.

## License

[FSL-1.1-Apache-2.0](./LICENSE.md) (Functional Source License): free to use,
copy, modify and redistribute for any purpose except building a competing
product or service - internal and production use in your company is explicitly
permitted. Each release automatically becomes **Apache 2.0 two years** after
its publication. The Enterprise code under `ee/` is source-visible and usable
only under a Softwarity commercial agreement; see
[the editions](https://www.softwarity.io/en/product/editions).

Edited by **[Softwarity](https://www.softwarity.io/)**.
