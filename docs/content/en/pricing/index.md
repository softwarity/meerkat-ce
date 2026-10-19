---
title: Pricing
section: Pricing
order: 1
layout: wide
summary: Four editions of one gateway. Two are free; Team costs EUR 4,000 a year, Enterprise EUR 24,000, updates and support included. The licence is perpetual, however many installations you run.
---

# Pricing

One gateway, four editions. All of them are a `docker pull` away. Two cost
nothing; the other two, Team and Enterprise, are the evaluation image given a
licence.

::: cards
### Community - free

**CE.** The whole gateway for one organisation on one instance: routing, the
sign-in pages, roles and access rules, TLS, the vault, two-factor and passkeys,
the audit trail, the traffic screens, the console, the agent endpoint.

Free for any use, **production included**, under the Functional Source
License. No account, no key, no expiry.

[Get started](/docs/start/quick-start)

### Evaluation - free

**Eval.** Everything, Enterprise included, to try before a conversation:
several organisations, SAML, LDAP and Active Directory, several instances kept in sync, ACME,
configurations in git, the OpenTelemetry export, the developer tunnel. No time limit, no counter, nothing
switched off.

What it carries instead is a notice - *evaluation version, not licensed for
production use* - on the sign-in pages, in the user button, in the e-mails and
across the console. Only a licence removes it.

[Get started](/docs/start/quick-start)

### Team - EUR 4,000 a year

**TE.** Enterprise for one organisation on one instance: the corporate
directory (SAML, LDAP, Active Directory), ACME, configurations in git, the
OpenTelemetry export, the page layouts and the mark removed, the OpenID
provider, the built-in developer tunnel. No evaluation notice. A perpetual
licence; the yearly price includes updates and standard support.

Made for the small company, with no scalability constraint.

Write to us: [sales@softwarity.io](mailto:sales@softwarity.io?subject=Meerkat%20Team)

### Enterprise - EUR 24,000 a year

**EE.** All of it, without a ceiling: as many organisations as you run, and the
gateway as an active/active cluster on PostgreSQL, scaled out by adding
instances. Extended support included.

One **perpetual** licence per company, for all its installations: **not per
installation, not per user, not per request, not per route**. The yearly price
includes updates and support; the right to use it is yours for good.

Write to us: [sales@softwarity.io](mailto:sales@softwarity.io?subject=Meerkat%20Enterprise)
:::

## What each edition carries

| | Community | Evaluation | Team | Enterprise |
| --- | --- | --- | --- | --- |
| **Price per year** (updates and support) | Free | Free | EUR 4,000 excl. VAT | EUR 24,000 excl. VAT |
| **Licence** | - | - | Perpetual | Perpetual |
| **Where it comes from** | Docker Hub | Docker Hub | The evaluation image and a licence | The evaluation image and a licence |
| **Production use** | Yes | No | Yes | Yes |
| **The Enterprise capabilities** | - | Yes | Yes | Yes |
| **Evaluation notice** | - | On every surface | - | - |
| **Organisations** | One | Not limited | One | Not limited |
| **Installations** | As many as you like | As many as you like | As many as you like | As many as you like |
| **Instances per installation** | One | Several, kept in sync | One | Several, kept in sync |
| **Users, routes, requests** | Not limited | Not limited | Not limited | Not limited |
| **Support included** | The public repository | The public repository | Standard | Extended |

What "the Enterprise capabilities" are, row by row, is on
[Editions](/product/editions).

## Support

The yearly price includes a year of updates and a level of support. Without a
renewal the licence stays valid: any version released during the period you
paid for is yours, to install and run for as long as you like. Only the
versions released afterwards are not covered. Support beyond what is included
is ordered separately.

| | What it is | For whom |
| --- | --- | --- |
| **Standard support** | By e-mail, on working days: bugs and how to use the product. | Included in Team |
| **Extended support** | A guaranteed first-response time, a named contact, help with upgrades. | Included in Enterprise |
| **Support options** | Wider hours and a short guaranteed response on a blocking incident. | Enterprise, on request |
| **Services** | By the day: integration on your premises, migration from another proxy or identity provider, an architecture review, training. | Every edition, on request |

Support answers about the product; a service works on your project.

## A licence, pasted into the console

Team and Enterprise are not separate images: they are the evaluation image
given a licence. You paste it into the console, it is checked on the spot, and
the evaluation notice is gone at once, with no restart, on every instance of the
installation. A deployment can mount it from a secret instead, and every
instance reads it from there.

- **Checked offline.** The licence is signed by Softwarity and the gateway
  verifies the signature itself. No activation server to reach: the gateway
  never calls us, and there is no usage report anywhere in it.
- **Perpetual.** A licence does not expire. Its date says how far the covered
  updates go: a version released after it is outside the agreement, and the
  console says so, but nothing switches off.
- **Nothing stops in the middle of a night.** A licence never cuts traffic.
  What goes past its ceilings is refused when it is created, with a sentence
  saying so: a second organisation on a Team licence is not created, and
  everything already in place keeps being served.
- **One licence per company, as many installations as you like.** The same
  licence is pasted into each of them at no extra cost: a Team installation
  keeps its ceilings, one organisation and one instance, but nothing limits how
  many you run. It covers the installations of the company that bought it, and
  only those: handing it on to another company, customer or partner, is not
  covered.
- **The licensee's name is shown** in the console, with the licence's
  identifier: a licence pasted elsewhere says whose it is.

## Evaluate first

The evaluation edition exists so that nobody has to buy on the strength of a slide. Run it
against your own directory, your own applications and your own cluster for as
long as the question takes - it holds nothing back, and it does not stop.

What it is not is a way to run production: the notice is on every page your
users see, and no setting removes it. Moving from an evaluation to a licensed
edition keeps everything: same image, same database, same configuration. Only
the licence is added.

## Questions people ask first

**Can we use the free edition in production, in a company, on a commercial
product?** Yes. The Functional Source License permits internal and production
use explicitly. The only thing it forbids is building a competing gateway with
it.

**Can we read and change the code?** Yes, the trunk is public and modifiable.
And **two years after each release, that version becomes plain Apache 2.0**,
with no conditions left - so what you deploy today cannot be taken away from
you later.

**Do you see our traffic, our users or our configuration?** No. There is no
telemetry in the product, and nothing in it sends anything to us. The licence
is checked on the spot.

**Team or Enterprise?** The capabilities are the same. Team holds one
organisation on one instance; as soon as you have to keep several customers or
subsidiaries apart inside one installation, or survive the loss of a machine,
it is Enterprise.

**Can we run several Team installations instead of one Enterprise?** Yes, with
the same licence and at no extra cost. They share neither their accounts, nor
their configuration, nor their sessions: that is what the Enterprise cluster
brings.

**What happens to an installation if we do not renew?** It keeps running,
licensed: the licence is perpetual. What stops is updates and support. You keep
the versions released during the period you paid for; later ones are not
covered, and a renewal opens them again.

**Do you sell support for the free edition?** Ask us. The community image is
supported by the repository - an issue on
[softwarity/meerkat-ce](https://github.com/softwarity/meerkat-ce/issues) is read.

::: cta
### Talk to us

Write to [sales@softwarity.io](mailto:sales@softwarity.io) saying what you are building and how big it is:
a Team or Enterprise licence, a support agreement, an integration, or just the
project you want to talk about.
:::
