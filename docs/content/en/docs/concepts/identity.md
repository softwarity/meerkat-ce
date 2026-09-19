---
title: Identity
section: Concepts
order: 22
summary: The three ways the gateway knows who is calling - a session, an API token, or nothing at all - and where the accounts come from.
---

# Identity

Before deciding whether a request may pass, the gateway works out who is making
it. There are exactly two answers it can find, and a third case it handles as
such.

| | What it is | Who uses it |
|---|---|---|
| Session | an opaque token in an httpOnly cookie | a person in a browser |
| API token | `Authorization: Bearer mk_...` | a script, a service, an agent |
| Anonymous | neither | anything a route lets through without asking |

## Sessions

A session is a row in the database; the browser holds a token whose hash is
what is stored. Destroying a session destroys the row, so a signed-out cookie is
not merely forgotten - it no longer matches anything.

Each plane has its own cookie, `MEERKAT_SESSION` on the data plane and
`MEERKAT_ADMIN_SESSION` on the control plane, and every session is stamped with
its plane. A token from one is refused on the other.

The lifetime measures **inactivity**, not time since sign-in: any request
carrying a live session pushes its deadline. The default is 30 minutes, and it
resolves from the member, then the organisation, then the installation.

> [!NOTE]
> That resolution exists, and only the global value is editable in the console
> today. The per-organisation and per-member overrides are stored but have no
> editor yet.

A session also carries what the sign-in flow settled: the active organisation,
the active group, and the step still owed. A session stuck on a step is bounced
back to it on every navigation until it completes.

> [!WARNING]
> The session cookie carries no `Domain` attribute, so it is not shared between
> sub-domains. Applications served under different host names do not share a
> session today.

## API tokens

A token is minted once and its clear value is shown once. It is prefixed `mk_`,
and presented in the standard place:

```bash
curl -H 'Authorization: Bearer mk_...' https://apps.example.com/billing/invoices
```

A token belongs to one **plane**, and the two are isolated: a data-plane token
never opens the admin port, and an admin token never passes on the data plane.
Admin tokens are minted by a global administrator only, from the console; a
person mints their own data-plane token on `/profile/tokens`.

A token can only ever hold **less** than its owner. Three axes narrow it: a
scope (read-only, full, or metrics only), a domain (the routing side, the
application side, or both), and a list of client networks - judged on the actual
peer address, never on a forwarding header.

> [!NOTE]
> Data-plane tokens carry no perimeter yet: they are minted with full scope, and
> they silently capture whichever group the session had. There is no console
> screen for them either - the self-service page is the only place.

## Where accounts come from

An account is recognised by an **authority**, and the local account table is one
authority among the others. One screen, **Infra > Authentication**, lists them
all and answers the question *by what may somebody sign in here*.

| Authority | State |
|---|---|
| Local accounts (password) | shipped |
| OIDC - any conforming provider: Keycloak, Entra ID, Okta | shipped |
| GitHub | shipped |
| LDAP / Active Directory | shipped, Enterprise edition |
| SAML 2.0 | not built - the kind can be stored, and is refused when used |
| Kerberos / SPNEGO | not built |

An external authority answers the **first factor only**. It says this is the
person; it never decides what they may do here - that is the gateway's, and it
comes from roles, groups and memberships.

Several settings are per-authority, with a third state that inherits the global
one: whether a second factor is required, whether passkeys are allowed, and
whether an unknown person signing in gets an account created.

## The sign-in flow

The pages are served by the gateway, on the data plane, in the visitor's
language and the installation's theme. The order is fixed:

1. **Password update**, if it is temporary or expired.
2. **Second factor**: a TOTP code if the account is enrolled, unless this browser has been marked trusted; forced enrolment if MFA is required and there is no factor yet.
3. **Organisation**, then **group** when the organisation works in single-group mode.

With no organisation at all, the person lands in a waiting room that explains
how to ask for access, rather than on a refusal.

The control plane skips steps 2's tenant half entirely: the console has no
organisation to choose.

## Read more

The **Authentication** section covers each of these in detail - the password
policy and sign-in throttling in [Local
accounts](/docs/auth/local-accounts), then the second factor, passkeys and
each external authority.

Who may pass, once we know who they are: [Access
control](/docs/concepts/access-control).
