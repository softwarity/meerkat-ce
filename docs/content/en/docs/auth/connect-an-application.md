---
title: Connecting an application
section: Authentication
order: 119
summary: Recipes to make an application behind Meerkat know who is using it - Grafana and GeoServer by headers, RabbitMQ and DbGate through Meerkat's OpenID provider - with the settings on both sides and the traps.
---

# Connecting an application

::: lead
Putting an application behind a route protects it. Making it **know** who is
using it - their name, their rights - is the second half, and each application
takes it its own way. These recipes cover a few common ones, one per family:
read the one closest to yours.
:::

Two families cover nearly everything:

- **Headers.** The application trusts an authentication header set by its proxy.
  Nothing to declare beyond the route: in **Identity**, pick the facts and give
  each the header name the application reads; in **Auth forward**, mechanism
  **Headers**. Meerkat removes any copy of those headers a caller sent, so
  through the gateway they cannot be forged.
- **OpenID Connect.** The application signs its users in itself, against a
  provider. Meerkat is that provider (Enterprise edition): declare a client, bind
  the route to it. See [Meerkat as an OpenID provider](/docs/auth/oidc-provider).

The examples below use `https://apps.example.com` for the gateway's public
address, a route per application under its own path, and `meerkat` for the
gateway's name inside the cluster (the OIDC / JWT drawer writes the real one).

> [!WARNING]
> Both families rest on one promise: the application is reachable **only through
> the gateway**. A header is trusted because nobody else can set it; an
> application also reachable directly - a NodePort, an ingress of its own, a
> published port - trusts whoever reaches it that way. Keep its Service internal
> (ClusterIP), put nothing else in front of it, and where the platform allows,
> add a network policy that admits only the gateway.

## Grafana, by headers

**Mechanism**: headers, read by Grafana's *auth proxy*.

**Grafana side** (environment variables; the same keys exist in `grafana.ini`
under `[auth.proxy]` and `[server]`):

```ini
GF_SERVER_ROOT_URL=https://apps.example.com/grafana/
GF_SERVER_SERVE_FROM_SUB_PATH=true
GF_AUTH_PROXY_ENABLED=true
GF_AUTH_PROXY_HEADER_NAME=X-WEBAUTH-USER
GF_AUTH_PROXY_HEADER_PROPERTY=username
GF_AUTH_PROXY_AUTO_SIGN_UP=true
GF_AUTH_PROXY_HEADERS=Email:X-WEBAUTH-EMAIL Name:X-WEBAUTH-NAME Role:X-WEBAUTH-ROLE
GF_AUTH_PROXY_WHITELIST=<the addresses the gateway calls from>
GF_AUTH_SIGNOUT_REDIRECT_URL=https://apps.example.com/logout
```

**Meerkat side**:

1. A route on `/grafana/**` to the Grafana service, path kept as is (Grafana
   serves from the sub-path).
2. **Identity**: `username` as `X-WEBAUTH-USER`, `email` as `X-WEBAUTH-EMAIL`,
   `fullname` as `X-WEBAUTH-NAME`.
3. **Auth forward**: mechanism **Headers**.
4. The role. Grafana wants one of `Viewer`, `Editor`, `Admin`. The plainest way is
   two routes on the same path, the first with an access rule on your Grafana
   administrators' role and an Incoming filter
   [set-request-header](/docs/filters/set-request-header)
   `X-WEBAUTH-ROLE: Admin`, the second for everyone else with `Viewer`. The first
   route that lets a person in serves them.

**Traps**:

- Without `GF_AUTH_PROXY_WHITELIST`, Grafana takes the header from anyone who
  reaches it. Set it to the gateway's addresses, and still keep Grafana off any
  other path.
- `GF_SERVER_ROOT_URL` must be the public address with the route's path, or
  Grafana writes links and redirects that leave the route.
- The role is applied when Grafana syncs the user (every `sync_ttl` minutes, 15 by
  default): a role change at the gateway reaches Grafana within that delay.

