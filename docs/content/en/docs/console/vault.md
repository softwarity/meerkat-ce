---
title: Vault
section: The console
order: 180
summary: Every named value and secret the configuration points at, referenced as $name.
---

# Vault

The **Vault** entry in the rail holds every value the configuration refers to, in
one place. Two kinds live in one namespace:

| Kind | Read back | For |
|---|---|---|
| **Secret** | Never. Encrypted at rest, and the API says it is set, not what it is | A client secret, an SMTP password, an HMAC key |
| **Value** | Yes, in the clear | A URL, a base DN, a client id - anything that is not a secret but changes per environment |

Both are referenced the same way, **`$name`**, wherever a field accepts one. That
is the point: promoting a value to a secret never touches what points at it.

![The Vault screen: three entries - two secrets and a value - with their kind, their value and what uses them](img/console/vault.webp)

Two secrets reading *encrypted, never shown*, one value readable in the clear,
and the *Used by* column naming the route that points at the first one.

## The list

Name and description, kind, value (or *encrypted, never shown*), and **Used by** -
the column that tells a live entry from a leftover. Clicking a row opens the
editor; the + creates an entry.

An entry carries:

- **Name** - what `$name` will say.
- **Kind** - value or secret, chosen on a toggle.
- **Value** - typed once for a secret.
- **Description** - for whoever finds it in six months.
- **Reminder date** - the day the secret expires at its source. **It is purely a
  reminder**: the gateway cannot know a token was renewed at the provider, so
  `$name` goes on resolving. The date feeds the
  [daily digest](/docs/console/mail-relay), which lists what is coming and what
  has just passed - never the value.

## Where entries come from

Most are created from the field that needs them. A sensitive field offers **Move
into the vault**, and refuses to be saved as a literal: the administrator holds
the value, so they are the one who files it. A field that accepts a plain value
offers the same for values.

Which means the usual order is the opposite of what you might expect: you fill in
a relay, an authority or a certificate, and the vault entry appears as a result.

## The vault as a file

**Export** writes the vault as a single encrypted file, under a passphrase Meerkat
does not keep. **Import** reads one back. This is the half a configuration export
never carries: it is what bootstraps a second environment or moves a gateway.

> [!WARNING]
> An exported vault is worth exactly what its passphrase is worth, and storing the
> two together undoes the encryption. The same is true of a
> [snapshot](/docs/console/configuration) whose master key sits beside the
> database.

## What you can see

The vault scopes itself to the caller: an infra admin sees the infra entries, an
app admin the application's. The screen is open to anyone administering a plane
that holds entries.

## Traps

- **A reference is public, a literal never is.** `$stripe_key` can appear in an
  export, a diff or a screenshot without consequence. That is the whole reason the
  vault exists.
- **A secret cannot be read back**, by you or by the API. Losing it means
  replacing it at the source.
- **A configuration import can leave holes**: entries the file refers to and this
  gateway does not have. The import lists them so you can fill them in straight
  away.
- **Deleting an entry that is used** breaks whatever pointed at it. Read the
  *Used by* column first.
