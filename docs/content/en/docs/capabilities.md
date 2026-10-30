---
title: What Meerkat does
section: Getting started
order: 1
summary: Every capability in broad lines, what it takes without Meerkat, the edition it comes in, and what is planned next.
---

# What Meerkat does

Everything the gateway does, in broad lines, beside what the same result takes
without it: the code to write, the service to run, the port to open. Each line
leads to its page.

**Community** is the free image. **Enterprise** is the paid one; the evaluation
image is Enterprise, free, to try all of it first ([the editions](/product/editions)).
*Partly* marks a capability you can use today with a named part still to come.
What the assembled alternatives cost is in [the case for Meerkat](/product/the-case).

## Your APIs, driven by an agent

| Capability | What Meerkat does | Without Meerkat | Edition |
|---|---|---|---|
| [Your services' APIs through MCP](/docs/agent/overview#calling-the-services-behind-the-routes) | Any assistant connected to the gateway reads the OpenAPI of the service behind a route, then calls it for real through the gateway, its rules and the identity it forwards, as any user or role. Every call is audited. | An MCP server inside **each** service, its own authentication and token handling, and its port exposed on the cluster - per service, per language. | Community |
| [The gateway itself through MCP](/docs/agent/overview) | Everything the console does, done by an assistant through the same endpoints and under the rules a person gets: routes, roles, organisations, themes, settings, OIDC clients, whole configurations and their history. A read-only token stays read-only, a restore point comes before every change, and the few things closed to agents - private keys, vault values, TLS doors - say why. | Community MCP servers for each tool, or a vendor's hosted one, and no common audit. | Community |
| [Agent connection](/docs/console/access-and-agents) | OAuth with no secret to copy, or a token scoped to a perimeter and network ranges, revoked in one click. | A credential per tool, stored in each assistant's configuration. | Community |

## Sign-in and identity

The ways in, all served by the gateway in your colours, in 21 languages.

| Method | Edition |
|---|---|
| [Local accounts and password policy](/docs/auth/local-accounts) | Community |
| [Two-factor: authenticator app, recovery codes, trusted browsers](/docs/auth/mfa) | Community |
| [Passkeys](/docs/auth/passkeys), as a first factor | Community, *partly* |
| [Sign in by e-mail code](/docs/auth/email-code) | Community, *partly* |
| [OpenID Connect](/docs/auth/oidc): Entra ID, Okta, Google, Keycloak, any compliant provider | Community |
| [GitHub](/docs/auth/github) | Community |
| [LDAP and Active Directory](/docs/auth/ldap) | Enterprise |
| [SAML 2.0](/docs/auth/saml) | Enterprise |
| Group rules: a directory group, a GitHub team or an OIDC claim becomes a membership and roles | Enterprise |
| [Meerkat as the OpenID provider of your applications](/docs/auth/oidc-provider) | Enterprise |
| Kerberos and SPNEGO | planned, Enterprise |

| Capability | What Meerkat does | Without Meerkat | Edition |
|---|---|---|---|
| [The sign-in pages](/docs/auth/flow-pages) | Sign-in, forgotten password, e-mail verification, optional sign-up, every factor above, served in your theme and translated. | An identity server - Keycloak, Zitadel, Authentik - or a service - Auth0, WorkOS, Clerk - plus a theme to build and keep up. | Community |
| [Sessions](/docs/auth/sessions) | One session for every application behind the gateway, revoked from the console. | Each application handles its own, or an OIDC client in each one. | Community |
| [API tokens](/docs/auth/api-tokens) | Personal and machine tokens, secret shown once, revocable. | A token system in each service, or an identity server's client credentials. | Community |
| [Applications with no login of their own](/docs/auth/connect-an-application) | Put behind a route, they are signed in by the gateway. | oauth2-proxy or Pomerium in front of each one. | Community |
| [Single sign-on into third-party UIs](/docs/auth/oidc-provider) | A broker's console, a database UI, any application that speaks OpenID Connect signs its people in through the gateway: no consent screen, the route decides who and with which roles. | A Keycloak realm and a client per application, roles mapped there by hand. | Enterprise |

## Access and organisations

| Capability | What Meerkat does | Without Meerkat | Edition |
|---|---|---|---|
| [Roles and groups](/docs/access/roles) | A hierarchical role catalogue, groups per organisation, cumulative or picked at sign-in. | Role tables in each service, or an identity server's roles mapped by hand into each one. | Community |
| [Organisations](/docs/access/tenants) | Customer organisations isolated by the gateway, with their members and admins. | Admin screens to build, or WorkOS, Frontegg. | single organisation Community, several Enterprise |
| [A route's access rule](/docs/access/route-access) | Who reaches an application - signed in, organisation, roles, named people - decided at the door; a refusal lands on a page that explains. | A check in each application, written again per framework. | Community |
| [Per-endpoint security](/docs/access/endpoint-security) | Rules per operation, posed on the route's OpenAPI, refused before the service is called. | Code in the service, or a gateway policy written by hand per path. | Community |
| [The roles an application declares](/docs/access/endpoint-security#the-roles-the-application-declares) | Read from its own spec, shown per operation, imported into the catalogue, and forwarded under the application's names to those who hold them. | Roles copied by hand between the code and the identity server, and kept in step. | Community |
| [Identity passed to your services](/docs/auth/service-jwt) | Headers, REMOTE_USER or a signed JWT with published, rotating keys; you choose the roles each service receives and their shape. | Each service validates a token and maps the claims itself. | Community |
| [Delegated administration](/docs/access/capabilities) | Super-admin, infrastructure, application and organisation admins, each with clear limits. | Rights on each tool, managed tool by tool. | Community |

## Routing and traffic

| Capability | What Meerkat does | Without Meerkat | Edition |
|---|---|---|---|
| [Routes](/docs/concepts/routes) | Changed live from the console, on every node within a second; 11 [predicates](/docs/predicates/overview), 34 [filters](/docs/filters/overview). | Kong, APISIX, Traefik or Envoy Gateway, configured in files and redeployed. | Community |
| [Rate limits and quotas](/docs/operations/rate-limits) | Per route, user, token, organisation or address, and per endpoint. | A plugin per gateway, often in its paid edition. | Community, *partly* |
| [Upstream health](/docs/operations/upstream-health) | Circuit breaker, timeouts at three levels, a heart per route from discovery and real traffic. | Mesh or gateway configuration, plus a dashboard to read it. | Community |
| WebSocket, gRPC, streaming | End to end; a gRPC call counted by its `grpc-status`. | Proxy tuning per protocol. | Community, *partly* |
| [Answers and files from the gateway](/docs/filters/respond) | A route answers by itself from a template, or serves the files you upload. | A small service deployed for each of them. | Community |
| [Maintenance](/docs/filters/maintenance) | One page per route or for the whole platform, translated, with a door for administrators. | A maintenance page per application, and a redeploy to show it. | Community |

## Inside your applications

| Capability | What Meerkat does | Without Meerkat | Edition |
|---|---|---|---|
| [User button and portal](/docs/concepts/data-plane-chrome) | Profile, sign-out, organisation, language, light or dark, and the navigation across your applications, added to the pages the gateway serves. | To build in each application. No product does it. | Community, navigation bar Enterprise |
| [Per-role CSS, your CSS and JavaScript](/docs/customise/injections) | Roles stamped on the page server-side, and your own code per route. | Front-end code in each application. | Community |
| [Notifying your users](/docs/operations/notifications) | A service sends one request; its users see it in whichever application they have open, with a count, a snackbar, a desktop notification, a mail if asked, and an acknowledgement when it matters. | A notification service and its inbox widget in every application, or Novu, Knock, Courier. | Enterprise, *partly* |
| Issue reporting | A screenshot and its technical context, tracked in the console. | Sentry self-hosted, or Marker.io, Jam, Userback. | Community |
| [Theme and branding](/docs/customise/theme) | Material 3 themes, light and dark, your logo and background; more page layouts and no Meerkat mark with Enterprise. | A theme per tool. | Community, layouts Enterprise |

## Platform security

| Capability | What Meerkat does | Without Meerkat | Edition |
|---|---|---|---|
| [TLS](/docs/operations/tls) | Certificates plane by plane, generated, imported or signed on request, ports opened live. | cert-manager, or files on each proxy. | Community |
| [Automatic certificates](/docs/operations/tls) | ACME: Let's Encrypt, ZeroSSL, Google or your own step-ca, several side by side. | cert-manager, or a higher gateway plan. | Enterprise |
| [The vault](/docs/operations/vault) | Secrets encrypted, referenced by name from the configuration, with a reminder before expiry. | Vault, OpenBao, Infisical or Doppler, wired to each tool. | Community |
| Security headers, HTTPS redirect, CSRF | Applied at the door. | Configured per application and per proxy. | Community |

## Operations

| Capability | What Meerkat does | Without Meerkat | Edition |
|---|---|---|---|
| [Configurations](/docs/console/configuration) | Named versions, comparison, YAML export and import, a restore point on every change; in a git repository with Enterprise. | A GitOps pipeline to set up. | Community, git Enterprise |
| [Audit trail](/docs/operations/audit) | Every admin action with a field-by-field diff, every sign-in and factor change; sent over OTLP with Enterprise. | Retraced or WorkOS Audit Logs, then every tool connected to it. | Community, *partly* |
| [Traffic and metrics](/docs/operations/traffic) | Traffic, latency and failures per route and per endpoint, nothing to install; metrics over OTLP with Enterprise. | Prometheus and Grafana. | Community |
| [Logs](/docs/operations/logs) and [traces](/docs/operations/tracing) | Structured logs and an access log, refusals included; a trace identifier on every request, the gateway on the trace with Enterprise. | Each piece instrumented on its own. | Community, *partly* |
| [Scheduled calls](/docs/operations/scheduler) | A route, a path and a cadence: called at least once, retried when too early, history in the console. | Cron jobs, or a scheduler in every service; Temporal, Inngest. | Community |
| [Several gateway instances](/docs/deploy/kubernetes) | Active/active on PostgreSQL, no session affinity, no primary. | Sticky sessions and a shared store to design. | Enterprise |
| [Deployment](/docs/deploy/shapes) | One image, embedded storage by default, Docker, Swarm or Kubernetes with a Helm chart. | One component per need above, each deployed and upgraded. | Community |

## For your developers

| Capability | What Meerkat does | Without Meerkat | Edition |
|---|---|---|---|
| [A local process in the cluster](/docs/operations/plug) | plug: the service on a developer's machine takes the place of the deployed one, then the cluster goes back as it was. Free and standalone beside the gateway; built into it, with SSH keys and every page flagging the substitution, with Enterprise. | mirrord, Telepresence or Gefyra, installed and run apart from the gateway. | plug free, built in Enterprise, *partly* |
| [The API docs of every route](/product/dev-mode) | A Swagger UI per route, calls through the gateway, *Try it out* as any user, group or role, with the roles each operation requires. | A Swagger UI per service, outside the real rules. | Community |
| UI test mode | Browse with a simulated identity to see what a role sees. | Test accounts to create per role. | Community |

## What comes next

Planned, and not built yet (the detail is on the [roadmap](/project/roadmap)):

- **A discovery wizard**: scan Docker or a Kubernetes namespace, pick a container, get a route.
- **Sign in as**: what a support desk does every day.
- **A response cache.**
- **Serving an application under a sub-path without rebuilding it.**
- **gRPC properly**: per-method security, and gRPC-Web.
- **A per-organisation portal**: each customer's own icon, title and arrangement.
- **HTTP/3**, once the gain can be measured.
- **An external vault**: HashiCorp Vault, Kubernetes secrets, Docker secrets.
- **Issues pushed to GitHub, GitLab or Jira.**
- **Web Push**: a desktop notification that reaches a person even when no application is open.
- **Kerberos and SPNEGO**, Enterprise.
