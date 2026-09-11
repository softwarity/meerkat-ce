# Release Notes

<!--
  Only the NEXT RELEASE section is written by hand.

  softwarity/release-flow stamps the version number onto it at release time,
  publishes that body as the GitHub Release, and opens a fresh empty one for
  the next cycle. Everything below a numbered heading is therefore PUBLISHED
  HISTORY: never insert into it, never edit it, never renumber it.

  What goes here is what a user gains, in their words - a derivative of
  FEATURES.md, not a list of commits. FEATURES.md stays the inventory: one line
  per feature, its state read from the code.
-->

## NEXT RELEASE

First public release.

Meerkat is an app-gateway that also holds the identity of the applications behind it.
One Go binary, one image, an embedded database: routes, accounts, roles and the pages
people sign in on come out of the box, and the services behind it receive a caller who
is already established.

### Routing

- **Declarative routes.** Predicates on path, host, header, cookie, method, query,
  client address, weight and time window; filters that add, remove, rename or rewrite
  what travels in the request and in the response. All of it edited in the console, and
  testable there against a composed request without sending any traffic.
- **One target per route**, chosen and not accumulated: proxy an upstream, redirect,
  answer from a template built on the caller's identity, or serve the built-in
  maintenance page.
- **A route carries everything** - there is no service entity to declare beside it.
- **End-to-end WebSocket**, HTTP/2 wherever TLS negotiates it, and **gRPC**: an upstream
  declares its transport in its own address (`h2c://checkout:50051`), and the trailers
  travel - which is the half that counts, since a gRPC call always answers 200 and puts
  its verdict in `grpc-status`. The metrics do not read that status yet, so a gRPC
  route's failure rate reads zero, and gRPC-Web is not translated.
- **What happens when an upstream misbehaves is configured, not suffered**: a deadline
  per route, a circuit breaker that opens on a failure rate and closes itself, and the
  state of every upstream visible in the console rather than deduced from a log.
- **Rate limits** written per route and per consumer - the rule says over what it counts
  (an account, a token, an organisation, an address), and a refusal says when to come
  back. It blocks rather than slows down, and it counts a window rather than a monthly
  envelope.
- **Its OpenAPI contract** is either published by the service or deposited on the route,
  and the gateway then serves it on the route's own prefix, as JSON whichever of the two
  forms it was written in.
- **The gateway knows what runs beside it**: creating a route, it offers the services its
  own runtime reports - Docker and Swarm through the socket, Kubernetes from inside the
  cluster - instead of asking for a hostname somebody has to go and read elsewhere. A
  kubeconfig from outside the cluster is not read, and there is no DNS discovery.

### Identity

- **Local accounts** and sign-in pages served by the gateway itself, in twenty languages
  (right-to-left included), wearing the application's own theme, brand and background,
  under one of five layouts. The catalogue is closed for now: an integrator chooses a
  layout, they do not yet supply their own HTML.
- **The language and the light/dark choice are preferences of the person**, not of the
  browser: both are kept on the account and re-applied at sign-in, on a machine that
  never saw them.
- **Second factor**: TOTP with offline enrolment and scratch codes, trusted browsers, and
  passkeys (WebAuthn) as a credential of their own.
- **External authentication** against OIDC providers, GitHub, and LDAP/Active Directory
  (Enterprise) - authentication only: the roles are Meerkat's, never the provider's.
- **A password policy that acts**: length and composition, history so an old one cannot
  come back, expiry, a forced change at the next sign-in for one account or for all of
  them, and a lockout that slows an attack down without locking the owner out for good.
- **Self-registration** with email confirmation, password policy, and the waiting room
  for an account that belongs to no organisation yet.
- **The account model is yours.** What this installation knows about a person beyond what
  this product invented - an employee number, a cost centre, a contract reference - is
  declared once under Infra, filled per account, and from then on travels like any other
  fact about the caller: a header or a claim for a service, an attribute on the page for
  a front-end. Defining a field and filling it are two different acts by two different
  people, which is what makes the value safe to forward.
- **An account can carry the dates it is valid between**, in days rather than instants:
  "until the 31st" works all of the 31st. Outside its window it is refused at sign-in,
  with the date, and never signed out mid-work by a clock. Once a day the gateway tells
  the administrators which accounts are about to lose access and which just did - and
  says nothing on a day with nothing to say.
- **The established identity travels upstream** as headers or as a signed JWT, under the
  names applications already read (`REMOTE_USER`, `X-Remote-User`), with the roles shaped
  by an expression the route carries.

### Authorization

- **Hierarchical roles** held gateway-wide, and **role groups per organisation**, so what
  someone may do is granted where they belong.
- **Access rules on a route**, taking part in choosing it: a caller a rule turns away
  falls through to the next route that matches, which is what makes one path per
  organisation possible.
- **A refusal on a UI route lands on a page**, never on a line of text: it names the rule
  that turned the caller away - organisation, role, named account - and offers what this
  session can open instead. A service route keeps its 403; nobody reads that in a
  browser.
