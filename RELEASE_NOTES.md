# Release Notes

## NEXT RELEASE

- **Endpoint audit rules travel as a file.** Export them from Endpoint audit,
  upload them onto any route from its OpenTelemetry section.
- **Saving a route in the editor no longer drops its endpoint audit.**
- **Configuration sits at the bottom of the Infra menu**, apart from the daily
  screens.
- **OpenTelemetry and Plug are laid out like Mail relay**, one framed card per
  section.
- **Logs in the console.** A new Logs screen, under Audit, shows the gateway's
  own lines live, with level filters and search. The level is set there, on
  every node, and goes back on its own after 30 minutes. One node at a time:
  the screen says which.
- **The access log names the API token** a machine called with, beside its
  account.
- **Helm chart: a startup probe.** A gateway migrating its database after an
  upgrade gets up to two minutes before liveness counts, instead of being
  restarted halfway.
- **Configurations can live in a git repository** (Enterprise). You name a
  location - a repository, a branch and a directory - and the same repository
  holds one directory per platform, the configuration written there as
  `meerkat.yaml` with its images beside it.
- **Importing from git applies nothing.** The configuration lands beside the one
  running, with the list of what serving it would change and the vault entries
  it expects; the gateway switches when you say so, not when the repository
  moves. Whoever can write to that branch cannot reconfigure a gateway.
- **Exporting to git commits under your own name**, so the repository's history
  answers who changed what. A branch somebody else has moved is refused, with
  what to do about it - there is no force push. The running configuration can go
  too: it is named on the way, because a repository holds named configurations.
- **The access token is a vault reference**, never a token typed into a field,
  and the form says what each forge wants - including the username it insists on
  beside the token, which every forge decided differently and all of them report
  a wrong one as the same authentication failure.
- **The startup line says which product it is**: "Meerkat Enterprise edition",
  where it used to say the word edition three times.
- **A role is its name.** The generated id beside it is gone: it identified
  nothing the name did not - names were already unique - and it made a
  configuration file unreadable (`parent: d040c48d...`) and incomparable
  between two installations that both have ROLE_ADMIN.
- **A role can no longer be renamed**, and that follows from the same thing.
  The name leaves this gateway - services read it out of a token, a customer's
  configuration holds it in a repository - so renaming it here would have
  rewritten our own rules and nothing they hold: an operation that looks
  complete and never is. Changing a name means adding the new role, pointing
  the rules at it, and deleting the old one, through the refusal that names
  every rule still pointing at it.
- **A value the gateway refuses is answered as such.** A rate limit keyed on
  something that does not exist, saved on a route, came back as "internal
  error"; it is a 422 now, with the sentence that says what is allowed.
- **A failing gRPC call counts as a failure.** It answers 200 with its verdict
  in `grpc-status`; the traffic screen, the metrics and the access log now read
  that verdict, so a gRPC route in trouble no longer looks healthy.
- **A temporary password runs out.** The one an administrator issues works for
  72 hours by default (Application, Security, *Temporary password*), then
  signing in with it is refused with a sentence that says whom to ask. Only
  passwords issued from now on are concerned.
- **TOTP secrets are sealed at rest** with the vault's master key, like the
  vault's own secrets; those stored before are sealed at the first start.
- **The master key can rotate.** Restart with the new key in
  `MEERKAT_VAULT_KEY` and the old one in `MEERKAT_VAULT_KEY_PREVIOUS`
  (`vault.previousKey` in the Helm chart): everything is sealed again under the
  new key, then the old one goes.
- **The License screen lists every Enterprise feature**, from the product's own
  contract: what each does, how far it is built, and the screen it lives on -
  or that it has none, like the active/active cluster. It opens from the
  account menu, under the version, which now says EE or CE.
- **The console's sign-in page says the edition** - an EE or CE badge beside
  the wordmark. Your applications' sign-in pages are unchanged.
- **Endpoint audit is locked on the community image**, as an Enterprise
  feature should be: its events only leave through the collector export. The
  screen and the upload are locked, the API refuses new rules, and rules
  carried over from an Enterprise configuration are kept but not applied.
- **Working hours are not enforced on the community image** any more, as an
  Enterprise feature should not be: windows a configuration brought over are
  kept and ignored. On Application, General, their Save button sits in their
  card and locks with it.
- **OpenTelemetry on the community image shows that nothing leaves**, rather
  than the export an imported configuration had switched on.
- **Plug's steps for a developer's machine open in a drawer**: help, not
  configuration.
- **Sessions moved to the rail**, out of Application: it is not configuration.
  Each administrator reads their own perimeter - root every session, an
  application administrator the applications', and now an organisation's
  administrator the sessions open in their organisations.
