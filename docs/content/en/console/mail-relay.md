---
title: Mail relay
section: The console
order: 160
summary: The SMTP server that delivers account e-mails, and the one message Meerkat sends on its own.
---

# Mail relay

**Infra > Mail relay.** The SMTP server that carries confirmations, password
resets and administrator notices. It is infra because it is a third-party service
reached by host and port, with credentials - the same nature as a route's
upstream.

Several features are switches with nothing behind them until this is set:
self-registration with a confirmed address, password reset links, the one-time
code by e-mail, and the daily digest.

## The relay

- **Host**, **Port**, **Security** - `STARTTLS`, `TLS` or none.
- **Sender address** - it lives here because a provider only accepts the account
  it authenticated. Empty means *the account*, as long as the account is itself
  an address. Below the field, the console shows what recipients will actually
  see, name and address combined.

Then the credentials, as two separate forms rather than one generic set of boxes:

| Mode | When |
|---|---|
| **Password** | An account and a secret. What every transactional relay uses, and what Gmail accepts with an application password |
| **OAuth2** | A token minted by client credentials stands in for the password (XOAUTH2). The only way into Microsoft 365, which no longer accepts a password over SMTP |

OAuth2 asks for the **token URL** (your tenant's token endpoint), the **client
ID**, the **client secret**, and a **scope** - left empty it uses Microsoft's
SMTP scope.

## Send a test

Pick which message, which language, and the recipient (your own address by
default), and a real message goes through the relay with fake values but the real
look. This is the fastest way to find out that a port is closed or a sender
refused.

## Daily digest

Once a day, the administrators who carry an e-mail address are told which
accounts are about to lose access, which just did, and which vault entries are
about to reach their reminder date.

- **Hour** - read in the gateway's own clock, which the field prints under it (time and zone).
- **Look ahead** - how many days forward to report, one to ninety.

A day with nothing to say sends nothing. With no relay configured the notice
stays owed and goes out the morning a relay answers.

## Traps

- **A typed secret blocks Save.** The password and the client secret must be
  moved into the [vault](/#/docs/console/vault) first; that is the only way they
  get stored. The button stays disabled until they are.
- **The sender name is not typed anywhere.** The display name in front of the
  address is the application's name, from
  [Built-in pages, Branding](/#/docs/console/built-in-pages), resolved when the
  message is sent. Only the address is here.
- **Test before you announce.** The relay is the judge of an address, not the
  console.
