---
title: Branding
section: Customising
order: 246
summary: Name, logo, favicon and the page background - including a different picture in light and in dark.
---

# Branding

The branding is the application's identity on the pages the gateway serves: the name a visitor
reads above the sign-in card, the mark in the browser tab, the picture behind it all (THEME-02,
THEME-06).

It is **global** - one identity per gateway, whatever theme is active. Themes are colour trials;
the identity does not fork with them. It is edited on the **Branding** tab of
**Application > Built-in pages**.

![The branding tab](img/console/built-in-pages-branding.webp)

## Name and description

The application name and a one-line description. A fresh installation ships with obvious
placeholders - `MY APP` and `My application description` - so it is clear that both are yours to
set rather than something to work around.

## The logo

An uploaded image, stored as a data URI. Accepted formats: PNG, JPEG, WebP, SVG and ICO. Past
about `195 KiB` the save is refused by name rather than silently truncated.

Its **size** is chosen, not guessed: normal, large or very large. A logo is not a fixed shape - a
square sentinel and a wide wordmark do not fill the same box, and the wordmark laid into the square
one comes out a third of its height. The alternative was to infer it from the image's aspect ratio,
which decides for you on a page that is yours. Three sizes, chosen once, next to the picture.

## The favicon

Optional, and deliberately so: left empty, **the logo serves as the tab icon**. A logo is nearly
always usable as one, and asking for a second image to see your own mark in the tab is a step most
people skip - after which the sign-in page of their application wears Meerkat's sentinel, which is
the one place it must not.

A favicon is a small square: past about `41 KiB` it is a full image somebody dropped in by mistake,
and it would ride on every page.

## The background

The picture behind the built-in pages belongs to the **branding** and not to a theme, on purpose: a
photograph of a building or a product shot is the application's identity, and it must survive the
colour trials a theme is.

| Field | What it does |
|---|---|
| Image | the picture, up to about `911 KiB` |
| Fit | `cover` fills the screen and crops, `contain` shows all of it, `tile` repeats it |
| Dim | how much of the surface colour is laid over the picture, from none to opaque |

**Dim earns its place.** Without it, any picture with a bright corner makes the sign-in card
unreadable in one scheme or the other, and the only fix left would be to edit the image.

The background is referenced by URL and never inlined into the page: it is the one asset here that
can weigh a megabyte, and a data URI would put it in every page of the flow instead of once in the
browser's cache.

## A different background in light and in dark

A photograph often reads in one scheme and not the other. So the dark scheme gets its **own**
image, its own fit and its own dim.

One switch decides which way it works:

- **one picture for both** - the light image is used in dark too, and the dark fields are ignored
  (the console disables them);
- **one picture each** - two images, two fits, two dims.

Uploading a light image and no dark one means "use it in both", which is the intuitive reading of a
single picture. Removing both puts the background back to *off*, one shape rather than "off with
settings still in it".

Under the hood, CSS cannot switch a `url()` through `light-dark()`, so the image follows the scheme
two ways: the system preference, and a class the server stamps when a scheme is imposed - which
outranks the preference, so the visitor's own choice holds even against their system's.

## The "powered by" mark

The served pages carry a discreet `powered by softwarity/meerkat` line. Removing it is a switch on
this screen.

> [!NOTE] Enterprise edition
> Hiding the mark is what the white-label feature grants. It is a **choice** rather than a side
> effect of holding a licence: an installation that never asked keeps the mark. Without the
> Enterprise image the switch is refused on save, naming why - a switch that saves and does nothing
> is worse than one that says it cannot.

Arranging the pages is sold with the same key - see
[the built-in pages](/#/docs/customise/built-in-pages).

## What is missing

The logo's own background colour was dropped rather than built: the background image replaced the
need for it (THEME-02).
