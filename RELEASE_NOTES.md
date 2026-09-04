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
- **End-to-end WebSocket**, and HTTP/2 wherever TLS negotiates it.
- **A route carries everything** - there is no service entity to declare beside it.
- **Its OpenAPI contract** is either published by the service or deposited on the route,
  and the gateway then serves it on the route's own prefix, as JSON whichever of the two
  forms it was written in.

### Identity

- **Local accounts** and sign-in pages served by the gateway itself, in twenty languages
  (right-to-left included), wearing the application's own theme, brand and background.
- **The language and the light/dark choice are preferences of the person**, not of the
  browser: both are kept on the account and re-applied at sign-in, on a machine that
  never saw them.
- **Second factor**: TOTP with offline enrolment and scratch codes, trusted browsers, and
  passkeys (WebAuthn) as a credential of their own.
- **External authentication** against OIDC providers, LDAP and Active Directory, and
  GitHub - authentication only: the roles are Meerkat's, never the provider's.
- **Self-registration** with email confirmation, password policy, and the waiting room
  for an account that belongs to no organisation yet.
- **The established identity travels upstream** as headers or as a signed JWT, under the
  names applications already read (`REMOTE_USER`, `X-Remote-User`), with the roles shaped
  by an expression the route carries.

### Authorization

- **Hierarchical roles** held gateway-wide, and **role groups per organisation**, so what
  someone may do is granted where they belong.
- **Access rules on a route**, taking part in choosing it: a caller a rule turns away
  falls through to the next route that matches, which is what makes one path per
  organisation possible.
- **Per-endpoint rules** posed on the route's OpenAPI operations, in a swagger-like
  editor, for an upstream that enforces nothing itself.
- **Administration is split**: the routing plane and the application's identity are two
  capabilities, and organisations have their own administrators.

### Front-ends

- **The gateway dresses the pages it proxies**: the signed-in user's roles and fields
  stamped server-side onto the HTML, a user button injected with its menus (language,
  light/dark, organisation, profile, sign out), per-route CSS, and a live channel that
  tells open pages when their session ends.
- **A developer mode**: an API documentation page over the routes' contracts, identities
  simulated to see what a role sees, and a strip naming what a developer's machine is
  serving.

### Operations

- **An admin console on its own port**, with an audit trail that records who changed
  what, field by field - not "object modified" but the value before and the value after.
- **Automatic TLS certificates** (ACME), certificates that belong to a name, and an HTTPS
  door per plane opened and closed without a restart.
- **A portable configuration**: the whole infrastructure as one YAML file, public by
  construction, alongside an encrypted vault it only ever references. Media - the brand's
  pictures, a deposited OpenAPI spec - travel in a package beside it.
- **Restore points** taken before anything that rewrites the configuration.
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
- **Connecting one copies no secret.** Give your assistant the gateway's address and a browser
  opens: you sign in, you choose what it may do, and it comes back connected. Nothing is written
  into a configuration file, which is a thing people commit. The console's MCP section carries the
  command for the common agents, lists what is connected, and disconnects one in a click.
- **Two editions from one commit**, told apart by the linker: the image IS the licence,
  there is no key to validate and nothing calls home.

### Known limits

This is a **0.x** release of a product still being designed, and the version says so:

- **The database schema moves with the model, and carries no data migrations.** An
  upgrade may need a fresh database. Do not put anything here you would be sad to
  recreate.
- **One instance.** PostgreSQL is spoken and the schema is portable, but nothing
  coordinates two nodes yet: session caches, brute-force counters and ACME issuance are
  per-process.
- **No rate limiting and no quotas**, no per-route timeouts, no circuit breaker or retry.
- **No observability surface**: no `/metrics`, no trace propagation, no traffic dashboard.
- **Backups are exports.** A snapshot and a file copy restore an embedded install by
  hand; there is no restore button.
- **No HTTP/3, no gRPC** through the gateway.
- **TOTP secrets are stored in clear** in the accounts table, where everything else
  sensitive goes through the encrypted vault.

The full inventory - every feature, how far it is built, what is missing from the partial
ones, and the edition that carries it - is in [`FEATURES.md`](FEATURES.md).
