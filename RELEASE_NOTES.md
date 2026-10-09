# Release Notes

## NEXT RELEASE

---

## 1.4.0

- **The portal's rail works on a phone.** A container's modules no longer run off the
  edge of a narrow screen: they fold into one button naming the module you are on,
  which opens them as a menu - on a phone, or whenever they do not fit.
- **A portal icon can come from an SVG file.** Beside pasting it, the SVG mode takes a
  file dropped on the box or chosen from the disk; a file that is not an SVG is named
  and refused.
- **The portal preview comes back after a mode change.** Going from Portal to Links
  or None and back left the preview's bar empty until the screen was opened again.

---

## 1.3.0

- **A route can serve files.** A new mode under Target, **Files**: upload a font, a
  stylesheet, a script or an image on the route and it answers them under its path -
  for the UI that needs a resource nothing behind the gateway serves, offline above
  all. Each file is served under a name you choose - not the one it had on disk - with
  a type you can correct; ETags and CORS are handled, and the files travel in a
  configuration export.
- **A route's own CSS and JavaScript is an ordered list.** Each block is written in the
  console or uploaded as a file, placed at the start or the end of the head or at the
  end of the body, and a script runs where it stands, deferred, async or as a module.
  A stylesheet now lands at the end of the head by default, after the application's
  own, so its rules win. The two free blocks a route had became its first two
  entries, where they were - no page changes.
- **The console updates live over HTTPS too.** Its live channel never connected on the
  console's HTTPS port: screens waited for a reload to show what had moved.
- **A theme is made the way Material Theme Builder makes one.** Six source colours -
  the primary, and a secondary, tertiary, error, neutral and neutral variant that are
  derived from it until you set them - one of the builder's three contrast levels, and
  its "Color match". Both schemes, every Material 3 role, light and dark, are generated
  from them, and they are the builder's own: a theme exported from the builder imports
  role for role, and one made here exports in the builder's JSON format. Every theme is
  converted on upgrade: one typed token by token becomes the six colours that come
  closest to it, so its primary stays and its surfaces and outlines take the Material 3
  tones.
- **The built-in pages and the mails wear the whole theme, not only its primary.** As
  Material 3 assigns them: the primary for the brand and the action, the secondary for
  what is chosen or browsed (a selected choice, the current language, the portal's
  current module), the tertiary for state and information (connected and current
  badges, a code to copy, a met password rule, an explanation).
- **A theme chooses its fonts**: a display face for the titles, a body face for the
  text and a monospace for codes and labels, among fourteen families the gateway ships
  and serves itself - no font CDN, nothing to fetch offline. Behind any choice, Noto
  draws Arabic, Hebrew, Devanagari and Thai, and Chinese, Japanese and Korean use the
  system's fonts, so all twenty languages render.
- **The theme preview follows as you pick.** Every page of the catalogue repaints as a
  colour moves, not only the specimen, and so do the mails and the portal bar.
- **SAML 2.0 sign-in** (Enterprise). A SAML identity provider - ADFS, Entra ID,
  Okta, Shibboleth - is an authority like an OIDC one: a button on the sign-in
  page, its signed answer verified against the certificate in its metadata, and
  refused for another audience, outside its validity window, unsolicited, or
  posted a second time. The screen hands over the entity ID, the reply URL and
  the gateway's own metadata for the provider's admin to import.
- **What an authority says, before writing a rule.** An authority's editor shows
  who came in through it, with the groups each reported at their last sign-in -
  GitHub, OIDC and SAML included - and asks a directory about anyone, without a
  password: the names a group rule is written against.
- **A group rule opens in a right drawer**, like every other editor, at its own
  address: a refresh or a pasted link comes back to it.
- **A portal module changes place without being recreated.** Its editor moves it
  along its surface - arrows that point the way the bar runs there, across or down -
  and "Move under" puts it in a container or back at the top level, with its label,
  icon and state.
- **plug 2.22.0, installed under the application's name.** The install command on
  the plug page and on a developer's key page creates the profile named after the
  application - shown in the command, so a second gateway with the same branding
  is renamed right there - and the key commands use that same name: a developer
  with several gateways tells them apart by name rather than by host. The install
  now hands the terminal back as soon as it is done, and ends by saying where the
  key goes: the developer's profile on this gateway. A plug installed from the
  gateway no longer looks for agent updates, announces them or asks: its version is
  the gateway's to decide.
