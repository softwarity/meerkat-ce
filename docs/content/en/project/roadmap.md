---
title: Roadmap
section: The project
order: 10
summary: What is built, what is being finished, and what is deliberately not on the list.
---

# Roadmap

The authority on state is
[FEATURES.md](https://github.com/softwarity/meerkat-ce/blob/main/FEATURES.md) in
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
  name. The same master key seals TLS keys and TOTP secrets, and it rotates
  with a restart.
- **TLS**: certificates and ACME issuance, serialised so a cluster asks once.
- **Audit**: every administrative change with its author and a field-level
  diff, and the security of the accounts - every sign-in, every refusal with its
  reason and address, every way in changed by its owner.
- **The console**: the whole administration, on its own port, with the
  capability split that decides who sees which half.
- **The agent endpoint**: MCP on the control plane, under the same rules a
  human gets.
- **The navigation portal**: one bar across the applications the gateway
  serves, injected into pages that ship no library for it.
- **Clustering**: several gateways behind one PostgreSQL, coordinating through
  the database rather than with each other.
- **Versioned configurations**: several coexist, one is active, export and
  import close the loop, a restore point on every change, and two saved
  configurations compared.
- **Open sessions**: your devices signed in, on your profile, with
  *Sign out everywhere else*. Administrators see and end sessions from the
  console.
- **OpenTelemetry export** (Enterprise): one collector address for traces,
  metrics, the audit trail and logs. See [traces](/docs/operations/tracing).
- **Scheduled calls**: the gateway calls a service on a cadence, a cron
  calendar or once, with retries and a run history. The service creates them
  through the API, from its own screens; the console watches and steps in. See
  [scheduled calls](/docs/operations/scheduler).
- **The gateway's own log**, live in the console, with its level turned up for
  half an hour from there. See [logs](/docs/operations/logs).

## Being finished

These work and are not finished. The table in the repository says, line by
line, what is missing from each.

- **Endpoint audit** (Enterprise) - one switch per operation sends its calls to
  the audit trail. What is missing is picking which body fields to keep, and
  auditing a refusal made before the operation is known.
- **Tracing** - the context crosses the gateway and the traces leave for your
  collector. What is missing is the gateway declaring itself in `tracestate`
  and `baggage`, and a link from an audit line to its trace.
- **Quotas** - they are posed per route, per endpoint and per consumer - user,
  token, organisation, address - and going over answers 429 with the standard
  headers. What is missing is the screen that shows consumption, throttling
  rather than refusing, and counters that stay right across a cluster.
- **Dev mode** - the tunnel works, sign-in halts on a page naming what is
  substituted and by whom, and a strip says it again while you work. What is
  missing is the scope of a substitution - today it holds for all traffic -
  then the console screen listing live sessions, and the audit of each
  substitution (a key deposited is already audited).
- **Notifications** - the SMTP relay is shipped, with one template in the
  theme's colours, and the daily summary of closing accesses goes out on its
  own. What is missing is a template per event, and translated.
- **Identity to upstreams** - the signed token is emitted, with its published
  JWKS and key rotation. What is missing is an exchange endpoint handing back
  an access and refresh pair, and the modes that carry a secret to the
  upstream: BASIC, FORM, third-party JWT.
- **Passkeys** - usable as a factor. Recovering an account whose only passkey
  is lost is not written yet.
- **One-time codes by email** - signing in with a code instead of the
  password is shipped, off by default, bound to the browser that asked and
  never open on the console. What is missing is the magic link, and being able
  to close that door account by account.

## Next

- **A discovery wizard**: the gateway can already read the Docker socket and a
  Kubernetes namespace. Turning that into "scan, pick a container, get a route"
  is the screen that is missing.
- **SAML**, for the enterprises whose identity provider does not speak OpenID
  Connect. It is registrable today and refuses at the factory, which is honest
  and not yet useful.
- **Sign in as**: what a support desk does every day, and which today is done
  by asking somebody for their password.
- **A response cache**, because the gateway is already the only place that sees
  the same request twice.
- **Serving an application under a sub-path without rebuilding it**: the prefix
  is stripped on the way in; what the application sends back is still to be
  rewritten.
- **gRPC properly**: per-method security, and gRPC-Web. The counters already
  read `grpc-status`.
- **A per-organisation portal**: each customer's own icon, title and
  arrangement. It would be the product's first per-tenant visual override.
- **HTTP/3**, once the gain can be measured rather than described.
- **An external vault**: HashiCorp Vault, Kubernetes secrets and Docker
  secrets, for installations that already run one and do not want a second.
- **Issues pushed to GitHub, GitLab or Jira**, rather than read in one more
  screen.
- **Web Push notifications**, for what an operator has to know without keeping
  a tab open.

## Not on the list

Kerberos and SPNEGO are written down as Enterprise, and nothing has been
started. A plugin system is not planned: the filter catalogue is curated on
purpose, and every filter in it is one we can explain and test.

> [!TIP]
> What the integration suite actually enforces is on the
> [test coverage](/project/tests) page - it is the file the suite executes, not
> a description of it.
