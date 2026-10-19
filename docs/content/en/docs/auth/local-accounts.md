---
title: Local accounts
section: Authentication
order: 102
summary: The accounts Meerkat holds itself - password policy, sign-in throttling, and letting people create their own account.
---

# Local accounts

The local authority is the one Meerkat answers on its own: a username, a
password hash it keeps, and nothing asked of anybody else. It is enabled on a
first start, and the root account created at that first start belongs to it.

It appears in **Infra > Authentication** as a row named *Local accounts*, next
to the identity providers and the directories. Switching it off is what makes
the remaining authorities exclusive: as long as it answers, every local
password is a door that bypasses them.

> [!NOTE]
> The administration port never closes that door. The console is what a broken
> authority gets repaired from, and putting it behind that same authority is how
> an installation becomes unrecoverable at the worst moment.

## The password policy

One policy for the whole installation, in **Application > Security**. A
password is checked where it is typed, and the places it is typed - the sign-up
form, the forced change at a first sign-in, the profile, a reset link - know
nothing about organisations.

| Field | What it asks | Ships as |
|---|---|---|
| Length | minimum number of characters | `8` |
| Lowercase, Uppercase, Digits, Special characters | minimum count of each kind | `0` |
| No reuse of the last | how many previous passwords are refused | `0` |
| Expires after (days) | forces a change after that many days | `0`, never |
| Temporary (hours) | how long a password an administrator issued works | `72` |

A zero means *do not care*: the rule is neither checked nor shown. What ships
therefore asks for a length and nothing else, deliberately - raising the bar
for everyone during an upgrade locks people out of the very password change
they were in the middle of.

Three things are worth knowing before you fill the boxes:

- Length is counted in **characters, not bytes**, so a Greek or Japanese passphrase is not three times longer than it looks.
- A letter in a script that has no case - kana, Chinese, Arabic, Hebrew - counts as neither lowercase, nor uppercase, nor special.
- If the length you ask for is below the sum of the four kinds, it is raised to that sum: four kinds at two each need eight characters, and a checklist that could never be satisfied is worse than no checklist.

The console shows the checklist exactly as the sign-in pages will draw it. It
is the same function on both sides, on purpose: a checklist that says one thing
while the server enforces another is believed.

**Expiry is read at sign-in, not by a clock.** A password expiring at three in
the morning signs nobody out; it refuses the next sign-in, which then lands on
the password-update page. The same screen carries a *Force a change for
everyone* button for the day you need it.

**A temporary password runs out.** The password an administrator issues, at
creation or on a reset, works for *Temporary (hours)*, or until its owner
sets their own. Past that, signing in with it is refused with a sentence that
says to ask an administrator for a new one. It travels by mail, chat or phone,
and one nobody used should not stay a way in. *Force a change* is not affected:
the password stays its owner's own.

Passwords are hashed with bcrypt. A hash made at a lower cost is re-hashed at
its owner's next sign-in, the only moment the password is known in clear.

## Throttling, never a lockout

Failed sign-ins are counted twice, over a sliding window, in **Application >
Security > Rate limiting**: per **client address and account**, and per
**account** whatever the address.

| Field | What it counts | Ships as |
|---|---|---|
| Failed sign-ins from one address | failures from one address on one account, before the sign-in form answers `429` | `10` |
| Failed sign-ins on one account | failures on one account from any address, before its sign-in also asks for a picture code | `5` |
| Wrong 2FA codes | wrong second-factor codes tolerated, same window | `5` |
| Within | the window, as an ISO-8601 duration | `PT15M` |

A successful sign-in clears both counters. The refusal happens **before** the
hash comparison, so a blocked attacker does not get to burn processor time
either. Setting a count to zero turns that limiter off.

The counter lives in the database, not in the process:

- ten attempts is ten for the installation, not ten per gateway - an attacker spraying a load balancer no longer gets the limit multiplied by the number of nodes;
- a restart no longer forgives whoever was being throttled.

If the database will not answer, the limiter lets the attempt through rather
than refusing it: the sign-in was about to need that same database anyway, and
a blip must not become a lockout.

**No account is ever locked.** Nothing an outsider does can take an account
away from the person who owns it.

### Many addresses, one account

The counter per address stops one machine guessing. It cannot see a thousand
machines guessing a little each - a botnet, a pool of rotating proxies - since
each address stays under its own budget for ever. What they all share is the
name they are trying, so the second counter counts that.