- **The routes screen shows each service's replicas, live.** A route's heart is
  green when every replica of its service is ready, orange when only some are, and
  red when none is, with the count under the heart (`2/3`) and the image they run
  under the route's name - two images during a rollout; the UI and traced marks sit
  at the end of that second line, faint when off. It follows the runtime's own
  events (Kubernetes, Docker, every node of a Swarm), so a replica that dies shows
  at once, and nothing is asked on a timer. The Helm chart grants the read it needs
  (`rbac.watch`, on by default), and in the other namespaces the routes point at when
  they are listed (`rbac.watchNamespaces`); the Compose file and the Swarm stack now run a
  read-only proxy of the Docker socket, which also brings the route editor's
  service list to Swarm.
- **An administrator's agent can test the services behind the routes.** Two new
  MCP tools: `list_operations` reads what a route's service offers from its
  OpenAPI spec, and `call_route` makes a real call through the gateway - as an
  identity holding every role by default, or as somebody chosen to see a rule
  refuse - and returns what came back. A refusal says why: the gateway now names
  the role an endpoint asks for even when the caller has no organisation, where it
  used to name only the missing organisation.
- **The portal bar shows the right sub-modules on a sub-module's page.** A
  sub-module whose address is not under its parent's - `/plug` under a module at
  `/docs`, say - showed the first module's sub-modules instead of its own.
- **Portal containers.** A portal entry is now a module - an application - or a
  container: a label, an icon and the modules grouped in it, with no route of its
  own. Clicking a container opens the first of its modules the visitor may open,
  and a visitor who may open none of them does not see it. There is no "home
  label" any more. An entry that was an application holding sub-modules is to be
  redefined as a container.

---

## 1.2.1

- **Who is signed in, live, on Users and Members.** A person icon heads each row,
  coloured while the account has a session open, with its sessions in the tooltip -
  browser, address, since when. On an organisation's Members screen it means signed
  in to that organisation. The green dot is gone from Users: it read as "online",
  and a greyed row already says disabled.
- **The Compose and Swarm files take their image from `MEERKAT_IMAGE`.** The same
  published file runs the community image, the evaluation one or the image built for
  a licence, without being edited; unset, nothing changes.
- **The API reference, the release notes and the licence moved under Meerkat**, at
  the foot of its sections. The release notes are a full page now, titled with the
  running version and edition, and the user menu keeps your profile and sign out.
- **Certificate requests and endpoint traffic arrive live.** The TLS screen no longer
  asks every few seconds while an ACME request is out, and Metrics reads the
  per-endpoint ranking when traffic comes in rather than on a timer.
- **Two rail entries instead of seven.** **Data plane** gathers what the applications
  are doing - their sign-ins, their sessions, the scheduled calls, the traffic, the
  issue reports - and **Meerkat** the gateway itself - the changes and the console's
  sign-ins, its log, who holds the console. Each opens on sections, like Infra, and
  lists only those you may open. The old addresses redirect.
- **The Sessions screen** shows one plane at a time, updates live, and draws its rows
  like every other list.
- **A menu entry granted by a role** keeps its layout: the role mechanism now only
  hides what no role grants, and no longer guesses an element's display to show it.
- **The audited operations are kept in Meerkat.** A call of an operation ticked in
  Endpoint audit used to leave for the collector and nowhere else, so without the
  export it was audited nowhere. It is now kept in the trail and shown under Data
  plane, Audit, Operations - who, the status, the path, the fields, the masked
  body - and still sent to the collector when the export is on. Written off the
  request, in batches, on a lifetime of its own: a month by default, set by root.
- **Move the database, in both directions.** Meerkat, Configuration, Snapshot now
  copies the whole database to the other kind as well: from the embedded one to
  PostgreSQL to scale, or back. Into a file - a `.db`, or a `.sql` dump for `psql` -
  or straight into an empty PostgreSQL server, every table counted on both sides.
  A **Pause** switch puts the applications on the maintenance page and stops every
  write first, and the screen prints what to change in the Helm chart or Compose
  file, the vault key included. `meerkat db dump` and `meerkat db copy` do the same
  from a script.
- **Configuration moved under Meerkat**: it is the whole installation, not the
  routing plane's. The old addresses redirect.
