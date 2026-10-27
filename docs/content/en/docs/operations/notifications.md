---
title: Notifying your users
section: Operations
order: 214
summary: A service behind the gateway tells its users something, and they see it in whichever application they have open - with a count, a snackbar, a desktop notification, and an acknowledgement when you ask for one.
---

# Notifying your users

A service behind the gateway has something to tell its users: an invoice is
overdue, a cold room is too warm, an export is ready. It sends **one request**
to the gateway, and the people concerned see it in whichever application they
have open. No push service to run, no list of devices to keep, no page to
write. Enterprise edition.

This page is for the developer of that service, and says what its users see.
The screens an administrator uses are on
[Notifications in the console](/docs/console/notifications).

## What your users see

- **A count** on the account button, or on the **Notifications** entry of the
  portal bar when the catalogue has one. One count for all their tenants.
- **A snackbar** on the page in front of them when a notification arrives: the
  level's icon, the title, **Open**, and **Acknowledge** when it asks for it.
  It goes away after 6 seconds, waits while the pointer or the focus is on it,
  and three stack at most.
- **A desktop notification** when the tab is not in front of them, once they
  turned it on with **Show on my desktop** on their notifications page, and only
  on a secure address (https, or localhost). Its buttons are **Open**, then your
  actions, run in the background with the result shown on the notification,
  and **Acknowledge** when asked. A click on it opens the notifications page.
- **The notifications page**, `/meerkat/notifications`, or the application's
  own page when the administrator chose one. Newest first. A click marks a
  notification read and unfolds it when there is more to see (a chevron says
  so): the whole text, **Open** for the detail, and your actions, each showing
  its result.
- **A mail**, when you ask for one and a mail relay is configured: the text, and
  a button to the notifications page. Never an action.

![The notifications page of a reader in two tenants: the tenant filter, a message sent to both tenants on one row, one waiting for an acknowledgement, and one unfolded with Open and its action](img/console/notifications-page.webp)

On the page, **Delete** takes a notification off the reader's list for good;
the sender and the other readers keep theirs. **Mark all as read** and
**Delete all** act on what the list shows. Everything follows live, in every
tab and on every device of the reader.

**Several tenants.** A reader receives the notifications of all their tenants.
Each row then names its tenant, and a filter narrows the list to one. A
notification from another tenant than the one they are working in can be read
and deleted; its detail and its actions wait until they switch to that tenant,
and the button says so. A message sent to every tenant is **one** row naming
them, counted once, read and deleted once.

**Withdraw and Delete are not the same thing.** Withdraw is the sender's, or an
administrator's: the notification leaves every reader's page at once. Delete is
the reader's: it leaves their own list and nobody else's.

## Who receives it

The audience is decided **once, at publication**, and kept: one line per
reader. It starts from the enabled members of the tenant, narrowed to the
`roles` and `users` you name (either matches; neither means every member), and
keeps only the people who may **open what it leads to**:

| The notification has | It goes to |
|---|---|
| a detail on a route | those the route lets in |
| actions on routes, and no detail | those who may use at least one action |
| neither, or links outside the gateway only | everyone named |

Rights are checked again each time a page shows it: someone who left the
tenant, or lost the role, no longer sees it, and an action a reader may not use
is not offered to them. A publication that would reach more people than the cap
an administrator set (5000 unless changed) is refused.

## Publishing

On the **internal plane**, port 7070, like
[scheduled calls](/docs/operations/scheduler): called from inside the cluster by
the gateway's internal name, never from a browser.

| | |
|---|---|
| `POST /api/notifications` | publish |
| `GET /api/notifications` | yours that are still live, newest first, filtered by `meta.<key>` |
| `GET /api/notifications/{id}/acks` | who acknowledged one of yours, and who has not yet |
| `DELETE /api/notifications/{id}` | withdraw one of yours |
| `DELETE /api/notifications?meta.<key>=<value>` | withdraw every one of yours that matches |

**Who may call it** is set in the console, **Data plane, Notifications,
Access**: the scheduler's three modes (no credential, a Meerkat token, the pod's
own ServiceAccount token on Kubernetes), then everyone or only the callers
listed. A new installation asks for a **Meerkat token**: an internal token of
perimeter `notifications`, minted under **Infra, Access tokens, Internal**. It
publishes notifications and opens nothing else. The **API** button beside
Access sums up this page for a service developer.

The name the access gives your service is the **sender**. "Yours" on this page
means sent under that name.

### The message

64 KiB at most; an unknown field is refused.

