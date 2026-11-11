---
title: Customising Meerkat
section: Customising
order: 240
summary: What can wear your customer's colours, where each piece is set, and what is still global.
---

# Customising Meerkat

Meerkat serves pages of its own - the sign-in flow, the unavailable page - and it injects a
little of itself into the applications behind it. Both can be made to look like your product
rather than ours.

## What is customisable, and where

| What | Where it is set | Scope |
|---|---|---|
| [The palette](/docs/customise/theme) | Application > Built-in pages, **Theme** tab | global |
| [The arrangement of the pages](/docs/customise/built-in-pages) | same screen, **Layout** tab | global, Enterprise |
| [Name, logo, favicon, background](/docs/customise/branding) | same screen, **Branding** tab | global |
| Imposed light or dark on those pages | same screen, on the preview | global |
| [The application catalogue](/docs/customise/portal) | Application > Portal | global |
| The offered languages | Routes > a route > Locales | declared per route, the offer is their union |
| [How an application consumes light and dark](/docs/customise/color-scheme) | the route | per route |
| [Extra CSS and JavaScript](/docs/customise/injections) | the route | per route |
| The user button, its corner and its menu | the route | per route |

## Six brands, the same pages

The same sign-in page, dressed for six brands we invented for the purpose. Each one is a
theme (colours and fonts), a branding (name, tagline, logo, background) and a layout, all set
from the screens listed above: nothing here is custom HTML or CSS.

::: gallery compact
![Glimmo, centered layout, light](img/login-examples/glimmo-centered-light.webp) **Glimmo**, a photo-sharing app: the centered layout, the default, in light.
![Glimmo, centered layout, dark](img/login-examples/glimmo-centered-dark.webp) The same Glimmo in dark, with its own [dark background](/docs/customise/branding).
![Halvern Bank, split layout, light](img/login-examples/halvern-split-light.webp) **Halvern Bank**, private banking: the split layout, a serif title and a sans-serif body.
![Halvern Bank, split layout, dark](img/login-examples/halvern-split-dark.webp) The same bank in dark: the [palette](/docs/customise/theme) has its dark side, the background its dark picture.
![Maison Velune, split layout, dark](img/login-examples/velune-split-dark.webp) **Maison Velune**, a fashion house: the split layout, the image on the left this time, in dark.
![Maison Velune, drawer layout, light](img/login-examples/velune-drawer-light.webp) The same house in light, in the drawer layout, its panel on the left.
![Nightreel, drawer layout, dark](img/login-examples/nightreel-drawer-dark.webp) **Nightreel**, a streaming service: the [drawer](/docs/customise/built-in-pages) layout, its panel against the right edge, in dark.
![Monoforge, bare layout, dark](img/login-examples/monoforge-bare-dark.webp) **Monoforge**, a developer platform: the bare layout, no card, and [Glow](/docs/customise/theme) unticked, for a flat design.
![Basketa, banner layout, light](img/login-examples/basketa-banner-light.webp) **Basketa**, grocery delivery: the banner layout, the brand in a band across the top.
:::

A background is read against the page it sits under: a picture that suits the dark scheme is
often too dark behind light text, and the reverse. When the pages offer both schemes, give the
background [one picture per scheme](/docs/customise/branding).

Every layout but `centered` is Enterprise. The script that builds these brands and takes the
captures is `e2e/scripts/capture-themes.mjs`: it runs against any gateway, which makes it a
starting point for your own.

## One sign-in, one identity

The theme and the branding are decided **once**, globally: one sign-in procedure
serves every application behind the gateway, so it has one look and one name. Several themes
can coexist, but exactly one is active - themes are colour trials, and the identity does not
fork with them.

The admin console keeps its own look and is not themed: it is an operator's tool, not part of
your product's surface.

## What is not customisable

- **Per tenant.** There is no per-tenant theme, branding or portal. That would be the
 product's first per-tenant visual override: it is planned, and written down rather than
 half-built.
- **Per application.** A group of routes sharing their own branding and locales does not exist:
 the configuration is global and a route belongs to no group.
- **Your own HTML for the flow pages.** The arrangement is a closed catalogue of layouts shipped
 with the product; replacing a page's markup is not built.

## What travels

Everything on this page is configuration, so it travels in a configuration export - except the
pictures. A plain YAML export never carries an image and says what it left behind; a `.zip`
package carries the logos and the background. See
[backup and restore](/docs/operations/backup-restore).
