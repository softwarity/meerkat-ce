---
title: The case for it
section: The product
order: 4
summary: What the foundation under every professional application costs when you assemble it, when you buy it as a service, and when it is already there.
printable: true
---

# The case for Meerkat

Before an application shows its first domain screen, it has to do everything a
buyer expects from professional software. None of it sets the product apart,
all of it is checked by procurement and security teams, and each piece costs
time to build and then to maintain.

::: lead
This page is the arithmetic of that foundation: what you assemble without
Meerkat, what it costs in the cluster, in licences and in engineering days.
The figures are sourced and dated, and where a number is an estimate it says
so.
:::

## The foundation expected in 2026

This is the list as it shows up in security questionnaires and tenders.

| | |
| --- | --- |
| **Polished sign-in** | pages in your brand, translated, light and dark |
| **Second factor and passkeys** | TOTP, recovery codes, WebAuthn |
| **Customer SSO** | Entra ID, Okta, Google, LDAP, Active Directory |
| **Multi-organisation** | isolated customers, members, groups, delegated admins |
| **Roles and fine-grained access** | per screen, per route, per API endpoint |
| **API tokens** | for integrations and machines |
| **Traffic protection** | rate limiting, quotas, circuit breaker, timeouts |
| **Automatic TLS** | certificates issued and renewed hands-free |
| **Secrets kept safe** | encrypted at rest, never in the configuration |
| **Audit log** | who changed what, with the before and the after |
| **Observability** | traffic, latency, errors, per route and per endpoint |
| **High availability** | several nodes, no session lost |
| **Announced maintenance** | a proper page instead of a 502 |
| **Scheduled work** | closings, reminders, purges, the morning report |
| **User feedback** | report a problem with a screenshot and context |
| **Developer tooling** | test against the cluster from your own machine |

What Meerkat covers of it, line by line and with the state read from the code,
is on [what it does](/product/features).

## What you assemble without it

For each need, the product usually chosen - as a free self-hosted version, or
as a commercial offer.

| Need | Free, self-hosted | Commercial or SaaS | Meerkat |
| --- | --- | --- | --- |
| Sign-in, MFA, passkeys | Keycloak, Zitadel, Authentik | Auth0, WorkOS, Clerk, FusionAuth | included |
| Branded pages, 20 languages | a theme to build | depends on the tier | included |
| Customer SSO: OIDC, LDAP, AD | Keycloak | Auth0, WorkOS, billed per connection | included, Enterprise for LDAP |
| Organisations managed by your customers | admin screens to build | WorkOS, Frontegg | included, Enterprise |
| Protect an application that has no authentication | oauth2-proxy, Pomerium Core | Pomerium Enterprise, Cloudflare Access | included |
| Routing, rate limits, quotas, circuit breaker | Kong OSS, APISIX, Traefik, Envoy Gateway | Kong Enterprise, Tyk, Traefik Hub, API7, Gravitee | included |
| Automatic TLS certificates | cert-manager | in the higher gateway tiers | included |
| Secrets vault | Vault Community, OpenBao | HCP Vault, Infisical, Doppler | included |
| Traffic dashboards | Prometheus and Grafana | Grafana Cloud | included |
| Audit of every administrative action | Retraced, then wire every tool | WorkOS Audit Logs, then wire every tool | included |
| User menu and portal inside your applications | **to build** | **no standard product** | included |
| Issue reporting with a screenshot | self-hosted Sentry | Marker.io, Jam, Userback, BugHerd | included |
| Developer workstation to cluster | plug, mirrord OSS, Telepresence, Gefyra | mirrord Team, Okteto | built in, Enterprise |
| Versioned configurations, rollback | a GitOps pipeline to set up | a GitOps pipeline to set up | included |
| Scheduled calls to your services | cron, Kubernetes CronJob, or Quartz and Celery in every service | Temporal Cloud, Inngest, Trigger.dev, EventBridge Scheduler | included |
| Driven by an AI agent | community MCP servers | Kong Konnect, SaaS only | included |

Two of those rows have no product behind them at all: the user menu and portal
inside your own applications, and cross-cutting audit that covers every tool at
once. Those you write.

> [!WARNING]
> Three facts, gathered in September 2026, weigh on the free route. Kong
> presents 3.9 as its last fully free release: the free branch now receives
> 3.9.x fixes only while Enterprise is at 3.16, and the Enterprise image's free
> mode was removed in 3.10. Vault Community is under the BSL licence, which
> forbids competing offerings. Traefik Hub offline is a licensed option whose
> token expires after a year, and the gateway then stops after a 30-day grace
> period.

## What it costs in the cluster

Every product added to the cluster brings its pods, its database, its backups,
its upgrades and its security advisories. The same front door, built both ways,
with the instance counts each product's own documentation recommends for
production.

