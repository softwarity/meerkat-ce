---
title: SAML 2.0
section: Authentication
order: 105
summary: Send people to an identity provider that speaks SAML - ADFS, Entra ID, Okta, Shibboleth - and verify its signed answer.
---

# SAML 2.0

A SAML authority does what an [OIDC](/docs/auth/oidc) one does, for the
companies whose identity provider speaks SAML: the browser leaves for the
provider, the person signs in there, and the provider sends back a **signed
assertion** saying who they are. Meerkat never sees their password. Enterprise
edition.

It is SP-initiated: the sign-in starts on the gateway's own page, with a button
per provider. An assertion the provider sends on its own, without a request
from this gateway, is refused - nothing would tie it to the browser presenting
it.

## Declare it in Meerkat

**Infra > Authentication**, a new authority, kind **SAML**:

- **Identity provider metadata URL** - where the provider publishes its
  metadata (ADFS: `/FederationMetadata/2007-06/FederationMetadata.xml`). It is
  read again every hour, so a new signing certificate published ahead of a
  rollover is picked up on its own. A provider that publishes no URL: paste its
  metadata instead.
- **NameID format** - how the provider names the person. Leave it to the
  provider, or ask for `persistent` or `emailAddress`.
- **Attributes** - which attribute carries the address, the name and the
  groups. Empty, the usual names are tried: the claim URIs of ADFS and Entra
  ID, the short names of Okta and Shibboleth.
- **Allowed e-mail domains**, as for OIDC.

**Test** reads the provider's metadata and says what is missing: no signing
certificate, or no sign-in endpoint for the HTTP-Redirect binding this gateway
sends its requests by.

## Declare Meerkat at the provider

The screen hands over three values, each with its copy button:

| What the provider calls it | What it is |
|---|---|
| Entity ID, identifier, audience | `https://<gateway>/login/<id>/metadata` |
| Assertion consumer service, reply URL | `https://<gateway>/login/<id>/callback` |
| Service provider metadata | the same `/metadata` address: most providers import it rather than have the two above typed |

The gateway's metadata says what it expects: signed assertions, posted back
(HTTP-POST).

## What the answer is checked against

Everything, and refused at the first miss:

- **the signature**, against the provider's certificate from its metadata - by
  a library that thousands of deployments exercise, never by hand: signature
  wrapping is where SAML implementations break;
- **the issuer**, the provider's entity ID;
- **the audience**, this gateway's entity ID;
- **the destination and the recipient**, this gateway's consumer service;
- **the validity window** (`NotBefore`, `NotOnOrAfter`), and a certificate that
  has expired;
- **the request it answers** (`InResponseTo`): the request this very browser
  made, kept in its own cookie;
- **a replay**: an assertion is accepted once. The ones already used are kept
  in the database until they would have expired, so a second post of the same
  one is refused on every gateway of a cluster.

The answer is posted back from the provider's site, and a browser leaves the
gateway's own cookie off such a post. The gateway answers it with a page that
posts the same answer to itself, from its own address: the cookie travels, and
nothing about the sign-in in progress is kept on the server.

## Who the person becomes here

The account is linked under the NameID - which must therefore be **stable**. A
**transient** NameID changes at every sign-in and would create a new account
each time, so it is refused, with a way out: name a stable attribute (the
address, an `objectGUID`) under **Subject attribute**, and the account is linked
by that.

What follows is every authority's story: a first arrival creates a pending
account, or is refused, as the authority's policy says; the second factor and
passkeys follow its policies; the groups it sends are what a tenant's group rule
maps to roles. See [OpenID Connect](/docs/auth/oidc#who-the-person-becomes-here).

## Signed requests, encrypted assertions

Most providers sign their answers and need nothing from the gateway. Two cases
need its own key pair:

- **Sign the requests**, for a provider that requires it;
- **encrypted assertions**, which the gateway reads with the same key.

Paste the gateway's certificate, and its private key - a secret field, kept in
the [vault](/docs/console/vault). The certificate then appears in the gateway's
metadata, for the provider to pick up.
