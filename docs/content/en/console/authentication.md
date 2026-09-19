---
title: Authentication
section: The console
order: 156
summary: The authorities people may sign in through, the accounts held here included.
---

# Authentication

**Infra > Authentication.** Every door into the data plane: the passwords this
gateway holds, and the identity providers and directories it delegates to.

An authority proves who someone is. It never decides what they may do: a first
sign-in creates an account that reaches nothing until an administrator places it
in an organisation and grants roles. Switch every authority off and nobody signs
in to the data plane, which is what a gateway serving only public routes may
want.

![The Authentication screen: the self-registration switch and one row for the local accounts](img/console/auth-providers.webp)

A fresh installation: self-registration off, and a single authority - the
passwords held here - which is switched on.

## The list

One row per authority: its name and identifier, its kind, the server it reaches,
and a switch. Turning one on or off is a click on the row - it is a decision, not
an edit.

An **Invite only** pill means this authority refuses to create accounts: only
people already linked to one may come in.

Above the table, **Self-registration** is the application-wide answer. Each
authority may say otherwise; with this off, none of them may.

## Local accounts

The first row is the accounts Meerkat holds itself. It has no server to reach,
but it owns two questions, and opening it asks them:

- **Self-registration** - inherited, allowed or refused. Allowed puts a sign-up
  form on the sign-in page, with the address confirmed by e-mail, so it needs a
  [mail relay](/#/docs/console/mail-relay).
- **Anti-robot check** - guards that form, and nothing else. Someone arriving
  through a directory was already made to prove themselves there.

> [!NOTE]
> Turning local accounts off closes password sign-in on the **data plane**. This
> console always keeps its own password sign-in, so you cannot lock yourself out
> of it here.

## Adding an authority

The kind is chosen first, and cannot be changed afterwards.

| Kind | What it is |
|---|---|
| **OpenID Connect** | An identity provider the browser is sent to. Its token is verified against its published keys |
| **Directory** | An LDAP directory or an Active Directory, asked directly with what the person typed. No button on the login page |
| **GitHub** | GitHub proves the account but signs nothing: the trust rests on the exchange and on TLS |
| **SAML** | Not available yet |

> [!NOTE]
> Enterprise edition: **Directory** (LDAP and Active Directory).

At the top of the editor, **Register these on the authority** hands you the
values to paste into the vendor's form, under the vendor's own labels, each with
a copy button. For GitHub there is a direct link to the form that creates an
OAuth app. If you reached this console by a local address, the block warns you:
a vendor handed a `localhost` callback sends everyone back to a machine that is
not yours.

### Fields worth a word

- **Identifier** - derived from the name, and the segment a sign-in URL travels
  through. It is frozen after creation, because changing it would break the
  callback already registered with the vendor.
- **Issuer** (OIDC) - the discovery document is read from there, so the rest of
  the endpoints need not be typed.
- **Allowed e-mail domains** (OIDC) - empty accepts every address the authority
  knows. Fill it when the provider is shared with people who are not yours.
- **Allowed organisations** (GitHub) - **empty lets in any GitHub account**.
  These organisations and teams also come back as groups, named `org` and
  `org/team`, which a [group rule](/#/docs/console/organisation) can turn into
  roles.
- **Service account** (Directory) - used to search the directory, never to sign
  anyone in.
- **User filter** (Directory) - `%s` is what the person typed; empty uses the
  dialect's default.
- **Skip the certificate check** - for a directory with a self-signed
  certificate. It is the one field here that removes a protection.

### Policies

Two questions every authority answers for itself, each able to defer to the
application:

- **Self-registration** - inherited, allowed, refused.
- **Two-factor** - inherited, always required, or left to the authority (for a
  provider that already challenges).

## Before you leave the editor

**Test the connection** actually reaches the server and says what came back.
Do it before telling anyone the authority is ready. Deletion is in the danger
zone at the bottom.

## Traps

- **A first sign-in grants nothing.** It creates an account with no organisation
  and no role. Either place people by hand on
  [Members](/#/docs/console/organisation), or write group rules.
- **Empty allow-lists are open doors.** GitHub with no organisation named lets
  in the whole of GitHub.
- **Secrets are not typed twice.** A client secret goes into the
  [vault](/#/docs/console/vault) and cannot be saved as a literal.
