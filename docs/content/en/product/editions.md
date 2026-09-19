---
title: Editions
section: The product
order: 3
summary: What the free edition does, what Enterprise adds, and the three questions that tell you which one you need.
---

# Editions

One product, two images. The **community** edition is not a demo and not a
trial: it is the whole gateway for the common case - one organisation, one
gateway, accounts of your own or an OpenID Connect provider. **Enterprise** is
what you need when the installation grows in one of three directions: more
organisations, more gateways, or more corporate plumbing.

::: lead
The line we hold, and it is the one that matters when you are comparing:
**a security primitive is never sold**. TLS and its certificates, the vault,
two-factor, passkeys, the password policy, the brute-force protection, the
audit trail, per-route and per-endpoint access rules: all of that is in the
free image, and always will be. Selling safety to the people least able to pay
for it is not a business model we want.
:::

## What the free edition already does

Everything the [features page](/product/features) describes, minus the rows in
the table below. Routing with its eleven predicates and thirty-three filters,
the sign-in pages wearing your colours, local accounts, OpenID Connect and
GitHub, roles and groups, the navigation portal, the vault, TLS with ACME, rate
limits, the audit trail, the traffic and metrics screens, the whole console,
and the agent endpoint. In production, in a company, commercially, for free.

## What Enterprise adds

| | Community | Enterprise |
| --- | --- | --- |
| **Organisations** | One. It is never named in the console, because there is nothing to tell it apart from. | Several, with members, group modes, an owner, selection at sign-in and a session policy per organisation. |
| **Corporate directory** | OpenID Connect, GitHub | LDAP and Active Directory as well, search-then-bind. |
| **Roles from the directory** | Granted in Meerkat | Group rules: an LDAP group, a GitHub team or an OIDC claim becomes a membership and its roles, at each sign-in. |
| **Business hours** | - | Access windows: time ranges, week days, time zone. Partial - see the [roadmap](/project/roadmap). |
| **Several gateways** | One gateway on its embedded storage | Active/active on one shared PostgreSQL: change bus, shared certificates, no session affinity to ask for. See [the cluster page](/docs/deploy/kubernetes). |
| **Monitoring** | Traffic and metrics screens, built in, nothing to install | The same screens, plus a Prometheus exposition so the stack you already run can scrape them. |
| **Saved configurations** | Three at a time, and the console says which one you are on before you hit the cap | As many as you like - one per customer, one per environment. |
| **Built-in pages** | Your colours, your logo, your name, the centred arrangement | The split, drawer, banner and bare arrangements too, and the Meerkat mark off the pages you serve. |
| **Developer tunnel** | plug runs beside the gateway, which is plug's own default | The tunnel inside the gateway, and every substitution attributed to the developer who posed it. See [Dev mode](/product/dev-mode). |

Announced and not built yet: SAML 2.0, Kerberos, and exporting the audit trail
to analytical formats. The [roadmap](/project/roadmap) says where each stands.

## Which one you need

Three questions, and one yes is enough:

1. Do several customers, subsidiaries or departments have to be **kept apart
   inside the same installation**?
2. Do your accounts live in **Active Directory**?
3. Does the gateway have to **survive the loss of a machine**?

If all three are no, the community image is the whole product, and there is
nothing to buy.

## Nothing to activate

The edition is the image. There is no licence key to install, no activation
server to reach, no entitlement to renew, and nothing that expires in the
middle of a night - most of the Enterprise code is simply not in the community
binary. The gateway never calls us: there is no usage report and no licence
check anywhere in it.

A community image asked for something it does not carry says so in a sentence
that names the act rather than a price-list word, and it never strands an
installation: what is already in place keeps being served.

## The licences

The trunk is
[FSL-1.1-Apache-2.0](https://github.com/softwarity/meerkat-ce/blob/main/LICENSE.md),
the Functional Source License. Read it, change it, run it in production, ship
it inside your own product. The one thing it forbids is building a competing
gateway with it. **Two years after each release, that version becomes plain
Apache 2.0**, with no conditions left at all.

The Enterprise sources are not in the public tree. They are readable by
customers and usable only under a Softwarity commercial agreement.

See [pricing](/product/pricing).