Past **Failed sign-ins on one account**, the sign-in page for that name asks for
the same picture code as the [registration form](#letting-people-create-their-own-account),
with the name already filled in:

- without the right code, no password is even checked, and the page says the code is missing rather than "wrong password" - someone who typed the right one must not be told it was wrong;
- the code is spent whatever the answer, and every page carries a fresh one;
- the right password **and** the right code sign in, and clear the counter.

Three seconds for a person, the whole economy of a script - and the account
stays open to its owner throughout. Zero turns this counter off.

An **unknown name gets the same treatment**: the same counter, the same code, the
same sentences. Doing otherwise would answer the one question the page refuses
to answer everywhere else - does this account exist.

**The owner is told.** When an account crosses the threshold and SMTP is
configured, its owner gets one mail - how many failures in the window, from how
many addresses, and to change the password if it was not them. One per window,
however long the hammering lasts, and none for a name that does not exist, a
disabled account or an address a self-registered account never confirmed. The
mail leaves after the answer, never before it: a refusal that waited for the
relay would take measurably longer on a real account than on an invented one.
The relay screen can send a sample of it (**Failed sign-ins**).

### Which address is counted

The address is the **TCP peer** - the end of the connection the gateway
accepted - unless that peer is a proxy the gateway trusts, in which case it is
what that proxy wrote in `X-Forwarded-For`. A caller who reaches the gateway
directly and writes the header themselves is ignored: otherwise inventing an
address per request would walk through every counter on this page. Which
proxies are trusted is deduced from their address by default, and said plane by
plane on **Infra > Domains & TLS** - who serves the public names, and the
addresses of a proxy in front - see [Behind a proxy or a load balancer](/docs/deploy/one-gateway#behind-a-proxy-or-a-load-balancer).
The security trail keeps both, "203.0.113.5 via 10.42.0.15", so it never shows
only what a header claimed.

An unknown username, a wrong password and a disabled account all answer the
same sentence, and cost the same time - an unknown username still gets a hash
comparison against a dummy, so the response time tells nothing apart. An
account outside its access window is the one exception: to someone who typed
the **correct** password, the page names the date, because without it every
expiry becomes a support call to ask the one thing the page could have said.

## Letting people create their own account

Self-registration ships closed. When it is open, the gateway serves
`/register`: a username, an address, a password checked against the policy
above, and a picture code to copy.

Four things must all hold for that page to exist at all:

1. the local authority is **enabled**;
2. self-registration is allowed for it - either *Allowed* on the authority itself, or *Inherited from the application* with the switch at the top of **Infra > Authentication** on;
3. a mail relay is configured in **Infra > Mail relay**, because the address has to be confirmed;
4. it is the data plane. The administration port never serves a sign-up form.

> [!WARNING]
> The switch at the top of the Authentication screen is a **default**, not a
> lock. An authority whose own policy says *Allowed* keeps creating accounts
> when that switch is off. To close self-registration for good, set the
> authority itself to *Refused*.

What a sign-up produces is deliberately useless: the account exists, it is
unusable until the address is confirmed, and it reaches nothing until an
administrator places it in an organisation and grants roles. The confirmation
link is a one-shot token valid for 24 hours, 48 hours or 7 days - **Confirmation
links valid for**, beside the self-registration switch. Someone who signs in
before confirming gets the link sent again rather than an explanation.

A username or an address already taken lands on the **same** page as a
successful sign-up, and nothing is created: the form does not tell a stranger
who already has an account here. The picture code is on by default and can be
turned off per authority; it is consumed whether the answer was right or wrong,
so a second try means a new image.

The write endpoints nobody has signed in to yet carry throttles per client
address: `/register` five tries per fifteen minutes, fixed; `/forgot-password`
its own counter, **Reset requests** under the rate limits (five by default,
within the sign-in window, and never off). The two no longer share one: a busy
sign-up page used to throttle resets, and a flood of resets closed sign-up.

## Changing one's address

The current password is asked first, where the account has one: a session
alone, a borrowed laptop or a stolen cookie, must not be enough to move the
place a reset lands.

The address is where a password reset lands, so it is a way into the account.
When a mail relay is configured, changing it from the profile does **not**
change it yet: a link goes to the **new** address, and the change is made when
that link comes back - valid for the same lifetime as a sign-up's. The **old**
address is told at once, which is the owner's alarm when it was not them. A
session somebody else got hold of is therefore no longer enough to point the
account's recovery at their own mailbox. Without a relay nothing could carry a
confirmation, and the address changes at once, as before.

## A link, or a code

Every one of these mails proves a mailbox: a sign-up's confirmation, a new
address, a reset. A **link** is mailed only when the gateway can vouch for the
name in it, in one of two ways:

- the request reached **its own TLS door**, and a certificate placed on that
  door (**Infra > Domains & TLS**, on Console or Application) covers the name
  the browser asked for;
- or the name is one the operator declared on that door as **reached through a
  proxy that terminates TLS** - nginx, an ingress, a load balancer in front. The
  request then arrives in plain HTTP, and the link is still `https://`.

The link then points at that name, so a reset asked on `shop.example.com` comes
back to `shop.example.com`. See
[Behind a proxy that terminates TLS](/docs/operations/tls#behind-a-proxy-that-terminates-tls).

Anywhere else - plain HTTP, a proxy in front whose names nobody declared, a name no
placed certificate carries - the mail holds a **six-digit code** and no link,
and the page that asked waits for it. The code is the e-mail sign-in's: valid
ten minutes, and only in the browser that asked. A new address is confirmed by
the code in the session that asked for the change.

This is what keeps a stranger from using the mails against their recipient. The
name in a request is whatever the caller wrote: a reset asked for someone else's
address with `Host: attacker.example` used to mail that person a genuine link,
with a genuine token, to the attacker's server. A code mailed to the victim is
worthless to whoever asked: they never see it, and it opens nothing in another
browser.

## A forgotten password

`/forgot-password` exists as soon as a mail relay is configured and local
passwords are accepted; it does not wait for self-registration to be open. The
outcome page is the same whether the address is known or not. An account born at
an authority (OIDC, LDAP, GitHub) has no local password and is sent no link: one
would open a second door the authority knows nothing about.

The link is valid for one hour and is spent on use (a code, ten minutes). The new password is checked
against the policy, including the no-reuse rule, and the change **destroys
every live session of that account** on every instance - whoever held one,
including an intruder, signs in again or is out. The owner is told by e-mail
that their password changed.

It also closes the other doors the old password could have opened: the
account's **API tokens** are revoked and its **trusted browsers** forgotten. A
reset is often the answer to "somebody else had my password", and that somebody
may have minted a token with it.
