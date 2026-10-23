---
title: Fitting an application
section: Concepts
order: 24
summary: An existing application behind Meerkat - mounted under a path, trusting the gateway's identity, given its own roles, called over TLS - and the symptoms that say which of those went wrong.
---

# Fitting an application

::: lead
Nobody rewrites Grafana, RabbitMQ or a ten-year-old internal tool to sit behind a
gateway. Meerkat is configured to fit the application, not the other way round:
this page goes through the five places where an application and its gateway have
to agree, with the cases met in real installations, then lists the symptoms and
what each one means.
:::

The five places, in the order a request meets them:

1. **The path** it is published under, and whether the application knows it.
2. **The address** the gateway calls it at, and the certificate it answers with.
3. **The identity** it trusts, and its own sign-in page.
4. **The roles**, under the application's names rather than the gateway's.
5. **The page**: its colour scheme, its language, what it keeps in the browser.

## 1. The path

A route publishes the application under a path, `/grafana/**`, and the
application usually believes it is served from `/`. Three situations, from the
easiest:

**It writes relative links.** Most single-page applications are built with
`<base href="/">` and every asset relative to it. Add
[strip-prefix](/docs/filters/strip-prefix): the application receives `/`, and on
the way back the gateway rewrites the page's `<base href>` to the route's path,
quoted or not (a minified page writes `<base href=/>`). Hubble UI is this case:
`strip-prefix`, nothing else.

**It writes absolute links but can be told its prefix.** Grafana
(`root_url` plus `serve_from_sub_path`), SeaweedFS's admin (`-urlPrefix`), and
most frameworks (`X-Forwarded-Prefix`, which `strip-prefix` announces) can serve
from a sub-path. Then the route **keeps** the path - no `strip-prefix` - and the
application answers under it. Watch what else the option moves: SeaweedFS's
`-urlPrefix` also moves its `/health`, and probes written for `/health` then fail.

**It writes absolute links and knows nothing else.** `/static/app.css` asked from
a page under `/tool/` leaves the route and lands wherever `/static/**` goes - the
catch-all, most often. No rewriting of the page will catch every link a script
builds. Give it its own host instead: a route with a
[host](/docs/predicates/host) predicate on a name of its own, and the path left
alone.

