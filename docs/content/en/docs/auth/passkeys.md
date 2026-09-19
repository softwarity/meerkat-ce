---
title: Passkeys
section: Authentication
order: 112
summary: WebAuthn sign-in with a security key, a fingerprint or Windows Hello - what works today, and what is not there yet.
---

# Passkeys

A passkey is a WebAuthn credential held by the browser, the phone or a security
key: Touch ID, Windows Hello, a YubiKey. Signing in with one asks for no
username and no password.

The feature is **partial**. What follows says exactly which half is built, and
you should read the gaps before you rely on it.

## What works today

Passkeys are allowed by default, gateway-wide: **Application > Security**,
*Allow passkeys*. There is no per-organisation setting - a passkey sign-in
happens before the organisation is known.

**Registering one** is self-service, from `/profile/passkeys`, and it needs a
finished session: you sign in the ordinary way first, then add a key. Keys are
listed with a label read off the browser (`Chrome - macOS`), the date they were
created and the date they were last used. The owner can delete any of them, and
can still do so after an administrator turns passkeys off.

**Signing in with one** is a button on the sign-in page, drawn when the browser
supports WebAuthn and at least one authority is enabled. It is *usernameless*:
Meerkat asks for a discoverable credential, and the credential says which
account it is. Discoverable keys are required at registration, which is what
makes that possible.

The key lands you straight on your organisation, or on the chooser if you belong
to several - exactly as a password sign-in would.

## A passkey is both factors at once

This is the design, and it is the thing to understand before turning it on:

> [!WARNING]
> A passkey sign-in **never asks for a second factor**, even for an account where
> two-factor is *required*. It goes straight to organisation resolution. If your
> policy is "everybody must have two factors", a passkey satisfies it by
> assumption rather than by verification - see below.

Why the assumption is not airtight in the current build:

- **User verification is not required.** Meerkat does not ask the authenticator to prove a PIN, a fingerprint or a face. A key that only proves someone touched it is accepted. On most phones and laptops the platform verifies the person anyway, because that is how it is built - but a bare security key left in a laptop is enough on its own.
- **Attestation is not checked.** There is no trust anchor, no FIDO metadata service and no allow-list of authenticator models. Any authenticator the browser offers is accepted.

## The hostname matters

The relying party is **derived from the request**: the host that served the page,
with the port stripped, and the origin it was served on. Nothing configures it.

Two consequences:

- a passkey registered while reaching the gateway on `apps.acme.io` does not work on another hostname, so put the public name in front of it before anyone enrols;
- the administration console is a different host, therefore a different relying party. An operator who wants a passkey on the console registers one **there**, from the console's own profile page. The console also ignores the *Allow passkeys* setting, on purpose: the integrator's choice for the application must not lock an operator out of their own key.

## What a directory adds

When the account is linked to a **directory** authority, a passkey sign-in asks
that directory whether it still knows the person before letting them in. A clear
"no such entry" revokes the sign-in; an unreachable server does not sign anyone
out.

Identity providers reached by redirect - OIDC, GitHub - cannot be asked that
question, so a passkey sign-in through them is never re-validated against the
provider. And any linked, enabled authority may forbid passkeys outright for the
accounts it knows (the authority's `passkeys` policy set to *no*), which blocks
both registering and using one.

## What is not there yet

- **No recovery.** Losing your only passkey is not a passkey problem to solve: you sign in with your password, or with the forgotten-password link, and register another. A passkey **supplements** the password here, it does not replace it - a local account always keeps one.
- **No administrator view.** There is no screen and no endpoint to list, name or revoke somebody else's passkeys. They belong to the account. What an administrator has is the blunt instrument: deleting the account takes its keys with it.
- **No per-key policy** - no way to demand user verification, or a particular kind of authenticator.
