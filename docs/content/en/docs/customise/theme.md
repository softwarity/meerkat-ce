---
title: The theme
section: Customising
order: 243
summary: Ten colour tokens, a light palette and a dark one, and how to carry a palette from one installation to another.
---

# The theme

A theme is a palette, and nothing else. It colours the pages the gateway serves itself - the
sign-in flow, the organisation picker, the password pages, the unavailable page - and the small
pieces it injects into proxied applications (THEME-03, THEME-04).

It is edited under **Application > Built-in pages**, on the **Theme** tab, with the real page
previewed beside it.

![The palette editor and its preview](img/console/built-in-pages-theme.webp)

## The tokens

Ten editable colours. Each one is emitted as a CSS custom property that the pages read; no page
hard-codes a colour.

| Token | CSS property | What it paints |
|---|---|---|
| primary | `--mk-primary` | the accent: buttons, links, focus |
| onPrimary | `--mk-on-primary` | text and icons on that accent |
| night | `--mk-night` | the tint of the ambient glow |
| surface | `--mk-surface` | the page itself |
| onSurface | `--mk-on-surface` | the main text |
| surfaceContainer | `--mk-surface-container` | the card |
| surfaceContainerHigh | `--mk-surface-container-high` | fields and raised areas inside it |
| onSurfaceVariant | `--mk-on-surface-variant` | secondary text, hints |
| outline | `--mk-outline` | borders and separators |
| error | `--mk-error` | refusals and invalid fields |

A value must be a hex colour: `#rgb`, `#rrggbb` or `#rrggbbaa`. Anything else is refused
by name - the block is emitted into a `<style>`, and nothing else may pass through.

## Light and dark are independent

There are two palettes, and neither is derived from the other. The pages emit a single token
block through the CSS `light-dark()` function, so the visitor's scheme picks the palette without
a second stylesheet.

A theme always leaves the store **complete**: a token you never touched is materialised at its
default, so the editor, the live preview and the served page all see the same palette. A
partially defined theme otherwise renders differently in each.

Four more tokens are structural rather than colours, and are not editable: the two corner radii,
the text font stack and the monospace one.

## The flat switch

One switch turns off the decorative effects of the built-in pages: the glows behind the logo, the
buttons and the status line, the ambient radial, and the gradient on the application name. It
drives them all through a single `--mk-glow` token, so there is nothing to tick one by one.

It lives on the palette because it is the same question: what does this page look like.

## Presets

Eight starting palettes ship with the product, and they share one surface and text system so that
only the **accent** changes between them: pick a hue and everything else stays coherent.

Sentinel's Watch, Midnight, Lavender, Orchid, Rose, Crimson, Ember, Forest. A fresh installation
gets all of them, with the first active. They are re-offered from the `+` button, so a palette you
wrecked is one click away from being recreated.

## Several themes, one active

Duplicate, tweak, preview, activate, roll back - the same philosophy as a saved configuration. The
active theme cannot be deleted: activate another one first.

The list keeps a stable order that does **not** depend on which theme is active, so activating one
does not reshuffle the tabs under your cursor.

## Carrying a palette elsewhere

The editor exports the palette as it stands, unsaved edits included, as a small JSON file: the two
palettes, the flat switch and the imposed scheme. Importing one **fills the editor** and does not
save - you review the colours in the preview, then save.

Only known tokens with a hex value survive an import, so a hand-edited or foreign file cannot
smuggle anything in.

## What is missing

- **Generating a palette from source colours** the Material Design way, a secondary palette, and
  elevations (THEME-04).
- **The console does not consume this palette.** It lives on its own Material tokens, so the
  console keeps the Softwarity look whatever you choose here (THEME-03).
