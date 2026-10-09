---
title: What it does
section: The product
order: 3
summary: Every capability of the gateway, domain by domain, with the state read from the code.
---

# What it does

Meerkat takes care of what every application needs and no team should write
twice. Here is the whole product, domain by domain.

**Enterprise** marks what the free Community image does not include. *partly*
marks a capability you can use today, with a named part still to come. What the
same list costs when you assemble it yourself is in
[the case for Meerkat](/product/the-case).

## Sign-in and identity

The pages your users see, served by the gateway, in your colours.

Read more: [How someone signs in](/docs/auth/overview).

- **Served sign-in pages**: sign-in, forgotten password, email verification, optional sign-up, in 20 languages, with your theme, logo and background. Any translation can be corrected, or a language added, from the console.
- **More page arrangements** - split, drawer, banner, bare - and the Meerkat mark removed from the pages you serve. **Enterprise**
- **Themes made the way Material Theme Builder makes them**: six source colours, three contrast levels, its JSON file in and out, and the fonts served by the gateway itself - nothing fetched from a CDN.
- **Passwords** with a configurable policy and history, brute-force protection shared by every node, and messages that never reveal whether an account exists.
- **TOTP second factor** with QR code and recovery codes, email code fallback, trusted browsers.
- **MFA enforced** globally, per authority or per user. *partly*
- **WebAuthn passkeys**: security key, fingerprint, Windows Hello, as a first factor. *partly*
- **OIDC and GitHub federation**: Entra ID, Okta, Google, Keycloak or any compliant IdP, for the first factor.
- **SAML 2.0** for directories that offer nothing else (ADFS, Entra ID, Okta, Shibboleth): signed assertions, a replayed one refused on every node. **Enterprise**
- **LDAP and Active Directory**, and group rules: a directory group, a GitHub team or an OIDC claim becomes a membership and roles. **Enterprise**
- **Sign in by e-mail code**: a one-time code instead of the password, valid ten minutes in the browser that asked for it, with the second factor still applied. Off until you turn it on. *partly*
- **API tokens**, personal and machine-to-machine, with the secret shown only once.
- **Accounts** with a validity window in days, custom fields for your domain passed to applications, and an email alert on sign-in from a new browser.

## Access and organisations

Who reaches what, decided at the door, never inside each application.

Read more: [Authenticating and authorising](/docs/access/overview).

- **Hierarchical roles** and per-organisation role groups, cumulative or picked at sign-in.
- **Customer organisations** isolated by the gateway, admin or user members, organisation picked at sign-in. Single-organisation mode is free, multi-organisation is Enterprise. **Enterprise**
- **Per-route access** on organisation, role and account; a refusal lands on a page that explains, never on a line of text.
- **Per-endpoint security** derived from the service's OpenAPI spec, edited in a Swagger-like console.
- **Delegated administration with clear boundaries**: super-admin, infrastructure admin, application admin, organisation admin, and self-service for each user.
- **Access time windows** per organisation, by day, date and time zone. **Enterprise** *partly*
- **Identity passed to your services** as headers, as REMOTE_USER or as a signed JWT (ES256, EdDSA, RS256), with published keys that rotate without downtime; you choose which roles each service receives.

## Scheduled calls

The gateway calls your services at the hour you name, with no broker to install.

Read more: [Scheduled calls](/docs/operations/scheduler).

- **A schedule is a route, a path and a cadence**: an interval, a cron calendar in a time zone, or a single date for a delayed action. The call goes through the front door, so every rule in front of the service applies to it.
- **No service account to create**: the call carries the roles the schedule asks for, and reaches exactly what those roles reach. A service can create and manage its own schedules through the API.
- **Reliable**: every call is made at least once, with an identifier to spot duplicates, even if a gateway stops mid-call. A long job reports its progress instead of holding a request open for hours.
- **Patient with a service that is not ready**: the answers that usually mean "a moment too early" are retried, and a service can say when to call back.
- **Nothing to install**: no broker, no lock; in a cluster the calls spread over the nodes.
- **A live console screen**: pause, run now, remove, and the history of every run - when, how it ended, what answered - with a failed one run again in one click.

## Routing and traffic protection

A complete API gateway, run from the console.

Read more: [Routes](/docs/concepts/routes), and what a request costs in
[performance](/product/performance).

- **Routes changed live**, with no restart, propagated to every node within a second.
- **11 predicates and 34 filters**: path, host, header, cookie, method, weight for canaries, time window; request and response rewriting.
- **Rate limiting** per route, user, token, organisation or address, several limits at once; **per-endpoint quotas**; standard 429 response. In a cluster, each node counts on its own for now. *partly*
- **Circuit breaker, timeouts** at three levels, and service health in the console: a heart per route, from discovery or a TCP check, and real traffic.
- **WebSocket, gRPC and body streaming** end to end; a gRPC call is counted by its `grpc-status`, not by the 200 it rides on. *partly*
- **Service discovery** for Docker, Swarm and Kubernetes when creating a route. *partly*
- **Routing tester**: compose a sample request and see which route takes it, and why.
- **Maintenance page** per route or for the whole platform in one move, translated, with a door for administrators.
- **Answers from a template**: a route can answer by itself with content built from the signed-in user, in the format an application expects - its identity contract configured rather than coded.
- **Files served by a route**: upload a font, a stylesheet, a script or an image and the route answers it under its path, with its type, ETag and CORS - for the UI that needs a resource nothing behind the gateway serves, offline above all.

## Inside your applications, untouched

What the gateway adds to the pages it serves.

Read more: [What the gateway injects](/docs/concepts/data-plane-chrome).

