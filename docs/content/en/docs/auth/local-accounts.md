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

Passwords are hashed with bcrypt. An old hash is not re-encoded when you raise
the cost - Meerkat has no transparent re-hash on sign-in yet.

## Throttling, never a lockout

Failed sign-ins are counted per **client address and account**, over a sliding
window, in **Application > Security > Rate limiting**.

| Field | What it counts | Ships as |
|---|---|---|
| Failed sign-ins | failures tolerated before the sign-in form answers `429` | `10` |
| Wrong 2FA codes | wrong second-factor codes tolerated, same window | `5` |
| Within | the window, as an ISO-8601 duration | `PT15M` |

A successful sign-in clears the counter. The refusal happens **before** the
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
2. self-registration is allowed for it - either *Allowed* on the authority itself, or *Inherited* with the switch at the top of **Infra > Authentication** on;
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
link is a one-shot token valid for twenty-four hours. Someone who signs in
before confirming gets the link sent again rather than an explanation.

A username or an address already taken lands on the **same** page as a
successful sign-up, and nothing is created: the form does not tell a stranger
who already has an account here. The picture code is on by default and can be
turned off per authority; it is consumed whether the answer was right or wrong,
so a second try means a new image.

The write endpoints nobody has signed in to yet - `/register` and
`/forgot-password` - carry their own fixed throttle of five tries per client
address per fifteen minutes, which no screen changes.

## A forgotten password

`/forgot-password` exists as soon as a mail relay is configured and local
passwords are accepted; it does not wait for self-registration to be open. The
outcome page is the same whether the address is known or not.

The link is valid for one hour and is spent on use. The new password is checked
against the policy, including the no-reuse rule, and the change **destroys
every live session of that account** on every gateway - whoever held one,
including an intruder, signs in again or is out. The owner is told by e-mail
that their password changed.

Two things a reset does *not* do yet: it does not revoke that account's API
tokens, and it does not forget its trusted browsers.
