---
title: Editions
section: The product
order: 5
summary: What the free edition does, what Enterprise adds, how to try all of it, and the three questions that tell you which one you need.
---

# Editions

One product, four editions. **Community** is not a demo and not a trial: it is
the whole gateway for the common case - one organisation, one gateway, your own
accounts or an OpenID Connect provider - free, production included.
**Enterprise** is for when you grow in one of three directions: several
organisations, several gateways, or your corporate directory. **Team** is
Enterprise for a cluster of a set size. And **Evaluation** is Enterprise with a
notice on it, free, to try everything first.

| Edition | What it is | How you get it |
| --- | --- | --- |
| **Community** (CE) | The whole free gateway, production included | `softwarity/meerkat` on Docker Hub |
| **Evaluation** (Eval) | All of Enterprise, with an evaluation notice; not for production | `softwarity/meerkat:eval` on Docker Hub |
| **Team** (TE) | Everything Enterprise adds, for a cluster of the size your agreement sets | Built for you - [talk to us](/pricing/index) |
| **Enterprise** (EE) | Everything Enterprise adds, without a ceiling | Built for you - [talk to us](/pricing/index) |

::: lead
The line we hold, and the one that matters when you compare:
**security is never sold**. TLS and its certificates, the vault, two-factor,
passkeys, the password policy, the brute-force protection, the audit trail,
per-route and per-endpoint access rules: all of it is in the free image, and
always will be. Selling safety to those least able to pay for it is not a
business model we want.
:::

The rule fits in one sentence: **what a growing organisation needs is paid
for; what protects the user is not.** Where identity products put single
sign-on and two-factor in a higher plan, Meerkat never makes anyone choose
between paying less and being safe.

## What the free edition already does

Everything the [features page](/product/features) describes, minus the rows of
the table below: routing with its eleven predicates and thirty-four filters,
the sign-in pages in your colours, local accounts, OpenID Connect and GitHub,
roles and groups, the navigation portal, the vault, TLS (generated, imported or
signed on request), rate limits, the audit trail, the traffic and metrics
screens, the whole console, and the AI agent endpoint. In production, in a
company, commercially, for free.

## What Enterprise adds

| | Community | Enterprise |
| --- | --- | --- |
| **Organisations** | One, and the console never makes you name it. | Several, each with its members, groups and owner, chosen at sign-in. |
| **Corporate directory** | OpenID Connect, GitHub | SAML 2.0, LDAP and Active Directory as well. |
| **Roles from the directory** | Granted in Meerkat | Group rules: an LDAP group, a GitHub team or an OIDC claim becomes a membership and its roles, at each sign-in. |
| **Certificates** | Generated, imported or signed on request, placed on the console and the application by hand | ACME as well: Let's Encrypt, ZeroSSL, Google or your own step-ca issue and renew them on their own, several authorities side by side. |
| **Business hours** | - | Access windows: time ranges, week days, time zone. Partial - see the [roadmap](/project/roadmap). |
| **Several gateways** | One gateway on its embedded storage | Active/active on one shared PostgreSQL: a change reaches every node within a second, certificates are shared, any node serves any session. See [the cluster page](/docs/deploy/kubernetes). |
| **Monitoring** | Traffic and metrics screens, built in, nothing to install. Logs written as OpenTelemetry JSON for a node agent. | The same screens, plus export to your OpenTelemetry collector: traces, metrics and logs pushed over OTLP. |
| **Audit** | The audit trail, in the console | The trail sent to your collector, Endpoint audit per operation, and CSV export. |
| **Saved configurations** | Three at a time, and the console says which one you are on before you hit the cap | As many as you like - one per customer, one per environment. |
| **Configurations in git** | Export and import of a file | Each platform in its directory of a git repository: pull, compare, activate, push - committed under the name of the operator. See [Configuration](/docs/console/configuration). |
| **Built-in pages** | Your colours, your logo, your name, the centred arrangement | The split, drawer, banner and bare arrangements too, and the Meerkat mark off the pages you serve. |
| **Developer tunnel** | plug, free, runs on its own beside the gateway; Meerkat knows nothing of it | plug integrated into the gateway: each developer signs in with their own key, and every substitution is named and announced to users. See [Dev mode](/product/dev-mode). |
| **Support** | The repository: an issue is read, and the answer stays where it helps the next person | Part of the agreement - you reach the people who wrote the gateway, not a tier. |

Announced and not built yet: Kerberos, and exporting the audit trail to
Parquet. The [roadmap](/project/roadmap) says where each stands.

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
2. Do your accounts live in **LDAP or Active Directory**, or behind **SAML**?
3. Does the gateway have to **survive the loss of a machine**?

If all three are no, Community is the whole product and there is nothing to
buy. If one is yes, try the evaluation edition on it before talking to anyone.

## Nothing to activate

The edition is the image. No licence key to install, no activation server to
reach, nothing that expires in the middle of the night: most of the Enterprise
code is simply not in the Community image, and a Team or Enterprise image is
built for your company. The gateway never calls us, and sends no usage report.

When the Community image meets something it does not include, it says plainly
what it cannot do, and it never leaves you stranded: everything already in
place keeps being served.

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
