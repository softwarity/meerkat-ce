---
title: The navigation portal
section: Customising
order: 252
summary: A header or a rail injected into your applications so several routes read as one product.
---

# The navigation portal

Several UI routes behind one gateway are several applications: a visitor lands on one and has no way
to reach the next. The portal is the bar that fixes that - injected into every proxied UI page, it
moves between the routes this gateway serves as if they were one product (PORTAL-01).

It promotes what used to be a submenu of the user button into real navigation: parent modules and
their children, a waffle launcher, overflow arrows, the brand's logo and the account menu all in one
strip.

It is set under **Application > Portal**, it is **global** like the theme, and it ships **off**.

![The portal editor and its live mock](img/console/portal.webp)

## Header or rail

One axis, and it swaps:

| Layout | Parents | Children |
|---|---|---|
| `header` | a top strip of tabs | a rail |
| `rail` | a rail | a top strip of tabs |

The rail takes the side you choose, and that side always means something since one of the two
surfaces is always a rail.

Entries render as the icon alone, the label alone, or both - one setting for the whole bar. The
brand's application name can sit beside the logo; it is off by default, because the logo alone is the
mark.

## Modules

A **parent** is a full application in its own right: it binds to a UI route, it is navigable, and it
may gather children shown on the secondary surface. When it has children, the bar offers a way back
to the parent itself.

| Field | What it does |
|---|---|
| Route | the UI route this entry opens. The entry inherits its address **and its access** |
| Icon | a glyph picked from the console's bank, stored as SVG and drawn as a CSS mask - so no icon font is ever loaded. Empty falls back to the label's initial |
| Label | overrides the route's own name in the bar |
| Home label | what the "back to this module" row reads, when the parent has children |
| Description | the entry's tooltip |
| Disabled | off for everyone, without removing it from the configuration |

A **child** is the same, minus the home label.

## Each visitor sees their own portal

A module inherits the **access of the route it binds to**, so a visitor is offered only what their
rights allow. The payload served to the browser carries no access rule at all: the filtering happens
before it is written, which is why there is nothing in the page to read or tamper with.

That is also why the portal is global rather than per organisation: it is already personalised, by
the routes.

## What it replaces

When the portal is on, it takes the place of the per-route user button on **every** UI route. The
button does not disappear - it moves **inside** the bar, and loses its Applications submenu, since
that is now the bar itself. Navigation and the account menu are one surface, not two corners.

The bar is never drawn inside an iframe: a page embedded somewhere else is not the place for a
navigation chrome.

Technically it is a plain custom element with a shadow DOM, served like the user button, wearing the
data plane's theme and the visitor's light or dark.

## Editing it

The console shows a live mock of the bar while you build the catalogue, so the arrangement is judged
where it will be read rather than from a list of fields.

## What is not built

- **The badge channel.** A module carries a badge key and the slot is reserved, but nothing pushes a
  count onto it yet - it is planned on the live channel.
- **Landing on the first accessible application.** A visitor who may not open the module they
  arrived at is not redirected to one they can.
- **Drag and drop in the catalogue**: reordering, or dragging an entry to change level.
- **Per-organisation arrangement** (PORTAL-02). It would be the product's first per-tenant visual
  override, and theme and branding are global today.
