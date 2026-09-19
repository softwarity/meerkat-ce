---
title: Editions
section: Getting started
order: 5
summary: What the community image does, what the Enterprise one adds, and why the difference is the image rather than a setting.
---

# Editions

Meerkat ships as two images built from the same commit:

| Image | Edition | Where |
|---|---|---|
| `docker.io/softwarity/meerkat` | community | Docker Hub, public |
| `ghcr.io/softwarity/meerkat-ee` | Enterprise | private registry |

What separates them is the `ee` build tag, decided at compile time. There is no
licence file to read and no per-feature key: most Enterprise code is simply not
in the community binary, so it refuses by being absent rather than by checking
anything. The startup line says which one came up:

```
level=INFO msg=edition edition=ce enterprise=false
```

## What only the Enterprise image does

| Capability | State |
|---|---|
| More than one organisation - the community image serves one, never named in the console | shipped |
| LDAP and Active Directory as an authentication authority (search-then-bind) | shipped |
| Group rules - what a directory, a GitHub team or an OIDC claim declares becomes a membership and roles | shipped |
| Cluster: several gateways on one shared PostgreSQL database, with the change bus and the advisory lock | shipped |
| Prometheus exposition of the counters, on the control plane, off by default | shipped |
| Removing the Meerkat mark from the served pages, and the page arrangements beyond the centred one (split, drawer, banner, bare) | shipped |
| More than three saved configurations at a time | shipped |
| Business access hours - time windows, week days, time zone | partial: no mid-session re-check, and the per-member window is not editable in the console |
| The developer tunnel: a developer's machine answers under a cluster name | partial - see the developer-mode pages |

Two more are **announced and not built**: SAML 2.0, and exporting the audit
trail to analytical formats. The SAML kind can be stored, and the factory then
refuses it.

## What both images do

Everything else, and that is most of the product: routing with its predicates
and filters, TLS and ACME, the vault, passwords and their policy, TOTP,
passkeys, OIDC and GitHub as authorities, the role catalogue, groups, per-route
and per-endpoint access rules, rate limits, the audit trail, the traffic
dashboards, issue reporting, the configuration document, and the console
itself.

Two of those deserve a word, because they are usually the paid part elsewhere:

- **The counters and the dashboards are in both images.** What Enterprise sells is exporting them to a monitoring stack you already run. A community installation has its curves with nothing installed.
- **The embedded storage is in both images.** What Enterprise sells is the external database, which is what makes several gateways one installation.

## How a refusal reads

An Enterprise capability refused by a community image names the act, not a
price-list word:

```
more than one organisation is part of the Enterprise edition,
and this is the community image
```

Two refusals are deliberately soft, because a hard one would strand an
installation:

- Switching to single-tenant mode with several organisations is **allowed** and deletes nothing; the others stop being served until someone switches back.
- A page arrangement already in place keeps being served, and returning to the default one is always allowed.

## Licences

The trunk is
[FSL-1.1-Apache-2.0](https://github.com/softwarity/meerkat-ce/blob/main/LICENSE.md):
free to use, copy, modify and redistribute for any purpose except building a
competing product or service - internal and production use in your company is
explicitly permitted. Each release becomes Apache 2.0 two years after its
publication.

The Enterprise sources are not in the public tree. They are source-visible to
customers and usable only under a Softwarity commercial agreement.
