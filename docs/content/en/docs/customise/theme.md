---
title: The theme
section: Customising
order: 243
summary: Six source colours, a contrast level and three typefaces, from which both schemes are generated the way Material Theme Builder generates them.
---

# The theme

A theme is how the pages the gateway serves look: the sign-in flow, the tenant picker, the
password and account pages, the unavailable and not-found pages, the mails it sends, and the
user button and portal bar it injects into the applications behind it.

It is made the way [Material Theme Builder](https://material-foundation.github.io/material-theme-builder/)
makes one, and it is the same theme: six source colours, a contrast level and a "color match",
from which every Material 3 role is generated, in light and in dark. Three typefaces and one
switch for the decorative effects complete it.

It is edited under **Application > Built-in pages**, on the **Theme** tab, with the real pages
previewed beside it.

![The theme editor: the source colours, the contrast and the typography on the left, the two previews on the right](img/console/built-in-pages-theme.webp)

The name, the logo and the background picture are not part of a theme: they are the
[branding](/docs/customise/branding), which stays the same whatever theme is active. The
arrangement of the page is the [layout](/docs/customise/built-in-pages). To see what the three
together can produce, see [six brands, the same pages](/docs/customise/overview).

## Six source colours

Only the **primary** is required. The others are derived from it until you set one; the editor
tags them **derived** and shows the value they are derived as, and a button puts a colour you set
back to derived.

| Source | What it paints on the pages |
|---|---|
| primary | the brand and the action: the main button, focus, links, the logo's halo |
| secondary | what is chosen or browsed: the selected option, the active language and scheme, the current tab of the portal bar |
| tertiary | state and information: the signed-in and current badges, satisfied password rules, explanations, codes to copy |
| error | refusals and invalid fields |
| neutral | the page, the card and the text |
| neutral variant | secondary text, hints, outlines |

## Both schemes are generated

On save, the gateway generates the **49 roles** of Material 3, for the light scheme and for the
dark one, from the source colours. It does it with the same algorithm as Material Theme Builder
(material-color-utilities), so the same six colours give the same schemes there and here, role
for role. The two schemes are generated together and therefore never disagree; the editor lists
every role, dark and light side by side, under **Generated roles**, read-only.

- **Contrast**: standard, medium or high, applied to both schemes.
- **Color match** is the builder's "stay true to my color inputs": the containers keep the tone of
  the colours you gave instead of the tones the specification derives. Without it, a very light or
  very dark brand colour comes back toned.

Which schemes the pages **offer** is decided in the same header: untick **Dark** or **Light** and
the pages stop proposing it. One of the two always stays. See
[light, dark, or the visitor's choice](/docs/customise/built-in-pages).

## Glow

One checkbox drives the decorative effects of the built-in pages: the ambient halo behind the
page, the glows on the logo, the buttons and the status line, and the gradient on the
application's name. Untick **Glow** for a flat design. In the API it is the theme's `flat` field.

A gradient name is a matter of taste, and of legibility: on a light background, a pale primary
makes a pale name. Flat paints it in one solid colour.

## Typography

Three typefaces, each one of the families the gateway ships, or none for the system's own:

| Slot | What it sets | Families |
|---|---|---|
| Display | the titles and the application's name | Inter, Roboto, Open Sans, Montserrat, Nunito, Raleway, Noto Sans, Source Serif 4, Lora, Merriweather, Playfair Display, Noto Serif |
| Body | the text and the buttons | the same twelve |
| Code | codes, keys, fields and labels | JetBrains Mono, Roboto Mono |

The files are in the binary and the gateway serves them itself: a visitor never contacts a font
CDN, an installation without internet access has them, and an application's
Content-Security-Policy (`'self'`) lets them through. A browser downloads a file only for a
character of its range, so a French page costs one Latin file.

The chosen family draws Latin and Cyrillic. Behind any choice, Noto draws Arabic, Hebrew,
Devanagari and Thai; Chinese, Japanese and Korean use the system's fonts. A mail carries the font
stack only, because most mail clients do not load web fonts.

## The preview

The right of the screen shows the page in both schemes, dark and light, and follows **as you
pick**, before anything is saved. A carousel above it walks through what a theme paints: the
sign-in specimen, every built-in page, the mails and the portal bar. Hovering a source colour or a
role makes what it paints blink in the preview: it is the fastest way to find out what a role
name means.

## Several themes, one active

Themes are colour trials; one is active, the one visitors see. The carousel between the two
previews is the list of themes:

- the **arrows** select a theme, to look at it and edit it; selecting does not serve it;
- the **pill in the middle** activates the selected theme, and warns while the theme under it is
  not the one visitors see;
- the **more** menu beside the name duplicates or deletes it. The active theme cannot be deleted:
  activate another one first.

Eight built-in themes ride in the carousel, read-only, all on the same base with a different
primary: Sentinel's Watch (the default), Midnight, Lavender, Orchid, Rose, Crimson, Ember and
Forest. A lock replaces the rename pencil; to start from one, duplicate it.

## Carrying a theme elsewhere

**Export** writes the Material Theme Builder JSON format: the source colours, the six schemes
(light and dark at three contrast levels) and the tonal palettes. What only Meerkat knows travels
under a key of its own, `meerkat`, which the builder's readers ignore: the name, the contrast, the
color match, the typefaces, Glow and the offered schemes.

**Import** reads a builder export, from Meerkat or from the builder itself, and **fills the
editor** without saving: you look at the result in the preview, then save. Whether Color match was
on is read off the schemes the file carries. An export older than the builder's 2025 contrast
rules imports by its colours, and the screen says that some roles differ from the file.

A theme is also part of a [configuration export](/docs/operations/backup-restore), with the rest
of the installation.

## The CSS properties

The pages read the theme as custom properties, emitted once per page with the light and the dark
value side by side (CSS `light-dark()`), so the visitor's scheme picks the palette with no flash
and no script:

- one per role: `--mk-` and the role's name in kebab case, such as `--mk-primary`,
  `--mk-on-surface-variant` or `--mk-surface-container-high`;
- `--mk-night`, the tint of the ambient glow, and `--mk-glow`, 1 or 0 as Glow is on or off;
- `--mk-font`, `--mk-mono` and `--mk-display`, the font stacks;
- `--mk-radius` and `--mk-radius-small`, the corner radii, which are not editable.

Every value the theme emits is a colour the gateway generated: nothing typed in the editor
reaches the page's style block as it was typed.

## What is missing

- **Elevations.** The Material 3 shadows are not generated.
- **Italic faces**, and adding a family from the screen rather than by shipping it.
- **The console does not wear the theme.** It keeps the Meerkat look on purpose: it is the
  operator's tool, and a badly set theme must not make unreadable the screen where it is fixed.
- **A theme per tenant.** The theme is global, like the sign-in it dresses.