> [!TIP]
> Routes are tried in ascending order, and **ties are broken by name**. Two
> routes at the same order whose paths overlap - `/geoserver/ows` and
> `/geoserver/**` - work only while their names happen to sort the right way.
> Give the wider one a higher order: renaming the narrow one must not change who
> answers. See [The order](/docs/concepts/routes#the-order-and-why-it-is-the-whole-story).

## 2. The address and the certificate

Point the upstream at the service's **full name** inside the platform:
`https://reference-data.canopy.svc:443` on Kubernetes, `stack_service` on Swarm.
The upstream picker offers the short name to read and stores the full one, and the
editor flags an `https` route stored with a bare short name, with a button that
writes the full one.

The full name matters as soon as the cluster's authority is **name-constrained**:
an authority allowed to certify `svc` and `cluster.local` names cannot certify
`reference-data`, and the call fails with `DNS name "reference-data" is not
permitted`. Two consequences:

- the route must call the name the certificate is allowed to carry;
- the service's certificate must not list a bare name either. Verification
  checks **every** name in the certificate against the constraints, whatever
  name was called: one bare name in the list fails the whole certificate.

When the cluster's authority is replaced, the gateway does not trust the new one
until it is told: the route answers 502 with `certificate signed by unknown
authority`. Add it on **Infra > Domains & TLS > Trusted authorities**
([Found in the cluster](/docs/operations/tls#found-in-the-cluster)). A service
that keeps serving the certificate it loaded at start needs a restart to pick up
the one reissued by the new authority.

## 3. The identity, and the application's own sign-in

The route says who may pass. Telling the **application** who passed is the
route's Identity section, in the form the application reads - headers, a signed
JWT, or OpenID Connect with Meerkat as the provider. The recipes per application
are in [Connecting an application](/docs/auth/connect-an-application).

Some applications cannot be told at all: they have their own users and their own
form, and no way to trust a proxy. Two honest choices:

- **Keep their sign-in.** The person signs in twice; nothing else changes.
- **Switch their sign-in off**, and let the route's access rule be the only door.
  SeaweedFS's admin runs without `-adminPassword` this way. Only with a network
  policy that admits the gateway alone: without one, any pod of the cluster
  administers the application without a password.

> [!WARNING]
> An application that trusts the gateway - a header, an open admin - must be
> reachable **only** through it. No NodePort, no ingress of its own, no published
> port: whoever reaches it another way is trusted the same.

## 4. The roles, under the application's names

The gateway's roles are the catalogue's: `ROLE_ADMIN`, `satellite-operator`,
`grafana-viewer`. The application has its own vocabulary. Three mechanisms
translate, each at its own place:

| Where | What it translates | Read more |
|---|---|---|
| An authority (LDAP, OIDC, SAML) | the groups a person arrives with, into catalogue roles | [Authentication](/docs/auth/overview) |
| A route's OpenAPI spec | the roles the application declares, onto catalogue roles (`roleMap`) | [Per-endpoint security](/docs/access/endpoint-security#the-roles-the-application-declares) |
| A route's Identity, attribute `roles` | what is sent: filtered, renamed, joined, or reduced to one value | [The shape of the roles](/docs/auth/service-jwt#the-shape-of-the-roles) |

The third is an expression, read as a pipeline: `.Keep`, `.Drop` and
`.Declared` select, `cutHead`, `addHead`, `lower`... reshape, `join` and `json`
render. It is Go's text/template, so it can also **decide**: `if`, `else if` and
`else` turn a set of roles into the one value an application wants.

Grafana wants one of `Admin`, `Editor`, `Viewer`. With three catalogue roles
tagged `grafana`, one route serves everybody:

```text
{{if .Roles | .Keep "ROLE_ADMIN" "grafana-admin"}}Admin{{else if .Roles | .Keep "grafana-editor"}}Editor{{else}}Viewer{{end}}
```

sent as `X-WEBAUTH-ROLE`, with the route's access rule on `ROLE_ADMIN`,
`grafana-admin`, `grafana-editor` and `grafana-viewer`. Grafana applies the role it
receives on every request: a role changed in Meerkat is the role in Grafana at
the next page.

RabbitMQ is the other common shape: a claim with a fixed value, `["admin"]`, on a
route reserved to the administrators.

## 5. The page

A UI route injects the user button, the portal and the colour scheme into pages
the application writes. Where the application keeps its own state, the two can
disagree:

- **Colour scheme and language.** Declare how the application reads them, and
  hide its own switch with the route's CSS, so there is one switch, the
  gateway's. See [Colour scheme](/docs/customise/color-scheme).
- **Sessions kept in the browser.** RabbitMQ writes a session deadline in
  `localStorage` at the first sign-in and never moves it; past it, the page shows
  its login button for good, though the gateway's session is valid. A one-line
  script injected at the head of the page removes the key
  ([Injections](/docs/customise/injections)).
- **Its own sign-out.** It ends the application's session only, and the next page
  signs the person back in from the gateway. Hide it, and leave signing out to the
  user button.

## Symptoms

| Symptom | Likely cause | Fix |
|---|---|---|
| The page arrives, blank; its scripts answer 404 or another application's page | absolute links, or a `<base href>` the gateway did not rewrite | `strip-prefix` for relative links; the application's own prefix option; or a host of its own |
| A 404 on the bare path, the page works with a trailing slash | the mount path without its slash | nothing: a UI route redirects `/tool` to `/tool/` |
| The application redirects to its internal address | it builds its redirects from its own name | [rewrite-location](/docs/filters/rewrite-location), or tell it its public address |
| 502, `certificate signed by unknown authority` | the gateway does not trust the authority that signs the service | add it to the trusted authorities; restart a service still serving the old certificate |
| 502, `DNS name "x" is not permitted` | a name-constrained authority, and a bare name called or listed in the certificate | full service name in the upstream; no bare name in the certificate |
| The application shows its own login page | it does not read the identity the route sends, or it reads another name | check the header or claim names on both sides; see the recipes |
| Everybody gets the same rights in the application | the roles are not sent, or not in its vocabulary | a roles expression on the route's Identity |
| A path answered by the wrong route | two routes at the same order, overlapping, decided by name | different orders |
| Signed in at the gateway, signed out in the page after a while | state the application keeps in the browser | clear it with an injected script, or align its lifetime |
</content>
</invoke>
