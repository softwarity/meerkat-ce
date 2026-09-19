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
| [The navigation portal](/docs/customise/portal) | Application > Portal | global |
| The offered languages | Application > Locales | global reserve, per-route offer |
| [How an application consumes light and dark](/docs/customise/color-scheme) | the route | per route |
| [Extra CSS and JavaScript](/docs/customise/injections) | the route | per route |
| The user button, its corner and its menu | the route | per route |

## One sign-in, one identity

The theme and the branding are decided **once**, globally (THEME-01): one sign-in procedure
serves every application behind the gateway, so it has one look and one name. Several themes
can coexist, but exactly one is active - themes are colour trials, and the identity does not
fork with them.

The admin console keeps its own look and is not themed: it is an operator's tool, not part of
your product's surface.

## What is not customisable

- **Per organisation.** There is no per-tenant theme, branding or portal. That would be the
  product's first per-tenant visual override, and it is written down as PORTAL-02 rather than
  half-built.
- **Per application.** A group of routes sharing their own branding and locales is SVC-05, and
  it does not exist: the configuration is global and a route belongs to no group.
- **Your own HTML for the flow pages.** The arrangement is a closed catalogue of layouts shipped
  with the product; replacing a page's markup is the other half of PAGE-02 and is not built
  (PAGE-02).
- **The language catalogues.** Twenty languages are embedded; adding one is still a rebuild
  (PAGE-03).

## What travels

Everything on this page is configuration, so it travels in a configuration export - except the
pictures. A plain YAML export never carries an image and says what it left behind; a `.zip`
package carries the logos and the background. See
[backup and restore](/docs/operations/backup-restore).
