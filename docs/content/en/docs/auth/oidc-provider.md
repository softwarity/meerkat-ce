---
title: Meerkat as an OpenID provider
section: Authentication
order: 118
summary: Let the applications behind your routes sign their users in through Meerkat, as with any OpenID Connect provider - the client, the route, the claims, two levels of access, the addresses and the sign-out.
---

# Meerkat as an OpenID provider

::: lead
Some applications know only one way to learn who is using them: send the person
to an OpenID Connect provider and read the tokens it hands back. Meerkat can be
that provider for the applications behind its routes. The person is already
signed in at the gateway, so they go through without seeing a page, and the
application receives standard tokens saying who they are. Enterprise edition.
:::

## When to use it

A route has three other ways to tell its service who is calling, and they are
simpler: nothing to set up in the application beyond trusting the gateway.

| Mechanism | What the service gets | Choose it when |
|---|---|---|
| **Headers** | one header per fact, on every request | the application has a "trust the proxy" mode (an authentication header) |
| **Signed JWT** | a short signed token on every request | the service verifies a bearer token against a JWKS ([The identity JWT](/docs/auth/service-jwt)) |
| **OIDC** | nothing on the request: the application signs in itself, through Meerkat | the application only accepts an OpenID Connect provider, or its browser code calls its API with an access token |

Recipes for common applications, one per mechanism, are in
[Connecting an application](/docs/auth/connect-an-application).

## How a sign-in goes

1. The person opens the application through its route. The gateway signs them in
   as it always does: the application is not involved.
2. The application sends the browser to Meerkat's **authorize** endpoint with its
   client id.
3. Meerkat takes the routes bound to that client, in routing order, and the first
   whose access rule lets the person in decides. It sends a code back to the
   application, at the address it asked for, provided its routes serve it. No
   page, no consent screen.
4. The application exchanges the code for an **ID token**, an **access token** and
   a **refresh token**: from its server, with its secret, or from its browser code
   with PKCE.
5. At every **refresh**, the route is chosen again, with who the person is now. A
   role removed, an organisation left, a route turned off: each applies at the next
   refresh, not at the next sign-out.

There is no consent screen because the clients are declared by an administrator:
whoever declared the application already decided it may know who signs in to it.

## Before anything: a public address

An application compares the issuer of its tokens with the one its provider's
discovery gave it, so the two must be one address, the one its users reach. The
issuer is not typed: it is the application plane's **public name**, declared in
**Domains & TLS**, **Application** tab ([Domains and TLS](/docs/operations/tls)),
written `https://<name>` - whoever serves it, the gateway or a proxy in front. A
name on this machine (`localhost`) gives `http://localhost:<data-plane port>`,
for a developer's gateway.

- **One public name**: it is the issuer.
- **Several**: the **OIDC / JWT** drawer offers them in **Public address**; until
  one is chosen, the first. A name removed takes the choice with it, and the
  issuer moves to the first that is left.
- **None**: the tokens say `meerkat`, the provider stays off - every endpoint
  answers with the sentence that says where to declare a name - and the drawer
  links to the Application tab.

> [!WARNING]
> The issuer is also the `iss` of the identity JWT that routes with the **JWT**
> and **Signed JWT** mechanisms send to their services. Declaring, removing or
> choosing a public name changes their next token too: update what those
> services expect at the same time.

## Declare the client

**Infra > Routes**, the **OIDC / JWT** button beside **Global**. Under the keys,
**OIDC clients**, then **New client**:

| Field | What it decides |
|---|---|
| **Client id** | what the application calls its `client_id`: lower-case letters, digits, `.`, `_` and `-`, up to 63 characters. Fixed once the client exists |
| **Client secret** | where the secret comes from, one choice of three (below) |
| **Audience** | the `aud` of the access tokens: the name the application expects to find there. Empty, the client id. The ID token's `aud` is always the client id |
| **Token lifetime** | of the ID and access tokens: 5 minutes by default, from 1 minute to 24 hours. A refresh renews them |

