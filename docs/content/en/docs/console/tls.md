---
title: Domains & TLS
section: The console
order: 158
summary: A pool of certificates placed on the console and the application - generating, importing, and letting an authority issue them - and the names a proxy in front serves.
---

# Domains & TLS

**Infra > Domains & TLS.** The certificates this gateway holds, where each one
is served, and the names a proxy in front of it serves over HTTPS. It sits in
the Infra plane for the same reason the relay does: it is a property of the
installation, not of the application it serves.

There is no *switch HTTPS on*. **A certificate placed on a plane is what opens
its door**, and taking it off is what closes it. A switch that can be on with
nothing behind it is a switch that lies.

![The Domains & TLS screen: the pool of certificates, then the Console, Application and Internal doors, each with its ports, then the trusted authorities](img/console/tls.webp)

Here the pool holds four lines: a certificate for `gateway.acme.example` served on
the console, one for `apps.acme.example` and `docs.acme.example` served on the
application, a signing request still waiting for its answer, and an order to
Let's Encrypt (staging) for `status.acme.example`, kept in reserve: placed on a
door, it would be asked at once.

## A pool, then a placement

A certificate answers for **the names it carries** - read from the material, not
typed beside it. One certificate can carry several names, a wildcard
(`*.acme.example`) or an IP address, and serves every one of them.

- **Certificates** - the pool: every certificate this gateway holds, served or
  not. Each line gives its names, where it came from (*Imported*, *Self-signed*,
  *Signed on request*, *File system*), how long it lasts, its key type, and
  where it is served.
- **Console**, **Application** and **Internal** - the three doors, the last one
  being what the services behind the gateway call. **Drag a certificate onto
  one**, or onto several: it is served there at once. Drag it from one door to the
  other to move it; drag it back onto the pool, or click its cross, to take it
  off. The
  **Serve on** entries of a certificate's menu do the same without a mouse.

Each door's title gives its two ports as the world reaches them -
`HTTP:19090  HTTPS:19443` - the HTTPS one lit when a certificate is placed there,
red when the deployment does not publish it. Under the door, the HTTPS links of
the names it serves. When the console is served over HTTPS and none of its
certificates carries the name it was reached by, the door says so: the browser
would get a certificate for another name, and refuse it. The Internal door goes
further: it lists, under **Called as**, every name the runtime resolves to it -
the four forms of a Kubernetes or OpenShift Service, a Swarm service and its
short name in the stack, a compose service and its container - and ticks those a
placed certificate carries. A certificate that carries none of them is refused:
its row shows no Internal tag. The other way round, the Console and Application
doors refuse a certificate that carries only those names - they reach the internal
plane's ports, never theirs - and its row shows only the Internal tag. The import dialog says it as soon as a PEM is
pasted, and the file system dialog beside each mounted certificate. Over HTTPS
the address given to the services is the first of these names the certificate
carries.
Its **Where does its certificate come from?** button opens a how-to for the
platform it runs on - Kubernetes (cert-manager), OpenShift, an authority of your own, Swarm,
compose. Its templates are written for any service of the platform, since all
of them serve HTTPS the same way, with `<service>` and `<namespace>` to fill
in, and say to add the authority to Trusted authorities so the gateway calls
them over HTTPS. **Only Meerkat** fills them with this gateway's own names, and
adds what is its own: the chart value that mounts it, and its Internal tag.

