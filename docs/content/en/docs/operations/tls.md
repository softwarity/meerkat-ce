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

`HSTS` exists as a per-route response header (SEC-03); there is no global switch for it
yet.

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

## What is missing

- **The expiry notification** (SSL-04). The console shows the countdown; nothing writes to
  you about it.
- **A global HSTS setting** (SSL-06): the header exists per route only.
- **HTTP/3** (SSL-07): it would need a QUIC dependency outside the standard library, a UDP
  listener and `Alt-Svc`.
