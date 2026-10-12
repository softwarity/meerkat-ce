---
title: TLS and certificates
section: Operations
order: 224
summary: The five ways a certificate gets in, the three doors it is placed on, what lives in the database, and what a cluster has to share.
---

# TLS and certificates

A certificate goes into a **pool** once, and is **placed** on the console, the
application, the internal plane, or several of them (SSL-08). Its names are its own - what the material says it answers
for - so a certificate carrying several names, a wildcard or an IP address serves every one
of them, and material both planes use is one entry, not two copies to keep in step.

There is no "switch HTTPS on": **a certificate placed on a plane is what opens its door**,
and taking it off is what closes it. A switch that can be on with nothing behind it is a
switch that lies.

![The Domains & TLS screen](img/console/tls.webp)

## The ways in

None of them is optional, because the places Meerkat runs in do not resemble each other
(SSL-01):

| Way in | When it is the right one |
|---|---|
| **Import** | you already hold the certificate and its key |
| **From the file system** | the runtime mounts it - a Kubernetes Secret, a Swarm secret, a compose volume - and renews it |
| **Ask cert-manager** | Kubernetes only, the internal plane: the gateway asks cert-manager itself, when nobody wrote its Certificate |
| **Self-signed** | a lab, an internal name, a first deployment |
| **Signing request** | the gateway makes the key and the CSR, your authority signs, you adopt the answer |
| **ACME** | an authority issues and renews on its own |

A signing request is a row that holds a key and serves nothing: it exists so the request
can be downloaded again while you wait for the answer.

## The ports

One HTTPS door per plane, beside the plain one, opened and closed while the gateway runs
(SSL-02):

| Plane | Plain | HTTPS |
|---|---|---|
| Data | `MEERKAT_ADDR`, `:8080` | `MEERKAT_TLS_ADDR`, `:8443` |
| Control | `MEERKAT_ADMIN_ADDR`, `:9090` | `MEERKAT_ADMIN_TLS_ADDR`, `:9443` |
| Internal | `MEERKAT_INTERNAL_ADDR`, `:7070` | `MEERKAT_INTERNAL_TLS_ADDR`, `:7443` |

A port whose protocol cannot be named is a port nobody can write a firewall rule or a
runbook against, which is why each plane has two. The internal plane's HTTPS door
presents a certificate for the name the services call it by - its Service's, inside the
cluster - which comes from the cluster's own PKI, read from the file system (below). No
public authority can see those names, so ACME is not offered there.

**Those are the ports inside.** Between them and a browser there is usually a
mapping: a Kubernetes Service publishing `9443` as `19443`, a Docker `-p`, a Swarm
ingress. The gateway asks its runtime - its own pod and the Services that select it,
or its container through the Docker socket - and the links on the Domains & TLS screen carry
the port the world reaches. When the runtime does not publish an HTTPS door, the
screen says so instead of offering a link that reaches nothing. The Helm chart
publishes both HTTPS doors by default (`service.appTlsPort`, `service.adminTlsPort`)
and grants the read it takes (`rbac.read`).

> [!NOTE] Docker Desktop
> Its Kubernetes publishes a LoadBalancer's ports on `localhost` when the Service is
> created, and ignores ports added later. After upgrading a release to a chart that
> adds the HTTPS ports, delete the two Services and run `helm upgrade` again.

Replacing a certificate is swapping a slice behind a lock: the handshake reads it through
a callback, so the listener never moves and no connection is dropped. Opening a door binds
**first** and returns the failure before anything has changed - a taken port must not leave
an operator with no HTTPS at all.

## On a local machine

A laptop or a lab has no public name and no public authority, and a self-signed certificate
makes every browser complain. The **On a local machine** button of the Domains & TLS screen opens the
steps, per system (macOS, Linux, Windows), with the names you type - the one the console is
reached by to start with - already in the commands:

1. **The hosts file** points the names at the gateway - `127.0.0.1` when it runs on the same
   machine (Docker, a local Kubernetes), its address otherwise - a VM, minikube, another
   machine; the console offers the address it was reached by when that is one. `localhost`
   needs no line.
