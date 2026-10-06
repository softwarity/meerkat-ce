---
title: Sessions
section: Authentication
order: 114
summary: What the session cookie is, how long it lives, and exactly what ends a session - and what does not.
---

# Sessions

A browser session is an **opaque cookie**. It carries no identity, no claims and
no signature: thirty-two random bytes, of which the gateway keeps only a hash.
Everything about the session - who it belongs to, which organisation is active,
which login step is still owed - lives server-side, in a row.

That is on purpose. JWTs are for the API path and for what is forwarded
upstream, never for the browser: a token in a cookie cannot be withdrawn, and a
row can.

## The cookies

One session per plane, because cookies are not scoped by port: on a host serving
both ports, a single name would make the application and the console share one
session.

| Cookie | Plane | What it holds |
|---|---|---|
| `MEERKAT_SESSION` | data plane | the session value |
| `MEERKAT_ADMIN_SESSION` | control plane | the console's session value |
| `MEERKAT_UNTIL` | data plane | when this session expires, as a timestamp |
| `MEERKAT_ADMIN_UNTIL` | control plane | the same, for the console |

Each name ends with this installation's identifier - `MEERKAT_SESSION_3fa9c1e2` -
generated at install and never exported with a configuration. One level up, the
same reason: two gateways a browser reaches under one host name, an Enterprise
and a community one side by side on `localhost`, would otherwise overwrite each
other's sessions. The trusted-browser cookie is suffixed the same way; the
language and colour-scheme cookies are the person's choice and stay shared.

Over HTTPS the names start with `__Host-` (`__Host-MEERKAT_ADMIN_SESSION_...`).
A browser forbids a page in the clear to overwrite a `Secure` cookie of the
same name, so one name for both schemes made signing in over plain HTTP
impossible after a session over HTTPS - and the console's plain door exists for
the day a certificate breaks. The prefix also makes the browser guarantee the
cookie was set over HTTPS, by this host, for the whole site.

The session cookies are `HttpOnly`, `SameSite=Lax`, `Path=/`, with a `Max-Age`
equal to the session's lifetime. `Secure` is set when the request arrived over
HTTPS - either TLS terminated by the gateway, or `X-Forwarded-Proto: https` from
the proxy in front of it.

The two `..._UNTIL` cookies are deliberately **not** `HttpOnly`: a page reads
them to notice that the session ended, or came back in another tab, without
polling an endpoint. They hold a deadline and nothing else.

> [!NOTE]
> No `Domain` attribute is set, so the cookie is host-only, and that is a
> choice. A `Domain` would send the session to **every** sub-domain, including
> the ones Meerkat does not serve - a marketing site hosted elsewhere, a SaaS
> behind a CNAME, a staging host - and any one of them compromised would receive
> everybody's session. So `app.acme.io` and `admin.acme.io` sign in separately;
> applications under one host name share the session.

A session from one plane is never accepted on the other. The answer to a
data-plane cookie on the admin port is not "forbidden", it is "no session".

## How long a session lives

The lifetime is **idle time, not total time**: every request pushes the deadline
to *now plus the lifetime*. To keep that cheap, the new deadline is written to
the database only once the session is past the halfway mark, so a thirty-minute
session writes at most every fifteen minutes.

A logout, or a revocation on another node, cannot be undone by a request that was
already in flight: the extension only applies if the deadline it was told about
is still the one stored.

The lifetime itself is resolved from the narrowest level that says something:

| Level | Where it lives | Editable |
|---|---|---|
| Membership, one person in one organisation | `sessionTTL` on the membership | admin API only |
| Organisation | `sessionTTL` on the organisation | admin API only |
| Installation | setting `session_ttl`, default `PT30M` | **Application > Security** |

Values are ISO-8601 durations: `PT15M`, `PT1H`, `P1D`. Days, hours, minutes and
seconds only - weeks, months and years are refused. The console offers a list
from fifteen minutes to a day, and keeps a value set elsewhere rather than
dropping it.

> [!WARNING]
> Two rough edges on the two lower levels, both worth knowing before you use
> them. A bad duration is **not** validated on write: it is accepted and then
> silently falls back to thirty minutes at the next sign-in. And the console's
> members screen sends an empty membership lifetime whenever you tick a
> membership or a group, which **wipes** a per-member lifetime set through the
> API.

A session still owed a login step - a password to change, a code to enter - uses
the installation-wide lifetime, because the organisation is not known yet.

## What ends a session

| Cause | Scope | Immediate |
|---|---|---|
| `POST /logout` | that one session | yes, the row is deleted |
| Idle expiry | that session | yes, refused on the wall clock |
| A password reset from the e-mailed link | **every** session of that account, on every gateway | yes |
| Deleting the account | every session of that account | yes |
| **Disabling** the account | every session of that account, on both planes | yes |
| **Sign out** on the profile's *Active sessions*, or *Sign out everywhere else* | that session, or every other one of that person | yes |
| **Sign out** on the console's *Sessions* screen | that session | yes |

Logout is a `POST` - there is no `GET /logout` - and it deletes the row rather
than merely clearing the cookie, then clears both cookies and tells the other
gateways to forget it. It ends **that** session: other browsers of the same
person keep theirs, and the console and the application are independent.

## What does not end a session

This list matters more than the previous one:

- **Changing a password does not.** Neither the voluntary change in the profile, nor the forced change at sign-in, nor an administrator's reset. Only the e-mailed reset link revokes sessions - with the account's API tokens and trusted browsers - because that is the flow that exists for an account someone else may be holding.
- **"Must change password" does not.** The flag is read at the *next* sign-in.
- **Role, group and membership changes do not.** They take effect within seconds: the remembered identity is dropped, the session is kept, which is why being added to an organisation can take effect without signing out. Removing a role works the same way.
- **Disabling an organisation does not.**
- **On the control plane, an account's validity window is not re-checked** on a cookie session: it is checked at sign-in and on every token call, but a console session already open runs until it expires.

## Seeing the sessions

A person reads their own on the profile, **Security, Active sessions**: each
browser signed in to the applications, when and from which address, *This
browser* marked; **Sign out** on any other one, or **Sign out everywhere else**.
Another account's session is never theirs to close, whatever id a form carries.

Administrators read them on **Data plane, Sessions** for the applications and
**Meerkat, Sessions** for the console: who, which browser and address, since
when - filtered by account, paged, updated live. Each reads their own perimeter.
Root reads both planes. An application administrator reads the
applications' sessions, across every organisation, but not the console's: who
runs the console, and from where, is root's business. An organisation's
administrator reads the sessions open in the organisations they administer, and
can end only those. Ending one writes `session.revoke` to the
[audit trail](/docs/operations/audit).

![Sessions: who is signed in, from which browser and address, and since when](img/console/sessions.webp)

