---
title: The application catalogue
section: Customising
order: 252
summary: The list of applications the gateway offers, and the three ways to offer it - nothing, a menu, or a navigation bar.
---

# The application catalogue

Several UI routes behind one gateway are several applications: a visitor lands
on one and has no way to reach the next. The catalogue is the list of the ones
you offer, in the order you choose.

It is set under **Application > Portal**, it is **global** like the theme, and it
ships empty.

![The Portal screen in Portal mode: the three-state control, the arrangement, and the real bar as a live mock](img/console/portal.webp)

## One catalogue, three renderings

A three-state control decides what is drawn. **The list itself does not move**:
going from a menu to a bar is a rendering decision, not a reason to retype your
applications.

| Mode | What the visitor gets |
|---|---|
| **None** | No list. The user button and the pages Meerkat serves itself carry your brand's name and nothing to click. That is the right answer when there is one application |
| **Links** | The list, in the order you chose, in the user button's **Applications** submenu and on the data-plane pages |
| **Portal** | A navigation bar on every page of every application. The built-in pages then offer **one** link, the first entry the caller may open: the bar is the navigation, and a page outside the applications only needs a door |

## What an entry carries

| Field | What it does |
|---|---|
| Route | the UI route this entry opens. The entry inherits its address **and its access** |
| Label | the name the application is offered under. Empty falls back to the route's name |
| Description | the entry's tooltip |
| Disabled | off for everyone, without removing it from the list |

In **Portal** mode an entry also carries what it takes to draw a bar: an icon,
children, and the label of the "back here" row when it has any.

| Field | What it does |
|---|---|
| Icon | a glyph picked from the console's bank, stored as SVG and drawn as a CSS mask - no icon font is ever loaded. Empty falls back to the label's initial |
| Home label | what the "back to this application" row reads, when it has children |
| Children | sub-applications shown on the secondary surface |

## The catalogue says what exists, the route says who sees it

An entry inherits the **access of the route it binds to**, so a visitor is
offered only what their rights allow. No access rule is decided here: that would
put a security decision in a display setting.

The payload served to the browser carries no rule either: the filtering happens
before it is written, which is why there is nothing in the page to read or
tamper with.

It is also why the catalogue is global rather than per organisation: it is
already personalised, by the routes.

> [!NOTE] A route no longer lists itself
> An application's name used to be decided on **each UI route**, in a `Link`
> field. Three places could therefore name the same application, and since the
> list was derived from the routes, something had to guess which ones were the
> same thing - an installation commonly fronts one product with several routes,
> one per organisation or per version, which differ in what they proxy and never
> in where you go.
>
> So a new UI route no longer shows up on its own: you add it here. It is the
> same number of decisions as before, taken in one place.

## Header or rail

**Portal** mode only. One axis, and it swaps:

| Layout | Entries | Their children |
|---|---|---|
| `header` | a top strip of tabs | a rail |
| `rail` | a rail | a top strip of tabs |

The rail takes the side you choose, and that side always means something since
one of the two surfaces is always a rail.

Entries render as the icon alone, the label alone, or both - one setting for the
whole bar. The brand's application name can sit beside the logo; it is off by
default, because the logo alone is the mark.

## What the bar replaces

In **Portal** mode it takes the place of the per-route user button on **every**
UI route. The button does not disappear: it moves **into** the bar, and loses
its Applications submenu there, since that is now the bar itself. Navigation and
the account menu are one surface, not two corners.

The bar is never drawn in an iframe: a page embedded elsewhere is not the place
for navigation.

Technically it is a plain custom element with a shadow DOM, served like the user
button, wearing the data plane's theme and the visitor's light or dark.

## Editing it

In **Links** mode the console shows the list as what it is: numbered entries,
two arrows for the order. In **Portal** mode it shows a live mock of the bar
while you build it, so the layout is judged where it will be read rather than in
a list of fields.

## What is not built

- **The badge channel.** An entry carries a badge key and the slot is reserved,
 but nothing pushes a count onto it yet - that is planned on the live channel.
- **Landing on the first reachable application.** A visitor who cannot open the
 application they arrive on is not redirected to one they can.
- **Drag and drop**: reordering by hand, or dragging an entry to change level.
 The arrows do the job.
- **A per-organisation arrangement.** It would be the product's first per-tenant
 visual override, and the theme and branding are global today.
