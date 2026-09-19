---
title: Light and dark
section: Customising
order: 255
summary: How to tell the gateway what your application can do with a colour scheme, and how to work with one that keeps its own.
---

# Light and dark

A visitor picks light or dark once, in the user button, and expects the whole thing to follow:
Meerkat's own pages **and** the application behind them. The pages are ours to paint. The
application is yours, and no two of them read a scheme the same way - which is why the mechanism is
declared **on the route** rather than guessed (UIF-03).

This page is written for the person integrating an application. For the pages Meerkat serves
itself, see [the built-in pages](/docs/customise/built-in-pages).

## Where the choice lives

The visitor's choice is a cookie, `MEERKAT_SCHEME`, holding `light`, `dark` or `auto`, kept for a
year and `SameSite=Lax`. It is also stored on their account, so it is put back on a browser that
has never seen them.

`auto` means "follow the system", and it is the default: nothing is imposed until somebody chooses.

## What the gateway always does

On a UI route whose scheme is not `none`, the gateway injects a small agent. When a scheme is
chosen, it sets on `<html>`:

- the CSS `color-scheme` property, so form controls, scrollbars and the default canvas follow;
- `data-meerkat-scheme="light"` or `"dark"`, for an application that would rather read an attribute
  than a computed style.

On `auto` both are removed, and the browser is back to following the system.

That alone is enough for an application whose CSS is written on `prefers-color-scheme` or on
`light-dark()`. Everything below is for applications that switch some other way.

## Declaring the mechanism

Four answers, and one of them is "there is nothing to switch".

![Declaring the colour scheme on a route](img/console/route-editor-color-scheme.webp)

| Mechanism | What the gateway writes | Typical markup |
|---|---|---|
| *(none chosen)* | the CSS `color-scheme` only | your CSS reads `prefers-color-scheme` |
| `attribute` | **one** attribute, named by you, always written, to the light value or the dark one | `<html data-theme="dark">` |
| `add-attribute` | the two values **are** attribute names, added bare and removed like classes | `<body dark-theme>` |
| `class` | the two values are class names, the two removed and the current one added | `<body class="dark">` |
| `none` | nothing at all | light and dark mean nothing to this UI |

Two more fields shape it:

- **the tag**, `html` unless you say otherwise. An application that reads its theme on `<body>`
  never saw it on `<html>`, and that is the single most common reason a switch appears to do nothing.
- **the light value and the dark value.** For `attribute` they are the attribute's two values; for
  the other two they are the names themselves.

Which is why an **empty value means something** in the last two: nothing on the tag in that state.
That is the most widespread shape there is - nothing in light, `dark` in dark:

| Mechanism | light | dark | Result |
|---|---|---|---|
| `class` | *(empty)* | `dark` | `<body>` in light, `<body class="dark">` in dark |
| `add-attribute` | *(empty)* | `dark-theme` | `<body>` in light, `<body dark-theme>` in dark |
| `attribute` on `data-theme` | `light` | `dark` | always written, one or the other |

The tag may not be parsed yet - the agent runs from the head, on purpose, so `<html>` is dressed
before the first paint. A `<body>` that does not exist yet is caught up with once the document is
parsed, and never written on `<html>` instead: a class left on the wrong element is a theme nothing
removes.

## What "none" is, and what it is not

`none` says this UI has **no colour scheme of its own**. It is not "takes the CSS `color-scheme` and
nothing more" - that is leaving the mechanism unset. It is: light and dark mean nothing here.

The switch is then not offered in the user button, and the agent leaves the document alone.

## Offering the switch, and dressing the button

Two things that used to travel together and are now separate:

- **offering the switch** is chrome. It belongs to the user button, and a route can offer it or not.
- **how the application consumes a scheme** belongs to the route, and holds whoever offers the
  switch - including a [portal](/docs/customise/portal) bar, where there is no per-route button to
  hang it on.

When a route does **not** offer the switch, it says instead what the injected button itself wears:
light, dark, or the visitor's own choice. That exists for the application with one look and no
switch, where the button otherwise follows the visitor's system and floats light on a dark page. It
dresses the chrome alone; the page is never touched.

## Applications that keep their own theme

Here is the case that bites, and the reason this page exists.

An application that remembers its own light or dark in `localStorage` **restores it on load**, over
whatever the gateway just applied. The visitor's choice holds until the app's own script runs, then
snaps back. The same portal then looks right in one browser and wrong in another, and what differs is
only what that browser had in store.

Fighting it on the document does not work. The way to work **with** it is to speak its own storage.

| Field | What it is |
|---|---|
| Override its storage | the switch. Without it the gateway never touches the application's storage |
| Key | the `localStorage` key the application keeps its choice under: `theme`, `color-mode`, `vuetify:theme`... |
| Light value | what to write for light. Defaults to `light` |
| Dark value | what to write for dark. Defaults to `dark` |
| Auto value | what "follow the system" is called there. **Empty removes the entry** instead |

The values are the application's **own vocabulary** - `dark`, `night`, `1` - and the gateway does not
guess them: guessing would be writing a dialect we do not speak.

An empty auto value is a choice of its own: with nothing stored, the application falls back to its own
default, which for most of them *is* the system.

The write happens in an inline script placed **before** the application's boot script, so the value is
already in place when the app first reads it - and the chrome then simply inherits the document.

> [!TIP]
> `ng-m3-theme` from the Softwarity ecosystem is the case that taught this: its service keeps
> `system`, `light` or `dark` under a key, and in `system` mode it **clears** the document's
> `color-scheme` on every run. Anything the gateway set was wiped a tick later. Writing the choice
> into that key instead lets the application apply it the way it already knows how, before its first
> paint - so here the auto value is `system`, not empty.

## Debugging it

What the route declared travels on the agent's own script tag, so one look at the served HTML says
what the gateway thinks it was told:

```html
<script defer src="/meerkat/page.js"
        data-scheme="select"
        data-scheme-mechanism="class"
        data-scheme-tag="body"
        data-scheme-light=""
        data-scheme-dark="dark"
        data-scheme-storage="theme"
        data-scheme-storage-light="light"
        data-scheme-storage-dark="dark"
        data-scheme-storage-auto="system"></script>
```

If a field you filled in is absent there, the route did not save it. If it is present and nothing
happens, the tag is usually the answer.
