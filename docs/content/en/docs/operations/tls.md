---
title: TLS and certificates
section: Operations
order: 224
summary: The four ways a certificate gets in, what lives in the database, and what a cluster has to share.
---

# TLS and certificates

A certificate belongs to a **name** (SSL-08). The console has one name and one
certificate; the application has one per host it serves. Material used by both is added
twice - two entries and two keys are cheaper to understand than one shared object plus
the rule that works out which name it covers.

There is no "switch HTTPS on": **having a certificate is what opens the door**, and
deleting it is what closes it. A switch that can be on with nothing behind it is a switch
that lies.

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

## Which certificate answers a handshake

By the name the client asked for. A handshake that names no host - a client reaching the
gateway by address, or anything older than the SNI extension - gets the **fallback**, and
without one it gets an error naming what *is* served, which is more useful than a random
certificate it will reject anyway.

Switching TLS on with nothing to present is refused: it would turn a working port into one
that refuses every visitor.

## The plain port can redirect

On the **data plane only**, the plain port can redirect to the HTTPS one (SSL-06). The two
health probes are exempt: a `308` reads as "not ready".

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

> [!WARNING]
> A browser keeps the promise for the whole duration, even if the certificates
> are taken away, and no longer offers to continue past a bad certificate. Start
> at a day; lengthen once HTTPS is settled.

## ACME is not Let's Encrypt

The directory is a **URL**, and that is the whole point: half the installations Meerkat is
meant for have no route to the internet at all. An internal `step-ca`, an EJBCA, a Windows
authority with the ACME role - any of them answers here.

| Field | What it is for |
|---|---|
| Directory URL | the authority. Empty means Let's Encrypt; staging is offered as a starting point |
| Contact e-mail | where the authority warns about expiries. Required by some private ones |
| Hosts | the **closed** list of names that may be requested |
| Root CA | the authority signing the ACME **server's own** HTTPS certificate, in PEM - a private ACME server usually is behind a private certificate |
| EAB key id and HMAC key | External Account Binding: which account this gateway registers under. Public authorities ignore it, most private ones refuse without it |
| Terms accepted | a legal act, so it is never assumed |

The host list is never empty: an open policy lets anyone reach the gateway by address with
a made-up SNI and burn the authority's rate limit on names nobody owns.

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
automatic one that reaches that window is one whose renewal has been failing.

## What is missing

- **HTTP/3** (SSL-07): it would need a QUIC dependency outside the standard library, a UDP
 listener and `Alt-Svc`.
