---
title: Audit and Issues
section: The console
order: 184
summary: The two screens read after the fact - who changed what, who signed in, and what your users reported.
---

# Audit and Issues

Two transverse screens, side by side in the rail, both read after the fact and
both scoped server-side to what the caller administers: root sees everything, an
infra admin the routing plane, an app admin the identity, a tenant admin their own
organisations.

## Audit

Every administrative change, with **the exact fields that moved and their before
and after**, and the security of the accounts: every sign-in, every refused one
with its real reason and address, every factor, passkey, password or token
changed by its owner. Read-only.

![The Audit screen: a list of events, each with its actor and the fields that moved](img/console/audit.webp)

Two configurations captured, two routes whose rate limits were written, a group
renamed, a tagline rewritten and a session TTL shortened - each with its before
struck through and its after beside it.

Each event reads as: when, what action, by whom, on what, and the field-level
diff underneath. Where an agent or a script acted, the token's name appears beside
the account's: *admin, via claude-desktop* rather than *admin*. That difference is
the whole point of naming it.

A security line reads the same way, with the reason or the method, and the
address under it. Its plane is the screen's: the applications' sign-ins under
Data plane, the console's under Meerkat. A
refused sign-in wears the error colour: scrolling for an attack, those are the
lines to find without reading. The full list of actions and reasons is in
[the audit trail](/docs/operations/audit).

The trail is read in two places. **Data plane, Audit** shows what happened in
the applications: their **Sign-ins**, and the **Operations** - the calls of the
operations a route audits (Enterprise), with the status, the path, the fields
the operation takes and the body when it keeps one. **Meerkat, Audit** shows the rest, with a toggle:
**All**, **Changes** or **Console sign-ins**. Under **Changes**, a **target** kind
narrows further. Then the **period** (24 hours,
7 days, 30 days, all time), and a free-text box that narrows what is already
loaded, reasons and addresses included.

At the top, **Export CSV** (Enterprise) downloads what the filters select, and -
for root alone, on Meerkat's - **Keep events for** sets how long the trail keeps
an event. See
[the audit trail](/docs/operations/audit).

- Secrets are redacted: the trail records that a field changed, not to what.
- A deleted account leaves an anonymised trace rather than a hole.
- The words a change is written with are the same ones the
  [restore point](/docs/console/configuration) for that change uses, at the same
  second. One event, one vocabulary.

## Issues

The reports your users file from the injected user button: a description, a
screenshot, and the context the browser captured.

At the top, **Collect issue reports** - an infra admin's switch, shipped **off**.
It sits here rather than on a screen of miscellaneous toggles, because an empty
list means nothing until you know whether anything is being collected. Off, the
entry disappears from the user button.

The list is light; opening a report fetches its detail into the drawer, and the
drawer is in the URL, so a refresh lands back on it. A report carries:

- Its **status** - open, in progress, closed - changed from the drawer, and a
  filter on the list.
- Who reported it, from which organisation, and when.
- The **URL**, the viewport and pixel ratio, the language, the user agent.
- The **screenshot**, clickable to open full size.
- The **console output** the page had captured, with each line's level.
- **Comments**, and a box to add one.
- Deletion, in the danger zone.

There is no connector to GitHub, GitLab or Jira: a report lives here.

## Traps

- **Audit is not a log of requests.** It records administrative changes and the
  ways into the accounts. What passed through the gateway is
  [Metrics](/docs/console/traffic), and each request is a line of the
  [access log](/docs/operations/logs).
- **You see your own perimeter.** Two administrators can read the same screen and
  count a different number of events; that is the scoping, not a bug.
- **An empty Issues list may mean collection is off.** Check the switch before
  concluding your users have nothing to say.
- **A screenshot is the page as the user saw it**, and it may carry their data.
  Treat a report as personal data.
