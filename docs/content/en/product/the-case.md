---
title: The case for it
section: The product
order: 4
summary: What the foundation of a professional application costs when you assemble it, when you rent it, and when Meerkat already provides it.
printable: true
---

# The case for Meerkat

Every professional application needs the same foundation before its first real
screen: a polished sign-in, two-factor, single sign-on for your customers,
roles, an audit log, certificates, monitoring. Your buyers check all of it. None
of it makes your product different. And all of it takes time to build, then
more time to maintain.

::: lead
Meerkat is that foundation, in one product. This page puts numbers on what it
saves you: in servers, in licences and in engineering days. Every figure is
sourced and dated, and estimates are labelled as estimates.
:::

**In short:**

- **One product instead of eight**, and one pod instead of about 38.
- **5.5 to 12 days** to set up, instead of 83 to 165.
- **$27,000 to $88,000 a year**: the licences of the equivalent assembled from
  paid services.
- **Free and complete** in its Community edition.

## What buyers expect in 2026

This is the list as it appears in security questionnaires and tenders.

| | |
| --- | --- |
| **A polished sign-in** | pages in your brand, translated, light and dark |
| **Two-factor and passkeys** | authenticator app, recovery codes, security keys |
| **Customer SSO** | Entra ID, Okta, Google, SAML, LDAP, Active Directory |
| **Several organisations** | each customer isolated, with its members, groups and admins |
| **Roles and fine-grained access** | per screen, per application, per API endpoint |
| **API tokens** | for integrations and machines |
| **Traffic protection** | rate limits, quotas, timeouts, a breaker for failing services |
| **Automatic HTTPS** | certificates issued and renewed on their own |
| **Secrets kept safe** | encrypted, never written in the configuration |
| **Audit log** | who changed what, before and after |
| **Monitoring** | traffic, response times and errors, per application and endpoint |
| **High availability** | several servers, no session lost |
| **Announced maintenance** | a proper page instead of an error |
| **Scheduled jobs** | closings, reminders, clean-ups, the morning report |
| **User feedback** | report a problem with a screenshot and its context |
| **Developer tooling** | test against the real platform from your own machine |
| **API documentation** | one Swagger for every API, tried live through the gateway, under the identity you choose |

What Meerkat covers, line by line, is on [what it does](/product/features).

## What you assemble without it

For each need, the product teams usually pick: a free one they host themselves,
or a paid service.

| Need | Free, self-hosted | Paid or SaaS | Meerkat |
| --- | --- | --- | --- |
| Sign-in, two-factor, passkeys | Keycloak, Zitadel, Authentik | Auth0, WorkOS, Clerk, FusionAuth | included |
| Branded pages, 20 languages | a theme to build | depends on the plan | included |
| Customer SSO: OIDC, SAML, LDAP, AD | Keycloak | Auth0, WorkOS, billed per connection | included, Enterprise for SAML and LDAP |
| Organisations your customers manage | admin screens to build | WorkOS, Frontegg | included, Enterprise |
| Protect an application that has no login | oauth2-proxy, Pomerium Core | Pomerium Enterprise, Cloudflare Access | included |
| Routing, rate limits, quotas | Kong OSS, APISIX, Traefik, Envoy Gateway | Kong Enterprise, Tyk, Traefik Hub, API7, Gravitee | included |
| Automatic HTTPS certificates | cert-manager | in the higher gateway plans | included, Enterprise |
| Secrets vault | Vault Community, OpenBao | HCP Vault, Infisical, Doppler | included |
| Traffic dashboards | Prometheus and Grafana | Grafana Cloud | included |
| Audit of every admin action | Retraced, then connect every tool | WorkOS Audit Logs, then connect every tool | included |
| User menu and portal inside your applications | **to build** | **no product exists** | included |
| Issue reporting with a screenshot | self-hosted Sentry | Marker.io, Jam, Userback, BugHerd | included |
| A developer's machine plugged into the platform | plug, mirrord OSS, Telepresence, Gefyra | mirrord Team, Okteto | Enterprise (plug alone stays free) |
| Saved configurations, rollback | a GitOps pipeline to set up | a GitOps pipeline to set up | included |
| Scheduled calls to your services | cron jobs, or a scheduler in every service | Temporal Cloud, Inngest, Trigger.dev, EventBridge Scheduler | included |
| Driven by an AI agent | community MCP servers | Kong Konnect, SaaS only | included |

Two needs have no product at all: a user menu and portal inside your own
applications, and one audit log across every tool. Without Meerkat, you write
those yourself.

