---
title: Replace an authentication proxy
section: The product
order: 6
summary: Moving an internal application from oauth2-proxy, Pomerium or a forward-auth setup in nginx or Traefik to Meerkat - what maps to what, and the steps.
---

# Replace an authentication proxy

An internal application with no login of its own is usually protected by an
authentication proxy in front of it: oauth2-proxy, Pomerium, or a forward-auth
rule in nginx (`auth_request`) or Traefik (`ForwardAuth`) calling one. The
proxy signs the person in against an identity provider and passes their name to
the application in a header.

Meerkat does the same job, and keeps the parts that usually sit around the
proxy in the same place: the accounts and their roles, the access rule of each
application, the routing, the rate limits and the audit trail.

## What maps to what

| In front of the application today | In Meerkat |
| --- | --- |
| One proxy, or one forward-auth rule, per application | One [route](/docs/concepts/routes) per application, all on one gateway |
| The identity provider the proxy delegates to | The same provider, through [OpenID Connect](/docs/auth/oidc); or local accounts, GitHub, and in Enterprise [LDAP](/docs/auth/ldap) and [SAML](/docs/auth/saml) |
| Allowed e-mail domains, allowed groups | The route's [access rule](/docs/access/route-access): signed in, a role, a list of people |
| The header that carries the user name | **Identity** on the route: each fact under the header name the application already reads |
| A token the application verifies | A [signed JWT](/docs/auth/service-jwt), checked against the gateway's published keys |
| Rules per path | [Per-endpoint rules](/docs/access/endpoint-security), read from the application's OpenAPI description |

The headers Meerkat sets are removed from the incoming request first: a caller
cannot forge them through the gateway.

## The steps

1. **Run Meerkat beside the current proxy.** The
   [quick start](/docs/start/quick-start) has it running in a few minutes, with
   Docker, Compose, Swarm or Helm.
2. **Connect the accounts.** Declare the identity provider the proxy uses
   today, or create local accounts. People sign in on Meerkat's own pages, in
   your colours.
3. **Add a route for the application.** A path or host predicate, and the
   application's internal address as the target. See
   [your first route](/docs/start/first-route).
4. **Say who may pass.** Signed in, or a role, on the route's access rule.
5. **Hand over the identity.** In **Identity**, give each fact the header name
   the application reads now, with the **Headers** mechanism; or send a signed
   JWT. [Connecting an application](/docs/auth/connect-an-application) has
   recipes for Grafana, GeoServer and others.
6. **Move the traffic.** Point the application's public address at Meerkat's
   data plane, and leave the application reachable only through the gateway.
7. **Remove the old proxy.**

> [!WARNING]
> A header is trusted because nobody else can set it. Keep the application off
> any other path - no published port, no ingress of its own - or anyone who
> reaches it directly can send the header themselves.

## What changes for the application

Nothing, when it reads the same headers as before. What changes is around it:
its access rule, its roles and its traffic are on the console with every other
application's, and every change to them is in the
[audit trail](/docs/operations/audit).

See the [editions](/product/editions) for what the free image includes.
