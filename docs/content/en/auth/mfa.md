---
title: Two-factor
section: Authentication
order: 110
summary: Codes from an authenticator app, a code by e-mail as a fallback, trusted browsers, and who has to use them.
---

# Two-factor

Meerkat's own second factor is **TOTP**: the six-digit code an authenticator app
shows. Two other pieces sit around it - a code by e-mail for the day the
authenticator is out of reach, and trusted browsers so the challenge is not
asked on every sign-in.

All of it is in both images.

## Codes from an authenticator app

Standard TOTP, RFC 6238: `6` digits, a new code every `30` seconds, and the
code before and the code after are accepted as well, so a phone whose clock is
slightly off still works. The secret is `160` bits.

**Enrolling** is self-service, on `/profile/mfa`. The page shows a QR code -
drawn by the gateway itself, with nothing fetched from anywhere - and the same
secret in text for a key you type by hand. Nothing is switched on until you
prove it works with a live code.

The QR code names your application, because the issuer it carries is the
application name from **Application > Built-in pages > Branding**. Change that
name and existing enrolments keep the old label: the issuer is baked into what
the app already stored.

**Backup codes.** Enrolment ends with `10` single-use codes, shown once and
never again. They read `k7m2p-3xrqh`, from an alphabet with no confusable
characters. Only their hashes are kept, so a lost sheet cannot be recovered -
it can only be replaced, from the same page, which mints ten new ones and
retires the old. The page says how many are left.

Enrolling again, or turning the second factor off, **forgets every trusted
browser** of that account. That is deliberate: the old challenge is gone, and
so is anything that was allowed to skip it.

> [!WARNING]
> TOTP secrets are stored **unencrypted** in the database. The vault's
> encryption at rest covers vault secrets, TLS private keys and the developer
> plug key - not these. Treat a database dump accordingly.

## A code by e-mail, as a fallback

Ships **off**. It is a way back in for someone who is already enrolled in TOTP
and cannot reach their authenticator - not a second factor of its own. Nobody
enrols in it, and an account without TOTP is never offered it.

The link only appears on the challenge page when all of this holds:

- the switch *Allow a one-time code by e-mail* is on, in **Application > Security**;
- a mail relay is configured;
- the account carries an address;
- the account is enrolled in TOTP, which is what put it on this page.

The code is `6` digits, valid for `10` minutes, single-use, and tied to the
account - it is typed in the same box as the TOTP code. Asking again within
`45` seconds shows the same "we sent it" page without sending a second mail.
A successful sign-in wipes any code still pending.

With the switch off, the link is absent and the endpoint behind it answers
`404`.

## Trusted browsers

Ships **off**, with a trust lasting `7` days when you turn it on (the console
offers one day, seven, fourteen or thirty, in **Application > Security**).

After a successful challenge, the person may tick *trust this browser*. The next
sign-in from it skips the code.

> [!WARNING]
> The trust is a **random token in a cookie**, not a fingerprint of the machine.
> Nothing about the browser, the address or the device is compared on the way
> back in. The name you see in the profile - `Chrome - macOS` - is a label read
> off the user agent for your benefit; it is never checked. Whoever holds the
> cookie skips the challenge, which is why the duration is worth keeping short.

Only the hash of the token is stored. The profile lists the trusted browsers,
marks the one you are on, and revokes them one at a time or all at once.
Turning the setting off restores the challenge for everyone immediately - the
policy is re-read on every sign-in.

## Who has to use a second factor

Two levels, and that is all there is:

| Level | Where | Values |
|---|---|---|
| The installation | **Application > Security**, *Require two-factor for everyone* | on or off, ships **off** |
| One account | **Application > Users**, in the account's editor | *Inherited*, *Required*, *Optional* |

The account's own setting wins; otherwise the installation's answers. *Optional*
on an account exempts it even when the installation requires it.

Someone for whom the second factor is **required** and who has none is sent
through enrolment at their next sign-in, before they reach anything, and cannot
turn it off afterwards - the self-service disable answers `403` and says why.

> [!NOTE]
> There is deliberately **no per-organisation** level: the second factor is
> asked before the organisation is known, so a rule per organisation could not be
> read in time. There is **no per-role level either**, for the same reason; it is
> the piece FEATURES.md still lists as missing on this feature.

An authority can **waive** the challenge for people arriving through it -
*Two-factor: left to the authority* on an OIDC, directory or GitHub authority.
See [OpenID Connect](/#/docs/auth/oidc) for what that setting does and does not
do.

## The one gap worth planning around

**An administrator cannot reset, clear or disable somebody's second factor.**
There is no such endpoint and no such screen: the enrolment belongs to the
account, and the admin API never touches those columns.

So a person who loses their authenticator **and** their backup codes, on an
installation where the e-mail fallback is off, has no way back into that
account. The ways back in are, in this order: a backup code, the code by
e-mail, or - if the second factor is not required for them - turning it off
themselves. Failing all three, what remains is deleting the account and making
another.

Turning on the e-mail fallback is the cheapest insurance against that call.
