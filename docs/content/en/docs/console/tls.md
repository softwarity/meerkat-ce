---
title: Domains & TLS
section: The console
order: 158
summary: The console, application and internal planes, one tab each - their certificates, generated, imported or issued by an authority, who serves their public names over HTTPS, and the addresses of a proxy in front.
---

# Domains & TLS

**Infra > Domains & TLS.** The planes of this gateway, one tab each: the
certificates each one serves, its public names, and whether the gateway or a
proxy in front serves them over HTTPS. It sits in
the Infra plane for the same reason the relay does: it is a property of the
installation, not of the application it serves.

There is no *switch HTTPS on*. **A certificate placed on a plane is what opens
its door**, and removing it is what closes it. A switch that can be on with
nothing behind it is a switch that lies.

![The Domains & TLS screen: one tab per plane, Console, Application and Internal, each with its ports, who serves its public names and its certificates, then the Trusted authorities tab](img/console/tls.webp)

The **ACME**, **Platform certificates** and **On a local machine** buttons sit
in the banner, beside the title; everything else is in the tab of the plane it
concerns.

## Plane by plane

A certificate answers for **the names it carries** - read from the material, not
typed beside it. One certificate can carry several names, a wildcard
(`*.acme.example`) or an IP address, and serves every one of them.

The screen gives each plane a tab, under the name the runtime gives it:
**Console**, **Application** and **Internal**, the last one being what the
services behind the gateway call. A fourth tab, **Trusted authorities**, holds
what the gateway accepts when it calls an upstream (see
[Trusted authorities](#trusted-authorities)). A small lock on a plane's tab says
its HTTPS is open. Each plane's tab holds its **Certificates** list and its own
**Add certificate** menu. Each line of the list gives a certificate's names,
where it came from (*Imported*, *Self-signed*, *Signed on request*, *File
system*, *cert-manager*) and how long it lasts.

- **A certificate added from a plane's menu lands on that plane**, served there
  at once. The same file may be added again from another plane's tab, to be
  served there too.
- **The bin of a certificate's line is the one way out.** When another plane
  serves the certificate too, it only leaves this plane, after a confirmation
  that names where it stays; when this plane is the only one, the certificate is
  deleted, its private key with it - and its Certificate too, when it was asked
  of cert-manager.
- **Unassigned certificates** appears only when a certificate is on no plane -
  one a replacement took off, for instance (see
  [Two certificates for one name](#two-certificates-for-one-name)). Click one of
  its plane tags to place it there, or delete it.

The first line of a tab gives, on the right, the plane's two ports as the world
reaches them - `HTTP:19090  HTTPS:19443` - HTTP with its switch, HTTPS with a
lock, closed when a certificate is served there, red when the deployment does
not publish it (see [HTTP and HTTPS ports](#http-and-https-ports)). On the left, the
**Internal names**: the names the runtime reaches the plane by, ticked when a
certificate served there carries them. They show on Internal, and on Console or
Application when a proxy is in front (see
[Who serves the public names](#who-serves-the-public-names)). Under a plane the
gateway serves itself, the HTTPS links of its names. When the console is served
over HTTPS by Meerkat and none of its certificates carries the name it was
reached by, the plane says so: the browser would get a certificate for another
name, and refuse it. On the Internal plane, the Internal names are every name
the runtime resolves to it - the four forms of a Kubernetes or OpenShift
Service, a Swarm service and its short name in the stack, a compose service and
its container. A certificate that carries none of them is refused:
it cannot land there. The other way round, a Console or Application plane the
gateway serves itself refuses a certificate that carries only those names - they
reach the internal plane's ports, never theirs; behind a proxy, names inside the
runtime are precisely what its certificate carries (see
[Who serves the public names](#who-serves-the-public-names)). The import dialog says it as soon as a PEM is
pasted, and the file system dialog beside each mounted certificate. Over HTTPS
the address given to the services is the first of these names the certificate
carries.
The **Platform certificates** button, in the banner, opens a how-to for the
platform the gateway runs on - Kubernetes (cert-manager), OpenShift (its service
CA), an authority of your own, Swarm, compose: where a certificate for a
service's names comes from, for the Internal plane, and for the hop from a proxy
in front to the Console or the Application. Its templates are written for any service of the platform, since all
of them serve HTTPS the same way, with `<service>` and `<namespace>` to fill
in, and say to add the authority to Trusted authorities so the gateway calls
them over HTTPS. **Only Meerkat** fills them with this gateway's own names, and
adds what is its own: the chart value that mounts it, and its Internal tag.

**Add certificate, Ask cert-manager**, on Kubernetes only, has the gateway ask
cert-manager for a certificate carrying the plane's names inside the cluster:
the Internal plane's, or the hop from a proxy in front to the Console or the
Application. At once when one issuer fits, after picking it when several do; the
rest is decided by the gateway. It is greyed elsewhere, with the reason - a plane
the gateway serves itself gets its public certificate from an authority, an
import or the file system. See
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

Each plane's **Add certificate** menu offers what fits that plane and its mode,
and places what it adds on that plane:

| Plane | Its menu |
|---|---|
| Console or Application, served by **Meerkat** | self-signed, PEM pair, file system, keystore, signing request, and one **Ask** per ACME authority |
| Console or Application, **A proxy in front** | PEM pair, file system, keystore, Ask cert-manager |
| Internal | self-signed, PEM pair, file system, keystore, Ask cert-manager, signing request |

The entries:

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
   its authority; added from a plane's menu, it is asked at once.

## Two certificates for one name

Placing a certificate on a plane where another one already answers for one of
its names asks first: it names the one in the way, and **Replace** takes that one
off the plane - it is kept, under **Unassigned certificates** when it is on no
other plane.
When the one replaced carried names the new one does not, the question says which
names stop being served over HTTPS there.

Renewing is the same gesture: add the new certificate to the plane, replace. The
old one is kept until you delete it.

A client that sends no server name - one that reached the gateway by its IP
address - receives the **oldest** certificate served on that plane.

Deleting - the bin on the only plane a certificate is on, or on the
**Unassigned certificates** card - destroys the private key. When the
certificate is served, the question says which plane goes back to plain HTTP.

On a laptop or in a lab, the **On a local machine** button opens the steps to a
certificate browsers trust without warning: the hosts file, a local authority
(mkcert), one certificate for every name, then Import - see
[TLS and certificates](/docs/operations/tls#on-a-local-machine).
The drawer ends on where the certificate goes, one or the other: imported on a
plane served by **Meerkat**, the gateway presents it; set on a proxy in
front, the plane says **A proxy in front** and declares the names.

## HTTP and HTTPS ports

The two ports on the right of a tab's first line are not two switches.
**HTTPS has none**: it is a state, shown by its lock - closed, in the primary
colour, when a certificate serves HTTPS on the plane; open and grey otherwise.
It opens with a certificate placed on the plane and closes when the last one
is removed. `HTTP:<port>` has a switch, on each of the three tabs, and
switching it asks first: **Apply** confirms.

**HTTP switched off does not stop the port.** It listens on `127.0.0.1` only:
out of the network's reach, it still answers a `kubectl port-forward` - the way
in to repair a broken certificate - and the health check the image runs on
itself (`meerkat health`). See
[Switching the plain port off](/docs/operations/tls#switching-the-plain-port-off).

Two rules keep every plane reachable:

- **HTTP goes off only while a certificate serves HTTPS on the plane.**
  Otherwise the switch goes back, with *Add a certificate first: with HTTP off,
  HTTPS is the only way in.*
- **The last certificate stays while HTTP is off.** Its bin then says *Switch
  HTTP on before removing the last certificate: nothing else would answer.*, and
  removes nothing.

**On the Console tab, each port also says who calls it**, live and pooled
across the gateway's replicas: *Receiving, from 10.42.1.178* when something
called it in the last 30 seconds - the address the connection comes from, a
proxy in front or the browser itself - or *Nothing in the last 30 s*. Health
checks are not counted. **The door in use is not switched off:**

- clicking HTTP while this page or a proxy comes through HTTP shows *Access the
  console through HTTPS:19443 before switching HTTP off.*, with the port the
  world reaches, and nothing changes;
- removing the console's last certificate while this page or a proxy comes
  through HTTPS shows *Access the console through HTTP:19090 before removing its
  last certificate.*

The gateway refuses the same through the API. To move a console behind a
proxy to HTTPS end to end, in this order:

1. place the certificate on the Console tab: HTTPS opens;
2. point the proxy at the console's HTTPS port;
3. open the console through it;
4. check that HTTP shows *Nothing in the last 30 s*;
5. switch HTTP off.

The Application and Internal tabs show no traffic and have no such rule: their
HTTP goes off as soon as a certificate serves HTTPS there.

## Force HTTPS

In the Application plane, when this gateway serves its names (behind a proxy,
redirecting is the proxy's business). Callers arriving on the plain port are sent to HTTPS -
those that used a name a certificate placed on the application carries. A service
calling the gateway by its cluster name (`http://meerkat:8080`) stays in the clear:
sent to HTTPS it would meet a certificate for another name, from an authority it
does not know, and stop working. The console is never forced, and neither is the
liveness probe. If every certificate expires, the redirect **stands itself
down** and says so, rather than sending callers to a door none of them will
open.

The switch is there, greyed while the application's HTTPS door is not
open - no certificate is served on the application yet - with a line saying so.
**HSTS duration** sits under it, active once HTTPS is forced: forcing HTTPS also
sends HSTS, so browsers stop sending even the first request in clear - the one a
redirect cannot protect. A day by default, up to two years. Never sent to
localhost or an IP address; a route or a service that sets its own value keeps
it. See [TLS](/docs/operations/tls).

## Who serves the public names

![The Application tab behind a proxy: who serves its HTTPS, the public names declared, the proxy addresses, then its certificates](img/console/tls-proxied.webp)

The Console and Application tabs each say first **HTTPS served by**: **Meerkat**
or **A proxy in front**. The toggle only proposes: picking the other mode shows
**Apply** and **Cancel**, and Apply asks for a confirmation before it saves.
Switching keeps the names and the certificates: a certificate that does not fit
the new mode stays placed, greyed and marked *Inactive in this mode*, and serves
again the moment the plane switches back. Each change travels with a
configuration export and reaches every node.

**A change of the Console plane that would sign you out is refused.** When you
reach the console through a proxy that tells Meerkat the browser used HTTPS, a
Console plane that no longer believes that proxy would see your own session as
plain http - its cookie would no longer come back - and sign you out. The
gateway refuses the save and says why: keep **A proxy in front**, or open the
console on its own HTTPS port first.

**Public names** are the full names browsers reach the plane by, one each. Type
a name and press Enter, a space or a comma; a list pasted in one go becomes one
chip per name, and the cross of a chip takes it away; each change is saved at once. A name
refused says why: a
scheme, a port, a path or an IP address has no place there, and neither has a
wildcard - a certificate may carry `*.acme.example`, a declared name may not,
since it would vouch for every name under it. A name belongs to one plane only.
On the Application, **the first name** is the one mailed links and absolute
addresses are written with when no request says which.

**Meerkat.** The gateway presents the names itself, with the certificates
placed on the plane. A name added has to be covered by one of them, or it is
refused (*no certificate placed on the application covers ...*): place the
certificate first, then declare the name. Names kept through a switch from a
proxy, that nothing placed covers, stay listed with a warning - browsers will
refuse their HTTPS until a certificate for them is placed. Once names are
declared, nobody's `X-Forwarded-For` is believed on the plane: nothing stands in
front, so a peer that writes one is a caller choosing its own address.

**A proxy in front.** An Ingress, nginx or a cloud load balancer serves the
names over HTTPS, and the gateway knows a request for one of them reached the
browser over HTTPS without trusting a header: its cookies are `Secure`, its links
`https://`, and a mail asked under it carries a link back to it rather than a
code. These names open no HTTPS door of the gateway, ask no authority, and are
never redirected by Force HTTPS nor sent HSTS: the proxy keeps those promises.

A certificate placed on such a plane is optional, and **encrypts the hop from
the proxy to the gateway** - an Ingress with a backend in HTTPS, an OpenShift
Route in `reencrypt`. For that hop to be encrypted, the proxy itself must be
pointed at the plane's HTTPS port and speak TLS to it: an Ingress that still
targets the HTTP port leaves the hop in clear, and the certificate unused - on
the Console, the line under each port shows which one the proxy really calls. It
carries the plane's names inside the runtime, never the public ones, and the tab
lists them as its **Internal names**, ticking those a served certificate covers: on Kubernetes, the chart's Service in its four forms
(`<release>-meerkat` for the application, `<release>-meerkat-admin` for the
console); on Swarm or compose, the one service name every port shares. When the
runtime says nothing, the plane serves every placed certificate except those
covering its public names, which are the proxy's to present. **Add certificate**
offers there a PEM pair, the file system, a keystore and **Ask cert-manager**.

**Proxy addresses (optional)** says whose `X-Forwarded-For` and
`X-Forwarded-Proto` the plane believes:

- **empty** - the default, for a proxy that runs in the cluster or on a private
  network: the loopback and the private ranges are believed, which is where an
  Ingress controller or a load balancer inside the cluster comes from;
- **its addresses** - CIDR ranges or single addresses, typed as chips, for a
  proxy that reaches Meerkat from public addresses: a CDN, or a cloud load
  balancer such as Google Cloud's (`130.211.0.0/22`, `35.191.0.0/16`). They
  replace the private ranges: name a private proxy too if one stands between.

A plane nobody said anything about keeps the old rule, the private ranges. See
[Behind a proxy or a load balancer](/docs/deploy/one-gateway#behind-a-proxy-or-a-load-balancer)
and [Behind a proxy that terminates TLS](/docs/operations/tls#behind-a-proxy-that-terminates-tls).

## ACME authorities

> [!NOTE] Enterprise
> ACME is part of the Enterprise edition (SSL-05). On the community image the
> button is locked; certificates are generated, imported or signed on request.
> A configuration imported there leaves its ACME part out, and its plan says so.

The **ACME** button, in the banner beside **On a local machine**, opens a drawer: a form at the top, the
authorities already set up underneath - each with what it is asked for. There
are a handful, never fifty, and each one saved becomes an **Ask** entry of the
Add certificate menu of a Console or Application plane Meerkat serves itself.

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

Nothing leaves when an authority is saved. **Placing a certificate asked of it
on a plane** is what sends the request, at once, in the background -
and adding it from a plane's menu places it:

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

The last tab, **Trusted authorities**, lists what the gateway accepts when
it calls an upstream in HTTPS, beyond the public authorities. **Add authority**
opens a dialog: a name, and the authority's certificate - choose its file, drop
it on the dialog, or paste it.

Each entry shows the certificates it holds and until when. See
[Calling upstreams in HTTPS](/docs/operations/tls#calling-upstreams-in-https).

## Traps

- **A certificate answers for the names it carries.** A browser asking for a
  name no certificate on that plane carries gets the oldest one, and refuses it.
- **A public authority cannot see your private DNS.** Use an internal authority
  for internal names, or import.
- **Do not forget the intermediates** when importing: the console will tell you,
  but only after the fact.
- **The EAB HMAC key is a secret** and goes through the
  [vault](/docs/console/vault) like every other one.
