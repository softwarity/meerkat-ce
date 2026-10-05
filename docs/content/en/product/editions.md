---
title: Editions
section: The product
order: 5
summary: What the free edition does, what Enterprise adds, how to try all of it, and the three questions that tell you which one you need.
---

# Editions

One product, four editions. The **community** edition is not a demo and not a
trial: it is the whole gateway for the common case - one organisation, one
gateway, accounts of your own or an OpenID Connect provider. **Enterprise** is
what you need when the installation grows in one of three directions: more
organisations, more gateways, or more corporate plumbing. **Team** is
Enterprise for a cluster of a known size, and the **evaluation** edition is
Enterprise with a notice on it, free, to try all of it first.

| Edition | What it is | How you get it |
| --- | --- | --- |
| **Community** (CE) | The whole free gateway, production included | `softwarity/meerkat` on Docker Hub |
| **Evaluation** (Eval) | Everything below, with an evaluation notice; not for production | `softwarity/meerkat:eval` on Docker Hub |
| **Team** (TE) | Everything Enterprise adds, for a cluster whose size the licence sets | Built for you - [talk to us](/pricing/index) |
| **Enterprise** (EE) | Everything Enterprise adds, without a ceiling | Built for you - [talk to us](/pricing/index) |

::: lead
The line we hold, and it is the one that matters when you are comparing:
**a security primitive is never sold**. TLS and its certificates, the vault,
two-factor, passkeys, the password policy, the brute-force protection, the
audit trail, per-route and per-endpoint access rules: all of that is in the
free image, and always will be. Selling safety to the people least able to pay
for it is not a business model we want.
:::

The rule, as it was settled on **8 August 2026**, is two sentences: **what
costs the growing organisation is paid for; what protects the user is not.**
Its consequence shows as soon as you compare: where identity products bill
single sign-on and two-factor at the next tier up, Meerkat never pushes anybody
to deploy something less safe in order to pay less.

## What the free edition already does

Everything the [features page](/product/features) describes, minus the rows in
the table below. Routing with its eleven predicates and thirty-three filters,
the sign-in pages wearing your colours, local accounts, OpenID Connect and
GitHub, roles and groups, the navigation portal, the vault, TLS (generated, imported or signed on
request), rate limits, the audit trail, the traffic and metrics screens, the
whole console, and the agent endpoint. In production, in a company, commercially, for free.

## What Enterprise adds

| | Community | Enterprise |
| --- | --- | --- |
| **Organisations** | One. It is never named in the console, because there is nothing to tell it apart from. | Several, with members, group modes, an owner, selection at sign-in and a session policy per organisation. |
| **Corporate directory** | OpenID Connect, GitHub | LDAP and Active Directory as well, search-then-bind. |
| **Roles from the directory** | Granted in Meerkat | Group rules: an LDAP group, a GitHub team or an OIDC claim becomes a membership and its roles, at each sign-in. |
| **Certificates** | Generated, imported or signed on request, placed on the console and the application by hand | ACME as well: Let's Encrypt, ZeroSSL, Google or your own step-ca issue and renew them on their own, several authorities side by side. |
| **Business hours** | - | Access windows: time ranges, week days, time zone. Partial - see the [roadmap](/project/roadmap). |
| **Several gateways** | One gateway on its embedded storage | Active/active on one shared PostgreSQL: change bus, shared certificates, no session affinity to ask for. See [the cluster page](/docs/deploy/kubernetes). |
| **Monitoring** | Traffic and metrics screens, built in, nothing to install. Logs written as OpenTelemetry JSON for a node agent. | The same screens, plus export to your OpenTelemetry collector: traces, metrics and logs pushed over OTLP. |
| **Audit** | The audit trail, in the console | The trail sent to your collector, Endpoint audit per operation, and CSV export. |
| **Saved configurations** | Three at a time, and the console says which one you are on before you hit the cap | As many as you like - one per customer, one per environment. |
| **Configurations in git** | Export and import of a file | Each platform in its directory of a git repository: pull, compare, activate, push - committed under the name of the operator. See [Configuration](/docs/console/configuration). |
| **Built-in pages** | Your colours, your logo, your name, the centred arrangement | The split, drawer, banner and bare arrangements too, and the Meerkat mark off the pages you serve. |
| **Developer tunnel** | plug runs beside the gateway, which is plug's own default | The tunnel inside the gateway, and every substitution attributed to the developer who made it. See [Dev mode](/product/dev-mode). |
| **Support** | The repository: an issue is read, and the answer stays where it helps the next person | Part of the agreement - you reach the people who wrote the gateway, not a tier. |

Announced and not built yet: SAML 2.0, Kerberos, and exporting the audit trail
to Parquet. The [roadmap](/project/roadmap) says where each stands.

## Trying Enterprise first

The evaluation edition is the Enterprise image with nothing taken out: no time
limit, no counter, no capability switched off. What it adds is a notice -
*Meerkat evaluation version. Not licensed for production use.* - on the pages
the gateway serves, in the user button, in the e-mails it sends, and as a
watermark across the console. No setting removes it: the notice is in the
binary, and the licensed images are simply built without it.

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat:eval
```

Moving to a licensed image later keeps the database and the configuration as
they are: it is another image on the same data.

## Which one you need

Three questions, and one yes is enough:

1. Do several customers, subsidiaries or departments have to be **kept apart
   inside the same installation**?
2. Do your accounts live in **Active Directory**?
3. Does the gateway have to **survive the loss of a machine**?

If all three are no, the community image is the whole product, and there is
nothing to buy. If one is yes, run the evaluation edition on it before talking
to anyone.

## Nothing to activate

The edition is the image. There is no licence key to install, no activation
server to reach, no entitlement to renew, and nothing that expires in the
middle of a night - most of the Enterprise code is simply not in the community
binary, and a Team or an Enterprise image is built for your company with its
licence already in it. The gateway never calls us: there is no usage report
anywhere in it.

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

See [pricing](/pricing/index).
