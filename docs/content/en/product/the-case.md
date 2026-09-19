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

| Building block | Product | Production instances | State to operate |
| --- | --- | --- | --- |
| Identity | Keycloak | 3 pods | PostgreSQL |
| API gateway | Kong OSS, database-less | 3 nodes | Redis, for shared rate limits |
| Authentication proxy | oauth2-proxy | 2 pods | Redis, for sessions |
| Certificates | cert-manager | 7 pods | - |
| Secrets vault | OpenBao or Vault | 5 Raft nodes | its own Raft storage |
| Monitoring | Prometheus, Grafana | 8 pods | its own series database |
| Audit | Retraced | 5 pods | PostgreSQL and Elasticsearch |
| Workstation to cluster | mirrord Operator | 1 pod, plus one job per session | - |

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
| **Total** | **83 to 165 days** | **48 to 101 days** | **5.5 to 12 days** |

The SaaS stack skips installing servers but keeps the integration, the wiring
between products, and the development that no product covers.

## Where these figures come from

Prices and product facts were gathered in **September 2026** from public
documentation and public price lists. The memory figure is measured by this
project's own CI; the pod counts are the ones each product's documentation
recommends for production; the person-days are estimates, given as a range
because that is what they are.

Prices move. If a line here is out of date, it is the line that is wrong - tell
us and we will correct it. What does not move is the shape of the argument: one
product instead of eight, one process instead of thirty-eight pods, and a
foundation that is already there instead of one to assemble.

What is actually built, and what is not, is
[one table read from the code](/project/roadmap). What the two editions carry
is on [editions](/product/editions).
