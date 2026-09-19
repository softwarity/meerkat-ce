---
title: General, Locales and Security
section: The console
order: 166
summary: What this installation is, which languages it speaks, and the policies every account lives under.
---

# General, Locales and Security

Three screens at the top of the **Application** plane. They hold what is true of
the whole application, and writing to them takes the `app admin` capability (or
`root`).

## General

![The General screen: the organisations switch, developer mode, and the weekly working-hours grid](img/console/general.webp)

The two switches, then the working hours: a time zone and one line per day, each
with its own window and a + to add a second one.

Two statements about what this installation **is**, and one access window.

**Several organisations.** Off, this gateway serves one organisation and never
names it: its groups and members are administered right here, under Application.
On, organisations get their own entry in the rail. Going **down** to a single
organisation asks first, and counts what stops being served: nothing is deleted,
and switching back brings the organisations and their access straight back.

> [!NOTE]
> Enterprise edition: several organisations.

**Developer mode.** On, accounts holding the `dev` capability get their tooling on
the applications this gateway serves: the Developer menu in the user button, the
UI test mode, the routes' API docs. Off, none of it is there at all, however many
accounts carry the capability. Free in both editions.

If the gateway is declared production where it runs, the switch is disabled and
says so: the developer surface stays closed here whatever the console says, so a
database restored from staging cannot carry it back open.

**Working hours** are the application-wide access window every organisation
inherits unless it defines its own.

> [!NOTE]
> Enterprise edition: working hours.

## Locales

The locales **your application** supports, as ISO codes (`fr`, `en-GB`,
`pt-BR`...). They fill the built-in pages and the user button, and a signed-in
user's choice follows every proxied request.

Add one with the autocomplete, which proposes the common languages and their
regional variants and refuses a code that is not valid. Each row shows the code,
its name in your language and its own name. Adding and removing save
immediately - there is no Save button here.

Empty leaves the built-in pages in English.

## Security

The policies every account lives under. One Save button at the bottom for the
whole screen.

- **Two-factor** - require a second factor for everyone; organisations and
  members can override it. Beside it, **a one-time code by e-mail** as a
  fallback for an enrolled user who cannot reach their authenticator: it needs a
  [mail relay](/docs/console/mail-relay) and only shows for accounts that carry
  an address and have already set up an authenticator.
- **Session TTL** - how long a session lives before the user must sign in again.
- **Passwords** - what a new password must contain: length, and how many
  lowercase, uppercase, digits and special characters. **Zero means the rule is
  not asked for**, and a rule that is not asked for is not shown to users either.
  The screen previews exactly the list your users will read. If the kinds already
  need more characters than the length, it says the length will be raised on save.
  - **No reuse of the last N** - previous passwords are kept as hashes and
    compared when a new one is chosen, the one in use included.
  - **Expires after (days)** - checked at sign-in, not by a clock: an expired
    password sends the person to the change page rather than ending a session they
    are working in. A password whose age is unknown never expires.
  - **Force a change for everyone** (root) makes every local account change at
    its next sign-in, yours excepted. One account at a time is on
    [Users](/docs/console/users).
- **Rate limiting** - failed sign-ins per address and account within a window,
  and wrong two-factor codes per account. Zero disables a limiter.
- **Passkeys** - let users register passkeys and sign in with them instead of the
  password and second factor.
- **API tokens** - let users mint personal access tokens from their profile, to
  call the API routes behind the gateway without a browser session. A token acts
  with the user's context at creation. These are the users' tokens, not the
  [admin tokens](/docs/console/access-and-agents).
- **Trusted browsers** - let users skip the two-factor challenge on a browser
  they mark as trusted, for a duration you choose.

## Traps

- **General saves on the button, its two switches do not.** The organisation mode
  and developer mode take effect on the click; the working hours wait for Save.
- **Locales are the application's, not the console's.** The console is English
  only.
- **An empty password policy accepts anything**, however short. The screen says
  so in as many words.
- **Session TTL is on Security, not General**: how long one stays signed in is a
  security policy, next to the second factor that gates the same session.
