---
title: The app-gateway for your internal applications
summary: One door in front of your internal applications. It takes care of authentication, access rules, routing, quotas and audit, so your services stay lean.
layout: wide
---

::: hero
# The sentinel at your application's door

Meerkat is an **app-gateway**: one entry point in front of the internal
application your teams build, which takes charge of everything that is not
their core business. Authentication, access rules, organisations, routing,
quotas, audit. One image, zero dependency.

- [Get started](/docs/start/quick-start)
- [See it](/showcase/index)
- [What it does](/product/features)

::: figure meerkat
:::
:::

::: stats
### 1 image
to deploy
### 70 MB
on disk
### {{memory.idle}} MB
in memory, at rest
### 0
dependencies
:::

And it is fast: measured next to Kong, APISIX and Traefik, each on the same
CPU, with [the figures](/product/performance) kept current by the CI.

::: lead
Your services receive requests that are already authenticated, carrying a
signed token with an identity, roles and an organisation. They stop shipping a
login page, a role model and a user table, and go back to being the thing you
actually sell.

Weighing it against what you would otherwise assemble?
[The case for Meerkat](/product/the-case) does the arithmetic: eight products
or one, thirty-eight pods or one, and what the same foundation costs in
licences and in engineering days.
:::

::: cards
### One door, not a stack

Install the gateway, then Prometheus, then Grafana, then write YAML for
everything: that is the pattern Meerkat exists to break. Traffic figures, audit
trail, quota rules and service health are screens in the console, and the whole
gateway is one binary with embedded storage.

### Identity is part of the product

Accounts, roles, groups, organisations and the sign-in pages are in the
gateway, not beside it. Your corporate directory - OpenID Connect, GitHub, and
with Enterprise SAML, LDAP or Active Directory - signs people in, and in
Enterprise its groups can grant roles.

### Strong sign-in, built in

Passkeys as a first factor, an authenticator app that remembers a browser you
trust, and a password policy that is a setting rather than a rewrite.

### It completes your pages

The account button, the navigation portal, light and dark mode and the per-role
CSS are added to the pages the gateway serves. Your application
gets them whatever it is written in, and it ships no library for it.

### Edited hot, never restarted

Eleven predicates decide that a request is for a route, thirty-four filters
transform it, and a change applies on the next request. There is no
configuration file to redeploy.

### Driven by an agent

The control plane exposes an MCP endpoint. An assistant reads the installation
and changes it under exactly the rules a human gets - read-only stays
read-only.
:::

## What it looks like

::: gallery
![The routes screen](img/console/routes-list.webp) Every route, what it matches and where it goes.
![The theme editor](img/console/built-in-pages-theme.webp) The sign-in pages wear your colours, in light and in dark.
![Traffic](img/console/traffic.webp) What went through, without a metrics stack.
:::

[Open the gallery](/showcase/index)

## Try it

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat
```

8080 is what your users reach, 9090 is the admin console. From there,
[Quick start](/docs/start/quick-start) puts one of your own services behind it
in about five minutes.

::: cta
### Free, and not a trial

The community edition is the whole gateway for one organisation on one
instance - in production, in a company, commercially, at no cost. Read the
code, change it, ship it inside your own product; two years after each release,
that version becomes plain Apache 2.0.

Enterprise is what you need once the installation grows: several
organisations, SAML, LDAP and Active Directory, several gateways behind one entry
point. Never
a security primitive - TLS, two-factor, passkeys, the vault and the audit trail
are free and stay free. Team is Enterprise for a cluster of a known size, and
the evaluation edition is Enterprise with a notice on it, free, to try all of
it first.

[Compare the editions](/product/editions) . [Pricing](/pricing/index)
:::
