---
title: The pages the gateway serves
section: Authentication
order: 118
summary: Sign-in, second factor, password change, organisation choice - the pages Meerkat draws itself, and how far you can restyle them.
---

# The pages the gateway serves

Everything a person has to be shown before they reach your application is served
by the gateway itself: no library in your app, no page to write, no template to
deploy. They are compiled into the binary, they carry your colours and your logo,
and they speak twenty languages.

They are also all `Cache-Control: no-store`: a page that names a person must
never sit in a shared cache.

## The pages

| Path | What it is |
|---|---|
| `/login` | the sign-in page: the credential form, one button per identity provider, the passkey button, and the links to sign up or recover a password when those are open |
| `POST /logout` | ends this session and comes back to the sign-in page |
| `/totp` | the second-factor challenge, with *trust this browser* and the code-by-e-mail link when they are available |
| `/totp-enroll` | forced enrolment, when a second factor is required and the account has none |
| `/update-password` | the forced change: a temporary or expired password |
| `/forgot-password`, `/reset-password` | recovery by e-mail, and the page the link lands on |
| `/register`, `/confirm` | self-registration and address confirmation |
| `/select-tenant` | which organisation this session works in |
| `/select-group` | which group, in exclusive-group mode |
| `/account-pending` | the waiting room: an account that exists and has been granted nothing yet |
| `/refused` | signed in, and turned away. It names the rule that refused and offers what this session *can* open |
| `/profile/...` | the person's own pages: identity, password, second factor, passkeys, authorities, sign-in history, API tokens |

The unavailability page has no path of its own: it answers **any** path with
`503` while maintenance is on, names the reason from a closed list, and gives an
administrator a way through.

The console's sign-in page is the same machinery on the admin port, with one
difference: it is always English and always dark. The console is Meerkat's own
door and does not take the integrator's settings.

## Colours

**Application > Built-in pages > Theme.** Ten colours, edited as hex values:
accent and its text, the surfaces (page, card, field), the text and its muted
variant, the outline, and the error colour. They are emitted as CSS variables -
`--mk-primary`, `--mk-surface`, `--mk-on-surface`... - once per page, with the
light and the dark value side by side, so the visitor's scheme picks the palette
with no flash and no script.

Several themes can coexist; exactly one is active. Duplicate one, edit it,
preview it, activate it, roll back. Eight presets ship, all on the same base with
a different accent: Sentinel's Watch, the default, then Midnight, Lavender,
Orchid, Rose, Crimson, Ember and Forest. A *Glow* checkbox turns the effects and
the gradient off in one gesture for a flatter look.

Only hex colours are accepted. Nothing else reaches the page's style block.

## Light and dark

The visitor's choice is a three-state button on the page - follow the system,
light, dark - stored in the `MEERKAT_SCHEME` cookie for a year **and** on the
account, so a browser that has never seen this person still gets their scheme
after they sign in.

To take the choice away, untick a scheme above its colour column in the theme
editor: the remaining one is imposed and the button disappears.

## Layout and branding

**Layout** picks the arrangement: *centered*, the card in the middle of the page;
*split*, a picture over a full-height half; *drawer*, an opaque panel against one
edge of a full-frame picture; *banner*, a brand band across the top; *bare*, the
fields on the picture with no card. For *split* and *drawer* you also choose a
side.

> [!NOTE] **Enterprise edition.**
> Only *centered* is in the community image. The other four arrangements come with
> the Enterprise edition, and so does hiding the *powered by softwarity/meerkat*
> mark. A community image handed an Enterprise layout draws the centred one rather
> than a page nothing dresses, and going back to *centered* is always allowed.

**Branding** carries the application name and tagline, the logo (uploaded, drawn
at normal, large or extra-large size), a browser-tab icon - which follows the
logo when you leave it empty - and a page background image with its framing
(cover, contain or tile), a dimming veil, and a separate picture for dark mode if
you want one.

A page displayed inside somebody's iframe drops the brand and fills the frame by
itself. That is decided in the browser, so the HTML served is the same for
everyone.

## Languages

Twenty catalogues are built in: Arabic, German, English, Spanish, French, Hebrew,
Hindi, Indonesian, Italian, Japanese, Korean, Dutch, Polish, Portuguese, Russian,
Thai, Turkish, Ukrainian, Vietnamese and Simplified Chinese. English is the
reference: a key missing from another catalogue falls back to the English
sentence rather than showing blank.

Which of them are offered is up to you, in **Application > Locales**. The offer
is your list intersected with what is built in, and it never ends up empty -
English is the floor.

For one request, the language is picked in this order:

1. the `MEERKAT_LANG` cookie, if it names a language you offer;
2. `Accept-Language`, matched on the language tag;
3. the first language you offer, and failing that English.

Someone's own choice is written to their account and reposed into the cookie
when they sign in, so their language follows them to a new browser. Arabic and
Hebrew are rendered right to left.

> [!NOTE]
> The catalogues are compiled into the binary. Adding a language, or changing one
> sentence of an existing one, is a rebuild - there is no directory to drop a JSON
> file into. FEATURES.md lists overridable catalogues as still missing.

## What you cannot change

**Your own HTML.** The pages are templates inside the binary; the seams are the
theme, the layout, the branding and the language catalogues. There is no
template-override setting and no file the gateway reads at startup. If you need a
sign-in page that is entirely yours, the honest answer today is that Meerkat does
not do that yet.
