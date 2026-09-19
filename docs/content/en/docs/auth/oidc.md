---
title: OpenID Connect
section: Authentication
order: 104
summary: Send people to your company identity provider - Keycloak, Entra ID, Okta, Auth0, Google - and let it prove who they are.
---

# OpenID Connect

An OIDC authority delegates the **first factor** to an identity provider your
organisation already runs. The browser leaves for the provider, comes back with
a code, and Meerkat exchanges it for an identity token it verifies against the
provider's published keys. Nothing is taken on trust.

It is in both images, community included.

An OIDC button appears on the sign-in page as soon as the authority is enabled.
Several OIDC authorities can live side by side: two providers, two buttons.

## Declare it in Meerkat

**Infra > Authentication > New authority > OpenID Connect.**

| Field | What to put in it |
|---|---|
| Name | what the button on the sign-in page will say, for instance `Acme SSO` |
| Identifier | the segment this sign-in travels through, derived from the name; **frozen after creation** |
| Issuer | the provider's base URL, from which its discovery document is read |
| Client ID | the client the provider gave you |
| Client secret | its secret, if the client is confidential |
| Allowed e-mail domains | comma separated. Empty accepts every address the provider knows |

Issuers look like this:

| Provider | Issuer |
|---|---|
| Keycloak | `https://sso.acme.io/realms/main` |
| Microsoft Entra ID | `https://login.microsoftonline.com/<directory-id>/v2.0` |
| Okta | `https://acme.okta.com/oauth2/default` |
| Auth0 | `https://acme.eu.auth0.com` |
| Google | `https://accounts.google.com` |

A trailing slash on the issuer is trimmed on both sides, so `https://acme.eu.auth0.com/`
and `https://acme.eu.auth0.com` are the same authority.

The domain allow-list is compared case-insensitively against what follows the
last `@`, and a leading `@` in the list is optional: `acme.io` and `@acme.io`
both work. When the list is not empty and the provider returns no address at
all, the sign-in is refused rather than let through.

The client secret can be a vault reference (`$acme-sso-secret`) rather than the
secret itself; references resolve in the infra scope. Typing a secret in clear
blocks the save until it is put away.

## Declare Meerkat at the provider

The drawer shows a **Redirect URI** with a copy button, and that is the one
value the provider needs:

```
https://apps.acme.io/login/acme-sso/callback
```

The shape is `/login/<identifier>/callback`, on the **data plane** host.

> [!WARNING]
> That address is built from the host you reached the gateway by. If you are
> setting this up through `localhost`, register the public domain name instead -
> a provider handed a localhost sends everybody back to a machine that is not
> theirs.

The sign-in buttons also appear on the console's own sign-in page, on the
administration port. A sign-in started there calls back on **that** host, which
is a different redirect URI. If you want SSO on the console too, register both
addresses with the provider.

## What the callback is checked against

Every sign-in is a full verification, not a trust exercise:

- the discovery document is read from `<issuer>/.well-known/openid-configuration` and cached for an hour; the signing keys come from its `jwks_uri` and are cached for fifteen minutes;
- `state` must match the one Meerkat minted, and **PKCE** (`S256`) is always sent, secret or no secret;
- the identity token's signature is verified against the published keys - an `alg` of `none` is refused outright;
- `iss` must be the configured issuer, the audience must contain the client ID, `exp` must be in the future, and `nonce` must match.

A provider that returns no `id_token` is told so plainly: check that the
`openid` scope is granted.

## Claims

| Identity | Claim | Changeable |
|---|---|---|
| Subject, the stable link to the account | `sub` | no |
| E-mail | `email` | no |
| Address vouched for | `email_verified` | no |
| Full name | `name` | no |
| Username | `preferred_username`, falling back to `email` | yes, `usernameClaim` |
| Groups | `groups` | yes, `groupsClaim` |

The requested scopes default to `openid profile email`.

> [!NOTE]
> `usernameClaim`, `groupsClaim`, `scopes` and `useUserinfo` - the last one
> merges what the provider's *userinfo* endpoint says into claims the token left
> out - exist in the authority's configuration but have no box in the console.
> They are set through the admin API or through an imported configuration file.

## Who the person becomes here

An identity from a provider is resolved to a local account in this order:

1. an **existing link** for this authority and this `sub` - the stable answer;
2. an account holding the **same address**, but only if the provider says that address is verified. An unverified address is never matched: it would be an account takeover;
3. a new account, if this authority is allowed to create one.

The link is recorded per authority, so someone who signed up with a local
password and later moves to SSO keeps their account, their organisations and
their roles. There is no *source* column on an account: an account can be known
by several authorities at once, and someone may sign in through any of them.

A freshly created account reaches **nothing**. It lands in the waiting room
until an administrator places it in an organisation and grants roles.

The groups the provider reports are recorded on the link. They turn into
memberships and roles only through a **group rule** on an organisation.

> [!NOTE] **Enterprise edition.**
> Writing group rules - what turns a claim into a membership and roles - is part
> of the Enterprise edition. Reading them, and deleting them, is not gated.

## The two policies

At the bottom of the drawer:

**Self-registration** - *Allowed*, *Refused*, or inherited from the application
switch. *Refused* means only people already linked may come in: a first arrival
is turned away with "not invited" rather than given an account.

**Two-factor** - *Left to the authority* skips Meerkat's own second factor for
people who came in this way: the provider already challenged them, and asking
twice is a tax, not a defence.

> [!WARNING]
> *Always required* on an authority behaves exactly like *Inherited from the
> application*: it does not force a second factor on. What decides is the
> per-account setting, and failing that the global one in **Application >
> Security**. Only *Left to the authority* changes anything here.

And the skip is a **declaration**, not a proof. Meerkat does not read the
`acr` or `amr` claims, so it cannot tell whether the provider really did ask for
a second factor. Set *Left to the authority* only where you know it did.

## Before you leave the screen

**Test the connection** fetches the discovery document and the signing keys.
Passing it means the issuer is reachable and publishes what it must; it says
nothing about the client secret, which only a real sign-in exercises.