**Add certificate, Ask cert-manager**, on Kubernetes only, has the gateway ask
cert-manager for its internal certificate: at once when one issuer fits, after
picking it when several do; the rest is decided by the gateway. It is greyed elsewhere, with the reason. See
[Asked of cert-manager](/docs/operations/tls#asked-of-cert-manager).

**Add certificate, From the file system** lists what the runtime mounted in the
container - a Kubernetes Secret, a Swarm secret, a compose volume - with the key
found beside each certificate. Only the paths are kept, and a renewal written
into the files is served on its own. See
[A certificate from the file system](/docs/operations/tls#a-certificate-from-the-file-system).

The line under a certificate answers *will this work*, never *here is a status
code*:

| It says | It means |
|---|---|
| Valid until *date* | Nothing to do |
| Expires in *N* days | Under thirty days, coloured accordingly |
| Expired on *date* | Browsers are already refusing it |
| Awaiting signature | A signing request was created and not answered yet |

Two more warnings appear where they matter: a **self-signed** badge (nobody
vouches for it but itself), and **No intermediate** - a chain that works in
`curl` and fails in a browser that has never met the issuer.

## Adding a certificate

The **Add certificate** menu has five entries. Each puts one certificate in the
pool, served nowhere until it is placed:

1. **Generate a self-signed one** - the names (separated by spaces or commas:
   host names, a wildcard, IP addresses), organisation, key type (ECDSA P-256 or
   P-384, RSA 2048 or 4096) and validity in days. Good for a lab, and browsers
   will warn. `localhost` also carries `127.0.0.1` and `::1`.
2. **Import a PEM pair** - the certificate (then its intermediates) and its
   private key: paste them, choose the files, or drop them on the dialog. Each
   file goes where it belongs, by what it holds.
3. **Import a keystore** - a `.p12` / `.pfx`, chosen or dropped, and its password.
4. **Create a signing request** - the names and the key, for your own authority.
   The line then says *Awaiting signature*; when the answer comes back, **Adopt**
   pastes it in, and the certificate can be placed.
5. **Ask** followed by an authority's name - one entry per ACME authority set up
   (see below). Host names only: no wildcard and no IP address, since the
   authority checks each name by connecting to it on port 443. The line names
   its authority and says *Asked as soon as it is placed on a door*.

## Two certificates for one name

Placing a certificate on a door where another one already answers for one of
its names asks first: it names the one in the way, and **Replace** takes that one
off the door - it stays in the pool. When the one replaced carried names the new
one does not, the question says which names stop being served over HTTPS there.

Renewing is the same gesture: add the new certificate, drop it on the door,
replace. The old one stays in the pool until you delete it.

A client that sends no server name - one that reached the gateway by its IP
address - receives the **oldest** certificate placed on that door.

**Delete** (in the menu) destroys the private key. When the certificate is
served, the question says which door goes back to plain HTTP.

On a laptop or in a lab, the **On a local machine** button opens the steps to a
certificate browsers trust without warning: the hosts file, a local authority
(mkcert), one certificate for every name, then Import - see
[TLS and certificates](/docs/operations/tls#on-a-local-machine).

## Force HTTPS

In the Application door. Callers arriving on the plain port are sent to HTTPS -
those that used a name a certificate placed on the application carries. A service
calling the gateway by its cluster name (`http://meerkat:8080`) stays in the clear:
sent to HTTPS it would meet a certificate for another name, from an authority it
does not know, and stop working. The console is never forced, and neither is the
liveness probe. If every certificate expires, the redirect **stands itself
down** and says so, rather than sending callers to a door none of them will
open.

The switch is always there, greyed while the application's HTTPS door is not
open - no certificate is placed on the application yet - with a line saying so.
**HSTS duration** sits under it, active once HTTPS is forced: forcing HTTPS also
sends HSTS, so browsers stop sending even the first request in clear - the one a
redirect cannot protect. A day by default, up to two years. Never sent to
localhost or an IP address; a route or a service that sets its own value keeps
it. See [TLS](/docs/operations/tls).

## Behind a proxy that terminates TLS

![The Application door open: its drop zone, then two names declared as reached through a proxy that terminates TLS, then Force HTTPS](img/console/tls-proxied.webp)

In the Console and Application doors, **Also reached through a proxy that
terminates TLS** holds the names a proxy in front of the gateway - nginx, an
ingress, a load balancer - serves over HTTPS. Type a name and press Enter, a
space or a comma; a list pasted in one go becomes one chip per name. The cross
of a chip takes it away. Each change is saved at once, and a name refused says
why: a scheme, a port, a path or an IP address has no place there, and a
wildcard stands for one label (`*.example.com`).

What it is for: behind such a proxy no request reaches the gateway's own TLS,
so a password reset or an address confirmation would mail a code. Asked under a
declared name, it mails a **link** back to that name instead, in `https://`.
What it is not for: these names open no HTTPS door, choose no certificate, are
never redirected by Force HTTPS and are never sent HSTS - the gateway has no
certificate to keep those promises with. When the console itself is reached by
one of its declared names, the warning about a certificate that does not carry
its name stays quiet: the browser was shown the proxy's. See
[Behind a proxy that terminates TLS](/docs/operations/tls#behind-a-proxy-that-terminates-tls).

## ACME authorities

> [!NOTE] Enterprise
> ACME is part of the Enterprise edition (SSL-05). On the community image the
> button is locked; certificates are generated, imported or signed on request.
> A configuration imported there leaves its ACME part out, and its plan says so.

The **ACME** button, beside Add certificate, opens a drawer: a form at the top, the
authorities already set up underneath - each with what it is asked for. There
are a handful, never fifty, and each one saved becomes an **Ask** entry of the
Add certificate menu.

Pick a **provider**: the form shows only what that one uses.

| Provider | What it asks for |
|---|---|
| Let's Encrypt (staging) | the terms, nothing else. Its certificates are not trusted by browsers, but it does not count against the weekly limits: the place to check that a name reaches the gateway |
| Let's Encrypt | the terms, nothing else. Fifty certificates a week per registered domain |
| ZeroSSL | an account key ID and HMAC key - ZeroSSL dashboard, Developer, *EAB Credentials for ACME Clients* |
| Google Trust Services | an account key ID and HMAC key - `gcloud publicca external-account-keys create` |
| Another authority | its directory URL, its root certificate when this gateway does not already trust it (`step ca root` for step-ca), a binding and a contact if it requires them - every value from whoever runs it |

Saving asks the authority's directory whether it answers: a wrong URL or an
unreachable authority is reported at once. No account is opened yet - that waits for
the first certificate asked. The HMAC key is a secret and goes through the
[vault](/docs/console/vault). An authority still asked for a certificate cannot
be deleted; the refusal names what asks it.

### What happens when it is asked

Nothing leaves when an authority is saved, nor when a certificate is asked of it
and kept in reserve. **Placing it on a door** is what sends the request, at once,
in the background:

1. the first time, an account is opened at the authority (its terms accepted);
2. the certificate is ordered, and the authority asks for proof of each name;
3. it connects to the name **on port 443**, where the gateway answers its
   challenge (TLS-ALPN: port 80 stays closed);
4. it signs, the certificate is stored and served at once, and renewed a month
   before it ends - with nobody touching anything.

The line says *Asking the authority...* while this runs - a few seconds, the
screen refreshes on its own - then shows the certificate's end, or **the
authority's refusal** with a **Retry** button. The usual refusals are explained:
the name does not point here (or is proxied - at Cloudflare it must be *DNS
only*), port 443 does not reach the gateway, a CAA record reserves the domain for
another authority, the rate limit is reached.

![The ACME authorities drawer: the provider and its fields, the authorities already set up with what they are asked for](img/console/tls-authorities.webp)

Expiry does not rely on the authority's e-mails - Let's Encrypt stopped sending
them in 2025: the [daily digest](/docs/console/mail-relay) names an automatic
certificate that enters its window, which means its renewal is failing.

## Trusted authorities

Below the two doors, **Trusted authorities** lists what the gateway accepts when
it calls an upstream in HTTPS, beyond the public authorities. **Add authority**
opens a dialog: a name, and the authority's certificate - choose its file, drop
it on the dialog, or paste it.

Each entry shows the certificates it holds and until when. See
[Calling upstreams in HTTPS](/docs/operations/tls#calling-upstreams-in-https).

## Traps

- **A certificate answers for the names it carries.** A browser asking for a
  name no certificate on that door carries gets the oldest one, and refuses it.
- **A public authority cannot see your private DNS.** Use an internal authority
  for internal names, or import.
- **Do not forget the intermediates** when importing: the console will tell you,
  but only after the fact.
- **The EAB HMAC key is a secret** and goes through the
  [vault](/docs/console/vault) like every other one.
