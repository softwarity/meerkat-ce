---
title: Routes
section: The console
order: 152
summary: The routing table in the order it is read, and how the route editor works.
---

# Routes

**Infra > Routes.** The whole routing table, in the order the gateway reads it,
and the editor every route is opened in. This is the screen you spend the most
time on.

![The Routes screen: five routes in order, with their access badges, what they match and their upstream](img/console/routes-list.webp)

Five routes in the order they are read. *Billing* and *Docs portal* carry the UI
mark, *Orders API* shows an upstream that has been failing, *Inventory* is in
maintenance, and *Catch-all* on `path: /**` sits last - a catch-all anywhere else
would answer for everything under it.

## The list

One row per route: its name, what it decides about access, what it matches, and
where it sends. Order is significant - **the first route that matches wins** -
so the table is ordered, not sorted.

- **The drag handle** moves a route up or down and saves at once. It only works
  on the whole list: with a search or a kind filter active the handle goes quiet
  and says why, because dragging row three above row one of a filtered view
  would move it above whatever really sits first.
- **The dot** before the name says enabled or disabled. A second mark appears
  for a **UI route** (one that serves pages in a browser), and a third when the
  gateway has actually seen the upstream fail.
- **The access badges** say what Meerkat itself requires: `AUTH` signed in,
  `ORG` in an organisation, `ORG-2` in one of two named ones, `DENY` nobody,
  a dash for delegated. A badge counting endpoints appears when the route
  refines access per operation, and clicking it opens
  [Endpoint security](/#/docs/console/endpoints).
- **Row actions**: open the route in the data plane (UI routes only), enable or
  disable, duplicate, delete.
- **The search** matches a route's name, its upstream and its path patterns -
  which is what you usually remember about it. The picker beside it narrows to
  UI routes or to service routes.

**Duplicate** makes a copy with a fresh identity, **disabled**, placed right
after the original. That is the intended way to try a variant: another upstream,
another organisation's access, compared side by side without retyping.

## Three buttons in the banner

- **Routing test** composes a fictional request and tells you which route takes
  it. Use it before moving anything: it answers the only question the order is
  about.
- **Global** holds what is true of every route at once: the **Unavailable**
  switch that closes all of them (sign-in pages keep working, and anyone who
  administers or develops here still gets through), how long any route waits for
  a service before answering 502, and the ceiling on what a body-rewriting
  filter may hold in memory. The button itself turns amber and reads
  *Unavailable* while the switch is on, so nobody has to open it to find out.
- **JWT** holds the keys that sign the identity JWT your services verify: the
  JWKS to hand a backend, each algorithm's public half, the rotation, and which
  routes sign with what.

## The editor

Clicking a row opens the route in the right drawer. The drawer is in the URL -
`/infra/routes/:id/:section` - so a section can be bookmarked and a refresh
comes back to it.

![The route editor open on Target, with the section list on the left](img/console/route-editor-target.webp)

The editor on *Orders API*: the section list on the left with its stars and its
counts, and the Target panel on the right - mode, upstream, the two waits, the
circuit breaker and the API contract.

The name sits in the header. The left column lists the sections, grouped:

| Group | Sections |
|---|---|
| - | **Target** |
| Filters | Security, Predicates, Gates, Rate limits |
| Modifiers | Incoming, Outgoing |
| Forwarders | Identity, Locales |
| UI | Color scheme, User button, User info, Injections |

### Reading the marks

- **A star** marks a section that is always required: Target and Predicates. It
  is a fact about routes, so it never goes away.
- **Red** marks a section with something missing right now. It goes away when
  the gap is filled.
- **A number in brackets** is how many items a section holds (three predicates,
  two gates).
- **The Security section** carries the level it poses (`AUTH`, `ORG`, `DENY`, or
  a dot when nothing is posed but users are excepted), so an open route and a
  closed one do not read alike in the list.

### What is missing, and Save

Save stays disabled until the route is valid **and** something has changed.
Beside it, a red **N to fix** button lists every gap: each line names the
section and jumps to it. There is no hunting through eleven sections for the
field that is wanted.

Saving keeps the drawer open and applies the route at once. Closing with unsaved
changes asks first, and while there are unsaved changes a click outside does not
close the drawer.

### Sections that go quiet

- The **UI** sections stay visible but disabled until the UI checkbox on their
  group is ticked. A route is always a service; UI comes on top.
![The Color scheme section of a UI route, with its mechanism, tag name and stored-theme override](img/console/route-editor-color-scheme.webp)

A UI section once the box is ticked: here Color scheme, which says how the served
application takes a light or dark choice.

- **Incoming** and **Identity** are disabled when the route answers by itself
  (redirect, maintenance, respond). This is not tidiness: the gateway drops
  every request filter on such a route, so editing them would write settings it
  throws away.

## The Target section

The mode decides everything else on the panel.

| Mode | What the route does |
|---|---|
| **Proxy** | Fetches from a service and hands back what it says |
| **Redirect** | Sends the browser elsewhere |
| **Maintenance** | Serves the built-in unavailable page |
| **Respond** | Builds an answer from a template, calling nothing |

On a proxy route you also set:

- **Upstream** - the scheme is chosen, never typed: `http` first because inside
  a cluster TLS ends at the gateway, `https` for a third party, `h2c` for a gRPC
  service. Services the gateway discovered are offered in the field.
- **When the service is slow or down** - connect and first-answer timeouts,
  which inherit the Global values unless set here. Past either the caller gets a
  502. What follows the first line is never bounded, so a download or a
  websocket runs as long as it needs.
- **Stop calling this service when it stops answering** - the circuit breaker:
  after N failures in a row, callers get the unavailable page at once, and after
  the delay the service is met by one request rather than by everything that
  piled up. A 500 does not count towards it.
- **The API contract** - no spec, one published by the service (a URL relative
  to the upstream), or one deposited here as a file. A spec is what unlocks the
  [endpoint screens](/#/docs/console/endpoints) and the developer swagger.

![The Predicates section: a path predicate with two patterns, a method predicate and a header predicate](img/console/route-editor-predicates.webp)

The bricks stack in a panel, each with its own explanation and its own remove
button. Here: two path patterns, four methods, and a header that accepts a short
list of values.

## Predicates, gates, filters

The sections that hold bricks are the vocabulary of routing, and they have their
own reference:

- **[Predicates](/#/docs/predicates/overview)** - the ways a route decides a request is for it.
- **[Filters](/#/docs/filters/overview)** - the ways it transforms one, and the gates that refuse it.

![The Incoming section: a strip-prefix filter and a set-request-header filter, each with arrows to reorder it](img/console/route-editor-filters.webp)

Filters apply in the order they are listed, and the arrows on each card move it
up or down.

## Common mistakes

- **Reordering with a search active.** The handle refuses and says so; clear the
  search first.
- **Expecting a new route to be reachable.** A duplicate is created disabled, on
  purpose.
- **Two routes matching the same paths.** Perfectly legal, and the reason order
  exists. Use the Routing test rather than reasoning about it.
- **Editing Incoming or Identity on a redirect.** The sections are disabled;
  what you want is probably Outgoing, which applies to every mode.
- **Looking for endpoint rules with no spec.** Declare the OpenAPI spec on the
  route's Target section first.
