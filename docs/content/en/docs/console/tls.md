---
title: TLS
section: The console
order: 158
summary: One name, one certificate - importing, generating, and letting an authority issue them.
---

# TLS

**Infra > TLS.** The names this gateway answers to, and the certificate each one
wears. It sits in the Infra plane for the same reason the relay does: it is a
property of the installation, not of the application it serves.

There is no *switch HTTPS on*. **Having a certificate is what opens the door**,
and removing it is what closes it. A switch that can be on with nothing behind it
is a switch that lies.

![The TLS screen: the console's name and certificate, two application names, the Force HTTPS switch and the ACME card](img/console/tls.webp)

Here the console answers on `localhost` with a self-signed certificate valid
until 2027, the application serves two names - one still in the clear, one with
its own certificate - and both Force HTTPS and ACME are off.

## The two sections

- **Console** - one name and one certificate. The name is a field, not a row:
  this console answers to one address.
- **Application** - one entry per host the gateway serves, each with its own
  certificate. **Add a name**, then give it a certificate.

Each entry reads as a line: the host, where the certificate came from
(*Imported*, *Self-signed*, *Signed on request*), what it says about itself, and
the links to open that name over plain HTTP and over HTTPS.

The sentence at the end of the line answers *will this work*, never *here is a
status code*:

| It says | It means |
|---|---|
| Valid until *date* | Nothing to do |
| Expires in *N* days | Under thirty days, coloured accordingly |
| Expired on *date* | Browsers are already refusing it |
| Awaiting signature | A signing request was created and not answered yet |
| This certificate does not answer for *host* | Right file, wrong name |
| No certificate: this name is not served over HTTPS | Plain HTTP only |

Two more warnings appear where they matter: a **self-signed** badge (nobody
vouches for it but itself), and **No intermediate** - a chain that works in
`curl` and fails in a browser that has never met the issuer.

## Putting a certificate on a name

The **Add certificate** menu, and the **Actions** menu on an entry that already
has one, offer the same three doors:

1. **Import a PEM or a keystore** - the pair a CA e-mails back, or a `.p12` /
   `.pfx` with its password.
2. **Generate a self-signed one** - organisation, key type (ECDSA P-256 or P-384,
   RSA 2048 or 4096) and validity in days. Good for a lab, and browsers will warn.
3. **Create a signing request** - the CSR to send to your own authority. The
   entry then says *Awaiting signature*; when the answer comes back, **Adopt**
   pastes it in.

Replacing never leaves a host bare: the new certificate is created first and the
old one dropped only once it is in, so a failed import leaves the name still
served. The last item of the Actions menu is the deliberate opposite - remove it
and serve this name in the clear.

## Force HTTPS

Callers arriving on the plain port are sent to HTTPS. The console is never
forced, and neither is the liveness probe. If every certificate expires, the
redirect **stands itself down** and says so, rather than sending callers to a
door none of them will open.

## Automatic certificates (ACME)

One account, and a tick box per name: what varies is never the plane, it is the
name.

- **Authority** - a URL rather than a vendor list, so a public authority and an
  internal `step-ca` are equally at home.
- **Contact email** - where the authority warns about expiries.
- **Which names the authority may issue** - the tick boxes. A name that only
  exists on this machine is flagged: a public authority has to reach the name to
  prove it, so it can never issue for one that does not resolve on the internet.
- **Private authority** - root certificate in PEM when the ACME server sits
  behind a private certificate, plus the external account binding pair (key id
  and HMAC key) most private authorities require.
- **I accept the authority's terms of service** - required by the protocol.
- **Already issued** lists what came back, with its expiry.

The proof runs on the HTTPS port this gateway already holds: **port 80 stays
closed**. Renewal happens weeks before expiry, with nobody touching a file.

## Traps

- **A certificate belongs to a name.** Importing the right file under the wrong
  host gives you *does not answer for this host*.
- **A public authority cannot see your private DNS.** Use an internal authority
  for internal names, or import.
- **Do not forget the intermediates** when importing: the console will tell you,
  but only after the fact.
- **The EAB HMAC key is a secret** and goes through the
  [vault](/docs/console/vault) like every other one.
