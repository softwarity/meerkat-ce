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

The session cookies are `HttpOnly`, `SameSite=Lax`, `Path=/`, with a `Max-Age`
equal to the session's lifetime. `Secure` is set when the request arrived over
HTTPS - either TLS terminated by the gateway, or `X-Forwarded-Proto: https` from
the proxy in front of it.

The two `..._UNTIL` cookies are deliberately **not** `HttpOnly`: a page reads
them to notice that the session ended, or came back in another tab, without
polling an endpoint. They hold a deadline and nothing else.

> [!NOTE]
> No `Domain` attribute is set, so the cookie is host-only. A session is **not
> shared between sub-domains** today - `app.acme.io` and `admin.acme.io` sign in
> separately. FEATURES.md lists this as the missing half of the session cookie.

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

Logout is a `POST` - there is no `GET /logout` - and it deletes the row rather
than merely clearing the cookie, then clears both cookies and tells the other
gateways to forget it. It ends **that** session: other browsers of the same
person keep theirs, and the console and the application are independent.

## What does not end a session

This list matters more than the previous one:

- **Disabling an account does not end its sessions.** The row stays. What happens instead is that every gate re-reads the account, so the disabled person stops passing route rules and stops being admitted by the admin API. But the gateway's own pages - `/profile` and the rest - still answer them until the session expires.
- **Changing a password does not.** Neither the voluntary change in the profile, nor the forced change at sign-in, nor an administrator's reset. Only the e-mailed reset link revokes sessions, because that is the flow that exists for an account someone else may be holding.
- **"Must change password" does not.** The flag is read at the *next* sign-in.
- **Role, group and membership changes do not.** They take effect within seconds: the remembered identity is dropped, the session is kept, which is why being added to an organisation can take effect without signing out. Removing a role works the same way.
- **Disabling an organisation does not.**
- **On the control plane, an account's validity window is not re-checked** on a cookie session: it is checked at sign-in and on every token call, but a console session already open runs until it expires.

## What is missing

There is **no list of active sessions** and **no "sign out everywhere"** - not
for a person on their own account, and not for an administrator. The table
exists and revocation by account exists in the code; what is missing is a screen,
a button, and a call to that revocation when an account is disabled.

Until then, the honest way to get someone out of everything is to reset their
password through the e-mail flow, or delete the account.