A client declares no return address: the routes bound to it say where the
application lives ([Where the code may go](#where-the-code-may-go)).

![A new client, its secret drawn by Meerkat in the form](img/console/oidc-client.webp)

### The client secret

One field under the three choices, whatever the choice:

| Choice | The field | When |
|---|---|---|
| **No secret** | off | code running in a browser or on a phone, which cannot hide a secret: it proves itself with **PKCE** (S256), which becomes mandatory |
| **Drawn by Meerkat** | a secret starting with `mko_`, drawn in the form, with a copy button and a button to draw another | the application takes the secret it is given: copy it into its configuration before saving |
| **Given by the application** | empty, to paste into | the application imposes its own: its chart generates it, or its vendor fixed it. 16 to 256 characters, no spaces |

Whichever it is, the gateway keeps only its sha256 and never shows it again: the
list shows its first characters, `mko_...` for a drawn one, *application's secret*
and four characters for the other. The minimum length is there because the sha256
travels in an exported configuration, and a short secret is found back from it.

Editing a client shows the start of its secret and keeps it as it is. Draw another
or paste another and the hint says it: **the current secret stops working when you
save**, and the application signs nobody in until it carries the new one. Cancel
leaves it working.

![The clients under the keys: a secret drawn by Meerkat, the application's own, and none](img/console/oidc-clients.webp)

Each row of the list names the routes that use the client; a click opens one on
its **Auth forward** section. A warning mark says when one of them covers a whole
host. The key icon opens the client on a **new secret** drawn by Meerkat, in force
once saved. A client a route still uses cannot be deleted, and the
refusal names the routes.

The clients travel in a [configuration](/docs/console/configuration), the secret as
its sha256 only (`secretHash`): imported on another gateway, a client keeps the
same secret. A confidential client imported without one has **no secret yet**, and
its row says so: the key icon draws the first.

## Bind a route to it

In the route editor, **Auth forward**, mechanism **OIDC (the service signs in
through Meerkat)**. Pick the **Client** - or make one there with **New client** -
and fill **Claims sent**: what the tokens say about the person. Each claim is a
name and a source:

- **a fact** that the route's **Identity** section declares: the username, the
  e-mail, the full name, the organisation, the roles... rendered as the JWT
  mechanism renders it, the roles through the expression Identity gives them;
- **a fixed value**, the same for everyone who signs in through this route.

![A route bound to a client, the claims its tokens carry and what each says for the person editing](img/console/route-editor-oidc.webp)

A new setup starts with the names every OpenID library reads first, each kept
only if Identity declares its fact: `sub` (the account id), `preferred_username`,
`email`, `name`. A fact the person does not have is left out rather than sent
empty.

The provider writes some claims itself, and they are refused by name: `iss`,
`aud`, `exp`, `iat`, `nbf`, `nonce`, `azp`, `auth_time`, `at_hash`, `jti`,
`client_id`, `scope`. `sub` is the exception: it defaults to the account id and
may be set, for instance to a fixed value when an application should see one
shared account.

The route still carries the application's traffic, as any route does. With OIDC
it adds nothing to the requests and leaves `Authorization` untouched: that is
where the application's own browser code sends its access token.

## Where the code may go

With every sign-in the application names the address it wants the person back
at (its `redirect_uri`), and the code goes there only if that address is the
application's own. The application is served by the routes bound to its client,
so those routes say where it lives, and nowhere else is accepted: an address is
accepted when, for **one** enabled route bound to the client,

- its scheme and host are the gateway's **public address** (the issuer's), or a
  name of the route's **host** predicate (https, or the issuer's scheme), and
- its path is under the literal start of one of the route's **path** patterns,
  segment by segment: `/rabbitmq/**` accepts `/rabbitmq/` and everything below it,
  never `/rabbitmqx`. A pattern stops at its first variable: `/api/{tenant}/**`
  accepts what is under `/api`.

The host of one route never goes with the path of another, and a disabled route
accepts nothing. A query string is kept; a fragment, or a `..` segment that would
walk out of the path, is refused. An agent reads these places in
`list_oidc_clients` (`acceptedUnder`), for instance
`https://apps.example.com/rabbitmq/`.

> [!WARNING]
> A route on `/**` covers its **whole host**: any address on it may then receive
> a code for the client, and the list of clients says so. Give such a route a host
> of its own, or a narrower path.

An application served outside the gateway, or under a name no route of its
client gives (a host predicate with a wildcard, `*.example.com`, names no
address), cannot use the provider: put it behind a route.

The application's browser code may call the token and userinfo endpoints from the
same origins: the gateway's public address and the routes' hosts.

## Who may sign in: the route's access rule

The client says nothing about people. Who may sign in to the application is the
**access rule** of the route bound to it, posed in **Security** like any other.
Somebody it turns away gets the gateway's own answer: the organisation chooser
when another organisation would let them in, otherwise the refusal page that
names the rule.

### Two levels of access

Two levels for one application - say administrators and readers - are **two
routes on the same paths with the same client**, each with its own access rule
and its own claims:

| Order | Route | Access rule | Claims sent |
|---|---|---|---|
| 1 | *Monitoring - admins* | role `MONITORING_ADMIN` | `roles` = `admin` |
| 2 | *Monitoring - readers* | signed in | `roles` = `viewer` |

The **first** route that grants the person decides, exactly as routing does for
requests: an administrator matches the first, everyone else falls through to the
second. Since the choice is made again at every refresh, losing the role turns
the next tokens into a reader's.

## The addresses

The application is told one thing: the **issuer**. Everything else is in the
discovery document. The top of the **OIDC / JWT** drawer gives the three
well-known addresses to copy: **Discovery (internal)**, **OIDC discovery
(public)** - shown while the provider answers - and **JWKS**.

| | Public address (data plane) | Internal plane |
|---|---|---|
| Discovery | `https://apps.example.com/.well-known/openid-configuration` | `http://meerkat:7070/.well-known/openid-configuration` |
| Authorize | `https://apps.example.com/meerkat/oidc/authorize` | - (a browser's door, never on the internal plane) |
| Token | `https://apps.example.com/meerkat/oidc/token` | `http://meerkat:7070/meerkat/oidc/token` |
| Userinfo | `https://apps.example.com/meerkat/oidc/userinfo` | `http://meerkat:7070/meerkat/oidc/userinfo` |
| Keys (JWKS) | `https://apps.example.com/meerkat/oidc/jwks` | `http://meerkat:7070/meerkat/oidc/jwks` |
| Sign-out | `https://apps.example.com/logout` | - |

- The **public** discovery document is answered only while at least one client
  is declared. Until then the path belongs to the routes, like any other: an
  application behind a route may serve its own.
- The **internal plane** is for the application's **server** inside the cluster:
  it exchanges its codes and reads the keys there without going out through the
  public name and back. Its discovery document names the public authorize
  endpoint - the person's browser goes there - and the others at the address the
  server used. Once a certificate is placed on the internal plane, the same is
  served in HTTPS on **7443**
  ([TLS](/docs/operations/tls#a-certificate-from-the-file-system)).
  `meerkat` stands for the gateway's name inside the cluster; the OIDC / JWT
  drawer writes the real one.
- A library that starts from the issuer alone reads the **public** discovery: the
  server then has to reach the public address. A library that takes the
  document's address can be given the internal one.

Supported: `response_type=code`, PKCE `S256`, `prompt=none`, the grants
`authorization_code` and `refresh_token`, client authentication
`client_secret_basic`, `client_secret_post` or none, the scopes `openid` (required),
`profile` and `email`, ID tokens signed **RS256** with the gateway's keys. Other
scopes are ignored rather than refused; the claims come from the route, whatever
the scopes. Userinfo answers the access token's claims about the person.

## Signing out

There is one session that counts, the gateway's, and one way to end it: the data
plane's **`/logout`**, which the discovery document gives as
`end_session_endpoint`. Point the application's sign-out at
`https://apps.example.com/logout`.

- From a page of the same site - the application served by the gateway - a
  `GET /logout` signs out at once and lands on the sign-in page.
- From anywhere else, or when a browser fetches it ahead of a click, a page asks
  first, with a **Sign out** button.
- No address to come back to is read from the request: the person always lands
  on the sign-in page.

Ending the gateway session revokes the codes and refresh tokens born in it. An
access token already issued stays valid until it expires, which is why the
default lifetime is short. An application that ends only its own session leaves
the person signed in at the gateway, and its next sign-in goes through without a
page.

## Try a sign-in

The arrow icon on a client's row tries a sign-in for anyone, without them and
without issuing anything. Pick the person, and the organisation their session
would be opened in (their only one by default):

- the route that lets them in - the first bound to the client, in routing order,
  whose rule does - or a refusal when none does;
- the routes whose rule turned them away;
- the claims of the **ID token**, the **access token** and **userinfo**, unsigned.

The claims are built by the code that builds the real tokens, with the person's own
roles and facts and the route's role mapping: what you read is what the application
receives. It answers "why does this person see nothing in the application" before
anyone asks them to sign in again.

![A sign-in tried for one person: the route that lets them in, and the ID token's claims](img/console/oidc-try.webp)

## When it does not work

What the person or the application sees, and the line the gateway logs
(`OIDC sign-in refused`, with the client and the reason, never a secret or a
token):

| Symptom | Cause |
|---|---|
| a page: *asked to sign in as client ..., which this gateway does not know* | the client id is not declared, or misspelt |
| a page: *asked to bring you back to an address it may not use* | the address is under no enabled route bound to the client: bind the application's route to the client. The log line names what came (`redirect_uri`) and what would have been taken (`accepted_under`) |
| `unauthorized_client` | no enabled route is bound to the client with the OIDC mechanism |
| the refusal page, or `access_denied` with `prompt=none` | no route bound to the client lets this person in |
| `temporarily_unavailable` with a sentence about the issuer | the application plane has no public name: declare one in Domains & TLS, Application tab |
| `invalid_client` | a wrong secret, or a secret sent by a client declared without one |
| `invalid_grant` on a refresh | the gateway session ended (sign-out, expiry), or no route grants the person any more |
| the application rejects the token's audience | its expected audience is not the client's **Audience** |

## Community and Enterprise

| | Community image | Enterprise image |
|---|---|---|
| The OIDC / JWT drawer: issuer, keys, internal discovery and JWKS | yes | yes |
| The public OIDC discovery line | locked with the Enterprise mark, no address | while the provider answers |
| OIDC clients, the OIDC mechanism on a route | visible, locked with the Enterprise mark; the API refuses to declare a client | yes |
| The provider's endpoints | not in the binary: the paths fall through to the routes | yes |

The control-plane API is `/api/oidc/clients`, every change audited with the secret
redacted, and an agent has the matching tools (`list_oidc_clients`,
`save_oidc_client`, `regenerate_oidc_client_secret`, `delete_oidc_client`,
`try_oidc_sign_in`), and for the issuer and the keys (`get_signing_keys`,
`set_issuer`, `renew_signing_keys`): an assistant can put an application behind
the gateway end to end and prove it with a trial. Through
the API or a tool, a confidential client given no `secret` has one drawn by the
server and shown once in the answer; given one, the application's, it keeps it and
sends nothing back.

## Read more

- [Connecting an application](/docs/auth/connect-an-application) - recipes, by
  mechanism.
- [Route access](/docs/access/route-access) - the rules that decide who signs in.
- [Sessions](/docs/auth/sessions) - what ends the gateway session.
- [The identity JWT](/docs/auth/service-jwt) - the keys and the issuer shared with
  this page.
