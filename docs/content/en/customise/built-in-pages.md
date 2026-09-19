---
title: The built-in pages
section: Customising
order: 249
summary: Which pages the gateway serves itself, how they are arranged, and in how many languages.
---

# The built-in pages

These are the pages Meerkat answers with itself, in front of your applications. They are plain
server-rendered HTML - no framework, no bundle, nothing fetched from a CDN - and they read the
theme's tokens, so they follow your palette without a rebuild (PAGE-01).

| Page | Where it appears |
|---|---|
| `/login` | the sign-in form, and the buttons of the external authorities |
| `/totp` and `/totp-enroll` | verifying and enrolling a second factor |
| `/update-password` | a password the gateway requires you to change |
| `/forgot-password`, `/reset-password` | the reset flow |
| `/register`, `/confirm`, `/account-pending` | self-registration and its confirmation |
| `/select-tenant`, `/select-group` | picking the organisation or the group to act in |
| `/refused` | why a caller may not pass |
| `/profile` and its sub-pages | the account's own pages: password, second factor, passkeys, authorities, history, tokens |
| the unavailable page | a route whose service is down, or the global switch |

## One screen, three tabs, one preview

Colours, arrangement and identity were three jobs, each with its own options on one side and the
**same** preview on the other, each showing a page the other two also decide. They are one
subject - what the visitor sees - so they are one entry with three tabs, and the tabs are on
the left only.
The preview never moves, and the theme carousel stays under it on all three: trying a colour while
judging an arrangement is the normal way round, not a special case.

The tabs are real URLs, so a bookmark on the layout gallery comes back to the layout gallery.

## The arrangement

A closed catalogue of layouts, each a block of CSS shipped with the product (PAGE-02):

| Layout | What it looks like |
|---|---|
| `centered` | the brand above, the card in the middle, the background behind everything. The default |
| `split` | the image takes a full-height half of the screen with the brand on it, the form takes the other |
| `drawer` | the image stays whole and an opaque panel is laid against one edge, carrying brand and form |
| `banner` | the brand in a band across the top, the card under it |
| `bare` | no card at all: the fields sit on the background |

`split` and `drawer` are built of two halves, so they carry a **side**: left or right. The others
ignore it, and the field is cleared rather than kept and silently unused.

> [!NOTE] Enterprise edition
> Changing the arrangement is part of white-label, the same purchase as removing the mark: making
> these pages look like your product rather than ours.

Three things follow, and each one is deliberate. A settings save carries the whole payload, so
every other screen sends the current layout back untouched - the gate is on the **change**, not on
the value, or saving a language on a community instance would be refused. An arrangement already
in place **keeps being served** if a licence lapses: the model is perpetual, and an expired file
silently redrawing every customer's sign-in page is the "it worked yesterday" this product
refuses. And going back to `centered` is **always** allowed, or an instance could be stranded on a
layout it cannot leave.

## Light, dark, or the visitor's choice

The integrator decides first, by unticking a scheme on the preview (THEME-05):

| Setting | What the pages do |
|---|---|
| the visitor decides | their system to begin with, then a switch that remembers |
| light | light only, and the switch disappears |
| dark | dark only, and the switch disappears |

Imposing one is not a whim. These pages sit in **front** of an application that may only know one
look: a sign-in page following the visitor's dark system, handing over to a portal that is
light-only, reads as two products - and you cannot fix that from your side.

When the visitor does choose, the choice lives in a cookie for a year **and** on their account, so
it is put back on a browser that has never seen them - exactly like their language. See
[light and dark](/#/docs/customise/color-scheme) for what a proxied application does with it.

## Languages

Twenty catalogues are embedded in the binary, one JSON file per language: Arabic, German, English,
Spanish, French, Hebrew, Hindi, Indonesian, Italian, Japanese, Korean, Dutch, Polish, Portuguese,
Russian, Thai, Turkish, Ukrainian, Vietnamese and simplified Chinese (I18N-01).

English is the reference: every other catalogue is compared to it at startup, and a key a
catalogue does not carry falls back to English rather than showing a blank. Backend error messages
are localised the same way (I18N-02).

The language comes from the visitor's choice - a cookie and their account - and otherwise from what
their browser asks for. Which languages are *offered* is set under **Application > Locales**, and a
UI route declares which of that reserve it serves.

> [!WARNING]
> The console itself is **English only**, and that is a decision rather than a gap (I18N-03): it is
> an operator's tool. The pages above are the ones your users see, and those are translated.

## What is missing

- **Your own HTML.** The arrangement is a catalogue; replacing a page's markup with your own
  template is the other half of PAGE-02 and is not built.
- **Overridable catalogues** (PAGE-03): adding a language is still a rebuild.
- The developer's variant-selection page (DEV-06), which only means something once per-developer
  override scoping exists.