| Building block | Product | Production instances | State to operate | Recommended memory |
| --- | --- | --- | --- | --- |
| Identity | Keycloak | 3 pods, the documentation's own example | PostgreSQL | 1,250 MB per pod |
| API gateway | Kong OSS, database-less | 3 nodes | Redis, for shared rate limits | 2 to 4 GB per node |
| Authentication proxy | oauth2-proxy | 2 pods | Redis, for sessions | not published |
| Certificates | cert-manager | 7 pods, the recommended practice | - | not published |
| Secrets vault | OpenBao or Vault | 5 Raft nodes | Raft, and the unseal shares | 8 to 16 GB per node |
| Monitoring | kube-prometheus-stack | 5 pods, plus 1 per node | its own series database | 512 MB minimum for Grafana |
| Audit | Retraced | 5 services | PostgreSQL, Elasticsearch and NSQ | not published |
| Workstation to cluster | mirrord Operator | 1 pod, plus one job per session | a Team licence is required | not published |
| **The whole baseline** | **Meerkat** | **1 pod, or 3 clustered** | **embedded database, or PostgreSQL** | **22 MB at rest** |

::: figure stack
The same front door, both ways. On the left a request crosses two products
before it reaches your services, and six more run alongside; on the right, one.
:::

That is **about 38 pods and five storage engines** to install, secure, upgrade
and back up - and a request crosses two of those products before it reaches
your services.

The other way: **one pod, 22 MB idle**, measured by the CI on an x64 runner.
Three pods and one PostgreSQL when you want it
[highly available](/docs/deploy/kubernetes).

## And it holds the load

One product instead of eight is only good news if the one product is not the
slow one. It is measured rather than claimed: Meerkat runs next to Kong,
APISIX and Traefik, each pinned to the same single CPU, in front of the same
service, under the same load, in the same run - so the table compares products
and not machines. The CI recomputes it on every commit and the site reads it
live, which is why no number is written here:
[the measurements](/product/performance).

## What it costs in licences

Free building blocks cost nothing in licences; their price is the cluster above
and the time below. Commercial offers do show a price. To compare them, one
scenario - a vendor serving business customers, which is where the foundation
is heaviest:

- 5,000 monthly active users
- 20 customer organisations, 10 of them with their own SSO
- 15 services, in production and staging
- 50 million requests a month
- a team of 10 developers

| Building block | Cheapest offer that fits | Typical offer | Meerkat |
| --- | --- | --- | --- |
| Identity, SSO, MFA | Descope Pro, $499 | Auth0 Essentials, $2,000 | included |
| API gateway | API7 Cloud, $750 | Gravitee Planet, $2,500 | included |
| Secrets vault | Infisical Pro, $200 | HCP Vault Essentials, $1,881 | included |
| Monitoring | Grafana Cloud Pro, $100 | Grafana Cloud Pro, $100 | included |
| Audit of admin actions | WorkOS Audit Logs, $224 | WorkOS Audit Logs, $224 | included |
| Issue reporting | Userback Business, $79 | Marker.io Team, $149 | included |
| Workstation to cluster | mirrord Team annual, $400 | mirrord Team monthly, $500 | plug: built in |
| Menu and portal in your applications | no product | no product | included |
| **Per month** | **$2,252** | **$7,354** | Community: free |
| **Per year** | **$27,024** | **$88,248** | Enterprise: [per production instance](/product/pricing) |

Public list prices in US dollars, excluding tax, excluding machines and
integration time. Products with no public price for this scenario are left out
rather than guessed: Kong Konnect Plus caps at 10 million requests so the
scenario moves to a quote, and Tyk, Kong Enterprise, Pomerium Enterprise and
Frontegg beyond five connections are quote-only. For scale, Traefik Hub sells
for $30,000 to $50,000 a year on the AWS Marketplace, and per-user offers such
as Cloudflare Access or Pomerium Zero at $7 would reach $35,000 a month at
5,000 users.

## Every customer that brings its own SSO

Identity offers bill SSO connections per customer, so the foundation gets more
expensive with every contract signed with a large enterprise - exactly when you
are winning.

::: figure sso-per-customer
What one more customer with its own single sign-on adds, every month.
:::

The quotas included before that meter starts: **five connections at Stytch and
Descope, three on Auth0 Essentials, one at Clerk, none at WorkOS**. Past that
threshold every customer arriving with their own Entra ID or Okta is billed,
and the amount has nothing to do with what they consume.

## What it costs in time

The heaviest cost of an assembled foundation is not the licence, it is
engineering. An estimate in person-days to reach the same scope, work package
by work package, low figure to high. It excludes adapting your own services to
read the forwarded identity, which every option needs.

::: figure person-days
Person-days to reach the same scope, from the low estimate to the high one.
:::

