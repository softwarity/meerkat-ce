---
title: What it does
section: The product
order: 3
summary: Every capability of the gateway, domain by domain, with the state read from the code.
---

# What it does

Meerkat takes charge of what an internal or customer-facing application needs
and no team should write twice. What follows is the whole product, domain by
domain.

**Enterprise** marks what
the community image does not carry; *partly* marks a capability that is usable with a
named part still to ship. What the same list costs when you assemble it
yourself is on [the case for Meerkat](/product/the-case).

## Sign-in and identity

The pages your users see, served by the gateway, in your colours.

Read more: [How someone signs in](/docs/auth/overview).

- **Served sign-in pages**: sign-in, forgotten password, email verification, optional sign-up, in 20 languages, with your theme, logo and background.
- **Passwords** with a configurable policy and history, brute-force protection shared by every node, and messages that never reveal whether an account exists.
- **TOTP second factor** with QR code and recovery codes, email code fallback, trusted browsers.
- **MFA enforced** globally, per authority or per user. *partly*
- **WebAuthn passkeys**: security key, fingerprint, Windows Hello, as a first factor. *partly*
- **OIDC and GitHub federation**: Entra ID, Okta, Google, Keycloak or any compliant IdP, for the first factor.
- **LDAP and Active Directory**, and group rules: a directory group, a GitHub team or an OIDC claim becomes a membership and roles. **Enterprise**
- **Sign in by e-mail code**: a one-time code instead of the password, bound to the browser that asked, ten minutes, never on the console, and the second factor still runs behind it. Ships off. *partly*
- **API tokens**, personal and machine-to-machine, with the secret shown only once.
- **Accounts** with a validity window in days, custom fields for your domain passed to applications, and an email alert on sign-in from a new browser.

## Access and organisations

Who reaches what, decided at the door, never inside each application.

Read more: [Authenticating and authorising](/docs/access/overview).

- **Hierarchical roles** and per-organisation role groups, cumulative or picked at sign-in.
- **Customer organisations** isolated by the gateway, admin or user members, organisation picked at sign-in. Single-organisation mode is free, multi-organisation is Enterprise. **Enterprise**
- **Per-route access** on organisation, role and account; a refusal lands on a page that explains, never on a line of text.
- **Per-endpoint security** derived from the service's OpenAPI spec, edited in a Swagger-like console.
- **Sealed delegated administration**: super-admin, infrastructure admin, application admin, organisation admin, self-service.
- **Access time windows** per organisation, by day, date and time zone. **Enterprise** *partly*
- **Identity passed to your services** as headers, as REMOTE_USER or as a signed JWT (ES256, EdDSA, RS256) with a published JWKS and zero-downtime key rotation; forwarded roles are filtered by expression.

## Scheduled calls

The gateway calls your services at the hour you name, with no broker to install.

Read more: [Scheduled calls](/docs/operations/scheduler).

- **A schedule is a route, a path and a cadence** (or a cron calendar, read in a timezone): the service asks to be called, the gateway makes the call through its front door, so every rule set in front of that service applies.
- **Or one date, once**: a delayed action - the trigger is something that happened, not a calendar. The service posts the moment it wants to be called back at, and the schedule is finished when it has gone.
- **It runs as `meerkat`, carrying the roles the schedule asks for**: no service account to create or keep in step, and a schedule reaches exactly what its roles reach. The service manages them on the control plane with a token of its own, and finds its own by its own metadata.
- **At least once**, with a run identifier to deduplicate on: a call cut short by a gateway that stopped is sent again by another, same identifier, and an answer - a failure included - is never replayed. A **202** keeps the run open and the service reports its progress, which makes a three-hour job expressible without a three-hour request.
- **No tick, no lock**: the gateway sleeps until the next turn owed and wakes on the second; in a cluster the calls spread over the nodes, and a slow service holds only its own call.
- **A live console screen**, filtered by organisation, service and metadata: pause, bring forward, remove.
- **Every turn is kept**: when it ended, how, what answered, which node made the call - and a dropped or failed one is run again from the screen or the API.
- **Three attempts, then the next turn**: the handful of answers that, from an internal service, usually mean a moment too early - account not known yet, roles not loaded, nobody answering - are tried again, twice, and never past the schedule's catch-up. A 500 is not: that is the service saying something.
- **Or the service names its own moment**: `424` with a `Retry-After` - the extract is not published yet - and the turn comes back then, with the reason kept on the run that gave it.

## Routing and traffic protection

A complete API gateway, run from the console.

Read more: [Routes](/docs/concepts/routes), and what a request costs in
[performance](/product/performance).

- **Routes changed live**, with no restart, propagated to every node within a second.
- **11 predicates and 33 filters**: path, host, header, cookie, method, weight for canaries, time window; request and response rewriting.
- **Rate limiting** per route, user, token, organisation or address, several limits at once; **per-endpoint quotas**; standard 429 response.
- **Circuit breaker, timeouts** at three levels, and service health in the console: a heart per route, from discovery or a TCP check, and real traffic.
- **WebSocket, gRPC and body streaming** end to end; a gRPC call is counted by its `grpc-status`, not by the 200 it rides on. *partly*
- **Service discovery** for Docker, Swarm and Kubernetes when creating a route. *partly*
- **Routing tester**: compose a sample request and see which route takes it, and why.
- **Maintenance page** per route or for the whole platform in one move, translated, with a door for administrators.

## Inside your applications, untouched

What the gateway adds to the pages it serves.

Read more: [What the gateway injects](/docs/concepts/data-plane-chrome).