- **Members has its membership column in single-organisation mode too**:
  join, leave and the admin badge work as in an organisation, and the
  whole-column box has members to act on.
- **A configuration import no longer brings Enterprise settings onto the
  community image.** Working hours, layouts, a hidden mark, the OpenTelemetry
  export and directories are left out, and the plan lists them - the rest of
  an Enterprise configuration still imports.
- **Helm chart: the HTTPS doors are published** beside the plain ones
  (`service.appTlsPort`, `service.adminTlsPort`), with a read-only role
  (`rbac.read`) that lets the gateway see its own Service and pod.
- **The TLS screen links to the port the world reaches** - 19443 rather than
  the container's 9443 - read from Kubernetes, Docker or Swarm, and says when
  an HTTPS door is not published at all.
- **Two gateways on one host name keep their own sessions.** Session cookies
  now end with an identifier generated at install, so an Enterprise and a
  community gateway side by side on `localhost` no longer sign each other out.
  Everyone signs in once after the upgrade.
- **HSTS only on port 443.** A browser keeps the port when HSTS switches it to
  HTTPS, so a promise made on 8443 broke every plain address of the name. Off
  443 the redirect alone does the work, and HTTPS answers tell browsers to
  forget an earlier promise - as they now do when Force HTTPS is switched off.
- **The redirect to HTTPS uses the published port** (8444 for a Service
  publishing 8443 as 8444), not the container's.

---

## 1.0.2

- **The console says which version it runs.** The account menu shows it, and a
  click opens the release notes.
- **A heart per route says whether its target is up.** Green or broken, with the
  reason, and it changes on screen as soon as the target does.

---

## 1.0.1

### Operations

- **Images for arm64 as well as amd64.** Both images, on both registries, now carry
  the two architectures from one build: `docker pull` on an Apple-silicon laptop or an
  ARM node gets a native binary. 1.0.0 was amd64 only.
- **Every release lands on the public repository too**: a tag and a GitHub Release on
  [softwarity/meerkat-ce](https://github.com/softwarity/meerkat-ce/releases), carrying
  these notes.

---

## 1.0.0

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
  The same trail holds the security of the accounts: every sign-in, every refused one with
  its real reason and address, the lock-out, and every password, second factor, passkey or
  token changed by its owner. Hammering an account does not fill it: the attempt that trips
  the throttle is a line, the ones after it are not.
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
- **Measured against the others, in the open.** Every commit is benchmarked by the CI next to
  Kong, APISIX and Traefik in their free editions, and next to Go's bare standard proxy as a
  reference - each on one CPU, in front of the same upstream, under the same load, on two
  machines - and the documentation's Performance page reads the latest run. It says how the
  others are configured, what is like for like (proxying, a checked credential, a rate limit)
  and what is shown as deployed (authentication delegated to an outside service, with the
  network round trip it costs), and why the runners' figures are a floor.
- **More throughput on one core.** Upstream connections are kept rather than dialled again,
  the proxy borrows its copy buffers, and who a caller is - token, account, organisation,
  roles - is read from memory between changes: revoking a token or editing a role still
  takes effect on the next request.

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
  traffic under its own name, announced to whoever browses it. Opened under Infra, Plug - its
  own switch, off until someone turns it on - which records the address developers use and
  hands them the commands for macOS, Linux and Windows: install, generate the key pair,
  plug a service in.
- **Metrics pushed over OTLP**: the counters behind the built-in dashboards, sent to the
  collector your traces go to, which writes them into Prometheus under names like
  `meerkat_requests_total` - with a ready-made Grafana dashboard to draw them. The OpenTelemetry
  page says what is sent - traces, metrics, or both.
- **Traces, with the gateway on them**: its own spans exported in OTLP to the collector you
  already run. **Each route answers for itself**, in its OpenTelemetry section: whether the
  gateway reports what it answers - off, and that route produces nothing at all, which is how
  an installation follows its own applications and leaves out the admin interfaces it proxies
  beside them - and, for an application with pages, whether the journey **starts in the
  browser**. That second answer injects an OpenTelemetry bundle Meerkat serves itself, never a
  CDN, so the trace begins at the click and shows the network, the queueing, the rendering and
  the calls that never reach the gateway at all.

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
- **No quotas** (rate limits exist, monthly envelopes do not), **nothing written into
  `tracestate` or `baggage`** (both cross through untouched, but the gateway does not
  declare itself in them), **no HTTP/3**, **no SAML and no Kerberos**, and the log level
  is chosen at startup rather than turned up while the gateway runs.
- **No operations guide yet**: what exists is this file, the README, and
  [`FEATURES.md`](FEATURES.md).

The full inventory - every feature, how far it is built, what is missing from the partial
ones, and the edition that carries it - is in [`FEATURES.md`](FEATURES.md).
