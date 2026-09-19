---
title: Pricing
section: The product
order: 4
summary: One edition is free and stays free. The other is priced per production instance, and starts with a conversation.
---

# Pricing

Two ways to run Meerkat. One of them costs nothing and is not a trial.

::: cards
### Community - free

The whole gateway for one organisation on one instance. Routing, the sign-in
pages, roles and access rules, TLS, the vault, two-factor and passkeys, the
audit trail, the traffic screens, the console, the agent endpoint.

Free for any use, **including in production and in a company**, under the
Functional Source License. No account to create, no key, no expiry.

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat
```

[Get started](/docs/start/quick-start)

### Enterprise - talk to us

Everything above, plus what an installation needs once it grows: several
organisations, LDAP and Active Directory, roles granted by your directory,
several gateways behind one entry point, the Prometheus exposition, the
developer tunnel, and the built-in pages without our mark.

Priced **per production instance**. Not per user, not per request, not per
route.

[What Enterprise adds](/product/editions)
:::

## How it is priced

- **Per production instance.** The unit is the installation you put in front of
  your users, not the number of people behind it. Nobody counts seats, and
  growing from thirty users to three thousand does not change the bill.
- **Nothing to meter.** The gateway never calls us: there is no usage report
  and no licence check anywhere in it. What you pay for is the image and the
  agreement that opens it.
- **Nothing to activate.** No key to install, no entitlement to renew, nothing
  that can expire in the middle of a night.

> [!NOTE]
> The figures, the terms and what a support agreement covers are being written,
> and they will be on this page. Until then the answer to "how much" is a
> conversation - and it is a short one, because the unit is simple.

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

**What happens to an installation if an agreement ends?** That is one of the
terms being written; ask us and we will answer plainly rather than
contractually. What the product itself does is not a cliff: a capability it no
longer carries is refused with a sentence, and everything already in place
keeps being served.

**Do you sell support for the free edition?** Ask us. The community image is
supported by the repository - an issue on
[softwarity/meerkat](https://github.com/softwarity/meerkat/issues) is read.

::: cta
### Talk to us

The commercial contact channel is being set up and its address will be here.
Until then, open an issue on
[softwarity/meerkat](https://github.com/softwarity/meerkat/issues) saying what
you are building and how big it is, and we will take it from there.
:::
