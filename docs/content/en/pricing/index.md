---
title: Pricing
section: Pricing
order: 1
layout: wide
summary: Four editions of one gateway. Two are free and one download away, two start with a conversation.
---

# Pricing

One gateway, four editions. Two of them are a `docker pull` away and cost
nothing; the other two are built for you.

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
several organisations, SAML, LDAP and Active Directory, the cluster, ACME,
configurations in git, the OpenTelemetry export, the developer tunnel. No time limit, no counter, nothing
switched off.

What it carries instead is a notice - *evaluation version, not licensed for
production use* - on the sign-in pages, in the user button, in the e-mails and
across the console. No setting removes it.

[Get started](/docs/start/quick-start)

### Team - talk to us

**TE.** Everything Enterprise does, for an installation of a known size: a
cluster of a few gateways, the number set by your agreement. No notice, and
support from the people who wrote the gateway.

The sizes and what each costs are being settled. Tell us how many gateways you
run and we will answer with a figure.

### Enterprise - talk to us

**EE.** The same image without a ceiling: as many gateways and as many
clusters as you run, across your environments and your sites.

Priced for the installation, **not per user, not per request, not per route**.
:::

## What each edition carries

| | Community | Evaluation | Team | Enterprise |
| --- | --- | --- | --- | --- |
| **Price** | Free | Free | On request | On request |
| **Where it comes from** | Docker Hub | Docker Hub | Built for you | Built for you |
| **Production use** | Yes | No | Yes | Yes |
| **The Enterprise capabilities** | - | Yes | Yes | Yes |
| **Evaluation notice** | - | On every surface | - | - |
| **Gateways in a cluster** | One | Not limited | Set by your agreement | Not limited |
| **Support** | The public repository | The public repository | Part of the agreement | Part of the agreement |

What "the Enterprise capabilities" are, row by row, is on
[Editions](/product/editions).

## An image built for you

A Team or an Enterprise image is **built for your company**, under your
agreement. There is still nothing to activate: no key to install, no activation server to
reach, no entitlement to renew, nothing that expires in the middle of a night.
The gateway never calls us, and there is no usage report anywhere in it.

The two free editions are the same binary for everyone, published in the open.

## Evaluate first

The evaluation edition exists so that nobody has to buy on the strength of a slide. Run it
against your own directory, your own applications and your own cluster for as
long as the question takes - it holds nothing back, and it does not stop.

What it is not is a way to run production: the notice is on every page your
users see, and removing it means building another binary, which the licence
forbids. Moving from an evaluation to a licensed image keeps everything: same
database, same configuration, another image.

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
telemetry in the product, and nothing in it sends anything to us.

**Team or Enterprise?** The capabilities are the same. Team fits an
installation whose size you can name today; Enterprise is for the one you do
not want to count.

**What happens to an installation if an agreement ends?** That is one of the
terms being written; ask us and we will answer plainly rather than
contractually. What the product itself does is not a cliff: a capability it no
longer carries is refused with a sentence, and everything already in place
keeps being served.

**Do you sell support for the free edition?** Ask us. The community image is
supported by the repository - an issue on
[softwarity/meerkat-ce](https://github.com/softwarity/meerkat-ce/issues) is read.

::: cta
### Talk to us

The commercial contact channel is being set up and its address will be here.
Until then, open an issue on
[softwarity/meerkat-ce](https://github.com/softwarity/meerkat-ce/issues) saying what
you are building and how big it is, and we will take it from there.
:::
