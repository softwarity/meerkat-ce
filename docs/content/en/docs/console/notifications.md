---
title: Notifications
section: The console
order: 183
summary: Turning notifications on at three levels, who may publish them, where users read them, what was sent to a tenant and who acknowledged it - and a drawer to send one yourself.
---

# Notifications

The services behind the gateway tell their users something, and the users see
it in whichever application they have open. This page is the administrator's
side: turning it on, letting services publish, choosing where users read, and
following what was sent. What a service sends, and what its users see, is on
[Notifying your users](/docs/operations/notifications). Enterprise edition.

## Turning them on

Three levels, and each one can only take away: a tenant receives notifications
when all three allow it.

| Level | Where | What it decides |
|---|---|---|
| root | **Data plane, Notifications, Settings** | the switch for the whole gateway, off at installation; then tenant by tenant; the audience cap |
| application admin | the same drawer | tenant by tenant |
| tenant admin (the owner, or an ADMIN member) | the tenant's **Notifications** section | their own tenant |

![The Settings drawer of Data plane, Notifications: the switch for the gateway, one checkbox per tenant, and the audience cap](img/console/notifications-settings.webp)

In the drawer, each tenant is a checkbox, read from your level:

- **ticked**: you allow it, and it receives them;
- **empty**: you turned it off;
- **a dash**: you allow it, and a level below turned it off - the tooltip says
  which. Clicking the dash turns it back on and clears what the levels below
  decided; they may turn it off again later;
- **greyed out**: a level above turned it off, and only that level turns it
  back on.

A new tenant is allowed at every level. On a gateway with a single tenant there
is no tenant level: root and the application admin decide.

A publication to a tenant that is off is refused, and the refusal names the
level that turned it off. A person gets the notifications page, its menu entry
and its badge as soon as one of their tenants receives notifications.

The **audience cap**, set by root, bounds how many people one notification may
reach: 5000 unless changed. Above it, publishing is refused.

## Data plane, Notifications

Root and the application admin follow here what every service sent, newest
first: the title and its level, the sender, the tenant, how many of its
recipients read it, and when it was sent. A withdrawn or expired notification
stays in the list, marked so.

![Data plane, Notifications: the notifications sent, with their sender, tenant and read count, and the Settings, API and Access buttons](img/console/notifications.webp)

A click opens one: its text, its sender and tenant, the address of its detail,
each action with its method and address, its metadata, when it was sent and
when it expires, and every recipient with the time they read it. **Withdraw**
takes a live one back: it disappears at once from every reader's page, and the
list of who read it is kept.

Three buttons sit in the header:

- **Settings** - the drawer above.
- **API** - *Publishing notifications*, for the developer of a service, in six
  folds: the endpoints and the token, the message field by field, every tenant
  at once, the responses, finding and withdrawing, and the acknowledgements
  with the call `onAck` makes.
