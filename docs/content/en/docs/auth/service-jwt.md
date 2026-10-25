---
title: The identity JWT
section: Authentication
order: 117
summary: The signed token a service receives with each request, how it verifies it from the discovery document on the internal plane, and what a key rotation does to it.
---

# The identity JWT

::: lead
A route can tell its service who is calling with a **signed JWT**: the gateway
signs a short token for each request it forwards, and the service checks it
against public keys the gateway publishes. The service needs no secret, no
account table and no call back to anybody.
:::

## What the service receives

The token arrives as `Authorization: Bearer ...`, and any `Authorization` the
client sent is removed first, so a caller cannot forge one. It carries:

| Claim | What it holds |
|---|---|
| `iss` | the issuer: the application plane's public address (`https://<public name>`), `meerkat` while none is declared |
| `sub` | the caller's account id |
| `aud` | the route's **Audience**, or its name when the field is empty: set one when the service checks for a value of its own, or so that renaming the route does not change its tokens. The same rule as an OIDC client's audience: any text up to 255 characters, spaces inside included, no control character |
| `iat`, `exp` | when it was signed, and when it stops being valid: two minutes later by default, set per route |
| your claims | the attributes the route chose to send - username, e-mail, roles, organisation - under the names you gave them |

It is chosen in the route editor, under **Auth forward**: the mechanism
`signed-jwt`, the algorithm (**ES256** recommended, EdDSA or RS256 for a
library that knows nothing else), the token lifetime and its audience, the
attributes and the shape of the roles.
The preview there shows the exact token a service would receive.

### The shape of the roles

The roles attribute carries one expression, read as a pipeline from left to
right. The same expression shapes the roles header, a JWT claim and an OIDC
claim; a result that is a JSON array lands in a claim as an array.

| Verb | What it does |
|---|---|
| `.Keep "tag:BILLING" "ROLE_USER"` | keeps the roles carrying a catalogue tag, or named |
| `.Drop ...` | the opposite: everything except those |
| `.Declared` | the roles the route's application declares in its spec, mapped on Endpoint security: those the caller holds, under the application's names |
| `cutHead "ROLE_"`, `cutTail`, `addHead`, `addTail` | reshape each name |
| `lower`, `upper` | change the case |
| `join ","`, `json` | the result: a separated list, or a JSON array |

