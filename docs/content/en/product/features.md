---
title: What it does
section: The product
order: 3
summary: Every capability of the gateway, domain by domain, with the state read from the code and the identifier you can check it against.
---

# What it does

Meerkat takes charge of what an internal or customer-facing application needs
and no team should write twice. What follows is the whole product, domain by
domain.

Each line names its identifier in
[FEATURES.md](https://github.com/softwarity/meerkat-ce/blob/main/FEATURES.md), the
one table in the repository whose state is read from the code rather than from
a plan - so every claim here is checkable against it. **Enterprise** marks what
only the paid image carries; *partly* marks a capability that is usable with a
named part still to ship. What the same list costs when you assemble it
yourself is on [the case for Meerkat](/product/the-case).

## Sign-in and identity

The pages your users see, served by the gateway, in your colors.

Read more: [How someone signs in](/docs/auth/overview).

- **Served sign-in pages**: sign-in, forgotten password, email verification, optional sign-up, in 20 languages, with your theme, logo and background. `AUTH-01 THEME-01`
- **Passwords** with a configurable policy and history, brute-force protection shared by every node, and messages that never reveal whether an account exists. `AUTH-10 AUTH-11`
- **TOTP second factor** with QR code and recovery codes, email code fallback, trusted browsers, MFA enforced globally, per authority or per user. `MFA-01 to 04`
- **WebAuthn passkeys**: security key, fingerprint, Windows Hello, as a first factor. *partly* `AUTH-15`
- **OIDC and GitHub federation**: Entra ID, Okta, Google, Keycloak or any compliant IdP, for the first factor. `AUTH-04`
- **LDAP and Active Directory**, and group rules: a directory group, a GitHub team or an OIDC claim becomes a membership and roles. **Enterprise** `AUTH-03 RBAC-10`
- **Sign in by e-mail code**: a one-time code instead of the password, bound to the browser that asked, ten minutes, never on the console, and the second factor still runs behind it. Ships off. *partly* `AUTH-16`
- **API tokens**, personal and machine-to-machine, with the secret shown only once. *partly* `AUTH-09`
- **Accounts** with a validity window in days, custom fields for your domain passed to applications, and an email alert on sign-in from a new browser. `MODEL-01 MODEL-02 AUTH-23`

## Access and organizations

Who reaches what, decided at the door, never inside each application.

Read more: [Authenticating and authorising](/docs/access/overview).

- **Hierarchical roles** and per-organization role groups, cumulative or picked at sign-in. `RBAC-01 to 03`
- **Customer organizations** isolated by the gateway, admin or user members, organization picked at sign-in. Single-organization mode is free, multi-organization is Enterprise. **Enterprise** `TENANT-01 to 08`
- **Per-route access** on organization, role and account; a refusal lands on a page that explains, never on a line of text. `RBAC-06`
- **Per-endpoint security** derived from the service's OpenAPI spec, edited in a Swagger-like console. `RBAC-07 CONSOLE-04`
- **Sealed delegated administration**: super-admin, infrastructure admin, application admin, organization admin, self-service. `RBAC-04 RBAC-05`
- **Access time windows** per organization, by day, date and time zone. **Enterprise** *partly* `TENANT-04`
- **Identity passed to your services** as headers, as REMOTE_USER or as a signed JWT (ES256, EdDSA, RS256) with a published JWKS and zero-downtime key rotation; forwarded roles are filtered by expression. `SAUTH-01 AUTH-07 ROUTE-18`

## Routing and traffic protection

A complete API gateway, run from the console.

Read more: [Routes](/docs/concepts/routes), and what a request costs in
[performance](/product/performance).

- **Routes changed live**, with no restart, propagated to every node within a second. `ROUTE-01`
- **12 predicates and 32 filters**: path, host, header, cookie, method, weight for canaries, time window; request and response rewriting. `ROUTE-03 to 05`
- **Rate limiting** per route, user, token, organization or address, several limits at once; **per-endpoint quotas**; standard 429 response. `ROUTE-08 QUOTA-05`
- **Circuit breaker, timeouts** at three levels, and service health in the console, observed on real traffic. `ROUTE-07 ROUTE-09 SVC-04`
- **WebSocket, gRPC and body streaming** end to end. *partly* `ROUTE-13 ROUTE-20`
- **Service discovery** for Docker, Swarm and Kubernetes when creating a route. *partly* `SVC-02`
- **Routing tester**: compose a sample request and see which route takes it, and why. `ROUTE-15`
- **Maintenance page** per route or for the whole platform in one move, translated, with a door for administrators. `LIFE-05`

## Inside your applications, untouched

What the gateway adds to the pages it serves.

Read more: [What the gateway injects](/docs/concepts/data-plane-chrome).

- **Injected user button**: profile, sign-out, organization switch, language, light or dark. `UIF-03 UIF-05`
- **Navigation portal** across your applications, as a bar or a rail, showing only what access rights allow. *partly* `PORTAL-01`
- **Hide UI by role** in pure CSS: roles are stamped on the page server-side. CSS and JavaScript injectable per route. `UIF-02 UIF-06`
- **Issue reporting**: screenshot, console, technical context, tracked in the admin console. `ISSUE-01 to 04`
- **Real-time channel** over WebSocket to applications, with no prior integration. `UIF-04`
- **Nothing personal in caches**: any page carrying an identity is made non-storable, whatever the application says. `SEC-10`

## Platform security

The building blocks usually installed alongside.

Read more: [The vault](/docs/operations/vault).

- **TLS certificates** per name, issued and renewed through ACME with Let's Encrypt or your internal authority, HTTPS ports opened live. `SSL-01 SSL-05 SSL-08`
- **Built-in vault**: secrets encrypted with AES-256-GCM, referenced by name from the configuration, encrypted export, reminder before expiry. `VAULT-01 to 06`
- **Security headers** HSTS, CSP, X-Frame-Options, Referrer-Policy, and CSRF protection for the console. `SEC-01 SEC-03`
- **Console on a separate port** from application traffic: administration is never exposed alongside the application. `CONSOLE-11`
- **Runs without internet**: no resource loaded from outside, suited to air-gapped environments. `DEPLOY-03`

## Operations

A console that replaces YAML files and configuration pipelines.

Read more: [Operations](/docs/operations/overview).

- **Versioned configurations**: several named versions, one active, comparison, YAML export and import, automatic restore point on every change. `CFG-01 to 06`
- **Audit log** of every administrative action, with a field-by-field diff, append-only, browsable in the console. `AUD-01 AUD-02`
- **Built-in dashboards**: traffic, latency and failures per route and per endpoint, with nothing to install. `OBS-01`
- **Prometheus export** with a ready-made Grafana dashboard and files ready for Swarm and Kubernetes. **Enterprise** `OBS-05`
- **Active/active cluster** on PostgreSQL, with no session affinity and no primary node. **Enterprise** `PERF-03 STORE-03`
- **Transactional emails** in your theme colors, and a daily digest of expiring accounts. `NOTIF-01 NOTIF-04`
- **Deployment**: one image, embedded storage by default, Docker, Swarm or Kubernetes with a Helm chart, liveness and readiness probes, seeding from a file. `DEPLOY-01 OBS-02 LIFE-02`

## For your developers

Test against the real cluster without deploying anything.

Read more: [Dev mode](/product/dev-mode).

- **Workstation to cluster** with **softwarity/plug**: the service running on a developer's machine joins the cluster under its name, replaces the deployed service for the session, then the cluster is restored exactly as it was. Any language, no code change, from Linux, macOS or Windows. `plug`
- **Standalone plug, with Community**: an agent container added to your Docker, Swarm or Kubernetes stack, free under the FSL license. `plug`
- **Built-in plug, with Enterprise**: the agent lives inside the gateway, with nothing to deploy alongside; each developer authenticates with their SSH key, and every page flags a service served from a workstation. **Enterprise** *partly* `DEV-02 DEV-03 DEV-04`
- **UI test mode**: browse with a simulated identity to see exactly what a role sees. `DEV-10`
- **Embedded Swagger UI** on the routes' OpenAPI specs, with no CDN. `DEV-09 LIFE-04`

## Driven by an AI agent

The operator asks in words, the gateway executes and records.

Read more: [The agent endpoint](/docs/agent/overview).

- **Built-in MCP server**: Claude Code, Gemini CLI and Codex CLI connect to the gateway to read, test or change routes. `MCP-01 MCP-04`
- **OAuth connection with no copied secret**, scoped token (read-only or full, domain, network ranges), revocable in one click. `MCP-02 MCP-07`
- **Every agent action is audited** under its name, with an automatic restore point to roll back. `MCP-03 MCP-05`

## An app-gateway, and what follows from it

Meerkat serves **one** application made of many services: users who have a
name, roles, an organisation, and a single door in front of all of it. That
decision explains the rest - why identity, roles, organisations and the sign-in
pages are in the product rather than beside it, and why the console is an
operator's tool rather than a YAML editor.

If what you need is to expose APIs to third parties - a developer portal, a key
per partner, billing per call - that is an API gateway's job, and saying so
here saves you time.

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
