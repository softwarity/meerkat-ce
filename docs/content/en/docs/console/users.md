---
title: Users and account fields
section: The console
order: 170
summary: Accounts, the capabilities they carry, their security, and the extra fields this installation records about a person.
---

# Users and account fields

**Application > Users** is the accounts: who exists on this gateway, what they may
do across it, and how they get in. **Infra > Model** is the other half: the shape
of an account, decided once.

The two are apart on purpose. Defining a field and filling it are two acts by two
people: one says *this installation records a cost centre*, the other says
*Alice's is B200*. That separation is also what makes a custom field safe to
forward, since nobody can grant themselves an attribute a service trusts.

![The Users screen: six accounts, each with its four capability badges](img/console/users.webp)

Six accounts. The badges are buttons: here `admin` holds everything, one account
is app admin and another infra admin, and the rest hold nothing.

## The users list

One row per account: the enabled dot, the username, the full name and the address,
and the **capability badges**. A badge is a button: clicking it grants or revokes
the power without opening the drawer.

| Capability | What it opens |
|---|---|
| `root` | The whole gateway: routes, users, organisations, settings |
| `infra admin` | The routing plane: routes and the built-in pages |
| `app admin` | The application identity: users, roles, settings |
| `dev` | The developer tooling on the served applications |
| `tenant creator` | Creating organisations (several-organisations mode only) |

Row actions enable and disable an account. You cannot revoke your own `root`, nor
disable or delete yourself.

The search matches the account. The **+** button creates one.

## The account drawer

Three pages in one drawer, and the first is the only one you **fill**.

**The account page**

- Username, full name, e-mail.
- **Access from** and **Access until** - the validity window, in **days, not
  instants**: *until the 31st* means the whole of the 31st. Outside the window
  sign-in is refused, naming the date, and a session already open is re-checked
  rather than cut mid-work.
- **This installation's own fields** - whatever the Model screen defines.

**Security** (one tap away, and it comes back)

- **Two-factor** - required, optional, or inherited from the application policy;
  the label says what inherited currently resolves to.
- **Password** - *Reset password* produces one to hand over, and *Force a change
  at next sign-in* keeps the password the person already knows and refuses to go
  further with it, which saves a phone call per person. An account born at an
  authority holds no local password, so the button reads *Set a password*: doing
  it opens a second way in, deliberately.
- **External authorities** - which authorities are linked to this account.

**Sign-in history** - every sign-in, where from and how. It is a door rather than
a section, because a list as long as the account is old would push everything
under it out of reach.

**Danger zone** - deletion takes the account, its memberships, its sessions and
its tokens. The audit trail keeps an anonymised trace.

## Infra > Model: the fields an account carries

What this installation knows about a person that the product could not have
guessed: an employee number, a cost centre, a contract reference.

Each definition has a **Name** (the key a route forwards), a **Label** (what the
account screen shows) and a **Type**: text, number, date, choice or yes/no. A
**choice** field also takes its list, comma separated - and the list is what earns
the type: it turns a cost centre into data rather than three spellings of the same
thing.

From then on a field travels like any other fact about the caller: pick it in a
route's identity forwarding to send it to a service, or in its user info to stamp
it on a page.

> [!NOTE]
> **No field is ever mandatory.** A field defined today is empty on every account
> that already exists, and demanding it would stop the next person who opens one
> of them to change something else.

Add a field, remove a field, then **Save** - this screen has one button for the
whole list.

## Traps

- **A capability is not a role.** Capabilities administer *this console*; roles
  are what your applications read. Granting `app admin` grants nothing inside a
  proxied application.
- **An account with no organisation reaches nothing.** Create it here, then place
  it on [Members](/docs/console/organisation).
- **A value outside a choice list is refused**, by a sentence that names what is
  allowed - as is a route that forwards a field the model does not define.
- **Removing a field from the model stops it travelling.** It does not sit
  dormant in the route.
- **The validity window is checked at sign-in and re-checked afterwards**: nobody
  is thrown out mid-work by a clock, but nothing new opens either.