- **Injected user button**: profile, sign-out, organisation switch, language, light or dark.
- **Navigation portal** across your applications, as a bar or a rail, showing only what access rights allow.
- **Hide UI by role** in pure CSS: roles are stamped on the page server-side.
- **Your own CSS and JavaScript per route**, written in the console or uploaded as files, placed at the start or end of the head or at the end of the body, in the order you set.
- **Issue reporting**: screenshot, console, technical context, tracked in the admin console.
- **Real-time channel** over WebSocket to applications, with no prior integration.
- **Nothing personal in caches**: any page carrying an identity is made non-storable, whatever the application says.

## Platform security

The building blocks usually installed alongside.

Read more: [The vault](/docs/operations/vault).

- **TLS certificates** in one pool - generated, imported or signed on request, each with its names, a wildcard or an IP - placed by drag and drop on the console or the applications, HTTPS ports opened live.
- **Automatic certificates** through ACME: Let's Encrypt, ZeroSSL, Google or your own step-ca issue and renew them, several authorities side by side. **Enterprise**
- **Built-in vault**: secrets encrypted with AES-256-GCM, referenced by name from the configuration, encrypted export, reminder before expiry. The same key seals TLS keys and TOTP secrets, and rotates with a restart.
- **HTTPS redirect and security headers**: plain HTTP sent to HTTPS, HSTS, CSP, X-Frame-Options, Referrer-Policy, and CSRF protection for the console.
- **Console on a separate port** from application traffic: administration is never exposed alongside the application.
- **Runs without internet**: no resource loaded from outside, suited to air-gapped environments.

## Operations

A console that replaces YAML files and configuration pipelines.

Read more: [Operations](/docs/operations/overview).

- **Versioned configurations**: named versions, one active, comparison, YAML export and import, automatic restore point on every change. Three at a time in Community, as many as you like in **Enterprise**.
- **No lost update**: two administrators editing the same thing at once are told, instead of one silently erasing the other.
- **Live console**: a change made by one administrator shows on the others' screens without a reload. *partly*
- **Configurations in a git repository**: one directory per platform in a shared repository; a pull files the configuration and applies nothing until you activate it, a push commits it under the name of the operator who clicked. GitHub, GitLab, Bitbucket, Azure DevOps, Gitea or your own server, with a token kept in the vault. **Enterprise**
- **Audit log** of every administrative action, with a field-by-field diff, and of the security of the accounts - every sign-in, every refused one with its real reason and address, every factor, passkey, password or token changed by its owner. Append-only, browsable in the console, kept from three months to five years as you choose. *partly*
- **Audit sent to the collector** over OTLP: the data plane's security events, and the console's changes too. **Enterprise**
- **Endpoint audit**: one switch per operation of a route's OpenAPI spec, and each call becomes an audit event. **Enterprise** *partly*
- **CSV export** of the audit trail. Parquet is still to ship. **Enterprise** *partly*
- **Built-in dashboards**: traffic, latency and failures per route and per endpoint, with nothing to install.
- **Metrics pushed over OTLP** to the collector the traces go to, which writes them into Prometheus - with a ready-made Grafana dashboard. **Enterprise**
- **Structured logs**, JSON or text, and an **access log**: one line per request, refusals included, with the account the gateway authenticated and the API token when a machine called. It is the half of an audit no service can write, since a service never sees the calls refused before it. Both logs are written as OpenTelemetry JSON for your agent, or pushed to the collector (**Enterprise**). The console shows them live and sets the log level on every node.
- **Distributed tracing** (W3C Trace Context): in every edition, each request gets an identifier - returned to the caller, written in the log, shown at the foot of the built-in pages - that **links a gateway log line to your service's own records**. In Enterprise the gateway also appears on the trace, with its own time measured, exports over **OTLP** to any OpenTelemetry collector, and can start the trace in the browser, at the click. **Enterprise** *partly*
- **Active/active cluster** on PostgreSQL, with no session affinity and no primary node. **Enterprise**
- **Transactional emails** in your theme colours, and a daily digest of expiring accounts.
- **Deployment**: one image, embedded storage by default, Docker, Swarm or Kubernetes with a Helm chart, liveness and readiness probes, seeding from a file.
- **Signed images**: each image is signed with cosign at build time, so you can check where it came from. *partly*
- **Database snapshot and move**: download a consistent copy of the database from the console; moving it into a PostgreSQL server, to go to a cluster, is **Enterprise**.

## For your developers

Test against the real cluster without deploying anything.

Read more: [Dev mode](/product/dev-mode).

- **Workstation to cluster** with **softwarity/plug**: the service running on a developer's machine takes the place of the deployed one for the session, then the cluster goes back exactly as it was. Any language, no code change, from Linux, macOS or Windows. `plug`
- **Standalone plug, with Community**: plug is free (FSL license) and runs on its own, as an agent container in your Docker, Swarm or Kubernetes stack, beside the gateway. Meerkat knows nothing of it: no developer keys, no names, no announcement. `plug`
- **plug integrated into Meerkat, with Enterprise**: the tunnel lives inside the gateway, with nothing to deploy alongside; each developer authenticates with their SSH key, and every page flags a service served from a workstation. **Enterprise** *partly*
- **UI test mode**: browse with a simulated identity to see exactly what a role sees.
- **Built-in Swagger UI** for the API of **every** route: calls go through the gateway and its rules, and *Try it out* can act as any user, group or role, to see what the API answers each of them.

## Driven by an AI agent

The operator asks in words, the gateway executes and records.

Read more: [The agent endpoint](/docs/agent/overview).

- **Built-in MCP server**: Claude Code, Gemini CLI, Kimi CLI and Codex CLI connect to the gateway to read, test or change routes, roles and schedules, import a whole configuration, and call your services through a route as any role.
- **OAuth connection with no copied secret**, scoped token (schedules only, read-only or full, and the network ranges it may come from), revocable in one click.
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
