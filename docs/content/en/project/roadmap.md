---
title: Roadmap
section: The project
order: 10
summary: What is built, what is being finished, and what is deliberately not on the list.
---

# Roadmap

The authority on state is
[FEATURES.md](https://github.com/softwarity/meerkat/blob/main/FEATURES.md) in
the repository: one line per feature, the state read from the code, and a box
that is ticked in the commit that ships the thing. This page is that table read
out loud.

## Built and in use

The critical path is whole. A request arrives, a route matches it, filters
transform it, an access rule decides, and what happened is visible afterwards.

- **Routing**: the predicate and filter catalogue, edited hot, applied on the
  next request. See [predicates](/docs/predicates/overview) and
  [filters](/docs/filters/overview).
- **Identity**: local accounts, OpenID Connect, LDAP and Active Directory,
  GitHub - all tested against real servers. Sessions, API tokens, signed
  tokens to upstreams.
- **Access**: a hierarchical role catalogue, groups per organisation,
  organisations themselves, a rule per route, and per-endpoint security read
  from a service's OpenAPI description.
- **Two-factor**: TOTP with trusted browsers, and passkeys.
- **The vault**: secrets sealed at rest and plain values, both referenced by
  name.
- **TLS**: certificates and ACME issuance, serialised so a cluster asks once.
- **Audit**: every administrative change with its author and a field-level
  diff.
- **The console**: the whole administration, on its own port, with the
  capability split that decides who sees which half.
- **The agent endpoint**: MCP on the control plane, under the same rules a
  human gets.
- **The navigation portal**: one bar across the applications the gateway
  serves, injected into pages that ship no library for it.
- **Clustering**: several gateways behind one PostgreSQL, coordinating through
  the database rather than with each other.

## Being finished

These work and are not finished. The table in the repository says, line by
line, what is missing from each.

- **Versioned configurations** - duplicate, edit as a draft, diff, switch
  atomically, roll back. The storage is there; the screens are half-built.
- **Quotas** - per consumer, with a calibration mode that only logs. Rate
  limits are shipped; quotas are not.
- **Dev mode** - the tunnel works. What is missing is what shows it: the page
  that names what is substituted, and the console screen listing live sessions.
- **Notifications** - the mail relay is shipped. The multilingual mail
  templates, and the daily digest, are not.
- **Identity to upstreams** - the signed token is emitted. An exchange endpoint
  handing back an access and refresh pair is not written.
- **Passkeys** - usable as a factor. Recovering an account whose only passkey
  is lost is not written yet.
- **One-time codes by email** - nothing is written; SMTP is the prerequisite
  and it is in place.

## Next

- **A per-tenant portal**: the navigation bar is global today. Letting an
  organisation carry its own icon, title and arrangement would be the first
  per-tenant visual override in the product, and it waits for that decision to
  be made properly rather than by accident.
- **A discovery wizard**: the gateway can already read the Docker socket and a
  Kubernetes namespace. Turning that into "scan, pick a container, get a route"
  is the screen that is missing.
- **SAML**, for the enterprises whose identity provider does not speak OpenID
  Connect. It is registrable today and refuses at the factory, which is honest
  and not yet useful.

## Not on the list

Kerberos and SPNEGO are written down as Enterprise, and nothing has been
started. A plugin system is not planned: the filter catalogue is curated on
purpose, and every filter in it is one we can explain and test.

> [!TIP]
> What the integration suite actually enforces is on the
> [test coverage](/project/tests) page - it is the file the suite executes, not
> a description of it.