2. **A local authority**, made by [mkcert](https://github.com/FiloSottile/mkcert) and trusted by
   this machine: the system store, Chrome, Edge, Safari, and Firefox through `nss`. Restart the
   browsers afterwards.
3. **One certificate for every name**, signed by that authority: `meerkat.pem` and its
   `meerkat-key.pem`.
4. **Import** it - Import a PEM pair, the two files dropped on the dialog - then drag it onto
   the console and onto the application.

![The drawer with the steps for HTTPS on a local machine: the names, the hosts file, a local authority, one certificate, the import](img/console/tls-local.webp)

> [!WARNING]
> The authority's private key stays in the folder `mkcert -CAROOT` prints. Whoever holds it can
> sign for any name this machine believes: never share it - another machine makes its own.

## Which certificate answers a handshake

By the name the client asked for, among the certificates placed on that plane: the one that
carries it, and when two do, the one valid longest. A handshake that names no host - a client
reaching the gateway by address, or anything older than the SNI extension - gets the
**fallback**, the oldest certificate placed on the plane; and with nothing placed it gets an
error naming what *is* served, which is more useful than a random certificate it will reject
anyway.

Placing a certificate where another one already answers for one of its names is refused
until the caller says **replace** (`PUT /api/certificates/{id}/placement` with
`"replace": true`): the one replaced leaves that door and stays in the pool.

Switching TLS on with nothing to present is refused: it would turn a working port into one
that refuses every visitor.

## The plain port can redirect

On the **data plane only**, the plain port can redirect to the HTTPS one (SSL-06). The two
health probes are exempt: a `308` reads as "not ready". So is any request for a name no
certificate placed on the application carries: a service inside the cluster calling
`http://meerkat-meerkat.ns.svc:8080` - an internal API call - would be sent to
a certificate for another name, signed by an authority it does not trust, and fail.

With the redirect on, every HTTPS answer of the data plane also carries **HSTS**
(`Strict-Transport-Security: max-age=...`). The redirect cannot protect the
request it redirects: that first one leaves in clear, and whoever sits on the
network - a public Wi-Fi, a compromised proxy - can answer it themselves and
never pass the redirect on, keeping the visitor on plain HTTP while they talk
HTTPS to the gateway (SSL stripping). With HSTS the browser remembers, and
rewrites `http://` to `https://` itself before sending anything.

It follows the redirect rather than being a switch of its own: forcing HTTPS is
already the commitment - a `301` is remembered by browsers too. Only the
**duration** is chosen, a day by default, up to two years. It retreats with the
redirect when every certificate has expired. It is never sent to `localhost`
(the promise covers every port of a host name, so a development gateway would
force HTTPS on all its developer's local applications) nor to an IP address
(browsers ignore it there), never over a value a service or a route's
[security-headers](/docs/filters/security-headers) filter already set, and
without `includeSubDomains`.

> [!NOTE] Only on 443 - a browser limitation
> HSTS is sent only when HTTPS is reached on **443**, with plain HTTP on 80. A
> browser applies the promise to every port of the name, and when it switches a
> request to HTTPS it keeps the port: given HSTS on `8443`, it would rewrite
> `http://name:8080` to `https://name:8080` - a plain port - and every address in
> the clear under that name, the console's included, would stop answering. On
> other ports the redirect alone does the work, on every visit, and HTTPS answers
> carry `max-age=0`, which makes a browser forget a promise made before. The same
> `max-age=0` is sent when Force HTTPS is switched off, so browsers stop insisting
> at their next HTTPS visit rather than when the duration runs out.

> [!WARNING]
> A browser keeps the promise for the whole duration, even if the certificates
> are taken away, and no longer offers to continue past a bad certificate. Start
> at a day; lengthen once HTTPS is settled.

## Behind a proxy that terminates TLS

Often the HTTPS a browser sees is not the gateway's: nginx, an ingress
controller or a cloud load balancer holds the certificate, and forwards plain
HTTP to the gateway. Nothing then arrives on the gateway's own TLS door, and the
mails that prove a mailbox - a sign-up's confirmation, a new address, a reset -
carry a code instead of a link, since a link is only mailed to a name the
gateway can vouch for (see
[A link, or a code](/docs/auth/local-accounts#a-link-or-a-code)).

On the Console and Application doors, **Also reached through a proxy that
terminates TLS** lists the names the operator vouches for instead: exact names
(`shop.example.com`) or a wildcard standing for one label (`*.example.com`
covers `eu.example.com`, not `example.com` nor `a.eu.example.com`). A mail asked
under one of them carries a link back to it, always in `https://`, with the port
the request carried. Each door has its own list: a name declared on the
application says nothing about the console.

| A declared name does | It does not |
|---|---|
| let a mailed link point at it, though the request came in plain HTTP | open an HTTPS door, choose a certificate, ask an authority, trigger Force HTTPS, send HSTS |

The gateway holds no certificate for these names, so it makes no HTTPS promise
on them: the redirect and HSTS still follow only the names a placed certificate
carries. A name is a host name only - no scheme, no port, no path - and never an
IP address, which nobody can vouch for by name. A wildcard over a top-level
domain (`*.com`) is refused. The names travel with a configuration export, with
the rest of the TLS settings, and a change reaches every node through the change
bus (`PUT /api/settings/tls/proxied/{console|app}`, audited as `tls.proxied`).

> [!WARNING]
> Declare only names that really reach this gateway through that proxy. A link
> is mailed to whatever name the request carried, as long as it matches: the
> declaration is what stands between a forged `Host` and the mail.

## ACME is not Let's Encrypt

ACME is part of the **Enterprise** edition. A configuration imported on the
community image leaves its authorities and orders out, and its plan names them. A
gateway moved from the Enterprise image to the community one keeps what its
database holds but asks no authority, and the Domains & TLS screen says so: those
certificates will not be renewed.

An installation sets up as many **authorities** as it deals with - Let's Encrypt for
one domain, ZeroSSL for a customer's, the company's `step-ca` for internal names -
each a named ACME account (SSL-05). The directory of a custom one is a **URL**, and
that is the whole point: half the installations Meerkat is meant for have no route
to the internet at all. An internal `step-ca`, an EJBCA, a Windows authority with the
ACME role - any of them answers here.

| Field | What it is for |
|---|---|
| Provider | a known one fixes the directory and says what else it needs; *Another authority* carries its own URL |
| Root CA | the authority signing the ACME **server's own** HTTPS certificate, in PEM - a private ACME server is usually behind a private certificate |
| EAB key id and HMAC key | External Account Binding: which account this gateway registers under. ZeroSSL and Google require it, Let's Encrypt ignores it |
| Contact e-mail | some authorities require one; expiry is watched by the daily digest |
| Terms accepted | a legal act, so it is never assumed |

Each authority keeps its account key and what it issued in **its own corner of the
shared cache**; changing its directory gives it a fresh one, since an account
registered at one authority is worthless at another.

The names are not a field of the account: they are **orders** in the certificate pool
(*Ask* followed by the authority's name), each placed on the console, the application
or both, like any certificate. A door asks an authority only for the names of the orders
placed on it, and the list is closed: an open policy would let anyone reach the gateway
by address with a made-up SNI and burn the authority's rate limit on names nobody owns.
An authority with no order placed is not armed at all.

**Placing an order sends the request at once**, in the background, through the same
path a handshake would take - the cluster lock included - rather than waiting for the
first visitor to meet the whole exchange or its failure. The node that asked keeps
what happened (requesting, or the authority's refusal, explained) for the pool to
show; the certificate itself lands in the shared cache every node reads. The orders
and authorities travel with a configuration export - an account's HMAC key as a
`$name`, never as a literal; what the authority issued stays in the cache.

## A certificate from the file system

On Kubernetes, Swarm or compose, the certificate a gateway should present usually already
exists, made and renewed by the platform: cert-manager writes it in a Secret, OpenShift's
service CA writes one for a Service, Swarm hands a secret, compose mounts a volume. The
gateway does not ask where: it reads its own **mount table**, which lists everything the
deployment mounted in its container, wherever the manifest put it.

**Add certificate, From the file system** shows the mounts that hold a certificate, each
with the private key found beside it - a certificate without one serves nothing and cannot
be picked. The mounts that are never a PKI are left out: the process's own pseudo-files,
what every container is given (`/etc/hosts`, `/etc/resolv.conf`), the service account's
token, and the data directory, where the database and the vault key live. The content of
a file is never shown: what it is, its names, its end.

Only the **paths** are kept: the key stays in its file and never enters the database. Each
node reads its own files, every thirty seconds, and reloads its doors when they changed -
a renewal by cert-manager or the service CA is served without a restart and without
anybody saving anything. The pool shows the new end date.

A file certificate is placed like any other. The **Internal** plane only takes one that
carries one of the names the services call it by - read from the runtime, a wildcard counts - and
there it opens the HTTPS door on `:7443`, and from then on the JWT drawer and the scheduler's page name the
`https://` address. The services calling it must trust the authority that signed it - the
cluster usually distributes it already: a ConfigMap annotated
`service.beta.openshift.io/inject-cabundle` on OpenShift, a trust-manager bundle with
cert-manager.

With the Helm chart, the Secret is mounted at `/etc/meerkat/tls/internal`:

```yaml
internalTls:
  # an existing kubernetes.io/tls Secret, from cert-manager for instance
  secretName: meerkat-internal-tls
  # or, on OpenShift, have the service CA issue and renew one
  openshiftServingCert: true
```

::: details A cert-manager Certificate for the internal plane
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: meerkat-internal
spec:
  secretName: meerkat-internal-tls
  dnsNames:
    - meerkat-internal
    - meerkat-internal.shop
    - meerkat-internal.shop.svc
    - meerkat-internal.shop.svc.cluster.local
  issuerRef:
    name: cluster-ca
    kind: ClusterIssuer
```
:::

## Asked of cert-manager

The usual way to give the internal plane its certificate on Kubernetes is the one above: a
Certificate declared with the deployment, its Secret mounted. **Add certificate, From
cert-manager** is the spare one, for a cluster that has cert-manager and nobody to write
that manifest. It is greyed, with the reason, anywhere else: on Swarm or compose, without
cert-manager, or when the chart did not grant it.

With one such issuer, the click asks it at once - deleting the certificate is the way
back; with several, the console picks the **issuer**, among the ClusterIssuers and the namespace's Issuers
the gateway found ready that sign with an authority - `ca`, `vault`, `venafi`. A
`selfSigned` issuer is left out, its certificate being its own authority, a new one at each
renewal; an ACME one too, a public authority seeing none of the cluster's names. Everything else is decided by the gateway: the names are the ones
the runtime resolves to the internal plane, the Certificate is created in its own
namespace, under the one name the chart granted (`<release>-meerkat-internal-issued`), with
a new ECDSA key at each renewal.

The Secret cert-manager writes is **read through the API and watched**, not mounted: no
Deployment to change, no restart. When cert-manager renews - it rewrites the same Secret
before the end - the API server tells every node, which reloads its door. Each node holds
the key in memory; the database keeps the Secret's name and the last certificate, never the
key. The pool shows who signs - `cert-manager: <issuer>` - and when it renews, or cert-manager's own reason when it
refuses. Placed on the Internal door when that door is empty; deleting it from the pool
deletes the Certificate and its Secret.

```yaml
internalTls:
  # lets the gateway create its Certificate, read its Secret, list the issuers
  certManager: true
```

The chart then grants, in the gateway's namespace, the creation of Certificates - Kubernetes
cannot narrow a create to one name - and everything else on that one name: its Certificate,
its Secret. Listing the ClusterIssuers is a cluster-wide read. With
[approver-policy](https://cert-manager.io/docs/policy/approval/approver-policy/), the
requests of the gateway's service account can be limited to its own names.

## Calling upstreams in HTTPS

The certificates above are what the gateway **presents**. When the gateway is
the one calling - a route whose target is `https://...` - it **checks** the
certificate the upstream presents, and that takes the other half: the
authorities it trusts. They have a store of their own, **Trusted authorities**
on the Domains & TLS screen, apart from the pool: public certificates only, never a key.

By default the gateway trusts the public authorities its image carries. An
upstream inside a cluster is usually signed by the cluster's own authority or
a company's, which no image knows: add it, and the routes to that upstream
verify. It is **in addition** to the public authorities, never instead, so a
public upstream keeps verifying.

An authority is added in one dialog: a name, and its certificate - a file
chosen or dropped on the dialog, or the PEM pasted. A PEM may hold several
certificates - a root and an intermediate, or the old and the new authority
while one replaces the other. A private key in it is refused, with the reason:
what the gateway presents goes in the pool.

Only the authority's **certificate** is needed to verify an upstream, never its
private key - that one signs the Services' certificates, and stays with the
cluster's PKI (cert-manager, a company authority). Take the certificate it
publishes.

**What an authority vouches for is not a DNS name.** Verifying an upstream
asks two questions: does its certificate chain to a trusted authority, and
does it carry the name the route calls - the host of the target URL, such as
`orders.shop.svc.cluster.local`. The first is this store's; the second is the
upstream certificate's own. Internal and external names never meet here: the
certificate the gateway presents to browsers plays no part in what it calls,
and one company authority that signs both internal and external certificates
is one entry for all of them. A route calling `https://orders:8443` when the
certificate only names `orders.shop.svc` fails on the name, with the right
authority in place.

## What lives in the database

| Stored | How |
|---|---|
| The certificate itself | in clear - it is what the gateway hands every visitor |
| The private key | **sealed** with the vault's master key. It never leaves the gateway: not in a payload, not in an export, not in a log |
| A certificate from the file system | its two paths, and the last certificate read - never its key |
| A certificate from cert-manager | the name of its Secret, and the last certificate read - never its key |
| The pending signing request | in clear, and downloadable |
| The ACME account key and issued material | sealed, in the ACME cache |
| A trusted authority | in clear - a public certificate |

The console also checks continuously that the material actually answers for the host it was
filed under. A certificate that covers nothing fails at handshake time, where nobody connects
it to this screen.

## In a cluster

> [!WARNING]
> Certificate keys and the ACME account key are sealed with the vault's master key, which is
> a **local file** unless `MEERKAT_VAULT_KEY` is set. A node with a different key cannot open
> them, and says so: *the private key cannot be unsealed - is this the vault key it was written
> with?* Give every node the same key.

Issuance is serialised by an advisory lock. Five nodes restarting together used to spend the
authority's weekly duplicate quota in one second; the node that waits now finds the certificate
the winner just wrote. The lock is taken only at a name's first handshake and as renewal
approaches - putting it in front of every handshake would cost a round trip to the database to
avoid an event that happens twice a year.

A certificate written on one node is reloaded by the others through the change bus.

## Before a certificate expires

The console shows each certificate's countdown, and the
[daily digest](/docs/console/mail-relay) mails it to the administrators: the
certificates expiring within its horizon, then the ones that have expired. An
automatic one that reaches that window is one whose renewal has been failing. A
certificate in reserve whose names a longer-lived certificate carries - what a renewal
leaves behind - is not named.

## What is missing

- **HTTP/3** (SSL-07): it would need a QUIC dependency outside the standard library, a UDP
 listener and `Alt-Svc`.
- **The trusted authorities serve the upstreams only** (SSL-09). The other connections the
 gateway opens - a directory in LDAPS, an OpenID Connect provider, the mail relay, the
 collector, a git location - still trust the public authorities alone; their expiry is not
 in the daily digest yet.
- **A client certificate for an upstream that asks for one** (SSL-10, mTLS).
- **The orders of an ACME authority on the internal plane**: refused, since a public
 authority cannot see the cluster's names.