Without an expression a route sends every role, comma-separated
(`{{join "," .Roles}}`), or, once it maps the roles its application declares,
`{{.Roles | .Declared | join ","}}` (see
[Per-endpoint security](/docs/access/endpoint-security#the-roles-the-application-declares)).
`.Keep`, `.Drop` and `.Declared` read catalogue names, so they go before any
verb that reshapes them.

The expression is Go's [text/template](https://pkg.go.dev/text/template), so it
can also decide: a selection that keeps nothing is false, and `if`, `else if`,
`else` turn a set of roles into the single value an application wants. Grafana's
organisation role, from three roles tagged `grafana`:

```text
{{if .Roles | .Keep "ROLE_ADMIN" "grafana-admin"}}Admin{{else if .Roles | .Keep "grafana-editor"}}Editor{{else}}Viewer{{end}}
```

More shapes, with the application they were written for, are in
[Fitting an application](/docs/concepts/fit-an-application#4-the-roles-under-the-applications-names).

## How a service verifies it

Everything a service needs is served on the **internal plane**, port
**7070**: the port the services behind the gateway call, never put behind an
ingress. Nothing outside the cluster has a reason to read it, so nothing
outside reaches it. It needs no token: it holds public keys only, which give
no power, and a service has to read them before it holds anything at all.
`meerkat` below is the gateway's name in our deployment files; the OIDC / JWT drawer
writes the real one, asked of the runtime - on Kubernetes the Service's full
name, such as `http://meerkat-internal.shop.svc.cluster.local:7070`, on Swarm
or compose the service's. Once a certificate is placed on the internal plane,
the same is served in HTTPS on **7443**, and the drawer gives the `https://`
address; the services then trust the authority that signed it
([TLS](/docs/operations/tls#a-certificate-from-the-file-system)).

### Recommended: the discovery document

```
http://meerkat:7070/.well-known/openid-configuration
```

```json
{
  "issuer": "meerkat",
  "jwks_uri": "http://meerkat:7070/.well-known/jwks.json",
  "id_token_signing_alg_values_supported": ["ES256", "EdDSA", "RS256"],
  "subject_types_supported": ["public"]
}
```

One address, and the library learns from it both things it has to check: the
**issuer** the tokens carry, and the **keys** they are signed with. Nothing to
copy into the service's configuration by hand, and a key rotation reaches it
on its own. `jwks_uri` is built from the address the caller used, so it names
the gateway the way the service reaches it.

Libraries take it in one of two ways:

- **The document's address**, as above - .NET's `MetadataAddress`, Node's
  `openid-client`. Whatever the issuer, this is the way a service inside the
  cluster should take. Over plain HTTP, say so where the library asks
  (`RequireHttpsMetadata = false` in .NET).
- **The issuer alone** - Spring's `issuer-uri`, go-oidc's provider. They read
  `{issuer}/.well-known/openid-configuration` on the gateway's public address,
  which answers only while the [OpenID provider](/docs/auth/oidc-provider)
  (Enterprise edition) has a client declared. Otherwise, give such a library the
  JWKS below and the issuer to check (Spring: `jwk-set-uri` beside the expected
  issuer) rather than the issuer alone.

### The JWKS, for a library that only takes keys

```
http://meerkat:7070/.well-known/jwks.json
```

The public half of the keys, as a standard JWKS - for a library that knows no
discovery, such as `jose`'s remote key set. The token's `kid` names the key,
and the key carries its algorithm, so the service takes the algorithm from the
key, never from the token's header. It then checks the issuer itself, against
the one the drawer shows.

### A PEM, for a service that cannot fetch anything

The **OIDC / JWT** drawer, beside **Global** on the Routes screen, shows each
algorithm's public key as PEM. That key must be replaced by hand at every
rotation, where the two addresses above follow it on their own.

## The issuer

Every token says who issued it in `iss`, and a service checks it. It is not
typed: it is the application plane's **public name**, declared in **Domains &
TLS**, **Application** tab, as `https://<name>` (a name on this machine,
`localhost`, gives `http://localhost:<data-plane port>`). With several names,
the **OIDC / JWT** drawer chooses one in **Public address**, the first until
then; with none, the tokens say `meerkat` and the drawer links to that tab. The
same issuer is the [OpenID provider](/docs/auth/oidc-provider)'s.

Declare, remove or choose a public name and the next token carries the new
issuer, on every node - so change what the services expect at the same time.

> [!NOTE]
> The discovery document describes how to verify a token, not how to obtain
> one: for this mechanism the gateway signs one short token for each request it
> forwards, and there is nothing to exchange. A library that insists on
> authorization and token endpoints is one to point at the JWKS instead. An
> application that wants to sign its users in itself is a different case: with
> the [OpenID provider](/docs/auth/oidc-provider) (Enterprise edition), the same
> document then also names those endpoints.

## Rotation

**Renew keys**, in the same drawer, makes a new pair for each algorithm and
signs with it straight away. The previous public keys stay in the JWKS for ten
minutes, so the tokens already signed keep verifying until they expire.

The JWKS is served with a cache of five minutes. A library that fetches the
JWKS again when it meets an unknown `kid` - most of them do - follows a
rotation at once. One that only trusts its cache can refuse new tokens for up
to five minutes after a rotation: check how yours behaves before rotating in
production.

## Moving from the application port

Before the internal plane, the JWKS was served on the **application port**
(8080), and services were pointed there. It is not any more: the application
port serves your users, and these keys are none of their business. On that
port the path is now an ordinary one, which a route may claim.

> [!WARNING]
> On upgrade, a service still reading `http://...:8080/.well-known/jwks.json`
> can no longer verify its tokens, and refuses the requests it receives. Point
> it at the internal plane before or with the upgrade - the discovery document,
> `http://<gateway>:7070/.well-known/openid-configuration`, or the JWKS beside
> it; the OIDC / JWT drawer gives both. Nothing else changes: same keys, same tokens.

## Read more

- [Architecture](/docs/concepts/architecture) - the three planes and what
  separates them.
- [Kubernetes cluster](/docs/deploy/kubernetes) - the internal Service, and a
  NetworkPolicy that keeps the console away from the backends.
- [Scheduled calls](/docs/operations/scheduler) and
  [notifications](/docs/operations/notifications) - the other APIs of the
  internal plane.
- [Meerkat as an OpenID provider](/docs/auth/oidc-provider) - when the
  application signs its users in itself, with the same keys.
