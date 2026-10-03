---
title: TLS and certificates
section: Operations
order: 224
summary: The four ways a certificate gets in, what lives in the database, and what a cluster has to share.
---

# TLS and certificates

A certificate goes into a **pool** once, and is **placed** on the console, the
application, or both (SSL-08). Its names are its own - what the material says it answers
for - so a certificate carrying several names, a wildcard or an IP address serves every one
of them, and material both planes use is one entry, not two copies to keep in step.

There is no "switch HTTPS on": **a certificate placed on a plane is what opens its door**,
and taking it off is what closes it. A switch that can be on with nothing behind it is a
switch that lies.

![The TLS screen](img/console/tls.webp)

## The four doors

None of them is optional, because the places Meerkat runs in do not resemble each other
(SSL-01):

| Door | When it is the right one |
|---|---|
| **Import** | you already hold the certificate and its key |
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

A port whose protocol cannot be named is a port nobody can write a firewall rule or a
runbook against, which is why there are four and not two.

**Those are the ports inside.** Between them and a browser there is usually a
mapping: a Kubernetes Service publishing `9443` as `19443`, a Docker `-p`, a Swarm
ingress. The gateway asks its runtime - its own pod and the Services that select it,
or its container through the Docker socket - and the links on the TLS screen carry
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
makes every browser complain. The **On a local machine** button of the TLS screen opens the
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
`http://meerkat-meerkat.ns.svc:8080` - a JWKS fetch, an internal API call - would be sent to
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

## ACME is not Let's Encrypt

ACME is part of the **Enterprise** edition. A configuration imported on the
community image leaves its authorities and orders out, and its plan names them. A
gateway moved from the Enterprise image to the community one keeps what its
database holds but asks no authority, and the TLS screen says so: those
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
| Root CA | the authority signing the ACME **server's own** HTTPS certificate, in PEM - a private ACME server usually is behind a private certificate |
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

## What lives in the database

| Stored | How |
|---|---|
| The certificate itself | in clear - it is what the gateway hands every visitor |
| The private key | **sealed** with the vault's master key. It never leaves the gateway: not in a payload, not in an export, not in a log |
| The pending signing request | in clear, and downloadable |
| The ACME account key and issued material | sealed, in the ACME cache |

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