- **Injected user button**: profile, sign-out, organisation switch, language, light or dark.
- **Navigation portal** across your applications, as a bar or a rail, showing only what access rights allow.
- **Hide UI by role** in pure CSS: roles are stamped on the page server-side. CSS and JavaScript injectable per route.
- **Issue reporting**: screenshot, console, technical context, tracked in the admin console.
- **Real-time channel** over WebSocket to applications, with no prior integration.
- **Nothing personal in caches**: any page carrying an identity is made non-storable, whatever the application says.

## Platform security

The building blocks usually installed alongside.

Read more: [The vault](/docs/operations/vault).

- **TLS certificates** per name, issued and renewed through ACME with Let's Encrypt or your internal authority, HTTPS ports opened live.
- **Built-in vault**: secrets encrypted with AES-256-GCM, referenced by name from the configuration, encrypted export, reminder before expiry. The same key seals TLS keys and TOTP secrets, and rotates with a restart.
- **Security headers** HSTS, CSP, X-Frame-Options, Referrer-Policy, and CSRF protection for the console.
- **Console on a separate port** from application traffic: administration is never exposed alongside the application.
- **Runs without internet**: no resource loaded from outside, suited to air-gapped environments.

## Operations

A console that replaces YAML files and configuration pipelines.

Read more: [Operations](/docs/operations/overview).

- **Versioned configurations**: several named versions, one active, comparison, YAML export and import, automatic restore point on every change.
- **Configurations in a git repository**: one directory per platform in a shared repository; a pull files the configuration and applies nothing until you activate it, a push commits it under the name of the operator who clicked. GitHub, GitLab, Bitbucket, Azure DevOps, Gitea or your own server, with a token kept in the vault. **Enterprise**
- **Audit log** of every administrative action, with a field-by-field diff, and of the security of the accounts - every sign-in, every refused one with its real reason and address, every factor, passkey, password or token changed by its owner. Append-only, browsable in the console. *partly*
- **Audit sent to the collector** over OTLP: the data plane's security events, and the console's changes too. **Enterprise**
- **Endpoint audit**: one switch per operation of a route's OpenAPI spec, and each call becomes an audit event. **Enterprise** *partly*
- **CSV export** of the audit trail. Parquet is still to ship. **Enterprise** *partly*
- **Built-in dashboards**: traffic, latency and failures per route and per endpoint, with nothing to install.
- **Metrics pushed over OTLP** to the collector the traces go to, which writes them into Prometheus - with a ready-made Grafana dashboard. **Enterprise**
- **Structured logs**, JSON or text, and an **access log** - one line per request crossing the front door, refusals included, carrying the account as the gateway itself authenticated it, and the API token when a machine called. That is the half of an audit no service can write: it never saw the call it was refused, and all it knows of the caller is what it was told. Both logs can be written as OpenTelemetry JSON for a Collector agent, or pushed to the collector (**Enterprise**). The console shows them live, and sets the level on every node.
- **Distributed tracing** (W3C Trace Context): the context travels in every edition and an identifier is put on every request - handed back to the caller, written in the log, at the foot of the built-in pages - which **joins a line of the gateway's to a service's own business audit**. In Enterprise the gateway puts itself on the trace: its inbound span, the span of the upstream call, and the gap between the two, which is its own time. **OTLP** export to an OpenTelemetry Collector or any OTLP endpoint. The OpenTelemetry bundle can be injected into UI pages - served by Meerkat, never by a CDN - so the trace starts at the click. **Enterprise** *partly*
- **Active/active cluster** on PostgreSQL, with no session affinity and no primary node. **Enterprise**
- **Transactional emails** in your theme colours, and a daily digest of expiring accounts.
- **Deployment**: one image, embedded storage by default, Docker, Swarm or Kubernetes with a Helm chart, liveness and readiness probes, seeding from a file.

## For your developers

Test against the real cluster without deploying anything.

Read more: [Dev mode](/product/dev-mode).

- **Workstation to cluster** with **softwarity/plug**: the service running on a developer's machine joins the cluster under its name, replaces the deployed service for the session, then the cluster is restored exactly as it was. Any language, no code change, from Linux, macOS or Windows. `plug`
- **Standalone plug, with Community**: an agent container added to your Docker, Swarm or Kubernetes stack, free under the FSL license. `plug`
- **Built-in plug, with Enterprise**: the agent lives inside the gateway, with nothing to deploy alongside; each developer authenticates with their SSH key, and every page flags a service served from a workstation. **Enterprise** *partly*
- **UI test mode**: browse with a simulated identity to see exactly what a role sees.
- **Embedded Swagger UI** on the OpenAPI specs of **every** route, with no CDN: the calls go through the gateway, so through the route's authentication and rules, and the identity behind *Try it out* is **simulated** - a user, groups or roles - to see what the API answers each of them.

## Driven by an AI agent

The operator asks in words, the gateway executes and records.

Read more: [The agent endpoint](/docs/agent/overview).

- **Built-in MCP server**: Claude Code, Gemini CLI and Codex CLI connect to the gateway to read, test or change routes.
- **OAuth connection with no copied secret**, scoped token (read-only or full, domain, network ranges), revocable in one click.
- **Every agent action is audited** under its name, with an automatic restore point to roll back.

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

## Four editions

Community, the free one, is the whole gateway for one organisation on one
instance. Enterprise is what an installation needs once it grows: several
organisations, your corporate directory, several gateways behind one entry
point. Team is Enterprise for a cluster of a known size, and the evaluation
edition is Enterprise with a notice on it, free, to try all of it first.

The rule for what falls on each side is one line, and it is the one to check
when you compare: **never a security primitive**. TLS, the vault, two-factor,
passkeys, the audit trail and endpoint security are in the free image and
always will be.

[Compare the editions](/product/editions), or go straight to
[pricing](/pricing/index).
