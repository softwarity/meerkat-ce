---
title: Applications
section: The console
order: 176
summary: The catalogue of applications the gateway offers, and the three-state control that decides how it offers them.
---

# Applications

**Application > Portal.** The list of applications this gateway offers, in the
order you choose, and how it is offered. Each visitor sees only the entries
their access allows.

It is a **global** setting, like the theme: one catalogue for the installation,
edited here.

![The Portal screen: the three-state control, the arrangement controls, and the real bar in edit mode below](img/console/portal.webp)

## The three-state control

| | What is drawn |
|---|---|
| **None** | nothing. The user button and the built-in pages carry your brand's name, with nothing to click |
| **Links** | the list, in the user button's *Applications* submenu and on the data-plane pages |
| **Portal** | a navigation bar on every page of every application; the built-in pages then offer one way back in |

**Changing mode does not cost the list.** You can build a menu, promote it to a
bar, come back: the entries stay.

## What you do here

1. **Pick the mode.** Nothing below shows in *None*.
2. **Add an application** for each UI the list should offer. In *Links* they
   stack in a numbered list with two arrows for the order. In *Portal* they are
   built on the canvas, which is **the real bar in edit mode**: clicking an
   entry opens it in the drawer.
3. **Choose the arrangement** (*Portal* mode): *header mode* or *rail mode*,
   which side the rail takes, whether header entries show the icon, the label or
   both, and whether the application name sits beside the logo.
4. **Check it narrow** (*Portal* mode). The three width buttons put the preview
   in tablet and phone, to watch the overflow chevrons and the waffle launcher
   appear when the tabs no longer fit. Only full width is editable, and the
   width is never saved.

## An entry

- **Application (route)** - the UI route this entry leads to. Only **enabled UI
  routes** are offered. The entry inherits the route's access, and that is what
  makes the list differ per visitor: the payload carries no access rule.
- **Label** - empty takes the route's name.
- **Description** - the tooltip.

In *Portal* mode, what it takes to draw a bar is added:

- **Icon** - search the embedded icon set, or paste an SVG.
- **Home label** - the *back here* row of a submenu. Empty takes the label.
- **Sub-applications** - the drawer's quick action adds one.

The quick actions at the top of the drawer move an entry up or down, disable it
without removing it, or take it out.

## Pitfalls

- **A new UI route does not show up on its own.** It is offered only if it is in
  this list: that is what replaces the route editor's old *Link* field.
- **No UI route, no catalogue.** An entry needs a route that is enabled and
  marked UI in [Routes](/docs/console/routes).
- **A visitor sees fewer entries than you do**, deliberately: the list is
  filtered by each entry's route access.
- **The bar never shows in an iframe.** A page embedded in another is not the
  place for a navigation bar.
- **Switching to *Portal* changes every UI route at once.** The per-route user
  buttons lose their Applications submenu to the bar.
