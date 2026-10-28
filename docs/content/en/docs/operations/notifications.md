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
- **The label EXERCISE** before the title, everywhere it shows, on a
  notification sent [to practise](#exercises).

![The notifications page of a reader in two tenants: the tenant filter, a message sent to both tenants on one row, one waiting for an acknowledgement, and one unfolded with Open and its action](img/console/notifications-page.webp)

A notification its sender **withdrew** stays in the list the way a deleted
message does in a chat: a row saying *This notification was withdrawn*, with
none of what it said (no title, text, link, action or tenant), read, out of the
count, never on a snackbar or the desktop. It leaves the list when it expires,
or when the reader deletes it. A notification its sender **replaced** is not
shown at all: the replacement stands in its place.

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
administrator's: the notification turns into the placeholder on every reader's
page at once and leaves their count. Delete is the reader's: it leaves their
own list and nobody else's.

## Who receives it

At publication the gateway works out the readers and keeps **one line per
reader**. It also keeps the audience you named, so someone who comes within it
later still receives it (see [Rights gained later](#rights-gained-later)). The
audience starts from the enabled members of the tenant, narrowed to the
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

### Rights gained later

Two moments give a live notification to someone who did not have it: they
**join or leave the tenant** (leaving gives nothing), or a **role is granted**
to them there. A role is held through a group of roles: it is granted when
they are put in a group of roles that holds it, or when a group of roles of
theirs is given it.
At that moment the gateway looks at the tenant's live notifications (not
expired, not withdrawn, not replaced) whose kept audience now takes them in,
with the same rules as above, and gives them those they do not hold.

- They are told **once**, whatever the count: a snackbar, or a desktop
  notification when the tab is not in front of them, saying *Notifications you
  missed: 3*, leading to their list. Never one per notification.
- When at least one of them asks for mail, they receive **one digest mail**
  listing them, through the same [mail queue](#mail), never one mail each.
- Opening their list does the same work again, quietly: whatever a moment
  missed is given then.
- The work runs in the background: a membership saved, or a sign-in that
  grants a tenant or a role, is never held up by it.

A right lost changes nothing to the lines: the page checks access each time it
shows them, and hides what the reader may no longer open. A notification
published before this version kept no audience, and is given to nobody
afterwards.

## Publishing

On the **internal plane**, port 7070, like
[scheduled calls](/docs/operations/scheduler): called from inside the cluster by
the gateway's internal name, never from a browser.

| | |
|---|---|
| `POST /api/notifications` | publish |
| `GET /api/notifications` | yours that are still live, newest first, filtered by `meta.<key>` |
| `GET /api/notifications/{id}/acks` | who acknowledged one of yours, and who has not yet |
| `DELETE /api/notifications/{id}` | withdraw one of yours by its id: every copy of one sent to every tenant |
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
| `exercise` | `true` for a message sent to practise. See [Exercises](#exercises) |
| `detail` | where it leads: a [link](#links-a-service-and-a-path), on a UI route |
| `actions` | buttons, 5 at most: a `label` (60 characters), a link, and [how to call it](#actions-a-method-and-a-body) |
| `mail` | `true` to send it by mail too. Refused when no mail relay is configured. See [Mail](#mail) |
| `expires` | a duration (`P7D`, `PT12H`) or a date (`2026-11-01T08:00:00Z`): 30 days by default, 90 at most. Then it leaves every list |
| `id` | required, a UUID of your choice: it lets you retry without duplicates, replace or withdraw the notification, and follow its acknowledgements and mails. 200 characters at most, letters, digits and `-_.:`. See [Its id](#its-id-replacing-and-withdrawing) |
| `replaces` | the id of an earlier notification of yours, which ends as replaced. See [Replacing](#its-id-replacing-and-withdrawing) |
| `meta` | your labels, such as `{"invoice": "INV-2026-0142"}`, to find and withdraw your own. Never shown to readers |
| `ack` | `none` (default), `single` or `all`: see [Acknowledgement](#acknowledgement) |
| `onAck` | your service, called at each acknowledgement. Needs `ack` |

The texts are in **one language** for every reader: write them in the language
your users share. An object by language is refused.

### Exercises

A drill must look like the real thing, and must never be taken for it. Send it
with `"exercise": true` and a realistic `level`: it carries the label
**EXERCISE**, in the reader's language, before its title on the page, on the
snackbar and on the desktop notification, and in the subject, the heading and
the first line of its mail. The console shows it with the same label. Nothing
else changes: its audience is worked out as for any other notification, so
choosing who takes part in the drill is your service's choice of `roles` and
`users`, and it counts and reads like the others.

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
on, each with its own readers, all under the one id of the send. A tenant where
nobody may receive
it is skipped, and the answer says so. The users named in `users` are taken in
the tenants they belong to. A reader who is a member of several of these
tenants sees **one** notification naming them, with one snackbar, one desktop
notification and one mail.

Sent again with the same id, the copies already published are answered and the
missing ones are published: a publication cut halfway finishes by sending it
again. Replacing or withdrawing by that id acts on every copy.

### Its id, replacing and withdrawing

**One id, yours, required.** Every message carries an `id`, a UUID of your
choice: it lets you retry without duplicates, replace or withdraw the
notification, and follow its acknowledgements and mails. A message without
one is refused (`422` on `id`). A service that will never come back to a
notification sends a fresh UUID and does not keep it. The gateway never makes
an id up. Its source is kept on every notification, and the console shows it.
An id is scoped by its source, the caller the internal
plane recognises: a Meerkat token's name, the pod's ServiceAccount, or the
workload behind the address (an address alone only when nothing better is
known). Another service may use the same id. Published again from the same
source under an id already used, before that notification expires, the answer
is the notification already there, with `"existing": true`: nothing is
created, nobody is told twice, and the content is not compared. This holds
even after you withdrew or replaced it: an id you ended does not come back by
being sent again.

Your actions are called with `notification=<id>`, and `onAck` receives the same
id: the one you know.

**Replacing.** Publish the new notification with `"replaces"` naming the
earlier one's id. Give the replacement an id of its own: under the id it
replaces, it would only find that one again, and that is refused.

```json
{ "id": "job-7-80", "title": "Export 7 is at 80%", "replaces": "job-7-40" }
```

The earlier one ends as **replaced**: it leaves every reader's list and count,
the replacement standing in its place, its mails not sent yet are cancelled,
and it asks for no acknowledgement any more. The answer says what it ended, in
`replaced`. Sending the same replacement again answers it with
`"existing": true` and ends nothing more.

**Withdrawing.** `DELETE /api/notifications/{id}` ends it as **withdrawn**:
the readers see the placeholder, its mails not sent yet are cancelled. An id
sent to every tenant withdraws every copy. The answer says whether it ended
now, or had ended before:

```json
{ "withdrawn": ["job-7-80"] }
{ "withdrawn": [], "already": ["job-7-80"] }
```

**When your calls arrive out of order.** Nothing guarantees that a withdrawal
or a replacement reaches the gateway after the notification it names. When it
names an id none of your notifications carries yet, the gateway keeps it: a
withdrawal answers `202` with `{"withdrawn": [], "pending": "<id>"}`, a
replacement is published and its `replaced` list is empty. When that id
arrives, the notification is stored **already ended**, withdrawn or replaced,
and nobody is told or mailed; the answer carries `"state"`. What is kept lives
as long as a notification would: the replacement's own life, or for a
withdrawal 30 days unless `?expires=` says otherwise (90 at most).

| A notification is | On the reader's page | In the count |
|---|---|---|
| live | as sent | while unread |
| withdrawn | the placeholder *This notification was withdrawn*, nothing of what it said | no |
| replaced | not shown: its replacement stands in its place | no |
| expired | gone, with every reader's state | no |

The console shows every one of them with its state, its sender and its id.

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
    "id": "overdue-INV-2026-0142",
    "meta": { "invoice": "INV-2026-0142" },
    "ack": "single",
    "onAck": { "service": "billing", "path": "/notifications/acked" }
  }'
```

```http
HTTP/1.1 201 Created
Content-Type: application/json

{"id": "overdue-INV-2026-0142", "recipients": 4}
```

### The answers

| Status | Body | Meaning |
|---|---|---|
| `201` | `{"id", "recipients"}` | published under your `id`. With `replaces`, `replaced` says what it ended; `"state"` when the id was named by a withdrawal or a replacement before it arrived |
| `200` | the same, with `"existing": true` | the id is already known: nobody is notified again. `"state"` says `withdrawn` or `replaced` when it ended |
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
  "id": "maintenance-2026-10-25",
  "recipients": 37,
  "tenants": [
    { "tenant": "7c1e2a90", "name": "Acme Logistics", "recipients": 21 },
    { "tenant": "b2f4c8d1", "name": "Acme Retail", "recipients": 16 }
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
Meerkat-Notification: overdue-INV-2026-0142
Meerkat-Attempt: 1

{"notification": "overdue-INV-2026-0142", "tenant": "7c1e2a90",
 "tenantName": "Acme Logistics",
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
what was withdrawn, `{"withdrawn": ["<id>", ...], "count": 2}`, and those notifications show
as withdrawn on every reader's page at once. The console keeps them, marked
Withdrawn, with who read them.

## Mail

With `"mail": true`, each reader who has an address receives one mail in their
language, in the gateway's mail template: the text, and a button to their
notifications page when the gateway knows its public address. A mail never
carries an action. A message sent to every tenant is mailed once to a reader
of several of them.

**The queue is in the database**, one mail per reader, each with its state on
the reader's line of the notification. The nodes send them one at a time, one
every 200 ms, and only one node sends at a time, so an audience of thousands
does not reach the relay at once. Two nodes never send the same mail: a node
claims a mail before sending it. A gateway stopped halfway, or restarted,
loses nothing: what is still to send is sent by the next node that runs, or by
the same one when it starts again.

| A reader's mail is | Meaning |
|---|---|
| `pending` | to send, now or at its next attempt |
| `sending` | a node is sending it |
| `accepted` | the relay accepted it. The most the gateway knows: never "delivered" |
| `failed` | the relay refused it for good (a `5yz` reply), or a temporary refusal (`4yz`) lasted past 24 hours, or past the notification's life. The reply code is kept |
| `unknown` | the connection went after the whole message was sent and before the relay answered, or the node sending it stopped: the relay may have taken it, so it is never sent again on its own |
| `cancelled` | the notification was withdrawn, replaced or expired before it left |
| `noaddress` | the reader has no address, or their account is disabled |

A temporary refusal, or a relay that cannot be reached, is tried again after
1, 5, 15 and 30 minutes, then every hour, for 24 hours at most. Every attempt at
one mail carries the same `Message-ID`, so a mailbox that receives it twice can
tell.

**Where they stand.** For a notification sent with mail, your list carries
`mails`, the count of its readers' mails by state:

```json
{ "id": "overdue-INV-2026-0142", "mail": true,
  "mails": { "accepted": 38, "failed": 1, "noaddress": 2 } }
```

Once nothing is left to send, the outcome is one line of the audit trail for
the notification, `notification.mail`, never one per reader. The console shows
each reader's mail beside the time they read it.

## What the audit trail keeps

What a service does on the internal plane is on the
[audit trail](/docs/operations/audit), with the sender as the actor, named as
the access recognised it (a token's name, a ServiceAccount, an address), in the
notification's tenant:

| Action | Written when |
|---|---|
| `notification.publish` | a notification is published: its sender, its tenant, how many recipients, whether it is an exercise, and whether it arrived already withdrawn or replaced. Sent to every tenant, one line per copy |
| `notification.replace` | a notification is replaced, naming the one that replaced it |
| `notification.withdraw` | a notification is withdrawn, by its sender or an administrator |
| `notification.mail` | a notification's mailing is over: one line for the notification, never one per recipient, with how many mails the relay accepted, how many failed, how many readers had no address, and how many were cancelled |
| `notification.given` | notifications were given to someone after their publication, having joined the tenant or been granted a role, or on opening their list: one line naming the person and the notifications, the gateway as the actor |
| `notification.ack` | a reader acknowledges one: one line per acknowledgement, the reader as the actor, kept after the notification expired |

Reading and deleting are the reader's, kept on their line of the notification,
and are not on the trail. The console's **Data plane,
Notifications** screen and the tenant's **Notifications** section follow every
publication live, the internal plane's as much as the console's.

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