## GeoServer, by headers

**Mechanism**: headers, read by GeoServer's *HTTP Header* authentication filter.

**GeoServer side** (web administration, **Security > Authentication**):

1. **Authentication Filters**, **Add new**, **HTTP Header**. Name it (`gateway`),
   and set **Request header attribute** to a name nobody would guess, such as
   `X-Gw-Auth-7f3c9a` - GeoServer's own documentation calls it a shared secret
   between the proxy and GeoServer.
2. **Role source**: either *User group service* (GeoServer's own users and roles,
   matched on the name), or *Request header* with a second header carrying the
   roles, separated by `;`.
3. **Filter Chains**: in each chain the browser and the clients use (`web`,
   `rest`, `gwc`, `default`), add the new filter before `anonymous`, and remove
   `basic` and `form` where people should only arrive through the gateway.
4. **Settings > Global**, **Proxy Base URL**: `https://apps.example.com/geoserver`,
   so the capabilities documents name the public address.

**Meerkat side**:

1. A route on `/geoserver/**` to the GeoServer service.
2. **Identity**: `username` as `X-Gw-Auth-7f3c9a` (the same name). With roles from
   a header, `roles` as the roles header, shaped to GeoServer's form, for instance
   `{{.Roles | .Keep "tag:GEOSERVER" | join ";"}}` for the roles you tagged for it,
   named as GeoServer names them (`ROLE_ADMINISTRATOR`...).
3. **Auth forward**: mechanism **Headers**.

**Traps**:

- **Never leave a path around the gateway.** GeoServer believes the header, and
  its Service, an old ingress or a published port answer anyone who knows the
  name. The obscure name is a second lock, not the first.
- The header name is a secret: keep it out of tickets and screenshots, and
  change it on both sides if it leaks.
- Remove the default `admin` account's password or disable it once the header
  works: the form login is the way around the gateway inside the application.

## RabbitMQ management, through the OpenID provider

**Mechanism**: OIDC (Enterprise). The management UI signs in through Meerkat with
PKCE, then calls the management API with its access token through the same
route.

**Meerkat side**:

1. The **issuer**: the application plane's public name, declared in Domains &
   TLS, Application tab - here `apps.example.com`, so `https://apps.example.com`.
