---
title: Showcase
section: Showcase
order: 1
layout: wide
summary: What the gateway looks like, from the sign-in page your users meet to every screen an operator works in.
---

::: hero
# See it

Two surfaces, and both of them ship with the product. On one side, the pages
your users meet - sign-in, the navigation bar, the account menu - already
wearing your colours. On the other, the console an operator lives in.

- [Get started](/docs/start/quick-start)
- [What it does](/product/features)

::: figure meerkat
:::
:::

## What your users meet

These pages are served by the gateway itself. Nothing in the application
behind them ships a line of code for this: the colours, the logo and the
navigation come from the console.

::: gallery
![The sign-in page](img/app/signin-light.webp) The sign-in page, with your name, your logo and your palette. Passkeys sit beside the password, not behind an option.
![The sign-in page in dark](img/app/signin-dark.webp) The same page in dark, with its own background picture if you want one.
![The navigation portal](img/app/portal-light.webp) One bar across every application the gateway serves, filtered by what the visitor may open.
![The navigation portal in dark](img/app/portal-dark.webp) The visitor's colour scheme is carried into the proxied application, even when the application has its own.
![The account menu](img/app/user-button.webp) The account button: who you are, the language, the light and dark switch, and the way out.
![A route under maintenance](img/app/maintenance.webp) A route closed on purpose answers a page, with a reason, rather than a 503 nobody can read.
:::

## Routing

::: gallery
![The routes screen](img/console/routes-list.webp) Every route in order, what it matches, where it goes, and whether it is on.
![A route's target](img/console/route-editor-target.webp) The upstream, and what the gateway offers to fill it in with.
![Predicates](img/console/route-editor-predicates.webp) The conditions a request must meet for this route to take it.
![Filters](img/console/route-editor-filters.webp) What happens to the request, and to the answer, on the way through.
![Per-endpoint security](img/console/route-editor-security.webp) A rule per operation, read from the service's own OpenAPI description.
:::

## Identity and access

::: gallery
![Users](img/console/users.webp) The accounts, what they can administer, and the fields you decided to ask for.
![Roles](img/console/roles.webp) A hierarchical catalogue: a role that inherits another gets everything it opens.
![Groups](img/console/groups.webp) Groups per organisation, so a role is granted by membership rather than one by one.
![Members](img/console/members.webp) Who belongs to an organisation, and as what.
![Authorities](img/console/auth-providers.webp) OpenID Connect, LDAP, Active Directory, GitHub. They authenticate; they never decide roles.
![API tokens](img/console/access-tokens.webp) Tokens with a plane and a perimeter, and a secret shown exactly once.
:::

## Looking after it

::: gallery
![Traffic](img/console/traffic.webp) What went through, per route, with the failures told apart from the silence.
![Metrics](img/console/metrics.webp) Latency and status classes over the last hour, without a metrics stack to install.
![The audit trail](img/console/audit.webp) Every administrative change, with its author and a field-level diff.
![The vault](img/console/vault.webp) Secrets sealed at rest and plain values, both referenced by name.
![TLS](img/console/tls.webp) Certificates, their names and their expiry, issued or uploaded.
![Configuration](img/console/configuration.webp) The whole installation as one document, to export and replay elsewhere.
:::

## Making it yours

::: gallery
![The theme](img/console/built-in-pages-theme.webp) A palette built from one seed colour, in light and in dark, applied to every page the gateway serves.
![Branding](img/console/built-in-pages-branding.webp) The name, the logo, the tagline and the background picture.
![The portal editor](img/console/portal.webp) The navigation bar, arranged: modules, submodules, icons, and a live preview.
![General settings](img/console/general.webp) What this installation is, and the policies every account lives under.
![The agent endpoint](img/console/mcp.webp) MCP on the control plane, so an assistant can work under the same rules.
:::
