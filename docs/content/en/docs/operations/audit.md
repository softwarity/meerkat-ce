---
title: The audit trail
section: Operations
order: 206
summary: Every administrative change with its field-by-field diff, every sign-in and refused sign-in, and who is allowed to read them.
---

# The audit trail

The trail has two halves, on one screen:

- **The changes.** Every mutation made through the control plane: who did it,
  what they touched, and - for an update - the exact fields that moved, before
  and after.
- **The security of the accounts.** Every sign-in, every refused sign-in with
  its real reason and the address it came from, and every way into an account
  that its owner added or removed.

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
| detail | a note, for creations, deletions and acts that have no diff; the reason of a refused sign-in |
| address | on the security half, the address the gateway resolved |

An import, a restore or a switch of configuration writes one line, and its
detail says what it did: the counts, then which objects were added, updated and
removed, and for a setting the fields that moved - `setting tls (appNames)`
rather than a bare "8 updated". Capped at eighty names, the rest counted.

An update whose diff comes out empty writes **nothing**: saving a form without
changing a value is not an event.

The diff is computed generically, by comparing the old and the new object field
by field, so it works for a route, an account, an organisation or a settings
payload without any per-type code. A nested object or a list compares by its
whole encoding: a change anywhere inside surfaces the whole field.

## The acting token is named

A change made by an agent or a script reads `admin, via claude-desktop`, not
`admin`. The stamp is applied inside the audit write itself rather than at its
call sites, so no endpoint added later can forget it. A trail that names
the agent in four cases out of five is worse than one that never does: the fifth
reads as if a person had done it.

## Secrets never land in the trail

A field whose name contains `password`, `secret`, `token` or `hash` is recorded as
`***`, at any depth inside a nested object. An image sent as a data URI is
summarised - its kind and its size - rather than stored twice over as base64: a
branding save carries two of them, the before and the after.

Fields that say nothing about a change are skipped: identifiers, server
timestamps, and the display-only names that ride along on a round trip.

## The security of the accounts

Written by the sign-in pages and the profile, never by the control plane, under
two target kinds of their own: `account` for an application account, `console`
for a sign-in to the console.

| Action | When | Detail |
|---|---|---|
| `signin` | a session was issued | how: `password`, `totp`, `passkey`, `email-code`, `external-totp`... |
| `signin.organisation` | an organisation entered, chosen after signing in or switched to later | its name |
| `signin.refused` | a door stayed shut | the reason (below) |
| `signin.locked` | the attempt that tripped the throttle | `throttled`, or `bad-code` for a second factor |
| `signout` | the account signed out | `remote` when closed from *Active sessions* |
| `password.change` | changed by its owner | `required` when the change was forced |
| `password.forgot` | a reset link was mailed | |
| `password.reset` | the link was used | |
| `mfa.enroll`, `mfa.remove` | a second factor added or removed | |
| `passkey.add`, `passkey.remove` | a passkey registered or revoked | the browser, on an addition |
| `email.change.request` | a new address asked for, its link mailed | the new address |
| `email.change` | the address changed (confirmed, or at once without a relay) | the new address |
| `token.create`, `token.revoke` | a personal API token | its name |
| `register`, `register.confirm` | a self-registration and its confirmation | |
| `devkey.add`, `devkey.remove` | a plug key deposited or removed, one per workstation | its fingerprint, never the key |

The reasons a sign-in is refused: `bad-credentials`, `disabled`,
`outside-validity`, `unconfirmed`, `outside-hours`, `bad-code`, `passkey`,
`not-recognised` (the authority no longer knows the account), `not-invited`,
and `provider:<id>` when an identity provider refused.

The page the visitor sees says the same thing for a wrong password, an unknown
name and a disabled account, so nothing is enumerated. The trail says which it
was: its reader is an administrator.

A refused name that matches no account has no actor. What was typed is recorded
as the target's name, clipped, so the line still says what was tried.

**Hammering does not fill the table.** The attempt that trips the throttle
writes `signin.locked`, and the attempts refused after it write nothing: an
attacker at a thousand tries a second is not a thousand rows a second.

Each line carries the **address** the gateway resolved, never an
`X-Forwarded-For` that the caller wrote. And the **organisation** when there is
one: on the sign-in itself for an account that belongs to a single one, on the
`signin.organisation` line when it is chosen afterwards. That stamp is what
shows the line to that organisation's administrators.

These lines are not pushed live to the console: a list of accounts re-read at
every sign-in of anybody would be noise. The account's own
[sign-in history](/docs/auth/flow-pages) stays on the profile, and it goes with the
account. The trail outlives it.

## Who sees what

The trail is a transverse screen of its own, and each caller reads the slice
their capabilities cover:

| Caller | What they read |
|---|---|
| root | everything, console sign-ins included |
| infra-admin | routes, identity providers, certificates, settings, issue reports |
| app-admin | accounts, roles, themes, languages, schedules, settings, issue reports, and the application accounts' security |
| an organisation's administrator | the events of the organisations they administer, their members' sign-ins into them included |
| anybody else | nothing, and the endpoint refuses rather than serving an empty page |

A sign-in to the console is root's alone: it says where and when the people who
run the gateway work.

The agent's `read_audit` tool shares that same function: an agent that saw a wider
trail than the console would be a way around the capability model, and it would be
nobody's fault in particular.

## Filters, retention, and what is missing

The screen filters on the part of the trail (**All**, **Changes**, **Data plane
sign-ins**, **Console sign-ins**), the target kind of a change, the period, and a free-text box that also searches the reason and the
address. The API adds the actor and the target id: `GET /api/audit?kind=security`.

**Retention** is a year by default, applied by the periodic sweep, and root
chooses it at the top of the screen - three months, six, one, two or five years
(**Keep events for**). Root's alone: whoever may shorten the trail may erase
their own traces with it. Shortened, the older events go at the next sweep.

**Export CSV** (Enterprise) takes the trail out as a file for an auditor or a
SIEM: the filters on screen, the perimeter of whoever asks - never more than
their screen shows - one row per event, the diff as JSON in its own column. The
export writes its own line (`audit.export`, read by root): the file carries
addresses and names. `GET /api/audit/export` takes the same parameters as the
list.

Not there yet: server-side pagination, the tunnel's own activity, a Parquet
export, and sending the trail to a collector over OTLP.