| Work package | Free stack | SaaS stack | Meerkat |
| --- | --- | --- | --- |
| Identity: installation, MFA, passkeys, federation | 10 to 20 | 5 to 10 | 0.5 to 1 |
| Branded, translated sign-in pages | 5 to 10 | 2 to 4 | 0.5 to 1 |
| Customer organisations and their administration | 15 to 30 | 5 to 15 | 0.5 to 1 |
| Authentication proxy, identity to services | 3 to 6 | 3 to 6 | 0.5 to 1 |
| API gateway: routes, rate limits, quotas, breaker | 8 to 15 | 5 to 10 | 1 to 2 |
| Certificates and secrets vault | 6 to 13 | 3 to 6 | 0.5 to 1 |
| Monitoring and dashboards | 4 to 8 | 2 to 4 | 0 to 0.5 |
| Cross-cutting audit of administrative actions | 8 to 15 | 5 to 10 | 0 |
| User menu and portal inside the applications | 8 to 15 | 8 to 15 | 0.5 to 1 |
| Issue reporting | 1 to 3 | 1 to 2 | 0 to 0.5 |
| A developer's machine into the cluster | 2 to 5 | 1 to 3 | 0.5 to 1 |
| Versioned configurations, rollback | 5 to 10 | 3 to 6 | 0 |
| End-to-end acceptance testing | 8 to 15 | 5 to 10 | 1 to 2 |
| **Setting it up** | **83 to 165 days** | **48 to 101 days** | **5.5 to 12 days** |
| At 650 EUR a day, before tax | 54 to 107 k | 31 to 66 k | 3.6 to 7.8 k |
| Maintenance, person-days a year | 20 to 40 | 10 to 20 | 2 to 5 |

The SaaS stack skips installing servers but keeps the integration, the wiring
between products, and the development that no product covers.

## Where these figures come from

A figure without its method is worth nothing, so here is the method, and the
sources with it.

**What is measured.** Memory and throughput come from this project's own CI
bench (`tools/bench`), last run on **17 September 2026** on the GitHub x64 and
arm64 runners, with the container's memory read at rest **and under load**. The
state of each feature is read from the code rather than from a plan: [the
repository's public inventory](/product/features) on **16 September 2026**
gives 106 features delivered, 87 delivered in part and 21 to come.

**What is gathered.** The prices are the PUBLIC prices, in US dollars before
tax, read from the official pages on **16 September 2026**, with no negotiated
discount - a buyer who negotiates will pay less, which is exactly why the
comparison is made at list price. The pod counts and the recommended memory are
the ones each product's documentation gives for production.

**What is estimated.** The person-days are Softwarity's estimate, given as a
range because that is what it is. The conversion into euros uses **650 EUR a
day, before tax**, for an experienced DevOps or security engineer.

::: details The sources, one by one
**Identity**

- auth0.com/pricing
- workos.com/pricing
- clerk.com/pricing
- descope.com/pricing
- stytch.com/pricing
- keycloak.org, memory and CPU sizing

**Gateways and proxies**

- konghq.com/pricing, and Kong's sizing guidance
- Kong: which versions are still entirely open source
- api7.ai/pricing
- gravitee.io/pricing
- Traefik Hub on AWS Marketplace, and its offline mode
- pomerium.com/pricing and Cloudflare Zero Trust

**Cluster, vault, monitoring, audit, tooling**

- cert-manager, its deployment best practices
- Vault, the Raft reference architecture, and OpenBao, integrated storage
- HCP Vault, Infisical, Doppler
- grafana.com/pricing, and the kube-prometheus-stack chart
- Retraced
- softwarity/plug and mirrord
- Userback, Marker.io, Jam
:::

Prices move. If a line here is out of date, it is the line that is wrong - tell
us and we will correct it.

## What you get instead

What does not move is the shape of the argument: **one product instead of
eight, one process instead of thirty-eight pods, a foundation that is already
there instead of one to assemble.**

For the team that will live with it, that means:

- **One thing to operate.** One image, one console, one audit trail, one
  backup. Not eight products to keep updated, each with its own security
  bulletins and its own release cadence.
- **A baseline that passes the questionnaires on day one.** Two-factor,
  passkeys, customer single sign-on, audit, TLS, vault: all of it is in the
  free edition, so a tender stops being a project.
- **Your developers given back to your product.** Five to twelve days of setup
  instead of eighty-three to a hundred and sixty-five - and, more to the point,
  instead of the twenty to forty person-days a year the assembled stack asks
  for again every year.
- **The right to change your mind.** The Community edition is complete and will
  stay that way; the code turns Apache 2.0 after two years. You are not betting
  your front door on our survival.

What is actually built, and what is not, is
[one table read from the code](/project/roadmap) - the same transparency as the
figures above. What the two editions carry is on
[editions](/product/editions), and if you would rather see before deciding, the
[showcase](/showcase/index) has the screens.
