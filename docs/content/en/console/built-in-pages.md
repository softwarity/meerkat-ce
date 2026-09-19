---
title: Built-in pages
section: The console
order: 174
summary: The look of the pages Meerkat serves itself - colours, arrangement and identity, with one live preview.
---

# Built-in pages

**Application > Built-in pages.** The pages the gateway serves in its own name:
sign-in, sign-up, the second-factor challenge, the unavailable page, the error
pages.

One screen, one preview, three tabs on the left: **Theme**, **Layout**,
**Branding**. They describe the same page, so the preview never moves, and the
theme picker stays under it on all three - trying a colour while judging an
arrangement is the normal way round.

The tabs are real routes: a bookmark on the layout gallery comes back to the
layout gallery.

![Built-in pages on the Theme tab: the token table on the left, the two previews on the right with the theme carousel between them](img/console/built-in-pages-theme.webp)

The sign-in page as it is served, dark above and light below, with the theme
carousel in the gap. On the left, one row per token, one column per scheme.

## The theme picker

The carousel in the middle of the preview is the list of themes. Click a palette
to look at it; click **the palette on the nav block** to make that theme **live**.
Selected and active are two different things, which is what lets you try a theme
without serving it.

The controls between the arrows add a theme (a duplicate of this one, or a start
from a preset) and delete one. The active theme cannot be deleted.

## Theme

The two palettes of the theme, **dark and light side by side**, one row per token.
Hovering a token name highlights the part of the preview it paints, which is the
fastest way to find out what a name means.

- **The Dark and Light checkboxes** in the header decide which schemes the served
  pages offer at all. Untick one and the pages stop proposing it; you cannot
  untick both.
- **Glow** is the decorative flow-page effects as one switch: the ambient halo
  behind the page, the logo and button glows, the app-name gradient. Unchecked
  gives a flat design, and the colour those effects use then goes unused.
- **Export** and **Import** carry a palette as a file, to move one between
  installations.

Save is on this tab: a theme is an object of its own, and saving it is what
changes what a live theme serves.

## Layout

**Arrangement** is a gallery of mock-ups - where the brand, the picture and the
form sit. Picking one updates the preview at once.

> [!NOTE]
> Enterprise edition: **keeping** an arrangement other than the centred one. The
> gallery stays clickable and the preview follows, because seeing an arrangement
> is what this screen is for; what the licence buys is keeping it.

- **Logo size** - the box the logo is drawn in. A wide wordmark laid in the normal
  box comes out a quarter of its height; *banner* draws the mark at its own size.
  There is nothing to size until a logo is set on the Branding tab.
- **Which side** - the edge the brand takes. Only the arrangements made of halves
  have a side.

A page opened inside an iframe drops the brand and fills the frame on its own,
whichever arrangement is chosen.

## Branding

![Built-in pages on the Branding tab: application name, tagline, logo and background drop zones, and the Meerkat mark card](img/console/built-in-pages-branding.webp)

The name and tagline typed here appear in the preview at once. Below, the Meerkat
mark card, capped Enterprise.

The application's identity: **name**, **tagline**, **logo**, **tab icon**
(favicon), and the **background picture** with its fit (cover, contain, tile) and
its dimming. The dark scheme can carry its own picture, or share the light one.

The **Meerkat mark** has a card of its own, because it is a licence question
rather than an identity one: the served pages carry a *powered by
softwarity/meerkat* line at the foot, and removing it is Enterprise.

> [!NOTE]
> Enterprise edition: removing the Meerkat mark.

## Traps

- **Selected is not active.** Editing a palette changes the theme you are looking
  at; serving it is the click on the nav block's palette.
- **The console does not wear this theme.** These are the pages the **data plane**
  serves to your users. The console has its own look.
- **The logo size does nothing without a logo**, and the side does nothing on an
  arrangement with no halves. The controls are disabled rather than silent.
- **A background picture is carried in a package export**, not in a plain YAML
  one. See [Configuration](/#/docs/console/configuration).