> [!WARNING]
> The free route is getting narrower (checked in September 2026):
>
> - **Kong** calls 3.9 its last fully free version. The free branch gets 3.9
>   fixes only, while Enterprise has reached 3.16, and the Enterprise image lost
>   its free mode in 3.10.
> - **Vault Community** moved to the BSL licence, which forbids competing
>   offers.
> - **Traefik Hub** offline needs a licence that expires after a year; the
>   gateway stops 30 days later.

## What it costs to run

Every product you add brings its own servers, database, backups, upgrades and
security alerts. Here is the same front door built both ways, with the number of
pods each product's documentation recommends for production.

| Building block | Product | Pods in production | Also needs | Memory recommended |
| --- | --- | --- | --- | --- |
| Identity | Keycloak | 3, the documentation's own example | PostgreSQL | 1,250 MB per pod |
| API gateway | Kong OSS, its lightest setup (no database) | 3 | Redis, to share rate limits | 2 to 4 GB per pod |
| Login proxy | oauth2-proxy | 2 | Redis, for sessions | not published |
| Certificates | cert-manager | 7, the recommended practice | - | not published |
| Secrets vault | OpenBao or Vault | 5 | its own storage, and the keys to unseal it | 8 to 16 GB per pod |
| Monitoring | Prometheus and Grafana (kube-prometheus-stack) | 5, plus 1 per server | its own time-series database | 512 MB at least for Grafana |
| Audit | Retraced | 5 | PostgreSQL, Elasticsearch and NSQ | not published |
| Developer tunnel | mirrord Operator | 1, plus one per session | a paid Team licence | not published |
| **Everything above** | **Meerkat** | **1, or 3 for high availability** | **nothing, or PostgreSQL** | **{{memory.idle}} MB at rest, {{memory.peak}} under load** |

::: figure stack
The same front door, both ways. On the left a request crosses two products
before it reaches your services, and six more run alongside. On the right, one.
:::

Assembled, that is **about 38 pods and five storage systems** to install, secure,
upgrade and back up.

With Meerkat: **one pod, {{memory.idle}} MB at rest and {{memory.peak}} under
full load**. Three pods and a PostgreSQL when you want
[high availability](/docs/deploy/kubernetes).

## And it is fast

One product instead of eight is only good news if that product is not the slow
one. So we measure it against Kong, APISIX and Traefik: each limited to the same
single CPU, in front of the same service, under the same load, in the same run.
The table compares products, not machines. The results are recomputed on every
change to the code and shown live: [the measurements](/product/performance).

## What it costs in licences

Free products cost nothing in licences: you pay for them in servers (above) and
in time (below). Paid services show a price. To compare them, take a software
vendor selling to businesses, the case where this foundation weighs the most:

- 5,000 active users a month
- 20 customer organisations, 10 of them with their own SSO
- 15 services, in production and staging
- 50 million requests a month
- a team of 10 developers

With 20 organisations, this case needs Meerkat Enterprise.

| Building block | Cheapest that fits | Typical choice | Meerkat |
| --- | --- | --- | --- |
| Identity, SSO, two-factor | Descope Pro, $499 | Auth0 Essentials, $2,000 | included |
| API gateway | API7 Cloud, $750 | Gravitee Planet, $2,500 | included |
| Secrets vault | Infisical Pro, $200 | HCP Vault Essentials, $1,881 | included |
| Monitoring | Grafana Cloud Pro, $100 | Grafana Cloud Pro, $100 | included |
| Audit of admin actions | WorkOS Audit Logs, $224 | WorkOS Audit Logs, $224 | included |
| Issue reporting | Userback Business, $79 | Marker.io Team, $149 | included |
| Developer tunnel | mirrord Team yearly, $400 | mirrord Team monthly, $500 | included in Enterprise |
| Menu and portal in your applications | no product | no product | included |
| **Per month** | **$2,252** | **$7,354** | Enterprise, on request |
| **Per year** | **$27,024** | **$88,248** | [see pricing](/pricing/index) |

Public list prices in US dollars, before tax, without servers or integration
work. Products with no public price for this case are left out rather than
guessed (Kong Konnect beyond 10 million requests, Tyk, Kong Enterprise, Pomerium
Enterprise, Frontegg beyond five connections). For scale: Traefik Hub sells for
$30,000 to $50,000 a year, and per-user offers such as Cloudflare Access or Pomerium
Zero, at $7 a user, would reach $35,000 a month for 5,000 users.

