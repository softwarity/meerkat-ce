---
title: Portal
section: The console
order: 176
summary: The navigation bar the proxied applications wear, built against a live preview.
---

# Portal

**Application > Portal.** One bar to move between the applications this gateway
serves. It is injected into the proxied pages, and each visitor sees only the
entries their access allows.

It is a **global** setting, like the theme: one bar for the installation, edited
here.

When the portal is on, it **replaces the per-route user button** across every UI
route: the account button moves into the bar.

![The Portal screen: the arrangement controls, and the real bar in edit mode underneath](img/console/portal.webp)

Header mode, icon and label, the application name beside the logo. The canvas
under it is the bar itself: *Acme Corp* on the left, three modules, and the
sub-modules of the selected one shown as a rail.

## What you do here

1. **Turn it on.** Nothing below appears until you do.
2. **Choose the arrangement**: *header mode* or *rail mode*, which side the rail
   takes, whether header entries show the icon, the label or both, and whether the
   application name sits beside the logo.
3. **Add a module** per application the bar should reach. Modules are built on the
   canvas, which is **the real bar in edit mode**: clicking an entry opens it in
   the drawer.
4. **Check it narrow.** The three width buttons put the preview at tablet and
   phone widths, so you can watch the overflow chevrons and the waffle launcher
   appear when the tabs no longer fit. Only the full-width one is editable, and the
   width is never saved.

## A module

- **Application (route)** - the UI route this entry leads to. Only **enabled UI
  routes** can wear the bar, and only those are offered. The module inherits the
  route's access, which is what makes the bar different per visitor: the payload
  carries no access rule of its own.
- **Label** - empty uses the route's name.
- **Home label** (parents only) - the *back to here* row of a sub-menu. Empty uses
  the label.
- **Description** - the tooltip.
- **Icon** - search the bundled icon set, or paste an SVG.

The quick actions at the top of the drawer move a module up or down, add a
sub-module to a parent, disable it without deleting it, or remove it.

## Traps

- **No UI route, no portal.** A module needs an enabled route marked UI on
  [Routes](/#/docs/console/routes).
- **A visitor sees fewer entries than you do**, and that is the design: the bar is
  filtered by each module's route access.
- **It never shows inside an iframe.** A page embedded in another is not the place
  for a navigation bar.
- **Turning the portal on changes every UI route at once.** The per-route user
  buttons lose their Applications sub-menu to the bar.
