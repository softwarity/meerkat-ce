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

![Built-in pages on the Theme tab: the six core colours, the contrast and the typography on the left, the two previews on the right with the theme carousel between them](img/console/built-in-pages-theme.webp)

The sign-in page as it is served, dark above and light below, with the theme
carousel in the gap. On the left, the colours the theme is made from.

## The theme picker

The carousel in the middle of the preview is the list of themes. Click a palette
to look at it; click **the palette on the nav block** to make that theme **live**.
Selected and active are two different things, which is what lets you try a theme
without serving it.

The controls between the arrows add a theme (a duplicate of this one, or a start
from a preset) and delete one. The active theme cannot be deleted.

## Theme

A theme is made the way [Material Theme Builder](https://material-foundation.github.io/material-theme-builder/)
makes one, and it is the same theme: the same six colours give the same schemes,
role for role.

- **Core colours.** The **primary** is the source. **Secondary**, **tertiary**,
  **error**, **neutral** (backgrounds and surfaces) and **neutral variant**
  (medium emphasis and outlines) are derived from it until you set them - the
  value shown greyed is the one they are derived as. The cross puts a colour back
  to derived.
- **Contrast**: standard, medium or high, for both schemes.
- **Color match** is the builder's "stay true to my color inputs": containers keep
  the tone of the colours given rather than the spec's.
- **Typography**: a **display** face for the titles and the application's name, a
  **body** face for the text and buttons, a **code** face for codes, keys, fields and
  labels - each one of fourteen families the gateway ships and serves itself, or the
  system's. Behind any choice, Noto draws Arabic, Hebrew, Devanagari and Thai; Chinese,
  Japanese and Korean use the system's fonts. Each family is shown in itself.
- **Generated roles** lists every Material 3 role the colours make, dark and light
  side by side - read-only, they follow.

Hovering a colour or a role makes what it paints blink in the preview, which is
the fastest way to find out what a name means. The preview follows **as you
pick**, on whatever page it shows: a mail and the portal bar are redrawn with the
colours on screen too.

- **The Dark and Light checkboxes** in the header decide which schemes the served
  pages offer at all. Untick one and the pages stop proposing it; you cannot
  untick both.
- **Glow** is the decorative flow-page effects as one switch: the ambient halo
  behind the page, the logo and button glows, the app-name gradient. Unchecked
  gives a flat design.
- **Export** writes the builder's JSON format (core colours, the six schemes, the
  palettes), with Meerkat's own settings under a key of their own. **Import** reads
  a builder export - whether Color match was on is read off the schemes it carries.
  An export older than the builder's 2025 contrast rules imports by its colours,
  and the screen says that some roles differ from the file.

A theme typed token by token in an earlier version is converted on upgrade into
the six colours that come closest to it: its primary stays, its surfaces and
outlines take the Material 3 tones.

Save is on this tab: a theme is an object of its own, and saving it is what
changes what a live theme serves.

## Layout

![The Layout tab: five arrangements, the logo size and the side, with the previews beside them](img/console/built-in-pages-layout.webp)

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

![Built-in pages on the Branding tab: application name, tagline, logo, tab icon and background drop zones, and the Meerkat mark card](img/console/built-in-pages-branding.webp)

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
  one. See [Configuration](/docs/console/configuration).