## Each customer that brings its own SSO

Identity services bill single sign-on per customer. So the foundation costs more
with every large customer you sign - exactly when you are winning.

::: figure sso-per-customer
What one more customer with its own single sign-on adds, every month.
:::

Connections included before the meter starts: **five at Stytch and Descope,
three on Auth0 Essentials, one at Clerk, none at WorkOS**. After that, every
customer arriving with its Entra ID or Okta is billed, whatever it uses. With
Meerkat, a customer's SSO is a setting, not a line on the invoice.

## What it costs in time

The heaviest cost is not the licence, it is the engineering. Here is our
estimate, in person-days, to reach the same result, from the low estimate to
the high one. Adapting your own services to read the user's identity is left
out: every option needs it.

::: figure person-days
Person-days to reach the same result, from the low estimate to the high one.
:::

| Work | Free products | SaaS products | Meerkat |
| --- | --- | --- | --- |
| Identity: install, two-factor, passkeys, SSO | 10 to 20 | 5 to 10 | 0.5 to 1 |
| Branded, translated sign-in pages | 5 to 10 | 2 to 4 | 0.5 to 1 |
| Customer organisations and their admin | 15 to 30 | 5 to 15 | 0.5 to 1 |
| Login proxy, identity passed to services | 3 to 6 | 3 to 6 | 0.5 to 1 |
| API gateway: routes, rate limits, quotas | 8 to 15 | 5 to 10 | 1 to 2 |
| Certificates and secrets vault | 6 to 13 | 3 to 6 | 0.5 to 1 |
| Monitoring and dashboards | 4 to 8 | 2 to 4 | 0 to 0.5 |
| One audit log across every tool | 8 to 15 | 5 to 10 | 0 |
| User menu and portal in the applications | 8 to 15 | 8 to 15 | 0.5 to 1 |
| Issue reporting | 1 to 3 | 1 to 2 | 0 to 0.5 |
| A developer's machine plugged in | 2 to 5 | 1 to 3 | 0.5 to 1 |
| Saved configurations, rollback | 5 to 10 | 3 to 6 | 0 |
| Testing it all end to end | 8 to 15 | 5 to 10 | 1 to 2 |
| **Setting it up** | **83 to 165 days** | **48 to 101 days** | **5.5 to 12 days** |
| At 650 EUR a day, before tax | 54 to 107 k | 31 to 66 k | 3.6 to 7.8 k |
| Maintenance, days a year | 20 to 40 | 10 to 20 | 2 to 5 |

SaaS saves installing servers, but not connecting the products to each other,
nor building what none of them covers.

## Where these figures come from

**Measured.** Memory and speed come from the project's own benchmark
(`tools/bench` in the repository), last run
on **{{memory.date.en}}** on GitHub's x64 and arm64 machines, at rest and under
load. What is built is read from the code, not from a plan:
[the public inventory](/product/features) counts today {{features.built}}
features delivered, {{features.partial}} in part and {{features.todo}} to come.

**Gathered.** Prices are the public list prices, in US dollars before tax, read
on the official pages on **16 September 2026**, without any discount. A buyer
who negotiates will pay less, which is why the comparison uses list prices: they
are the only ones anyone can check. Pod counts
and memory are the ones each product's documentation gives for production.

**Estimated.** The person-days are Softwarity's estimate, given as a range
because that is what it is, converted at **650 EUR a day before tax** for an
experienced DevOps or security engineer.

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

Prices change. If a line here is out of date, tell us and we will fix it.

## What you get instead

**One product instead of eight. One pod instead of thirty-eight. A foundation
that is already there, instead of one to build.**

For the team that lives with it:

- **One thing to run.** One image, one console, one audit log, one backup. Not
  eight products to keep up to date, each with its own security alerts.
- **Ready for the security questionnaire on day one.** Two-factor, passkeys,
  single sign-on over OpenID Connect, audit, TLS, vault: all in the free
  edition. Answering a tender stops being a project.
- **Your developers back on your product.** Five to twelve days of setup
  instead of 83 to 165, and above all two to five days of upkeep a year instead
  of twenty to forty, every year.
- **No lock-in.** The Community edition is complete and will stay so, and the
  code becomes Apache 2.0 after two years. You are not betting your front door
  on us.

What is built and what is not is in [one table read from the code](/project/roadmap).
What each edition includes is on [editions](/product/editions). And to see it
before deciding, the [showcase](/showcase/index) has the screens.
