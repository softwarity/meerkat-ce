---
title: How someone signs in
section: Authentication
order: 100
summary: The authorities that can recognise a person - local accounts, OIDC, a directory, GitHub - and the order the sign-in runs in.
---

# How someone signs in

Every door into the data plane is a row on one screen: **Infra >
Authentication**. The accounts Meerkat holds itself are one of those rows, next
to the identity providers and the directories - one list answers "who can prove
who they are here".

An authority proves **who** someone is. It never decides what they may do: a
first sign-in produces an account that reaches nothing until an administrator
places it in an organisation and grants roles. That half is
[access control](/docs/access/overview).

## The four kinds

| Kind | What it is for | On the sign-in page | Edition |
|---|---|---|---|
| **Local accounts** | accounts Meerkat holds itself, with a password it stores | the username and password form | both |
| **OpenID Connect** | a company identity provider: Keycloak, Entra ID, Okta, Auth0, Google | a button per provider | both |
| **Directory** | an LDAP directory or an Active Directory, asked straight | no button: it answers the same form | **Enterprise** |
| **GitHub** | a GitHub account, restricted to the organisations you name | a button | both |

> [!NOTE]
> SAML appears in the console, greyed out, and Kerberos does not appear at all.
> Neither is implemented: the SAML choice exists so the shape is visible, and
> saving one is refused. Do not plan an integration on them.

Several authorities of the same kind can coexist - two OIDC providers, two
directories - each with its own name, its own button and its own policies. The
local authority is the exception: there is exactly one, it cannot be created,
duplicated or deleted, only disabled.

Turning every authority off is legal and sometimes right: a gateway that serves
only public routes lets nobody sign in, and the page says so without saying which
door is shut.

## What every authority carries

| Field | What it decides |
|---|---|
| Name | what the button on the sign-in page says |
| Identifier | the segment of the sign-in URL, and part of the callback address a provider is given. Frozen after creation |
| Enabled | whether it answers at all |
| Order | where it sits on the page |
| Self-registration | whether a first arrival gets an account here, or is turned away as not invited |
| Two-factor | whether Meerkat still asks for its own second factor |

Its connection details - an issuer, a client secret, a server URL, a service
account - are per kind, and each of them can be a vault reference rather than the
value itself.

## The order a sign-in runs in

1. **The first factor.** A password typed into the form, a redirect to a provider, or a passkey. For a typed password, the local accounts are asked first; if they say no, each enabled directory is asked in turn, in the order on the screen. A wrong password, an unknown username and a disabled account all answer the same sentence.
2. **A password that must change** - temporary, or expired under the policy. The session exists but is stuck on that step, and every navigation lands back on it.
3. **The second factor**, if the account owes one.
4. **The organisation.** None, and the person waits in `/account-pending`. One, and it is stamped on the session silently. Several, and they choose.
5. **The group**, in exclusive-group mode when the organisation has more than one.

A passkey answers steps one to three at once: it is treated as both factors, and
lands directly on the organisation.

## The rule about stacking factors

The authority that recognises the account has the last word on whether Meerkat
asks for its own second factor. An authority set to *Two-factor: left to the
authority* means "this provider already challenged them" and Meerkat does not ask
again.

Two precisions that matter:

- It is a **declaration, not a proof**. Meerkat does not read the `acr` or `amr` claims a provider may send, so nothing verifies that the provider really asked. Set that value only where you know it did.
- The other two values behave the same as each other: *Always required* on an authority does **not** force a second factor on. What decides is the account's own setting, then the installation's. See [Two-factor](/docs/auth/mfa).

There is no *source* column on an account. Which authorities know a person is
recorded as links - one per authority and subject - so the same account can be
reached through a password, a provider and a directory at once, and someone who
signed up locally keeps everything when they move to SSO.

## Where things are configured

| Screen | What lives there |
|---|---|
| **Infra > Authentication** | the authorities, their connection details and their policies, and the self-registration default |
| **Infra > Mail relay** | the relay that confirmation, reset and second-factor mails go through |
| **Application > Security** | the password policy, the sign-in throttle, two-factor, trusted browsers, passkeys, personal API tokens, the session lifetime |
| **Application > Built-in pages** | how the sign-in pages look |
| **Application > Users** | the accounts themselves, and their per-account overrides |