| Field | |
|---|---|
| `tenant` | the tenant, by id or name; optional when there is only one. `"*"` sends it to [every tenant](#every-tenant-at-once) |
| `roles` | only members holding one of these roles |
| `users` | only these members, by username. With `roles`, either matches |
| `title` | required. Plain text, 200 characters at most |
| `body` | plain text, 4000 characters at most |
| `level` | `info` (default), `success`, `warning` or `error` |
| `detail` | where it leads: a [link](#links-a-service-and-a-path), on a UI route |
| `actions` | buttons, 5 at most: a `label` (60 characters), a link, and [how to call it](#actions-a-method-and-a-body) |
| `mail` | `true` to send it by mail too. Refused when no mail relay is configured |
| `expires` | a duration (`P7D`, `PT12H`) or a date (`2026-11-01T08:00:00Z`): 30 days by default, 90 at most. Then it leaves every list |
| `key` | your deduplication key, 200 characters. Sent again to the same tenant while the first is live, it answers that one and notifies nobody again |
| `meta` | your labels, such as `{"invoice": "INV-2026-0142"}`, to find and withdraw your own. Never shown to readers |
| `ack` | `none` (default), `single` or `all`: see [Acknowledgement](#acknowledgement) |
| `onAck` | your service, called at each acknowledgement. Needs `ack` |

The texts are in **one language** for every reader: write them in the language
your users share. An object by language is refused.

### Links: a service and a path

You do not know the gateway's routes, and you do not have to. Name a link the
way you call it inside the cluster: the **service**, and the **path as the
service sees it**. The gateway finds the route in front of that service, undoes
its rewrites, and gives the reader the public address.

```json
{ "service": "billing", "path": "/invoices/INV-2026-0142" }
```

`billing` is the host of the route's upstream, or its first label:
`billing.svc.acme.internal` is found by `billing`. Behind a route that matches
`/billing/**` and strips its first segment, the reader gets
`/billing/invoices/INV-2026-0142`. Behind one that keeps its path, the service
sees `/billing/invoices/...` itself; a path given from inside the application,
`/invoices/INV-2026-0142`, is placed under the route's entry all the same.

A link is refused, with the reason, when no enabled route serves that service,
when several do (the gateway never picks one at random), or when the route
rewrites its paths in a way that cannot be undone (`rewrite-path`, `set-path`).

Outside the gateway, give `{"url": "https://..."}`: https only.

### Actions: a method and a body

An action is a request the **reader's browser** makes through the gateway, with
the reader's session. Your service sees the reader, with the identity the route
forwards, and decides. The address of an action on a route carries
`notification=<id>`, so you know which notification the click came from.

| Field | |
|---|---|
| `method` | `GET` (default), `POST`, `PUT`, `PATCH` or `DELETE` |
| `body` | any JSON value, sent as is with `Content-Type: application/json`. Not with `GET` |

```json
{
  "label": "Send a reminder",
  "service": "billing",
  "path": "/invoices/INV-2026-0142/remind",
  "method": "POST",
  "body": { "channel": "email" }
}
```

> [!WARNING]
> The reader's browser receives the body in order to send it: **never put a
> secret in it**. What proves the call is the reader's session, not the body.

The body counts in the message's 64 KiB. An action on an `{"url"}` outside the
gateway is sent without the session; with a method other than `GET` and a JSON
body, the browser first asks that server for permission (a CORS preflight),
which it has to grant.

### Every tenant at once

`"tenant": "*"` sends one copy to every enabled tenant whose notifications are
on, each with its own id and its own readers. A tenant where nobody may receive
it is skipped, and the answer says so. The users named in `users` are taken in
the tenants they belong to. A reader who is a member of several of these
tenants sees **one** notification naming them, with one snackbar, one desktop
notification and one mail.

The key is kept per tenant: sent again with the same key, the copies still live
are answered and the missing ones are published. A publication cut halfway
finishes by sending it again.

### A full example

```bash
curl -X POST http://meerkat:7070/api/notifications \
  -H "Authorization: Bearer $MEERKAT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant": "Acme Logistics",
    "roles": ["BILLING"],
    "level": "warning",
    "title": "Invoice INV-2026-0142 is overdue",
    "body": "Northwind Traders has not paid the 4820.00 EUR due on 2026-10-20.",
    "detail": { "service": "billing", "path": "/invoices/INV-2026-0142" },
    "actions": [
      { "label": "Send a reminder", "service": "billing",
        "path": "/invoices/INV-2026-0142/remind",
        "method": "POST", "body": { "channel": "email" } }
    ],
    "key": "overdue-INV-2026-0142",
    "meta": { "invoice": "INV-2026-0142" },
    "ack": "single",
    "onAck": { "service": "billing", "path": "/notifications/acked" }
  }'
```

```http
HTTP/1.1 201 Created
Content-Type: application/json

{"id": "ntf_5c0e9a1f2b7d4e6a8c3b1d0f", "recipients": 4}
```

### The answers

| Status | Body | Meaning |
|---|---|---|
| `201` | `{"id", "recipients"}` | published |
| `200` | the same, with `"existing": true` | the key is already live: nobody is notified again |
| `400` | `{"error"}` | not JSON, an unknown field, or over 64 KiB |
| `422` | `{"error"}` | refused: the sentence names the field and what is allowed |
| `401`, `403` | `{"error"}` | the access refused the caller: what is missing, and who is allowed |

Some refusals, as they are written:

```text
level "urgent": allowed are info, success, warning, error
tenant "Acme": no such tenant; there are Acme Logistics, Acme Retail
notifications are off for the tenant "Acme Retail", turned off by the tenant admin: only that level turns them back on
users: zoe not members of this tenant
action 1 method "FETCH": allowed are GET, POST, PUT, PATCH, DELETE
action 1: a GET sends no body; choose POST, PUT, PATCH or DELETE to send this one, or remove the body
nobody in this audience may open what the notification leads to: check the roles and accounts named, and the access of the routes behind its links
```

Sent to every tenant, the `201` lists the copies and the tenants left out:

```json
{
  "recipients": 37,
  "tenants": [
    { "tenant": "7c1e2a90", "name": "Acme Logistics", "id": "ntf_5c0e9a1f2b7d4e6a8c3b1d0f", "recipients": 21 },
    { "tenant": "b2f4c8d1", "name": "Acme Retail", "id": "ntf_e81d07c3a9b24f5e6d1c8a72", "recipients": 16 }
  ],
  "skipped": [
    { "tenant": "40aa9e3b", "name": "Acme Labs", "reason": "notifications are off, turned off by the tenant admin" }
  ]
}
```

When no tenant received it, the answer is a `422` naming each tenant and why.

## Acknowledgement

`ack` asks the readers for more than reading:

- `single`: the first reader to acknowledge closes it for everyone, as for an
  on-call alert. The others see who did.
- `all`: each reader acknowledges their own, and it is complete when the last
  one did, as for a safety instruction.

While a reader owes it, the row is framed, carries **Acknowledge**, and cannot
be deleted; **Delete all** keeps it and says how many it kept, and **Mark all
as read** never acknowledges. The snackbar and the desktop notification carry
**Acknowledge** too. Once done, the row says when, and in `single` the other
readers see who did, live. A message sent to every tenant is acknowledged whole,
from a session in any of the reader's tenants; in `single`, one acknowledgement
closes every copy. A withdrawn notification asks for nothing any more.

Each acknowledgement is a line of the [audit trail](/docs/operations/audit),
`notification.ack`, with the reader as the actor. It stays after the
notification expired.

**`onAck`** names your service, `{service, path}` only. At each acknowledgement
the gateway sends it a POST through its route, as the user `meerkat` in the
notification's tenant, with no role, like a scheduled call: if the route or an
endpoint rule asks for a role, let that path through. Anything but a 2xx is
tried twice more.

```http
POST /notifications/acked
Content-Type: application/json
Meerkat-Notification: ntf_5c0e9a1f2b7d4e6a8c3b1d0f
Meerkat-Attempt: 1

{"notification": "ntf_5c0e9a1f2b7d4e6a8c3b1d0f", "tenant": "7c1e2a90",
 "tenantName": "Acme Logistics", "key": "overdue-INV-2026-0142",
 "meta": {"invoice": "INV-2026-0142"}, "ack": "single",
 "by": {"id": "u_41f2", "username": "alice", "name": "Alice Martin"},
 "at": "2026-10-25T09:12:44Z", "acknowledged": 1, "recipients": 4, "complete": true}
```

`complete` is `true` on the acknowledgement that closed it: the first one in
`single`, the last one in `all`.

**Where it stands.** Your list carries `ackState` (`pending` or `done`),
`acknowledged` (how many readers did) and, in `single`, `ackBy`.
`GET /api/notifications/{id}/acks` gives it person by person: `acks` (who and
when, in order) and `missing` (who has not yet). A notification that asks for no
acknowledgement answers `409`.

## Finding and withdrawing yours

`meta` finds your notifications again without keeping their ids:

```bash
curl "http://meerkat:7070/api/notifications?meta.invoice=INV-2026-0142" -H "$AUTH"
curl -X DELETE "http://meerkat:7070/api/notifications?meta.invoice=INV-2026-0142" -H "$AUTH"
```

`meta.<key>=<value>` matches a value, `meta.<key>=~<regex>` a regular
expression (RE2), and every condition must match. Withdrawing by filter needs at
least one: everything a sender ever sent is not one request. The answer lists
what was withdrawn, `{"withdrawn": ["ntf_..."]}`, and those notifications leave
every reader's page at once. The console keeps them, marked Withdrawn, with who
read them.

## A notifications page of your own

The built-in page is one choice. An application may show the notifications in
its own screens: the administrator then points the gateway at that page
(**Application, Portal, Notifications page**: a UI route and a path under it),
and the bar, the account menu, the snackbar and the mails lead there. Someone
who may not open that route still gets the built-in page, which stays open to
everyone.

Your page reads the reader's notifications on the data plane, with the session:

| | |
|---|---|
| `GET /meerkat/notifications.json` | a page of the reader's notifications, from all their tenants |
| `GET /meerkat/notifications/counts` | the unread count |
| `POST /meerkat/notifications/{id}/read` | mark one read; `POST /meerkat/notifications/read` marks them all |
| `POST /meerkat/notifications/{id}/ack` | acknowledge one |
| `DELETE /meerkat/notifications/{id}` | delete one for this reader; `DELETE /meerkat/notifications` deletes them all, except what waits for an acknowledgement |

Each action in the list comes with its method and body, ready to send. The
full contract is in the data plane's API reference (**Meerkat, API reference**
in the console).
