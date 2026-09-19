---
title: What it does
section: The product
order: 2
summary: Every area of the gateway, what it takes off your services, and how far each one is built today.
---

# What it does

Meerkat takes charge of what an internal application needs and no team should
write twice. The areas below are the whole product. Nothing here is a
projection: the repository carries one table,
[FEATURES.md](https://github.com/softwarity/meerkat/blob/main/FEATURES.md), one
line per feature, its state read from the code, and shipping something means
ticking its box in the same commit.

## The areas

| Area | What Meerkat takes on | Where it stands |
| --- | --- | --- |
| [Authentication](/docs/auth/overview) | Sign-in pages served by the gateway itself, local accounts, and enterprise authorities - OpenID Connect, LDAP and Active Directory, GitHub - used for authentication only. | Shipped, tested against real servers. SAML and Kerberos are not written. |
| [Two-factor](/docs/auth/mfa) | TOTP with trusted browsers, and [passkeys](/docs/auth/passkeys) as a first-class factor rather than a bolt-on. | TOTP shipped; passkeys usable, recovery still to write. |
| [Authorisation](/docs/access/overview) | A hierarchical role catalogue, groups per organisation, a rule per route, and [per-endpoint security](/docs/access/endpoint-security) read from a service's OpenAPI description. | Shipped. |
| [Organisations](/docs/access/tenants) | Several tenants in one installation: members, group modes, owner, selection at sign-in, per-tenant session policy. | Shipped. Enterprise. |
| [Routing](/docs/concepts/routes) | Eleven [predicates](/docs/predicates/overview) and thirty-three [filters](/docs/filters/overview), edited hot, applied without a restart. | Shipped. Versioned configurations are half-built. |
| [Injected chrome](/docs/concepts/data-plane-chrome) | The account button, the [navigation portal](/docs/customise/portal), the colour scheme and the role-based CSS are added to your pages by the gateway, whatever the application is written in. | Shipped. |
| [The vault](/docs/operations/vault) | Secrets sealed at rest and plain values, both referenced by name, so a configuration can be exported without exporting what it hides. | Shipped. |
| [TLS](/docs/operations/tls) | Certificates managed by the gateway, ACME issuance included, serialised so a cluster asks once. | Shipped. |
| [Identity to upstreams](/docs/concepts/identity) | A short-lived signed JWT carrying who is calling, their roles and their organisation, so your service reads a header instead of authenticating. | Signing shipped; the exchange endpoint is not written. |
| [Quotas and rate limits](/docs/operations/rate-limits) | Limits per route and per caller, with standard 429 semantics and a log-only calibration mode. | Rate limits shipped; quotas per consumer are being built. |
| [Audit](/docs/operations/audit) | Every administrative change recorded with its author and a field-level diff, readable per domain, secrets redacted. | Shipped. |
| [Traffic and health](/docs/operations/traffic) | Traffic figures, upstream health and anomalies in the console. No Prometheus, no Grafana, no YAML. | Shipped, with more to show. |
| [Driven by an agent](/docs/agent/overview) | An MCP endpoint on the control plane, so an assistant can read and change the installation through the same rules a human gets. | Shipped. |
| [Dev mode](/product/dev-mode) | A developer's workstation stands in for a deployed service, and everyone is told which one and by whom. | The tunnel works; what shows it to users is being built. |
| Deployment | One binary, embedded storage, [one gateway](/docs/deploy/one-gateway) or a [cluster](/docs/deploy/kubernetes) behind one PostgreSQL. | Shipped. |

## What it refuses to be

Meerkat is an **app-gateway**, not an API gateway. It exists to serve one
application made of many services, not to expose APIs to third parties. That
one decision is why identity, roles, organisations and the login pages are in
the product rather than beside it, and why the console is an operator's tool
rather than a YAML editor.

> [!NOTE]
> The anti-pattern it exists to break: install the gateway, then Prometheus,
> then Grafana, then write YAML for everything. Here you launch one binary,
> configure it in the console, export, and replay the export anywhere.

## Two editions

The free one is the whole gateway for one organisation on one instance.
Enterprise is what an installation needs once it grows: several organisations,
your corporate directory, several gateways behind one entry point.

The rule for what falls on each side is one line, and it is the one to check
when you compare: **never a security primitive**. TLS, the vault, two-factor,
passkeys, the audit trail and endpoint security are in the free image and
always will be.

[Compare the editions](/product/editions), or go straight to
[pricing](/product/pricing).
