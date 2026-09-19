---
title: Sign in by e-mail code
section: Authentication
order: 108
summary: Signing in with a one-time code mailed to the address, instead of a password, and the frame that makes that door defensible.
---

# Sign in by e-mail code

Signing in without a password: the person types their address, receives a
six-digit code, types it back, and the session opens.

**Application > Security**, the *Allow signing in with an e-mailed code*
switch. It ships off, and it needs a relay (Infra > Mail relay). Off, the link
is not on the sign-in page and the addresses answer 404: a control that appears
and then refuses is worse than one that is absent.

## What it moves

This door makes the **mailbox the credential**. The account is then worth what
the mailbox is worth, no more. The argument is less clear-cut than it looks: if
"forgot password" is open, the mailbox is already a way in, and the code only
makes explicit what the reset flow implies in silence. But it is a decision,
not a setting, which is why the switch arrives off.

## The frame

| Rule | What it prevents |
|---|---|
| **Data plane only** | The console is never openable by a mailbox: the pages are not mounted on the admin port, whoever the account belongs to |
| **Bound to the browser** | The code is sealed with a random request id, held in a cookie. A code read out over the telephone opens nothing on the caller's machine |
| **First factor only** | The second factor runs behind it, as it does behind a password. The code and a reset link travel the same channel: skipping the factor would hand both halves to whoever holds the mailbox |
| **Ten minutes, single use** | A code left in an inbox cannot be replayed, and an inbox opened next week opens nothing |
| **One live code** | Asking for a new code kills the previous one |
| **No enumeration** | The same page, status and cookie whether the address exists or not |
| **Throttled twice** | Sending is limited per address, guessing per browser |

> [!NOTE]
> `code` becomes a reserved authority id. An authority named that would live
> behind `/login/code`, which is this page: it would be shadowed with nothing
> saying so. The console refuses the name and says why.

## What the person sees

1. On the sign-in page, **Sign in with a code by e-mail**.
2. They type their address. The next page says a code is on its way, without
   saying whether an account is behind it.
3. They type the code back. If their account owes a second factor, it is asked
   for next, exactly as after a password.

The sign-in history names the door: *e-mailed code*, or *e-mailed code + code*
when a second factor followed.

## What is not there

The **magic link** - one click in the mail instead of a code - is not written.
That is a choice: a link is clickable by anything that scans mail, and it
forwards in one gesture.

Forbidding this door **per account or per role** is not written either: the
switch is global. For an administration account, today's protection is
structural - the console never opens this way.