- **Per-endpoint rules** posed on the route's OpenAPI operations, in a swagger-like
  editor, for an upstream that enforces nothing itself.
- **Administration is split**: the routing plane and the application's identity are two
  capabilities, and organisations have their own administrators.

### Front-ends

- **The gateway dresses the pages it proxies**: the signed-in user's roles and fields
  stamped server-side onto the HTML, a user button injected with its menus (language,
  light/dark, organisation, profile, sign out), per-route CSS and JS, and a live channel
  that tells open pages when their session ends.
- **The light/dark switch speaks the application's own dialect**: the route declares
  which tag carries the scheme and how - an attribute with two values, a bare attribute,
  or a class - because no two front-ends read it the same way.
- **A page carrying somebody's name is never cached.** The moment the gateway stamps an
  identity into a response it marks it non-storable, whatever the application said about
  caching it: a CDN, a company proxy or a shared browser would otherwise hand one
  visitor's page to the next. An anonymous request is untouched and stays as cacheable as
  its author meant.
- **A developer mode**: an API documentation page over the routes' contracts, identities
  simulated to see what a role sees, and a strip naming what a developer's machine is
  serving.

### Operations

- **An admin console on its own port**, with an audit trail that records who changed
  what, field by field - not "object modified" but the value before and the value after.
- **Automatic TLS certificates** (ACME), certificates that belong to a name, and an HTTPS
  door per plane opened and closed without a restart.
- **A portable configuration**: the whole infrastructure as one YAML file, public by
  construction, alongside an encrypted vault it only ever references - secrets that never
  come back out, and plain values that do. A field holding a secret refuses to be saved
  until the secret is put away. Media - the brand's pictures, a deposited OpenAPI spec -
  travel in a package beside it.
- **Restore points** taken before anything that rewrites the configuration.
- **Counters and a dashboard built in**: traffic, failures, latency by route and by
  endpoint, over a window the table itself draws - no exporter to install, nothing to
  stand up beside the gateway.
- **A maintenance switch**, global or per route, that serves a page wearing the
  installation's clothes and lets an administrator through to check.
- **Issue reports** filed by users from the injected button, with their context and an
  optional screenshot, followed in the console.
- **An agent can read the gateway.** Meerkat answers the Model Context Protocol on the admin
  port - one endpoint, no port to open - so an administrator's assistant can list the routes,
  ask where a given request lands, read the audit trail or take the whole configuration in a
  single call. It ships off. A control-plane token carries a perimeter on three axes - how far
  (read only or full), over what (the routing plane, the application's identity, or everything),
  and from where (address ranges, judged on the connecting address) - and it can only ever take
  away, so a token is at most the person who minted it. Whatever it does is recorded under the
  token's own name, beside the account it was minted on. It writes, too - a route it saves is
  live at once - and the net is the one already there: every change on this plane records a
  restore point by itself, so going back is a click.

### The Enterprise edition

Two images out of one commit, told apart by the linker: the image IS the licence, there
is no key to validate and nothing calls home. What the Enterprise one adds:

- **Several gateways serving one installation**, active/active on a shared PostgreSQL: a
  change made on one node is reloaded by the others in milliseconds, sessions and
  counters are shared, and certificate issuance is taken by one node at a time.
- **LDAP and Active Directory** as authentication sources.
- **Roles granted by a rule** rather than one by one, on what an authority reported about
  the person.
- **Business hours** per organisation - days, hours and timezone, checked when somebody
  signs in rather than during a session - and the single/multi-organisation mode.
- **The developer tunnel**: a service on a developer's machine serving the cluster's
  traffic under its own name, announced to whoever browses it.
- **A Prometheus endpoint** beside the built-in dashboards.

### Known limits

This is a **0.x** release of a product still being designed, and the version says so:

- **The database schema moves with the model, and carries no data migrations.** An
  upgrade may need a fresh database, and upgrading a database seeded by an older version
  is not yet proved in CI. Do not put anything here you would be sad to recreate.
- **The first start prints the generated administrator password in the log**, and forces
  a change at the first sign-in. The proper setup page is not written yet.
- **Writes are protected by `SameSite=Lax` alone**: there is no CSRF token and no `Origin`
  check on the control plane.
- **TOTP secrets are stored in clear** in the accounts table, where everything else
  sensitive goes through the encrypted vault. The vault's own master key cannot be
  rotated yet.
- **Sessions are not listed one by one**, and disabling an account does not end the
  sessions it already holds.
- **Backups are exports.** A snapshot and a file copy restore an embedded install by
  hand; there is no restore button.
- **No quotas** (rate limits exist, monthly envelopes do not), **no trace propagation**,
  **no HTTP/3**, **no SAML and no Kerberos**, and the log level is not configurable.
- **No operations guide yet**: what exists is this file, the README, and
  [`FEATURES.md`](FEATURES.md).

The full inventory - every feature, how far it is built, what is missing from the partial
ones, and the edition that carries it - is in [`FEATURES.md`](FEATURES.md).
