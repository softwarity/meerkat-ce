---
title: TLS and certificates
section: Operations
order: 224
summary: The five ways a certificate gets in, the three doors it is placed on, what lives in the database, and what a cluster has to share.
---

# TLS and certificates

A certificate is held once, and **placed** on the console, the application, the
internal plane, or several of them (SSL-08) - added from a plane's own menu, it lands
on that plane. Its names are its own - what the material says it answers
for - so a certificate carrying several names, a wildcard or an IP address serves every one
of them, and material both planes use is one entry, not two copies to keep in step.

There is no "switch HTTPS on": **a certificate placed on a plane is what opens its door**,
and removing it is what closes it. A switch that can be on with nothing behind it is a
switch that lies.

![The Domains & TLS screen](img/console/tls.webp)

## The ways in

None of them is optional, because the places Meerkat runs in do not resemble each other
(SSL-01):

| Way in | When it is the right one |
|---|---|
| **Import** | you already hold the certificate and its key |
| **From the file system** | the runtime mounts it - a Kubernetes Secret, a Swarm secret, a compose volume - and renews it |
| **Ask cert-manager** | Kubernetes only, for a plane's names inside the cluster - the internal plane, or the console and the application behind a proxy that re-encrypts to them: the gateway asks cert-manager itself, when nobody wrote its Certificate |
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

## Switching the plain port off

HTTPS has no switch; the plain port has one (SSL-15). On the first line of each plane's tab -
Console, Application, Internal - `HTTP:<port>` is switched off or on after a confirmation,
and the HTTPS beside it shows as a state: a closed lock when a certificate serves HTTPS on
the plane, an open one otherwise.

**Switched off, the plain port does not stop: it listens on `127.0.0.1` only.** Nothing on the
network reaches it any more, and two things still do:

- **a port-forward**, the way back in when a certificate breaks the HTTPS door:
  `kubectl port-forward service/<release>-meerkat-admin 9090:9090` lands on the pod's
  loopback, where the console still answers in plain HTTP;
- **the health check the image runs on itself**, `meerkat health`, which asks
  `http://127.0.0.1:8080`. The Helm chart's startup, liveness and readiness probes run that
  command inside the container (`exec`) rather than an HTTP request from the kubelet, so a
  plane whose plain port is off stays ready (see [Kubernetes cluster](/docs/deploy/kubernetes)).
  Anything else that probes the plain port from outside - a cloud load balancer's health
  check - loses it: point it at the HTTPS port, which answers `/readyz` as well.

Two rules keep a plane reachable, on every plane. **HTTP goes off only while a certificate
serves HTTPS there** - otherwise HTTPS would be the only way in, and closed. **The last
certificate stays while HTTP is off** - removing it would leave nothing to answer.

**On the console, the door in use is not switched off.** Each of its two ports shows who called
it in the last 30 seconds - *Receiving, from 10.42.1.178*, the TCP peer, a proxy in front or the
browser itself, or *Nothing in the last 30 s* - live, and pooled across the replicas through the
change bus; health checks are not counted. HTTP is not switched off while the request itself or
anything else in that window comes through HTTP, and the console's last certificate is not
removed while something comes through HTTPS: the screen asks to reach the console through the
other port first, and the gateway refuses the same (`409`). To move a console behind a proxy to
HTTPS end to end:

1. place the certificate on the console: its HTTPS opens;
2. point the proxy at the console's HTTPS port;
3. open the console through it;
4. check that HTTP shows *Nothing in the last 30 s*;
5. switch HTTP off.

The application and internal planes have no traffic display and no such rule.

The switch is `PUT /api/settings/tls/ports/{console|app|internal}` with `{"http": false}` or
`{"http": true}`, audited as `tls.ports`. `GET /api/settings/tls` answers three more fields:
`closed`, per plane (`["http"]` when its plain port is off); `consoleTraffic`, which of the
console's doors was called and from where; and `trafficWindow`, that window in seconds (30).

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
4. **Import** it - Add certificate on the Console tab, Import a PEM pair, the two files
   dropped on the dialog - then the same on the Application tab.

![The drawer with the steps for HTTPS on a local machine: the names, the hosts file, a local authority, one certificate, the import](img/console/tls-local.webp)

> [!WARNING]
> The authority's private key stays in the folder `mkcert -CAROOT` prints. Whoever holds it can
> sign for any name this machine believes: never share it - another machine makes its own.