- **The Helm chart keeps the embedded database's volume** when a release is
  pointed at PostgreSQL. It used to stop rendering the claim on that upgrade, and
  Helm deleted it - the volume with it under a Delete reclaim policy - which took
  away the way back from a migration. Delete it yourself once the new database
  has proven itself; before upgrading an existing release, download a snapshot.
- **plug 2.21.9 for the developer tunnel.** On macOS, a plug launcher up to 2.21.0
  wrote the profiles of `~/.plug` as root, and `plug rm`, `plug rn`, `plug config` or
  redefining a profile then failed with "owned by root". A root profile is now
  managed like any other and handed back to its user, and `~/.plug` is created as
  the user's.

---

## 1.2.0

- **An application published under a prefix it does not know about works.** Three
  things sent the browser out of the route, each without an error:
  the page's `<base href="/">`, which made its scripts resolve at the gateway's root -
  the prefix is now put in front of it; a redirect to the application's own address
  or to a path from its root - it now comes back under the route; and the trailing
  slash `strip-prefix` dropped, which made a static host answer every page with a
  redirect to itself. `X-Forwarded-Prefix` was wrong on such paths too.
- **Pages served by a CDN get their portal.** Browsers offer zstd and CDNs answer in
  it; the gateway cannot read it, so nothing was injected into those pages. It no
  longer asks upstreams for an encoding it cannot read back.
- **A third image, to evaluate the Enterprise edition without a time limit.**
  Every feature, no counter, no date, nothing that switches the data plane off:
  a trial that bridles what it is meant to sell stops selling it, and a counter
  kept in a database the evaluator administers resets with one statement. What
  the image carries instead is a notice on each surface somebody reads - a
  banner on the pages the gateway serves, a line in the user button's menu, a
  foot on transactional mail, a watermark across the console, and the Licence
  screen saying what this build is licensed for. That is what makes it
  unusable in front of a production without taking anything away from an
  evaluation. The notice answers to the build alone: no setting removes it, and
  hiding the Meerkat mark - which the Enterprise licence does buy - does not
  touch it. `docker pull softwarity/meerkat:eval`, and `X.Y.Z-eval` for a
  release.
- **The portal and the user button show on pages that carry a Content-Security-Policy.**
  A page that lists the scripts it accepts - an Angular build does, in a `<meta>` -
  had the browser refuse the gateway's own, and came out without its portal, with
  nothing but a line in the browser's console. The gateway now signs the scripts it
  injects with a nonce and tells the page's policy about it; nothing else is opened.
- **A new route to an internal application keeps the caller's Host.** Typing an
  upstream that is a service of the cluster, a container or a private address adds
  `preserve-host` to the route's filters - where it shows and can be removed. It is
  what an application reached directly needs to accept a WebSocket (Grafana's live
  channel refuses one whose origin is not the Host it received) and to write its
  links. A public name gets nothing, and existing routes are not touched.
- **The developer tunnel's volume mount no longer needs root** (plug 2.21.8), which is
  what kept it out of OpenShift and OKD. The helper that serves the volume over SMB
  runs as the workload's own user, without any privilege: the developer reaches
  exactly what the workload does. It runs the Meerkat image itself, with the SMB
  server compiled in: a disconnected cluster mirrors one image, and that image
  carries no Samba.
- **Runs on OpenShift and OKD** under the default `restricted-v2` SCC, and on any
  namespace enforcing Pod Security `restricted`: the image's user is numeric, `/data`
  is writable by the root group, and the chart sets a security context (non-root, no
  capability, no privilege escalation, default seccomp) without pinning a uid.
- **All three images are signed**, with cosign and no key: the signature carries the
  identity of the workflow that built the image, so there is no public key to go
  and fetch. Install has the `cosign verify` command and a Kyverno policy that
  refuses an unsigned image cluster-wide. What is signed is the digest, and
  recursively, so a release keeps the signature of the digest it re-tags.

---

## 1.1.0

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
- **TLS: a pool of certificates, placed on the console and the application.**
  A certificate is generated or imported once - its names are the ones it
  carries, several names, a wildcard or an IP address included - then dragged
  onto the console, the application, or both (or placed from its menu). No
  more names to declare first, and no more material imported twice. A
  certificate already answering for one of the same names on that door is
  named, and replaced only when you say so. On upgrade, every certificate keeps
  the plane it was on, and two copies of the same one become a single entry
  placed on both.