- **Access** - [who may publish](#who-may-publish).

![The API panel, Publishing notifications: the endpoints and the token, then the message field by field](img/console/notifications-api.webp)

## Who may publish

Services publish on the internal plane, `POST :7070/api/notifications`. The
**Access** drawer decides what a caller must present, exactly as the
scheduler's does ([who may call it](/docs/operations/scheduler#who-may-call-it)):
no credential, a Meerkat token, or the pod's own ServiceAccount token on
Kubernetes, then everyone or only the callers listed. Each API has its own
setting: the services that notify are not always the ones that schedule.

A new installation asks for a **Meerkat token**: an internal token of perimeter
`notifications`, minted under **Infra, Access tokens, Internal** (**Mint a
token** in the drawer leads there). It publishes notifications and opens
nothing else. Changing the access takes root or an infrastructure
administrator; the application admin reads it.

## Where users read them

When root turned notifications on, **Application, Portal** shows a
**Notifications page** card:

- **Built-in** - `/meerkat/notifications`, served by the gateway in the theme
  of the sign-in pages;
- **Your page** - a page of one of your applications: a UI route, and a path
  under it such as `/#/inbox`. It is saved once a route is chosen.

The bar, the account menu, the snackbar and the mails lead to that page. A
person who may not open its route gets the built-in page, which stays open to
everyone.

**One entry, one badge.** In Portal mode, **Add notifications** puts a
**Notifications** entry in the bar. It is placed, moved and removed like the
others, and an empty label reads Notifications in each user's language. That
entry carries the one badge of unread notifications. Without it - in Links or
None mode, or a bar without the entry - the entry and the badge are on the
account button. The entries of the applications themselves carry no badge.

![The Portal screen with the Notifications page card, and the bar carrying the Notifications entry with its badge](img/console/portal-notifications.webp)

The notifications page is also among the
[built-in pages](/docs/console/built-in-pages) the theme editor previews, with
three snackbars stacked over it as the account button draws them.

## A tenant's Notifications section

**Tenants**, a tenant, **Notifications**: for root, the tenant's owner and its
ADMIN members, on a gateway with several tenants.

![A tenant's Notifications section: the tenant's switch, New notification, and what was sent with who read it and who acknowledged it](img/console/tenant-notifications.webp)

- **Notifications on for this tenant** is the tenant level. It is greyed out,
  with the reason, when root or the application admin turned the tenant off.
- The table lists what was sent to this tenant: the title, the sender, how many
  read it, and for one that asks to be acknowledged, where that stands - *by
  alice* or *Pending* when one reader is enough, *37 / 52* when every reader
  must. It follows live.
- A click opens one: its text, sender and times, who acknowledged it and who
  has not yet, every recipient with the time they read it, and **Withdraw**.

![A notification that asks one reader to acknowledge it: who closed it, and every recipient with the time they read it](img/console/tenant-notification-detail.webp)

### Sending one yourself

**New notification** opens a drawer to try a notification on this tenant, or
to send one by hand. It is offered while the tenant receives notifications,
and has two tabs.

**Edit** builds it:

- **Audience** - roles and members. Both empty: every member who may open its
  links.
- **Message** - the level, the title and the text.
- **Detail** - none, a route and a path under it, or an https address. The
  field shows the address the reader will open.
- **Actions** - up to five buttons: a label, a route and a path or an https
  address, and in front of them the **Method**: GET, or POST, PUT, PATCH or
  DELETE. Any method but GET opens **Body (JSON)**, a JSON editor; invalid JSON
  is shown on the field and holds Send back. The body reaches the reader's
  browser: never put a secret in it.
- **Acknowledgement** - None, One reader, or Every reader, and the route of
  your service to call at each acknowledgement.
- **More** - the lifetime, the key, the metadata, and Also send by mail (only
  when a mail relay is configured).

![The New notification drawer, Edit tab: a detail on the Billing route, and an action calling it with POST and a JSON body](img/console/notification-compose.webp)

**JSON** shows the exact message a service would send, coloured and ready to
copy: the quickest way to write the call your service will make.

![The JSON tab: the message as a service sends it, with its action's method and body](img/console/notification-compose-json.webp)

**Send**, from either tab, publishes it to this tenant at once, with you as the
sender. It goes through the same checks as a service's publication, and a
refusal comes back under the field it concerns, on the Edit tab.

## In the audit trail

| Action | Written when |
|---|---|
| `notification.publish` | an administrator sends one from a tenant's page |
| `notification.withdraw` | an administrator, or an agent, withdraws one |
| `notification.ack` | a reader acknowledges one; the reader is the actor |
| `settings.notifications` | the switch, a tenant's state or the cap changes, at root's or the application admin's level |
| `tenant.notifications` | the switch of a tenant's Notifications section changes |
| `settings.notifications-access` | who may publish changes |

An assistant connected to the gateway can list what was sent and withdraw a
notification, never publish one: a notification is a service telling its users
about its own work.