2. A client `rabbitmq-ui`, **without a secret** (the UI is browser code), and
   **Audience** `rabbitmq` (RabbitMQ's `resource_server_id`). A client declares no
   redirect URI: the UI's callback page is under the route's path, which is what
   the provider accepts.
3. A route on `/rabbitmq/**` to the management port (15672), mechanism **OIDC**,
   client `rabbitmq-ui`, claims `preferred_username` from `username` and a `roles`
   claim. Two levels are two routes on the same path and client: the first for
   your RabbitMQ administrators with `roles` fixed to `admin`, the second for the
   others with `roles` fixed to `viewer`.

**RabbitMQ side** (`rabbitmq.conf`):

```ini
auth_backends.1 = rabbit_auth_backend_oauth2
# during the switch only, so the existing accounts still work:
auth_backends.2 = rabbit_auth_backend_internal

auth_oauth2.resource_server_id = rabbitmq
auth_oauth2.issuer = https://apps.example.com
auth_oauth2.preferred_username_claims.1 = preferred_username
auth_oauth2.additional_scopes_key = roles
auth_oauth2.scope_aliases.admin = rabbitmq.tag:administrator rabbitmq.read:*/* rabbitmq.write:*/* rabbitmq.configure:*/*
auth_oauth2.scope_aliases.viewer = rabbitmq.tag:monitoring rabbitmq.read:*/*

management.path_prefix = /rabbitmq
management.oauth_enabled = true
management.oauth_client_id = rabbitmq-ui
management.oauth_scopes = openid profile
# during the switch only: keeps the username and password form
management.oauth_disable_basic_auth = false
```

**Traps**:

- **The callback path.** The UI comes back to its own callback page,
  `/rabbitmq/js/oidc-oauth/login-callback.html`, under the route's path: accepted
  because the route is bound to the client. If Meerkat refuses it, the gateway's log names the
  address the UI sent (`redirect_uri`) beside the places it accepts: an address
  outside `/rabbitmq` means the UI does not know where it is published - check
  `management.path_prefix`.
- **The keys must be fetched over https.** RabbitMQ refuses an http address for
  the keys. It finds them from the issuer, so the broker must reach the public
  address and trust its certificate. To stay inside the cluster, give it the
  internal plane, but **only through its TLS port** (a certificate placed on the
  internal plane): `auth_oauth2.jwks_uri = https://meerkat:7443/meerkat/oidc/jwks`
  and `auth_oauth2.https.cacertfile` = the authority that signed that certificate.
- **No secret.** Declare the client without one: the UI is public code and sends
  none, and a secret sent by a client declared without one is refused.
- **Rights come from `roles`, not from `scope`.** Meerkat's tokens carry only the
  scopes it knows (`openid profile email`); RabbitMQ's permissions arrive through
  `additional_scopes_key` and its aliases. The audience must be the
  `resource_server_id`, or every token is refused.
- Once everyone signs in through Meerkat, remove `auth_backends.2` and the
  `oauth_disable_basic_auth = false` line: until then the old passwords still open
  the broker.
- Sign-out: RabbitMQ's button ends its own session; send people to
  `https://apps.example.com/logout` to end the gateway's.

## DbGate, through the OpenID provider

**Mechanism**: OIDC (Enterprise). DbGate's server runs the sign-in and keeps a
secret.

**Meerkat side**:

1. The **issuer**: the public name `apps.example.com`, so
   `https://apps.example.com`.
2. A client `dbgate`, **with a secret** (copy it when it is shown). DbGate comes
   back under `/dbgate/`, the route's path: nothing to declare.
3. A route on `/dbgate/**` to DbGate, mechanism **OIDC**, client `dbgate`, claim
   `preferred_username` from `username` (and `email`, `name` if you like).

**DbGate side** (environment variables):

```ini
WEB_ROOT=/dbgate
OAUTH_AUTH=https://apps.example.com/meerkat/oidc/authorize
OAUTH_TOKEN=http://meerkat:7070/meerkat/oidc/token
OAUTH_LOGOUT=https://apps.example.com/logout
OAUTH_CLIENT_ID=dbgate
OAUTH_CLIENT_SECRET=mko_...
OAUTH_SCOPE=openid profile
OAUTH_LOGIN_FIELD=preferred_username
LOGIN_PERMISSIONS_alice=*
LOGIN_PERMISSIONS_bob=~*, widgets/database
```

**Traps**:

- `OAUTH_AUTH` is where the **browser** goes: the public address, always.
  `OAUTH_TOKEN` is called by DbGate's **server**: the internal plane is the short
  way (or the public token endpoint, if DbGate reaches it).
- DbGate comes back under `WEB_ROOT`. If Meerkat refuses the sign-in, the
  gateway's log names the address DbGate sent: one outside `/dbgate` means
  `WEB_ROOT` is not the route's path.
- `BASIC_AUTH` cannot be used with OAuth: leave it unset.
- Permissions are per login, from the claim `OAUTH_LOGIN_FIELD` names:
  `LOGIN_PERMISSIONS_<login>` for each person, `PERMISSIONS` for everyone. A
  `preferred_username` claim with a **fixed value** on the route makes everyone one
  shared DbGate account; two routes with two fixed values give two shared
  accounts, one per level.
- Without `OAUTH_LOGIN_FIELD`, DbGate gives every user the same login.

## Read more

- [Meerkat as an OpenID provider](/docs/auth/oidc-provider) - clients, claims,
  addresses, sign-out.
- [The identity JWT](/docs/auth/service-jwt) - the third way: a signed token on
  every request.
- [Route access](/docs/access/route-access) - the rules that decide who gets in.
