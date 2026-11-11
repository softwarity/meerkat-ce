# Meerkat

> The sentinel at your application's door.

[![plug](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fmeerkat.softwarity.dev%2Fassets%2Fplug.json&query=%24.version&label=plug&color=7c3aed)](https://softwarity.github.io/plug/)

**Meerkat is an authentication proxy and API gateway that puts access control
in front of your internal applications.** It signs people in, decides who may
reach which application and which endpoint, routes the request, and records
what happened. Your services receive requests that are already authenticated,
with a signed JWT carrying the identity, the roles and the tenant.

One binary, no external dependency: no database to install, no cache, no
broker.

<p align="center">
  <img src="docs/public/img/readme/app-gateway.gif" width="520" alt="Users reach the services through Meerkat, which signs them in, routes their requests and records what happened">
</p>

## What it replaces

The stack teams usually assemble in front of an internal application:

| Need | Usually assembled from | In Meerkat |
| --- | --- | --- |
| Protect an application that has no login | oauth2-proxy, Pomerium | Sign-in pages and session, per-route access rules |
| Sign-in, two-factor, single sign-on | Keycloak, Authentik | Local accounts, TOTP, OpenID Connect, GitHub; LDAP and SAML in Enterprise |
| Routing, rate limits, quotas | Kong, Traefik, APISIX | Hot-reloaded routes, predicates and filters, per-endpoint quotas |
| Roles and tenants | admin screens to build | Hierarchical roles, groups, tenants |
| Audit of every admin change | a log pipeline to connect | Built-in audit trail, before and after |
| Calls on a timer | cron jobs in every service | Scheduled calls to your services |

The [case for Meerkat](https://meerkat.softwarity.dev/product/the-case/) sets
each line against its alternatives.

## The pages your users see

An API gateway routes requests; it serves no page. Put Kong, Traefik or APISIX in
front of an application and every screen of the account journey is still yours
to build, in every application, or to theme in an identity server beside the
gateway. Meerkat serves some forty of them:

- **Getting in**: sign-in by password, passkey, emailed code or your directory;
  sign-up with address confirmation; forgotten password; an expired link that
  says what to do next.
- **Securing the account**: authenticator app enrolment and challenge, recovery
  codes, passkeys, trusted browsers.
- **Managing the account**: profile, password, address, active sessions, sign-in history,
  API tokens.
- **Being let in, or not, clearly**: the choice of a tenant and a group, a
  waiting room for an account with no access yet, refusals that say why and whom
  to contact, a maintenance page that comes back by itself.

Every one of them in 22 languages, with translations editable from the console;
in your colours, light and dark, with the contrast of every text measured by the
test suite; with your logo, and previewed in the console before anyone sees it.
The emails they send wear the same theme. An application behind Meerkat writes
none of them.

## Try it

The community edition, built from this repository, free and fit for production:

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat-ce
```

Everything Meerkat does, as an evaluation until a license is added:

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat
```

- **http://localhost:9090** is the administration console. Sign in as `admin`
  with the password you chose.
- **http://localhost:8080** is what your users reach. A fresh gateway has no
  route, so it answers 404 until you add one.

The evaluation carries a notice on its pages; a Team or Enterprise license
removes it, with no restart.

<!-- Screenshot of the administration console goes here:
     docs/public/img/readme/console.webp, referenced as
     ![The Meerkat administration console](docs/public/img/readme/console.webp) -->

The [quick start](https://meerkat.softwarity.dev/docs/start/quick-start/) adds
a first route and sends a request through it. Files for Docker, Compose, Swarm
and Helm are on the
[install page](https://meerkat.softwarity.dev/docs/start/install/).

## Community and Enterprise

| | Community (CE) | Enterprise (EE) |
| --- | --- | --- |
| **Price** | Free, production included | Commercial license |
| **Sign-in** | Local accounts, TOTP two-factor, OpenID Connect, GitHub, API tokens | Also LDAP, Active Directory and SAML 2.0 |
| **Access** | Hierarchical roles, groups, per-route and per-endpoint rules | Also roles granted from directory groups at sign-in |
| **Tenants** | One | Several, isolated, chosen at sign-in |
| **Routing** | Hot-reloaded routes, predicates, filters, WebSocket, rate limits, per-endpoint quotas | Same |
| **TLS** | Generated, imported or signed certificates | Also ACME issuance and renewal |
| **Secrets** | Encrypted vault | Same |
| **Audit** | Audit trail in the console | Also CSV export and sending to an OpenTelemetry collector |
| **Scheduled calls** | Recurring and one-off calls, with history and retries | Same |
| **High availability** | One gateway on embedded storage | Active/active nodes on a shared PostgreSQL |
| **Configuration** | Saved configurations compared and composed value by value, export and import | Also configurations kept in a git repository |
| **Single sign-on for other tools** | - | Meerkat acts as their OpenID Connect provider |
| **Developer tunnel** | plug runs on its own, beside the gateway | plug's agent built into the gateway |
| **Image** | `softwarity/meerkat-ce` | `softwarity/meerkat` with a license; without one it is an evaluation |

The `ee` build tag decides what the binary contains: the Community image does
not carry the Enterprise code. Without a license, `softwarity/meerkat` is an
evaluation, with a notice on its pages, and is not for production. The
[editions page](https://meerkat.softwarity.dev/product/editions/) details each
line and [pricing](https://meerkat.softwarity.dev/pricing/) gives the terms.

## plug

[plug](https://softwarity.github.io/plug/) is Softwarity's developer tunnel: it
serves a service from a developer's machine in place of the one deployed in the
cluster. It runs on its own with any edition. The Enterprise image carries
plug's agent, pinned to the version in the badge above, so developers sign in
to it with their own key and the gateway shows who is plugged in.

## Documentation

- [Documentation](https://meerkat.softwarity.dev/docs/)
- [What Meerkat is](https://meerkat.softwarity.dev/product/what-is-meerkat/)
- [Editions](https://meerkat.softwarity.dev/product/editions/) and
  [pricing](https://meerkat.softwarity.dev/pricing/)
- [Release notes](./RELEASE_NOTES.md)

Building Meerkat itself: see [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

The Community edition is source-available under
[FSL-1.1-Apache-2.0](./LICENSE.md), the Functional Source License: use, copy,
modify and redistribute it for any purpose except building a competing product
or service. Internal and production use in a company is permitted. Each
release becomes Apache 2.0 two years after its publication.

The Enterprise code is not in this repository. It is available to customers
under a Softwarity commercial license; see
[the editions](https://meerkat.softwarity.dev/product/editions/).

Edited by [Softwarity](https://meerkat.softwarity.dev/softwarity/).