## Which certificate answers a handshake

By the name the client asked for, among the certificates placed on that plane and used in
its mode (see [Behind a proxy that terminates TLS](#behind-a-proxy-that-terminates-tls)): the one that
carries it, and when two do, the one valid longest. A handshake that names no host - a client
reaching the gateway by address, or anything older than the SNI extension - gets the
**fallback**, the oldest certificate placed on the plane; and with nothing placed it gets an
error naming what *is* served, which is more useful than a random certificate it will reject
anyway.

Placing a certificate where another one already answers for one of its names is refused
until the caller says **replace** (`PUT /api/certificates/{id}/placement` with
`"replace": true`): the one replaced leaves that plane and is kept, under *Unassigned
certificates* when it is on no other one.

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
controller or a cloud load balancer holds the certificate, and forwards to the
gateway - in plain HTTP, or re-encrypted. Then the gateway cannot tell from its
own connection that the browser used HTTPS, nor which names are really its own -
and the mails that prove a mailbox - a sign-up's confirmation, a new address, a
reset - would carry a code instead of a link, since a link is only mailed to a
name the gateway can vouch for (see
[A link, or a code](/docs/auth/local-accounts#a-link-or-a-code)).

So the Console and Application planes each say who serves their public names,
**HTTPS served by** (SSL-14): **Meerkat**, with the certificates placed on the
plane, or **A proxy in front**. Both declare their **public names**, one full name each
(`shop.example.com`); a name belongs to one plane, and the first name of the
Application is the address of a mailed link or an identity provider's callback
when no request says which. Switching mode keeps the names and the certificates:
a certificate that does not fit the new mode stays placed, marked *Inactive in
this mode*, and serves again when the plane switches back. On the screen the
toggle only proposes; **Apply**, then a confirmation, saves. A change of the
Console plane that would make the operator's own session be seen as plain http -
reached through a proxy that says HTTPS, which the new setting would no longer
believe - is refused (`409`), since it would sign them out.

| | Meerkat | A proxy in front |
|---|---|---|
| Who presents the public names | the gateway, with a certificate placed on the plane - a name added that none covers is refused | the proxy |
| HTTPS known by | the gateway's own TLS door | the name itself: a request for a public name is https, no header needed |
| A certificate placed on the plane | carries the public names (ACME, import, file system) | optional: it encrypts the hop from the proxy, and carries the plane's names inside the runtime |
| Force HTTPS, HSTS | on the names a served certificate carries | never: the proxy's business |
| `X-Forwarded-For`, `X-Forwarded-Proto` believed from | nobody, once names are declared | the proxy's addresses, or the private ranges when none is declared |

**The hop from the proxy.** An Ingress with a backend in HTTPS, or an OpenShift
Route in `reencrypt`, opens TLS to the gateway under the name it reaches it by:
the Service in front of the plane on Kubernetes - the chart's `<release>-meerkat`
for the application, `<release>-meerkat-admin` for the console, in their four
forms - or the one service name every port shares on Swarm or compose. For the
hop to be encrypted, the proxy itself must be pointed at the plane's HTTPS port
and speak TLS to it: an Ingress that still targets the HTTP port leaves the hop in
clear, and the certificate unused. The certificate placed on a plane behind a
proxy carries those names, and the plane lists them as its *Internal names*,
ticked when a served certificate covers them. The gateway serves, on such a plane, the certificates carrying one of these
names; when the runtime cannot say them, every placed certificate except those
covering the public names, which are the proxy's to present. On Kubernetes, the
plane can **Ask cert-manager** for it, as the internal plane does (below).

**With Cilium**, a classic Ingress cannot speak HTTPS to its backend: re-encrypting takes
the Gateway API - a Gateway and an HTTPRoute pointed at the Service's HTTPS port, and a
**BackendTLSPolicy** naming the authority to trust and the host name the Meerkat
certificate carries, such as the Service's name in the cluster.

::: details The console's hop through the Gateway API
```yaml
# The HTTPRoute sends the console's public name to the Service's HTTPS port...
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: meerkat-console
spec:
  parentRefs: [{ name: public }]
  hostnames: ["console.example.com"]
  rules:
    - backendRefs: [{ name: <release>-meerkat-admin, port: 9443 }]
---
# ...and this says how the Gateway checks the certificate it meets there.
apiVersion: gateway.networking.k8s.io/v1
kind: BackendTLSPolicy
metadata:
  name: meerkat-console
spec:
  targetRefs:
    - { group: "", kind: Service, name: <release>-meerkat-admin }
  validation:
    caCertificateRefs:
      - { group: "", kind: ConfigMap, name: cluster-ca }
    hostname: <release>-meerkat-admin.<namespace>.svc
```
:::

**Where the proxy comes from.** *Proxy addresses (optional)*, left empty, means
the proxy runs in the cluster or on a private network: the loopback and the
private ranges are believed (an Ingress controller in the cluster, nginx on the
host). Filled, it names the proxy's CIDR ranges or single addresses, for one that
reaches Meerkat from public addresses - a CDN, Google Cloud's load balancers
`130.211.0.0/22` and `35.191.0.0/16`. Only from there are the forwarded headers
believed on that plane; see [Behind a proxy or a load balancer](/docs/deploy/one-gateway#behind-a-proxy-or-a-load-balancer).
A plane nobody said anything about keeps the rule of before: the private ranges.

A name is a host name only - no scheme, no port, no path - never an IP address,
which nobody can vouch for by name, and never a wildcard: `*.example.com` would
vouch for every name under it, including one left pointing at a service nobody
runs any more, and a reset asked under that name would mail its token there. A
certificate may carry a wildcard; a declared name may not. The planes travel with
a configuration export, with the rest of the TLS settings, and a change reaches
every node through the change bus (`PUT /api/settings/tls/doors/{console|app}`
with `{"proxied": true, "names": [...], "proxyAddresses": [...]}`, audited as
`tls.door`). `GET /api/settings/tls` answers them under `doors` - `console`, `app`
and `internal`, each with what was said, its `runtime` names, and the declared
names no certificate covers (`uncovered`).

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

The names are not a field of the account: they are **orders** among the certificates
(*Ask* followed by the authority's name), each placed on the console, the application
or both, like any certificate. A door asks an authority only for the names of the orders
placed on it, and the list is closed: an open policy would let anyone reach the gateway
by address with a made-up SNI and burn the authority's rate limit on names nobody owns.
An authority with no order placed is not armed at all.

**Placing an order sends the request at once**, in the background, through the same
path a handshake would take - the cluster lock included - rather than waiting for the
first visitor to meet the whole exchange or its failure. The node that asked keeps
what happened (requesting, or the authority's refusal, explained) for the screen to
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
anybody saving anything. The screen shows the new end date.

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
Certificate declared with the deployment, its Secret mounted. **Add certificate, Ask
cert-manager** is the spare one, for a cluster that has cert-manager and nobody to write
that manifest. It is greyed, with the reason, anywhere else: on Swarm or compose, without
cert-manager, or when the chart did not grant it.

The Console and Application planes offer it too, **behind a proxy in front** that
re-encrypts to them: the certificate then carries the plane's names inside the cluster, for
the hop from the proxy. A plane the gateway serves itself does not: its public names are a
public authority's business (ACME), an import's or the file system's.

With one such issuer, the click asks it at once - deleting the certificate is the way
back; with several, the console picks the **issuer**, among the ClusterIssuers and the namespace's Issuers
the gateway found ready that sign with an authority - `ca`, `vault`, `venafi`. A
`selfSigned` issuer is left out, its certificate being its own authority, a new one at each
renewal; an ACME one too, a public authority seeing none of the cluster's names. Everything else is decided by the gateway: the names are the ones
the runtime resolves to the plane, the Certificate is created in its own
namespace, under the one name the chart granted for that plane -
`<release>-meerkat-internal-issued`, `<release>-meerkat-console-issued`,
`<release>-meerkat-app-issued` - with a new ECDSA key at each renewal. The API takes the
plane as `?plane=` on `GET /api/certificates/cert-manager` and as `"plane"` in the body of
the `POST`, the internal plane when none is given.

The Secret cert-manager writes is **read through the API and watched**, not mounted: no
Deployment to change, no restart. When cert-manager renews - it rewrites the same Secret
before the end - the API server tells every node, which reloads its door. Each node holds
the key in memory; the database keeps the Secret's name and the last certificate, never the
key. The plane shows who signs - `cert-manager: <issuer>` - and when it renews, or cert-manager's own reason when it
refuses. Asked from the Internal plane, it is placed there when that plane is empty; asked
from the Console or the Application, it is placed on that plane. Deleting it deletes the
Certificate and its Secret.

```yaml
internalTls:
  # lets the gateway create its Certificates, read their Secrets, list the issuers
  certManager: true
```

The chart then grants, in the gateway's namespace, the creation of Certificates - Kubernetes
cannot narrow a create to one name - and everything else on the three names above only:
their Certificates, their Secrets. Listing the ClusterIssuers is a cluster-wide read. With
[approver-policy](https://cert-manager.io/docs/policy/approval/approver-policy/), the
requests of the gateway's service account can be limited to its own names.

## Calling upstreams in HTTPS

The certificates above are what the gateway **presents**. When the gateway is
the one calling - a route whose target is `https://...` - it **checks** the
certificate the upstream presents, and that takes the other half: the
authorities it trusts. They have a store of their own, the **Trusted authorities** tab
of the Domains & TLS screen, apart from the certificates the gateway presents: public
certificates only, never a key.

By default the gateway trusts the public authorities its image carries. An
upstream inside a cluster is usually signed by the cluster's own authority or
a company's, which no image knows: add it, and the routes to that upstream
verify. It is **in addition** to the public authorities, never instead, so a
public upstream keeps verifying.

An authority is added in one dialog: a name, and its certificate - a file
chosen or dropped on the dialog, or the PEM pasted. A PEM may hold several
certificates - a root and an intermediate, or the old and the new authority
while one replaces the other. A private key in it is refused, with the reason:
what the gateway presents is added on a plane's tab.

Only the authority's **certificate** is needed to verify an upstream, never its
private key - that one signs the Services' certificates, and stays with the
cluster's PKI (cert-manager, a company authority). Take the certificate it
publishes.

### Found in the cluster

On Kubernetes and OpenShift the gateway finds the authorities the cluster
publishes, whatever they are called: **From the cluster**, on the Trusted
authorities tab. Several found, you pick. Each source, and what it needs:

| Source | Where the gateway finds it | Right in the chart |
|---|---|---|
| OpenShift service CA | `service-ca.crt`, mounted in every pod: the authority of the Services' serving certificates | none |
| cert-manager | the ClusterIssuers, and the Issuers of its namespace, of type `ca`, `vault` or `venafi` | `trustDiscovery.certManager` (on by default) |
| trust-manager | the Bundles whose ConfigMap reaches its namespace | `trustDiscovery.trustManager` (on by default) |
| Kubernetes cluster CA | `ca.crt`, mounted in every pod: it signs the API server, rarely the services | none |

**An issuer's key is never read.** A cert-manager CA issuer keeps its
certificate and its key in one Secret, and Kubernetes grants a Secret whole: a
right to read the certificate there would be a right to read the key that
signs every service. So the gateway asks the issuer for a throwaway
certificate, `<release>-meerkat-trust-probe`, for a name nothing resolves;
cert-manager writes the issuer's certificate beside it (`ca.crt`), the gateway
takes that, and deletes both. The chart grants creating Certificates in the
gateway's namespace - Kubernetes cannot grant a create on one name - and
reading and deleting that one Certificate and its Secret.

**What each one signs here** is shown under it, to help the pick - indicative,
nothing is decided from it. Each route in https the gateway refuses today is
dialled once (one that verifies already, or does not answer, says nothing
about which authority to pick): the certificate its upstream presents is
matched to the authority that signed it,
by that authority's certificate, or, for a cert-manager issuer, through the
names cert-manager records it signed. And the services: every cert-manager
`Certificate` names its issuer and its dnsNames (listed in the gateway's
namespace and in `rbac.watchNamespaces`; a Certificate holds no key), and
OpenShift marks with an annotation each Service it gives a serving
certificate. A service that has not asked yet, or is not deployed, cannot be
seen.

**Routes still in http** are offered in https once an authority is taken, when
their Service answers in TLS on one of its ports, under a name its certificate
carries, with a chain the gateway now accepts. The port and the name are tried,
not guessed: a service's https port is rarely its http one. Each is the
operator's to switch.

The certificate taken is **copied**, as an import would: an authority renewed
at its source is taken again. On Swarm, compose or a bare binary nothing
publishes an authority, and the button is not offered.

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
