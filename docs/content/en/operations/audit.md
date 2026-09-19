---
title: The audit trail
section: Operations
order: 206
summary: Every administrative change, with its author and a field-by-field diff, and who is allowed to read it.
---

# The audit trail

Every mutation made through the control plane is recorded: who did it, what they
touched, and - for an update - the exact fields that moved, before and after
(AUD-01).

The log is **append-only**. It knows insertion and purge and nothing else: no
endpoint can modify an event.

![The audit trail](img/console/audit.webp)

## What one event holds

| Field | What it says |
|---|---|
| when | the second it happened |
| actor | the account, plus the **token** when one was used |
| action | `route.update`, `theme.branding`, `settings.update`, `maintenance`, `backup.snapshot`... |
| target | the kind of object, its id and its name |
| organisation | when the change belongs to one |
| changes | one entry per field that moved, with its `from` and its `to` |
| detail | a note, for creations, deletions and acts that have no diff |

An update whose diff comes out empty writes **nothing**: saving a form without
changing a value is not an event.

The diff is computed generically, by comparing the old and the new object field
by field, so it works for a route, an account, an organisation or a settings
payload without any per-type code. A nested object or a list compares by its
whole encoding: a change anywhere inside surfaces the whole field.

## The acting token is named

A change made by an agent or a script reads `admin, via claude-desktop`, not
`admin`. The stamp is applied inside the audit write itself rather than at its
call sites, so no endpoint added later can forget it (MCP-03). A trail that names
the agent in four cases out of five is worse than one that never does: the fifth
reads as if a person had done it.

## Secrets never land in the trail

A field whose name contains `password`, `secret`, `token` or `hash` is recorded as
`***`, at any depth inside a nested object. An image sent as a data URI is
summarised - its kind and its size - rather than stored twice over as base64: a
branding save carries two of them, the before and the after.

Fields that say nothing about a change are skipped: identifiers, server
timestamps, and the display-only names that ride along on a round trip.

## Who sees what

The trail is a transverse screen of its own, and each caller reads the slice
their capabilities cover (RBAC-05):

| Caller | What they read |
|---|---|
| root | everything |
| gateway-admin | routes, themes, tokens |
| app-admin | accounts, roles, settings, issue reports |
| an organisation's administrator | the events of the organisations they administer |
| anybody else | nothing, and the endpoint refuses rather than serving an empty page |

The agent's `read_audit` tool shares that same function: an agent that saw a wider
trail than the console would be a way around the capability model, and it would be
nobody's fault in particular.

## Filters, retention, and what is missing

The screen filters on actor, target, target id and a time range.

Retention is **one year**, applied by the periodic sweep, and it is not
configurable yet.

Not there yet (AUD-01, AUD-02): server-side pagination, developer and tunnel
activity, configurable retention, and an analytic export. Sign-ins are not in this
trail either - they have their own history, on the account.