- **Each TLS door shows its ports in its title** (`HTTP:19090 HTTPS:19443`, as
  the world reaches them) and lists only HTTPS links. The console warns when
  none of its certificates carries the name it is reached by.
- **The daily digest no longer names a renewed certificate** left in reserve:
  a longer-lived certificate carries its names.
- **Force HTTPS only redirects the names a certificate carries.** A service
  inside the cluster calling the gateway by its service name - a JWKS fetch,
  an internal API call - stays in plain HTTP instead of being sent to a
  handshake it cannot complete.
- **Taking the console's own certificate away no longer hangs.** Closing an
  HTTPS door from a request that came through it waited on itself; the
  console now asks first, then continues on its plain HTTP address.
- **Importing a configuration file now works on the Community edition**:
  "Import a file" previews the plan and applies it (or keeps it as a saved
  configuration). Enterprise-only parts of the file - ACME included - are left
  out and listed in the plan. Git import and export stay Enterprise.
- **Saving under a name, setting as current, duplicating and restoring a
  configuration are no longer locked on the Community edition** - the gateway
  always allowed them; the console had them greyed. The Community edition keeps
  three saved configurations at a time.
- **A git location starts with its forge**: GitHub, GitLab, Bitbucket Cloud,
  Azure DevOps, Gitea / Forgejo or another server. A known forge fills in its
  host (editable for a self-hosted GitLab or Forgejo) and only the repository
  is typed; the steps to make the token are written for that forge, with a link
  to this repository's own token page, and the username is asked only where
  the forge leaves it open. The forge is kept, so a self-hosted GitLab still
  sends oauth2.
- **Pushing to and pulling from git shows what is happening**, on the row
  itself: a turning icon and *pushing to undrstry...* while it runs, and the
  configuration and location named when a push is done.
- **A saved configuration says where it stands against git in words**:
  *synced* with the date of the last push or pull, *changed here since*, or
  *never pushed*; the repository, branch and directory are on hover.
- **Push and pull are plain**: a pull replaces the saved configuration with the
  repository's version, a push replaces the location's directory with the saved
  configuration - and only that directory, so another platform in the same
  repository is left alone. A push is no longer refused because the branch
  moved; what it replaces stays in the git history.
- **ACME is now Enterprise.** On the community image the TLS screen still
  generates, imports and signs certificates on request; asking an authority
  is part of the Enterprise edition.
- **Several ACME authorities, set up in a drawer** (Enterprise). The ACME
  button, beside Add certificate - its tooltip says what ACME is: pick Let's Encrypt (staging or not), ZeroSSL, Google Trust
  Services or another authority, and the form asks only what that one needs -
  saying where to find it. Each authority saved is an *Ask* entry of Add
  certificate, so one domain can come from Let's Encrypt and another from
  ZeroSSL. The ACME card at the bottom of the TLS screen is gone. The account
  set up before becomes the first authority, its certificates kept.
- **A certificate is asked of its authority as soon as it is placed**, not at
  the first visit. The line says *Asking the authority...*, then shows the
  certificate - or the authority's refusal, explained (name not pointing here,
  port 443 not reaching the gateway, a CAA record, the rate limit), with a
  Retry button.
- **Saving an authority checks that it answers**: a wrong URL is said at once.
- **Five ways in, each its own dialog:** generate a self-signed certificate for
  the names you type, import a PEM pair (pasted, chosen or dropped - each file
  lands where it belongs), import a keystore, create a signing request, ask an
  ACME authority.
- **TLS explains HTTPS on a local machine**, in a drawer: the hosts file, a
  local authority the browsers trust (mkcert), one certificate for every name
  and its import, per system, with the names you type in the commands.
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
- **Signing in over plain HTTP works after a session over HTTPS.** A browser
  refused to let the plain page replace the Secure cookie, so the console's
  plain door - the one for a broken certificate - silently did nothing. Cookies
  set over HTTPS now carry the `__Host-` prefix. The console's HTTPS also tells
  browsers to forget an earlier HSTS promise.
- **A redirect route's audit upload is greyed**, with the reason: a redirect
  has no operations to audit.
- **Field info icons no longer look like buttons**, still reachable with Tab
  (the tooltip opens on focus), and a screen reader reads their explanation
  once instead of twice.

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
