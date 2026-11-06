---
title: Infrastructure administrator
section: Access control
order: 141
summary: What the infra admin capability opens in the console - the routing plane, its certificates and authorities - and what stays out of reach.
---

# Infrastructure administrator

An account carrying the **infra admin** capability runs the **routing plane**:
what the gateway answers, for which addresses, behind which certificates, signed
in through which authorities. It does not decide who the people are or what the
pages look like - that is the [application administrator](/docs/access/app-admin).

The capability is a flag on the account, set by an application administrator or
root in **Application > Users**. See [Administration capabilities](/docs/access/capabilities).

![The console of an infrastructure administrator: Infra, Vault, Data plane and Meerkat in the rail, the Infra menu open on Routes](img/console/capability-infra-admin.webp)

## What you see

The console opens on **Infra > Routes**. The rail holds four entries:
**Infra**, **Vault**, **Data plane** and **Meerkat**.

| Section | Screens |
|---|---|
| Infra | Topology, Routes, Endpoint security, Endpoint rate limits, Endpoint audit, Authentication, Mail relay, Domains & TLS, Access tokens, MCP, Model, OpenTelemetry, Plug |
| Vault | the entries of the routing plane |
| Data plane | Metrics, Issues |
| Meerkat | Audit, Logs, API reference, Release notes, License |

## What you do there

- **[Topology](/docs/console/topology)**: the instances serving the installation,
  the database they share, the services behind the routes and whether each answers.
- **[Routes](/docs/console/routes)**: the routing table and the route editor -
  where a request goes, who may reach it, the filters on the way, the routing tester.
- **[Endpoint security, rate limits and audit](/docs/console/endpoints)**: who may
  call each operation of an API, how much it may carry, and what was called.
- **[Authentication](/docs/console/authentication)**: the authorities people sign
  in through - the local accounts, OIDC, LDAP and Active Directory, GitHub.
- **[Mail relay](/docs/console/mail-relay)**: the SMTP server the account mails go through.
- **[Domains & TLS](/docs/console/tls)**: the public names and their certificates,
  on each plane.
- **[Access tokens and MCP](/docs/console/access-and-agents)**: a token for a
  script, a connection for an assistant - within your own capability.
- **Model**: the extra fields a tenant carries. **OpenTelemetry**: where traces go.
  **Plug**: opening the developer tunnel.
- **[Vault](/docs/console/vault)**: the secrets and values the routing plane refers to.
- **[Metrics](/docs/console/traffic)** and **[Issues](/docs/console/audit-and-issues)**:
  what passed through the gateway, and what users reported.
- **Logs** and **Audit**: what the gateway says about itself, and what was
  changed on your plane.

## What is not yours

| You do not see | It belongs to |
|---|---|
| Users, roles, the served pages, the portal, the security policies | the [application administrator](/docs/access/app-admin) |
| A tenant's groups, members and rules | the [tenant's administrators](/docs/access/tenant-admin) |
| Sessions, the scheduler, notifications | the application administrator |
| Configuration (export, import, backup) and the console's own sessions | root |

A screen you may not open sends you back to Routes, even typed into the address
bar, and the admin API answers `403` on its calls: hiding a link is comfort, the
API is the rule.
