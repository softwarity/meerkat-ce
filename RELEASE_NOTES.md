# Release Notes

## NEXT RELEASE

- **plug 2.31.4 in the Enterprise image.** The client developers install with `ssh get@<gateway> install`, and the agent built into the gateway.
---

## 1.20.0

- **A configuration pulled back from git no longer shows every route as changed.** The
  activation plan, the pull plan and Compare now read both sides the way an export writes them
  and the gateway keeps them: a route saved from the console, captured and read back, shows no
  line to update. "Matches what runs" has one answer for the console, the agent and the restore
  points, so it is true right after a save, logo included.
- **A notification's link opens the screen it names.** Behind a route that keeps its path, a
  path given from inside the application (`/en/registry`) is placed under the route's entry, so
  Open lands on the screen instead of the gateway's root.
- **A notification has one id, the sender's, and it is required.** Every message carries
  `"id"`, a UUID of the service's choice: it lets the service retry without duplicates, replace or
  withdraw the notification, and follow its acknowledgements and mails. A message without one is
  refused; a service that will never come back to a notification sends a fresh UUID and does not
  keep it. The gateway never makes an id up. The id belongs to the service as the internal plane
  recognises it (a token's name, a ServiceAccount, the workload behind the address), so two
  services may use the same one; the source is kept on every notification and shown in the
  console. Published again under the same id before it expires, even after it was withdrawn or
  replaced, the answer is the notification already there, with `"existing": true`: nothing is
  created and nobody is told twice. Sent to every tenant, the id names the whole send. Actions are
  called with `notification=<id>` and `onAck` receives the same id. The `key` field is gone.
- **Replace and withdraw by id.** A message carrying `"replaces": "<id>"` ends the earlier one:
  its readers no longer see it, the replacement stands in its place. `DELETE
  /api/notifications/{id}` withdraws one, every copy of a message sent to every tenant with it,
  and answers `{"withdrawn": [...]}` (or `"already"` when it had ended). A withdrawn
  notification stays in the reader's list as a row saying it was withdrawn, with nothing of
  what it said, out of the count. In both cases the mails not sent yet are cancelled, and the
  console keeps both with their state.
- **Calls that arrive out of order are handled.** A withdrawal or a replacement naming an id not
  received yet is kept (a withdrawal answers `202`, for 30 days or what `?expires=` says, 90 at
  most): when that id arrives, the notification is stored already ended, and nobody is told or
  mailed.
- **Exercises say so.** `"exercise": true` keeps a realistic level and the same audience, and
  labels the notification EXERCISE, in the reader's language, on the list, the snackbar, the
  desktop notification, the mail's subject and first lines, and the console. The New
  notification drawer has the switch.
- **Notification mails survive a restart.** Each reader's mail is queued in the database and
  sent one at a time by one node: a gateway stopped halfway sends the rest when it, or another
  node, runs again. A temporary refusal is tried again for 24 hours, a permanent one fails with
  the relay's code, and a connection lost after the message was sent is marked unknown and never
  sent twice. Every attempt carries the same Message-ID. A service reads the outcomes in its list
  (`mails`: accepted by the relay, failed, unknown, cancelled, no address), and the console shows
  each reader's mail.
- **Someone who joins later still receives what is live.** A notification keeps the audience its
  sender named. When someone joins the tenant, or is granted a role there (through a group of
  roles), the live notifications that now target them are given to them in the background, with
  one message "Notifications you missed: N" leading to their list and one digest mail when at least
  one asks for mail. Opening the list catches up too. Saving a membership or signing in is never
  held up by it.
- **The audit trail follows the internal plane.** A service's publication, a replacement, a
  withdrawal, a notification's mail outcome (one line per notification) and the notifications
  given after their publication are written with the sender, or the gateway, as the actor, next
  to each acknowledgement. Reading stays off the trail. The Data plane Notifications screen
  follows publications from the internal plane live.

---

## 1.19.0

---

## 1.18.0

- **Tenants carry their own fields (Enterprise, several tenants).** Infra > Model now has a
  second list, Tenant fields, with the same kinds as the account fields. Root and app-admins fill
  them on the tenant's General screen; the tenant's own administrators see them read-only. A
  route forwards them with the other identity attributes, marked `tenant` in the editor, taken
  from the session's tenant. A name belongs to one list only: the model refuses a name already
  used on the other list.
- **Fields with several values.** Account and tenant fields get two new kinds: Multiple choice
  (several values from a list, kept in the list's order) and List (values typed freely, kept in
  the order typed, without repeats). A header or a page attribute carries them on one line
  (`FR,ES`), a JWT or an OIDC claim as an array. None of their values may contain a comma.
- **A field added to the model reaches the routes at once.** Saving Infra > Model ticks a new
  account or tenant field on every route that already forwards identity attributes, and removes a
  deleted one from every route, page attributes included. The screen names the routes it
  rewrote, and each change is in the audit trail. Putting a field on a page stays a choice made
  on the route.
- **Creating a tenant asks for its name and group mode only.** The description is filled
  afterwards on the General screen.

---

## 1.17.0

- **plug 2.31.2 in the Enterprise image.** The client developers install with `ssh get@<gateway> install`, and the agent built into the gateway.
- **A schedule says what it is for.** The scheduler API takes a `description`, a sentence or two
  under the schedule's name in the console. An operator writes it in the schedule's drawer when
  the service gave none, and a service `PUT` without one keeps it. While a schedule runs, the
  line under its name shows the run instead.
- **A long job can be told to stop, and its report says so.** A report on a run (`PATCH
  /api/schedules/{id}/run`) is answered `204` to carry on, or `410 Gone` to stop, with
  `{"stop": "paused" | "cancelled" | "deleted" | "lost" | "closed"}`. The run is already closed
  on the gateway's side when the service hears it. Pausing a schedule stops its job at the next
  report (a closing report is still taken). **Cancel run** in the drawer, the agent's
  `cancel_schedule_run` or `DELETE /api/schedules/{id}/run` closes the run at once as the new
  `cancelled` state. Before, a pause let the job run on and a deleted schedule showed as a 404.
  `POST /api/schedules/pause` and `/resume` on the internal plane act on every schedule a filter
  matches (`meta.svc=...&meta.job=...`).
- **A lost run can be sent again.** With `retryLost` (an ISO duration, none by default), a run
  the service accepted and then went quiet on is sent again after that delay, as the same run
  (same `Meerkat-Job-Run`, next `Meerkat-Job-Attempt`), three attempts per turn at most. A
  service restarted mid-job no longer loses its turn until the next day.
- **A running job's own words show live.** The `detail` of a progress report ("7 233 / 9 882
  records") shows under the schedule's name and in its drawer, which now follows the live row
  instead of the row that was clicked.
- **The 202 contract is written out end to end** - headers, report, answers, lease - in the
  Scheduler's documentation and in the console's API drawer.
- **For a service that runs long jobs, two answers change.** A progress report on a paused
  schedule is now answered `410 Gone` (it was `204`): stop the work, the run is closed. And
  `overlap: "run"` is refused when a schedule is written.
- **`overlap: "run"` is refused.** It was accepted and never acted on: a schedule holds one run
  in flight, so a turn due during a run waits for it. Recreate a schedule that asked for it
  without the field.
- **A schedule's state is a bar along its row.** The State column is gone: a thin bar across
  the bottom of each row is a quiet track while nothing runs, moves while a call waits for its
  answer, fills with the progress the service reports, and runs backwards in red for a lost run
  waiting to be sent again. What the run is doing (accepted, the attempt, the gateway that
  claimed it, the service's own words) reads under the schedule's name, and Last run reads
  "running" and since when rather than a previous result the bar contradicts. A running row keeps
  its height - in 1.16.0 the claiming pod's name wrapped at every dash and stretched it. The
  roles a schedule calls with leave the row for its drawer, where they already were.
- **A schedule's drawer: the runs, the payload, and the actions at the bottom.** The runs fill
  the drawer and scroll on their own, each as tall as its words; the payload folds against
  them, one open at a time, and Pause, Run now and Delete stay at the bottom. "Last run" is
  gone: it was the first run a second time. So is the button that ran a past turn again: it
  sent the same call as Run now, with today's payload. A service can still name the run it
  replays, through the API.
- **The Scheduler's dates follow the profile's timezone, and nothing else.** The UTC switch in
  its banner is gone: the timezone chosen on the profile already says which hours a person reads,
  and an account without one reads UTC. A disabled switch only asked what it was for.

---

## 1.16.0

- **Who may call the scheduler's API is a setting.** On the internal plane, the scheduled calls
  API asked every service for a token of perimeter `schedules`. It now has three modes, chosen
  from the Scheduler's new **Access** button, beside API: **None** asks nothing, and names the
  caller by its token or its ServiceAccount when it sends one, otherwise by the workload
  (Kubernetes) or the container (Docker, Swarm) at its address; **Meerkat token** is the
  previous behaviour; **Cluster token** takes the pod's own ServiceAccount token, checked by
  Kubernetes. A list of callers can narrow any mode, and a caller nobody could identify is then
  refused. Each new schedule records who created it and how that was known, shown as **Created
  by** in its drawer. An installation that holds a `schedules` token keeps the Meerkat token
  mode on upgrade, so nothing opens; a new installation starts in None. The report on a run a
  service answered 202 to is proven by the run identifier the call carried (`Meerkat-Job-Run`)
  in every mode: the mode decides who writes schedules, not who answers the gateway's calls.
  Under Infra, Access tokens, `schedules` tokens are listed apart under an **Internal** filter,
  where **New internal token** creates one, and existing ones keep working. **Mint a token**
  moves from the API panel to the Access drawer, shown only under Meerkat token since the other
  modes need none, and the example curl is gone: the console mints the token. The Helm chart
  grants `create` on TokenReviews, `list` on ServiceAccounts (`rbac.internalCallers`) and `get`
  on ReplicaSets where it already reads the pods.
- **The Scheduler's table keeps its order, and gives the name its room.** It was sorted by next
  run, so Run now moved the schedule to the top while it ran and the progress showed on the line
  above, as if another schedule had started. It is sorted by name now; the agent tool still
  lists the soonest first. The Calls column is gone - method, path and route side by side read
  as a smear, and the drawer shows them - and When, Next run, State and Last run have fixed
  widths, so a name no longer wraps over two lines while the dates sit half empty. The API and
  Access buttons lose their icons.
- **Deleting a schedule asks for its name.** A schedule belongs to the service that wrote it,
  and that service is never told it is gone: it is simply not called again. Deleting one, from
  its row or its drawer, now asks for the name to be typed, as deleting an organisation does.
- **The upstream picker lists each service once, and stores its full name.** The route editor's
  list showed one entry per port, so a service on three ports read as three services, each with
  a replica count. It now shows each service once, by its short name, with the full one smaller
  on a second line (`reference-data.canopy.svc` on Kubernetes, `stack_service` on Swarm, nothing
  more on Compose), and that full name is what the route keeps. Its **Port** sits beside it,
  always shown and filled: a list when the service has several, read-only on a single one, typed
  when the runtime knows none, and a port the list does not carry (a route set up by hand) stays
  shown. A cluster CA constrained to cluster names (`svc`, `cluster.local`) can certify the full
  name, and never the bare `reference-data`, so an https upstream verified against such a CA
  failed. An existing https route still on the short name is flagged in the editor, with a
  button that rewrites it; nothing changes until it is saved.
- **A roles expression can decide, and the console reads it.** The expression a route uses to
  shape the roles it sends is Go's text/template: `if`, `else if` and `else` turn the roles into
  the one value an application wants (Grafana's Admin, Editor or Viewer from three roles), and a
  plain value (`["admin"]`) is sent as it is. The gateway always ran them; the console's preview
  did not, showed them as errors and would not apply them. It reads them now, its "Start from"
  menu offers them under One value, and its legend explains if / else and links the
  documentation of the syntax.
- **A certificate asked of cert-manager follows what the cluster CA may sign.** On Kubernetes
  the gateway asks cert-manager for its doors' certificates with every name of its Service,
  the bare one included (`meerkat-meerkat`). Under a cluster CA with name constraints that
  permit only cluster names (`svc`, `cluster.local`, the namespace), cert-manager's CA issuer
  signs that bare name anyway, and every verifier (Go, OpenSSL) then refuses the whole
  certificate, whatever name was called. The gateway now reads the constraints of the chain the
  Secret carries and brings the Certificate down to the names they permit, and cert-manager
  issues it again; asked again from the console, the refused name does not come back. A name
  the CA permits stays, even bare (`meerkat-meerkat-internal`). When it permits none of them,
  the Certificate is left as it is and its row on Domains & TLS says which names are refused
  and by which constraints. Under a CA without name constraints nothing changes.

---

## 1.15.3

- **An agent connected over OAuth no longer has to sign in again every morning.** Its access
  token lasts twelve hours and its refresh token ninety days, but the periodic upkeep dropped the
  connection as soon as the access token lapsed, and its refresh token with it: an agent left idle
  overnight (Claude Code, for one) was sent back to the sign-in page instead of refreshing. The
  connection now stays while it can be refreshed.

---

## 1.15.2

- **A page whose `<base href>` has no quotes keeps working under a prefix.** With `strip-prefix`,
  the gateway puts the route's prefix back in front of the page's `<base href>`, but only when the
  value was quoted. A minified single-page application writes `<base href=/>` (Hubble UI does), so
  its scripts and stylesheets were asked at the gateway's root and the page stayed blank.

---

## 1.15.1

- **plug 2.31.1 in the Enterprise image.** The client developers install with `ssh get@<gateway> install`, and the agent built into the gateway.
---

## 1.15.0

- **The Links mode keeps the catalogue's containers.** A container was flattened into the list;
  it is now a heading over its modules, in the account button's Applications menu and on the
  built-in pages (profile, sign-in). The Portal screen shows, beside the list, the real account
  button with that menu open, drawing the list being edited.
- **Breaking: the navigation bar (Portal mode) is Enterprise.** The Team and Enterprise licences
  carry it, and the Community image no longer contains it. That image refuses to save Portal mode
  and reads a catalogue already in it as Links: the same list, in the user button's menu, nothing
  to retype.

---

## 1.14.0

- **plug 2.31.0 in the Enterprise image.** The client developers install with `ssh get@<gateway> install`, and the agent built into the gateway.
- **Ctrl+S / Cmd+S saves in the console's code editors.** The custom CSS and JavaScript of a
  route, its scheme and language scripts, the roles expression, the configuration YAML being
  edited, and anywhere in the route editor: the key does what the Save button does, and the
  browser's "Save page as" no longer opens over the console.
- **A selection shows in the code editors.** Selected text on the line being edited was not
  highlighted: the active line's opaque background covered it.
- **A schedule's health reaches the audit trail.** One line when a schedule starts failing
  (`schedule.failing`: what broke, and the run that tipped it), one when it comes back
  (`schedule.recovered`, with since when it was failing), signed `meerkat` - not one per turn,
  which the run history already keeps. A failure the automatic retry absorbs, or a turn the
  service asked to have later, writes nothing.
- **The portal and the user button agree with the gateway on a session's organisation.** A
  session opened before its account joined an organisation acts in the only one the account
  has; the gateway already let it into a route gated by a role held there, while the portal
  hid that route (and a container holding only such routes) and the user button showed no
  role. All of them now read the session the same way.
- **A route's signed JWT takes an audience.** The `aud` of the jwt and signed-jwt tokens was always
  the route's name; the route editor now has an Audience field (and `identity.audience` in the
  API), the name still being the default. A service that checks for a value of its own gets it,
  and renaming a route no longer changes what its tokens say.
- **One rule for an audience, a route's or an OIDC client's.** Any text up to 255 characters,
  spaces inside included, no control character: an OIDC client's audience refused a space, which
  no standard asks, and a route's default audience is its name, which may hold one.
- **The license has its place.** On the Enterprise image, root changes it on the License screen
  (**Change license**: paste the file, drop it or choose it), and it is verified before it is kept;
  or the deployment mounts it from a secret (`MEERKAT_LICENSE_FILE`, `license.existingSecret` in the
  chart), which the screen then shows and does not replace. The screen names the licensee and how
  far its updates go. It switches nothing yet: the image still decides the edition.
- **An agent creates a git location.** `save_git_location` creates one, or changes the one of
  that name (only the fields given), with the console's rules: https, a branch, and the token as
  a vault reference, never the token itself.
- **The dev capability decides alone; developer mode is gone.** The installation-wide switch on
  Application, General said a second time what the `dev` flag of each account already says. Root
  now always has the developer tooling; any other account when it carries `dev`. To close the
  developer surface of a production gateway, **Remove developer mode** on the Users screen takes
  the flag from every account but root's. **If you had turned developer mode off**, the accounts
  that still carry `dev` get their tools back with this version: use that button once. A saved
  configuration that still holds `dev_mode` imports as before, the key is ignored.
- **A session ends when its working hours close** (Enterprise). They were checked at sign-in and
  never again: a session opened at 17:55 in a window closing at 18:00 lasted its whole lifetime.
  It now ends with the window, the page going back to sign-in, which says the hours are closed.
- **Working hours for an account, and for a member from the console** (Enterprise). The most
  precise hours set win whole: the member's, then the group's, then the account's, then the
  organisation's, then the gateway's. An account's own are set in its drawer (Security); a
  member's own hours and access dates in **the member's drawer**, opened by the clock on its row
  in the members table. A member's dates now hold when it inherits the hours as well.
- **The gateway's working hours hold for an account with no organisation** (Enterprise). They
  were never checked for it at sign-in.
- **The organisation chooser greys out a closed organisation** (Enterprise), marked *Outside
  working hours*, instead of refusing it after the click; a sign-in with every organisation closed
  is refused.
- **Working hours for a group, in exclusive group mode** (Enterprise). Where one group is chosen per
  session, a group can have hours of its own - a day shift, a night shift - between the member's
  and the account's. The group chooser greys out a closed group, and the session ends with the
  chosen group's hours. In cumulative mode a group has no hours of its own.
- **A group and a member are edited in a drawer.** On the Groups screen, a group's name, now a
  tonal button, opens its drawer (name, description, working hours, and its deletion in a danger
  zone, after a confirmation); groups are created and edited there only, the Members screen no
  longer has a group menu nor a + button; the clock and history buttons on a member's row open the
  member's drawer, each on its own page. Both cover the page's whole height.
- **The route editor offers the services of the other Kubernetes namespaces.** A Namespace choice
  above the upstream offers the namespaces that hold a Service, and External. A namespace chosen,
  the upstream is picked from its services, with only the base path typed beside it; External is
  the free entry, for a public site or a machine. An existing route opens on the namespace its upstream points into. The
  chart grants it by default (`rbac.clusterServices`: a ClusterRole that reads services, nothing
  else); set it false and the gateway keeps to its own namespace, with no choice shown. The `list_services` agent tool takes the namespace too.
- **The License screen's legend sits on the title's line**, aligned right.
- **The developer tunnel is marked built on the License screen**: laptop to cluster, cluster to
  laptop, the substitution's lifecycle, several substitutions at once, its security and its audit.
  What they were said to lack was either already there (a substitution ends with its plug session),
  the rule itself (one developer per service name; the laptop's calls go straight to the services;
  a name nobody serves yet is taken as it is, to start new services), or no business of a
  development tool (rights per service, an event for what a plugged process reads of its
  workload's environment).
- **Invite a new account by mail, or reset a password by mail.** Creating an account asks how its
  owner gets in: a temporary password shown once, or an invitation mailed to the account, a link
  valid 48 hours to choose a password. A password reset, on the account or on a member, asks the
  same. Offered when a mail relay is configured, local passwords are allowed and the application
  has a public name the gateway vouches for. Two new mails, in the 20 languages, are in the mail
  relay's sample list.
- **Starting and ending a UI test are in the audit trail**: `uitest.start` and `uitest.stop` on the
  developer's account, naming the route, the simulated user and its roles. A test also ends the
  moment its developer loses the dev capability.
- **The UI test mode works with several replicas.** The running test lived in the memory of the
  node that took the Apply, so the reloaded page, answered by another node, came back unchanged
  and the bar closed. It now rides a cookie signed with the key every node reads from the
  database, bound to the session and to the route.
- **The UI test mode is reachable under the navigation portal.** The portal's account button did
  not know which route the page came from, so its Developer menu lacked the entry. It does now.
- **The plug and DEV tabs read as text**: the word and its chevron in the strip's orange, with no
  filled badge.
- **The route editor's user info fields line up behind the longest fact**: a custom one such as
  employeeNumber ran into its field, and each field's label sat on the one above.
---

## 1.13.0

- **plug 2.30.0 in the Enterprise image.** The client developers install with `ssh get@<gateway> install`, and the agent built into the gateway.
- **The roles screen says what uses each role.** A **Used by** column lists the routes and
  scheduled calls whose rules name the role (access rule, endpoint override, declared role tied
  to it), read off the configuration rather than typed, so it never drifts from it; a filter in
  the banner narrows the catalogue to what one route or schedule relies on. Tags stay what an
  operator writes. `GET /api/roles` and `list_roles` carry it as `usedBy`, and a write ignores it.
- **Who is plugged in, and a way to end it.** Application, Plug (Enterprise, while the tunnel is
  open, for the application administrator who grants the developer capability) lists the plug
  runs open now, and who may plug in with their keys, one row
  per run however many replicas its connections landed on: the developer, where
  from, which plug, when it was plugged in, and what it serves - or "client only". **End this session** stops
  the developer's command as a Ctrl-C would, with the reason in their terminal, and puts back at
  once what it had set aside, on every replica. Removing a key, the developer capability, or the
  account ends what that key holds at once, whichever screen did it.
- **A plug run says what it is** (plug 2.30.0 and later): serving or client only, and whether it
  is a `--dockerrun` (the Docker logo beside its version), on the plugged-in list and in the
  trail's opening line. Each name served says whether the run parked the cluster's service or
  added a name the cluster did not have.
- **plug in the audit trail.** Under **plug**: each run's opening and end, once whatever the
  number of replicas, with how it ended and how long it lasted, and each name served and given
  back.
- **An agent stays connected.** An assistant that sent two requests at once after its access token
  expired refreshed twice with the same refresh token, lost the race once, and had to sign in
  again. A refresh token presented a second time within 30 seconds is now served like the first.
- **A role an application's declared role is tied to is in use.** Deleting it is refused and names
  the route, as for an access rule; `list_role_references` lists it (kind `roleMap`).
- **The Helm chart gives each pod its own address to the developer tunnel** (`PLUG_POD_IP`), so
  that with several replicas a parked service always names the one serving it.
- **The agent reaches the whole control plane.** Everything the console does, an assistant can do
  through MCP, under the same guards and audit; the few things closed to agents say why.
- **Try an OIDC sign-in for anyone.** The arrow on a client's row picks a person and their
  organisation and answers what the application would get: the route that lets them in, those
  that turn them away, and the claims of the ID token, the access token and userinfo - built by
  the code that builds the real tokens, nothing issued. The agent has `try_oidc_sign_in`.
- **The issuer and the signing keys are in an agent's reach.** `get_signing_keys` (the public
  keys only), `set_issuer`, `renew_signing_keys` and `preview_identity`: an assistant putting an
  application behind the gateway no longer stops at choosing the issuer.
- **`save_configuration` can refresh a saved configuration** from the running state
  (`replace: true`), as the console's Capture does, before a push.
- **An OIDC client's secret, one choice of three.** No secret (PKCE alone), drawn by Meerkat, or
  given by the application when it imposes its own - its chart generates it, its vendor fixed it.
  One field under the choice: a drawn secret sits in the form, to copy before saving, so the
  window that showed it once is gone; the application's is pasted there. Either way only its
  sha256 is kept. Editing a client keeps its secret until another is drawn or pasted. The API and
  the agent's tools take an optional `secret`.
- **No more "Only listed operations are reachable".** The switch at the foot of Endpoint security
  closed every operation no rule lists, from a place nobody looks when a new endpoint answers 403.
  It did what the route's own rule set to **Nobody** already does: closed by default is now said
  on the route, where its rule is shown, with the operations opened one by one.
- **What an application requires, read from its own spec.** When a route's OpenAPI says which
  roles an operation needs (the scopes of its security requirement, as nestjs-granted or springdoc
  write them), Endpoint security shows it per operation, read-only: the application enforces it,
  not Meerkat. The developers' API docs write it at the head of each operation, since swagger-ui
  never shows those roles.
- **The roles an application declares, mapped onto the catalogue.** Endpoint security lists every
  role the spec names and ties each one to a catalogue role: created under a prefix and a tag (the
  route's name by default), or an existing one. A route that maps its roles forwards, by default,
  only the declared roles the caller holds, under the application's names
  (`{{.Roles | .Declared | join ","}}`): someone holding `orders-ROLE_ADMIN` reaches the service as
  `ROLE_ADMIN`.
- **A route whose service is down no longer holds the API docs.** Endpoint security, the developers'
  API docs and the agent's list of operations used to wait up to twenty seconds on a spec the
  service could not serve. A route the health watch knows is down (its target down, or its circuit
  open) is now answered down at once, and a service that does not take the connection gives up
  after five seconds. Choosing another route or spec while a slow one loads no longer lets the slow
  one paint over it.
- **OIDC clients travel in a configuration.** Export, import, comparison, the startup file, restore
  points and git now carry the applications Meerkat is the OpenID provider of (`oidcClients`), so
  an imported route no longer names a client the gateway does not have. A client's secret travels as
  its sha256 only, never in clear: imported elsewhere it keeps the same secret, and the application
  changes nothing. A confidential client imported without one has no secret yet, and its row says
  so. An import never takes a route's sign-in away: a client a route still uses survives a prune,
  so a configuration exported before clients travelled still activates, and a route naming a
  client nobody declares imports and is said in the plan. The community image leaves the clients
  out and says so in the plan.

---

## 1.12.0

- **plug 2.26.3 in the Enterprise image.** The client developers install with `ssh get@<gateway> install`, and the agent built into the gateway.
- **The License screen says how far each feature is built with an icon.** A word on every row
  ("partly built", "planned") gave way to one icon per state, explained once in a legend, and the
  contract's keys (SVC-02, AUTH-03...) no longer show: nobody knows a feature by them.
- **An entry turned off in the portal is crossed out in its preview.** It looked like any other
  in the console, though the people's bar never offers it.
- **The issuer is chosen among the application's public names, no longer typed.** Every token
  Meerkat signs - the identity JWT of a route, the OpenID provider's - and both discovery
  documents carry one issuer: `https://<name>` for a public name declared on the Application tab
  of Domains & TLS (`http://localhost:<port>` for a name on this machine). With several names,
  the OIDC / JWT drawer's **Public address** chooses; adding, removing or reordering them moves
  the issuer on every node. With none, tokens still say `meerkat` and the drawer links to that
  tab. A typed issuer is gone: an installation that had set one now signs with its first public
  name, so the services checking `iss` must expect it. A library that started from an issuer set
  to the internal plane's URL now reads the internal discovery document's address instead.
- **Meerkat is the OpenID provider of the applications behind its routes (Enterprise).** An
  application that only knows how to sign in through an OpenID Connect provider now signs in
  through the gateway: authorization code with PKCE, refresh tokens, userinfo, discovery, tokens
  signed RS256 with the gateway's keys. Who may sign in is the route's access rule, and what the
  tokens say is the route's own list of claims; two levels of access for one application are two
  routes on the same paths and client, and the first that grants the person decides, again at
  every refresh. There is no consent screen: the clients are declared by an administrator. A
  client declares no redirect URI: a code goes back only under the path of a route bound to
  it, on that route's host. The provider lives at the issuer, the application plane's public
  name, and servers inside the cluster may exchange their codes on the internal plane. The
  community image refuses to declare a client and serves none of the provider's paths.
- **The JWT drawer is now "OIDC / JWT", and a route can sign in through a client.** At its top,
  the three well-known addresses to copy: the internal discovery, the provider's public
  discovery while it answers, the JWKS. Beside the issuer and the keys, it lists the OIDC
  clients and the routes using each: id, whether the application keeps a secret (shown once,
  regenerated on demand), a warning mark when one of its routes covers a whole host, audience
  and token lifetime. In a route's Auth forward, the OIDC mechanism picks a client and lists the
  claims sent, each a fact of the Identity section or a fixed value.
- **Applications can sign out through the gateway by a link.** `/logout` on the data plane now
  also answers a GET, which is all a sign-out button inside an application can send: from a page
  of the site it signs out at once, like the POST; from anywhere else, or ahead of a click, it
  asks first. No address to return to is read from the request.
- **Expired sign-in leftovers are now swept.** The agents' OAuth codes and refresh tokens, the
  one-shot challenges of passkeys and of the sign-up check, and trusted browsers past their trust
  were written with an expiry that nothing ever acted on: the ones abandoned halfway stayed in the
  database for good. The periodic upkeep now removes them.
- **A route's OpenAPI spec is read with the route's own trust.** A service whose certificate an
  authority trusted in Domains & TLS served its pages through the route but refused its spec
  ("certificate signed by unknown authority"): the spec was fetched trusting the system's
  authorities only.
- **The shape of the roles says what it does, and shows what it sends.** In a route's Auth
  forward, the roles block explains the expression (keep or drop by name or tag, cut or add a
  prefix, change the case, a separated list or a JSON array) and renders the configured one on
  the real catalogue, for someone holding every role.

---

## 1.11.1

- **The portal's rail lines up and fits its labels.** Collapsed, an application's icon sat a few
  pixels left of its pill and of the menu button, and a label longer than the pill was cut
  ("Refere..."): the icons are centred on one axis and a label takes the rail's width. Opened,
  the rail took a fixed 280px; it is now as wide as its widest label, or its logo and name, with
  the same padding on both sides.
- **A portal entry may point at a route turned off.** The portal was refused as a whole as soon
  as one of its entries pointed at a disabled route ("not an enabled UI route"), so a single such
  entry blocked every save, even one that only added a valid application. A route turned off is
  now accepted: the bar shows its entry in place, greyed out and leading nowhere, until the route
  is back on, and a container whose modules are all off is greyed out too. A route deleted or no
  longer UI after its entry was saved no longer blocks the save either: the bar skips it and the
  console names it so the entry can be removed.
- **A new sub-module no longer takes its container's name.** Added from a container's drawer, it
  came up with the container's label and description already filled in, and so did any entry
  clicked in the preview while another one was open. A new entry now starts empty, and an empty
  label is the route's name.

---

## 1.11.0

- **Duplicating a route works again.** The copy carried the original's revision on a new id, which
  the gateway read as a route deleted since it was opened, and refused. A second copy of the same
  route now takes the next free name (`-copy-2`...), and a refusal shows the gateway's own reason
  instead of "Request failed".

---

## 1.10.2

- **A route with no language gets no language menu under the portal bar.** The bar fell back to
  the gateway's whole offer, so the account menu listed the sign-in page's languages over an
  application that speaks none of them, and picking one changed nothing on the page.

---

## 1.10.1

- **The access log is a console switch.** Infra, OpenTelemetry, Logs tab, **Access log**: on and
  off live, on every node, kept across restarts. It replaces `MEERKAT_ACCESS_LOG`, and the
  trace export set by `MEERKAT_OTLP_ENDPOINT` and `MEERKAT_OTLP_SAMPLE` is set on the same
  screen: the three variables are no longer read, and set they only log a warning. If you
  relied on one of them, switch the matching setting on in the console.
- **Developer mode and plug are switched in the console, in production too.** `production: true`
  in the chart (`MEERKAT_PRODUCTION`) used to close them whatever the console said; it is no
  longer read, and set it only logs a warning. Logs start as text; JSON for an agent is the
  console's switch (Infra, OpenTelemetry).
- **The plug page says how the tunnel is published.** It reads the gateway's own Services: a
  LoadBalancer and its address, a NodePort, or "inside the cluster only" with the chart value to
  change. With no address recorded, the commands on the plug page and on a developer's key page
  use what it finds; an address recorded by hand still wins, for a mapping the runtime cannot see.
- **Every released chart stays installable.** The Helm repository on the site listed only the
  newest chart, so a version pinned with `--version`, or in Argo CD, Flux or Terraform, stopped
  installing the day the next one shipped. It now lists every release, each rebuilt from its tag.

---

## 1.10.0

- **The color scheme switch in the user button reads in both schemes.** Its glyph took a colour
  meant for another background, and some themes drew a dark moon on dark and a white sun on light.
- **An agent can send a route back whole.** `save_route` refused a route read with `get_route`
  on its revision or its telemetry switch; every field a route carries is now declared.
- **The evaluation's line in the user button no longer trips a strict page policy.** On an
  application that only allows styles carrying its nonce, such as GeoServer, the line was
  refused with an error in the browser console on every page; it now renders there too.
- **Trust the cluster's authority without fetching a file.** On Kubernetes and OpenShift,
  Trusted authorities offers **From the cluster**: the authorities the cluster publishes -
  OpenShift's service CA, the cert-manager issuers, the trust-manager bundles - are found
  whatever their name, and the one picked is trusted. Routes to services in HTTPS then verify.
  A cert-manager issuer's key is never read: its certificate is learnt from a throwaway one.
  Under each authority, what it signs here - the routes refused today, the services that
  asked it for a certificate - and the routes still in http that could now go in https are
  offered, tried port and name included, to switch in one click.

---

## 1.9.0

- **plug 2.26.1 in the Enterprise image.** The client developers install with `ssh get@<gateway> install`, and the agent built into the gateway.
- **Domains & TLS, plane by plane, and a proxy that re-encrypts.** One tab per plane -
  Console, Application, Internal - and the trusted authorities in a fourth. Each plane has
  its own *Add certificate* and its certificates, and one bin takes a certificate off it
  (deleted when it was on that plane alone); the shared pool and the drag and drop are
  gone (a certificate on no plane is listed apart). Console and Application say who
  serves their **public names**, declared in full (no wildcard): *Meerkat*, each name
  covered by a certificate placed there, or *A proxy in front*. The switch only
  proposes: *Apply*, then a confirmation, saves it, and the gateway refuses a console
  switch that would see your own session as plain http and sign you out. Behind a
  proxy, a certificate on the plane now encrypts the hop from the proxy to the gateway:
  it carries the plane's names inside the cluster, shown as *Internal names* at the top
  of the tab, and *Ask cert-manager* is offered there as on the internal plane.
  Switching mode keeps the certificates: one that does not fit is marked *Inactive in
  this mode* and serves again on the way back. The first application name is the one
  mailed links carry. Where a certificate comes from (cert-manager, OpenShift, your own
  authority, Swarm, compose) is the *Platform certificates* button of the banner.
- **Each plane switches its plain HTTP door off and on.** A switch on the ports line
  of each tab, with a confirmation; HTTPS has none - it opens with a certificate and
  closes with the last one. HTTP switched off listens on 127.0.0.1 only: out of the
  network's reach, still there for a `kubectl port-forward` and for the health check
  the image runs on itself. **The chart's probes now run `meerkat health` inside the
  container** instead of an HTTP GET, so a plane whose plain port is off stays ready.
  On the console, each door shows who calls it, live, and the door in use stays: reach
  the console through HTTPS before switching HTTP off, and through HTTP before
  removing its last certificate.
- **A passkey removed from an account is forgotten by the browser too.** Removing one
  from the profile left it on the device, which kept offering it at sign-in, where it
  was refused with no reason given. The browser is now told: on the passkeys page, the
  list the account holds (`signalAllAcceptedCredentials`), and at a refusal, that this
  one is unknown (`signalUnknownCredential`) - where the browser supports them. And the
  log now says why a passkey was refused.
- **Whose forwarded headers are believed is said on that screen, plane by plane.** A plane
  served by the gateway itself believes no `X-Forwarded-For`; a plane behind a proxy
  believes it from a private network, or from the proxy addresses declared there (a CDN,
  a public load balancer). **`MEERKAT_TRUSTED_PROXIES` and the chart value
  `trustedProxies` are removed**: the gateway warns if the variable is still set, and the
  chart refuses to render while `trustedProxies` is set - declare the proxy on the
  screen, then remove it. A plane nobody configured keeps the old behaviour. The API
  `PUT /api/settings/tls/proxied/{plane}` becomes `PUT /api/settings/tls/doors/{plane}`.
- **A service behind a TLS-terminating proxy is told https.** Behind an Ingress or a
  load balancer that ends TLS, the gateway passed `X-Forwarded-Proto: http` on to the
  service - the scheme of its own connection - and the service built `http://` links
  under an `https://` page (GeoServer's login posted to http:// and its own policy
  refused it). The service is now told what the gateway believes: `https` when its own
  door was TLS or a trusted proxy said so. From a caller that is not a trusted proxy,
  `X-Forwarded-For` is now its address alone, never the chain it wrote. And
  `Forwarded` (RFC 7239) is written too, saying the same things.
- **The user button and the portal keep their look behind a strict style policy.** An
  application whose Content-Security-Policy restricts styles by nonce (GeoServer's does)
  refused the `<style>` they draw with, and the portal's bar spread unstyled over the
  whole page. The gateway now tells every policy its nonce for styles as it did for
  scripts - each policy of a header, the `<meta>` one and the report-only one - its
  scripts set that nonce on the styles they create, and a CSS block a route injects
  carries it. A policy that already lets inline styles run is left as written.
- **The collector files deploy two tiers, so traces can be sorted once complete.** *No
  collector yet?* on the OpenTelemetry page now gives an **agent on every node** (the
  address every service sends to, and the reader of the containers' stdout), which
  writes metrics to Prometheus and logs and audit to Loki, and a **gateway** that
  receives the traces alone, every span of a trace on the same replica, and keeps them
  or not by tail sampling before writing them into Tempo. One collector per node could
  not sort traces whose spans are made on several nodes. The files are new:
  `agent.yaml` and `gateway.yaml` (Swarm), `agent-values.yaml` and
  `gateway-values.yaml` (Kubernetes); the old collector, logs agent and Service files
  are gone.
- **The MCP screen connects an assistant to all your projects.** The command for
  Claude Code and Gemini CLI now carries their user scope (`-s user`, `--scope user`),
  so the gateway is there in every folder rather than only where the command ran. And
  while the agent endpoint is off, the screen shows the switch alone.
- **The MCP screen says how an agent trusts a private authority.** Over HTTPS, a
  console whose certificate comes from your own authority is refused by the agent until
  it trusts that authority. A new panel gives the command per agent and per system:
  `NODE_EXTRA_CA_CERTS` for Claude Code and Gemini CLI, the system store for Codex and
  Kimi, on macOS, Linux or Windows.
- **A service that does not answer in time is now a 504, not a 502.** When an upstream
  is reached but gives no answer within the route's first-answer timeout, the gateway
  answers `504 Gateway Timeout` (the portal's unavailable page too). `502 Bad Gateway`
  stays for a service that cannot be reached: connection refused, no route, or no
  connection within the connect timeout. **If you alert on 502 to catch slow services,
  add 504.** The circuit breaker counts both, as before, and the access log still reads
  `upstream-down` for either.
- **The identity signing keys are sealed at rest.** The private keys that sign the JWT
  forwarded to your services, and the key that signs the routing simulator's test tokens,
  were stored in clear in the database: a copy of it was enough to forge an identity every
  service believes. They are now sealed under the master key like the vault's secrets,
  taken along by a master key rotation, and sealed at the first start of this version.
  The keys do not change: tokens signed before keep verifying, and the JWKS is the same.
- **The console cannot be shown in another site's frame.** Its pages now carry
  `X-Frame-Options: DENY` and `Content-Security-Policy: frame-ancestors 'none'`: a page
  that framed the console under a decoy could have turned an operator's click into one on
  the console (clickjacking). The previews and the API reference the console frames
  itself are other pages, and they are unchanged.

---

## 1.8.0

- **Apply a configuration line by line.** Making a saved configuration current,
  importing a file onto the gateway or saving the running document now shows the plan
  with a tick on every line: untick one and the gateway keeps what it holds for it - a
  route the configuration would remove stays, an object it would change keeps its
  version, an object it would add is not created. The trail records what was kept.
  A pull from git offers it too, and serves the configuration from the same window.
- **Compare a saved configuration with what runs.** *Compare* is on every saved row and
  opens on what serving it would change here, without applying anything: the running
  state needed a name, and a second saved configuration, before it could be compared.
- **The Snapshot tab is now Database, and its procedures follow your platform.** It
  backs up, restores and moves the database, and the old address still leads to it.
  The switch to PostgreSQL is written for what the gateway runs on, with its names: a
  Secret and `helm upgrade` on the real release, `docker service update` on the real
  Swarm service and the line for the stack file, the `environment:` lines for the
  compose file, `-e` on `docker run`. *Written for* picks another.
- **A migration is one numbered procedure, from the start, and survives a reload.** The
  SQL that creates the role and the database with the rights the test checks, the
  test, the pause, the copy, the switch, the checks and the way back, each ticked when
  done. The gateway remembers the last copy (where it went and how much it carried,
  never the password), so a reload or another node comes back to the same step;
  *Dismiss* forgets it, and it goes by itself once the gateway runs on that database.
- **A stored secret can be changed again.** Its value box is closed, since the value
  never comes back, and a pencil opens it: hiding the tools of a disabled field had
  taken the pencil too, so no stored secret could be retyped from the console.
- **The vault says which secrets the master key cannot open.** After a migration under
  another key every name is listed and no value resolves, which nothing said until a
  route, a relay or an authority failed. Each such secret now reads *cannot be decrypted
  with this master key*, and the vault counts them; the value is never sent.
- **A forge's repository is typed between its fixed parts.** In a git location, the
  repository field shows `https://github.com/` before what is typed and `.git` after it
  (each forge with its own host), so it reads at once that only `owner/repo` is asked.
- **The console opens routes, and names callbacks, by the gateway's public name.** The
  address of *Open this route* and of the callback an authority is given came from the
  console's own address, which behind an ingress is one nobody outside reaches. It is
  now the first name of the Application door declared behind a proxy, else a name a
  certificate placed on that door carries, on its published HTTPS port.
- **A door is served over HTTPS by the gateway or by a proxy, one or the other.** Each of
  the Console and Application doors now says who presents its names: *This gateway*,
  with the certificates placed on it, or *A proxy in front*, with the names declared.
  A door holding one refuses the other. The names a proxy presents are one full name
  each, no wildcard: a wildcard would vouch for every name under it, including one left
  pointing at a service nobody runs any more, and a reset asked under that name would
  mail its token there.
- **The sign-in pages, the profile and the agent consent refuse another site's page.**
  The console's API already checked that a write came from the console; these pages,
  on the same cookie, did not. A page on the data plane of the same host could submit
  the agent consent and obtain a root token, or turn the second factor off. Every
  write they take now refuses a request another origin's page sent, and they can only
  be framed by their own origin (the consent by nobody). The consent names the host
  the code is sent to. Replacing the authenticator, new backup codes or turning the
  second factor off now ask for a current code (or a backup code), and changing the
  address asks for the current password.

---

## 1.7.0

- **plug 2.25.1 in the Enterprise image.** The client developers install with `ssh get@<gateway> install`, and the agent built into the gateway.
- **The OpenTelemetry export can be turned on from the console again.** The collector
  address only appeared once the export was on, and an export with no address is
  refused: the switch sprang back and the field never showed. The address is now
  always there on the Enterprise and evaluation images, and the switch turns on once
  it is filled in.
- **A mailed link only points at a name the gateway can vouch for.** A reset, a
  sign-up's confirmation and an address change used to mail a link built from the
  request's `Host` - which the caller writes: a reset asked for someone else's address
  with `Host: attacker.example` mailed them a genuine token to that host. A link is now
  mailed only when the request reached Meerkat's own TLS door and a placed certificate
  covers the name. Anywhere else - plain HTTP, TLS terminated by a proxy in front - the
  mail carries a six-digit code and the page waits for it, in the browser that asked.
- **Behind a proxy that terminates TLS, links again.** The TLS screen is now
  **Domains & TLS**, and on its Console and Application doors you can declare the
  names a proxy in front - nginx, an ingress, a load balancer - serves over HTTPS
  (`shop.example.com`, `*.example.com`). A reset or a confirmation asked under one of
  them mails an `https://` link back to it, though the request reached Meerkat in
  plain HTTP. They are only that: no certificate, no Force HTTPS redirect, no HSTS.
- **Released versions are signed.** Until now only `latest` and `eval` carried a
  signature: a release stamps the tested image with its version, which makes a new
  manifest, and nothing signed that one. Every tag a release pushes is now signed and
  verified before the version is cut; versions up to 1.6.0 stay unsigned. The identity
  to check is narrower too - two workflow files on `main` of `softwarity/meerkat-ce`,
  given on the install page with its Kyverno policy.
- **The community release is the image the mirror tested.** A release was meant to
  stamp it with its version and, through a misread label, rebuilt it every time.
  It now stamps it when the mirror's build of the same commit is there.
- **A caller can no longer choose its own address.** `X-Forwarded-For` was believed
  from anybody, so a caller reaching the gateway directly wrote a new address per
  request and walked through every counter keyed on it: fifty wrong passwords against
  a budget of three gave fifty refusals and not one `429`. It is now believed only from
  a trusted proxy - by default the loopback and the private ranges, which is where an
  Ingress, the Docker bridge or a local nginx lives - and `X-Forwarded-Proto` follows
  the same rule. **If a CDN or a cloud load balancer reaches your gateway from public
  addresses**, name its ranges in `MEERKAT_TRUSTED_PROXIES` (`trustedProxies` in the
  chart), or every caller is counted as that balancer and, behind TLS it terminates,
  the redirect to https loops. `none` trusts nobody. The history and the audit keep the
  proxy beside the address: `203.0.113.5 via 10.42.0.15`.
- **Failed sign-ins are counted per account too, from any address.** Past five in the
  window (Application, Security, Rate limiting: "Failed sign-ins on one account"), the
  sign-in for that name also asks for the picture code - never a lockout, an unknown
  name is treated exactly the same, and the owner gets one mail per window saying how
  many failures came from how many addresses.

---

## 1.6.0

- **plug 2.25.0 in the Enterprise image.** The client developers install with `ssh get@<gateway> install`, and the agent built into the gateway.
- **The image has no shell any more.** It is distroless, like plug's: no shell, no package
  manager, smaller, and nothing for a command to reach but the gateway itself. plug's
  download account, the last reason for a shell, is answered by the gateway's own binary.
  A healthcheck that ran `wget` inside the container now runs
  `["CMD", "/meerkat", "health", "http://127.0.0.1:8080/readyz"]`, as the Swarm stack does;
  `docker exec ... sh` has nothing to open - debug from a sidecar or `kubectl debug`.

---

## 1.5.0

- **A third port for what your services call.** The internal plane (`:7070`,
  `MEERKAT_INTERNAL_ADDR`) serves the JWKS your services verify the identity JWT
  with, and the scheduler's API, inside the cluster only: the chart gives it its own
  ClusterIP Service that nothing publishes. Publishing the console no longer
  publishes the API services write their schedules with, and a NetworkPolicy can now
  keep the console away from the backends.
- **The console reads schedules and writes none.** It pauses, resumes, runs, replays
  and deletes them; creating or changing one is the service's, with its token, on
  the internal plane. A service token still writing on the console's port keeps
  working while its backend moves, and the log says once which token it is.
- **Breaking: the JWKS left the application port.** It is served on the internal
  plane only, `http://<gateway>:7070/.well-known/jwks.json`, and the JWT drawer gives
  the address. A service still reading it on 8080 can no longer verify its tokens:
  point it at the new address before or with the upgrade. Same keys, same tokens.
- **The issuer is yours to set, and the internal plane says it.** The `iss` of the
  identity JWT is `meerkat` until you change it, in the JWT drawer. Beside the JWKS,
  the internal plane now serves `/.well-known/openid-configuration` with the issuer
  and the keys' address, and it is the recommended way to verify the tokens: one
  address, from which a library learns both. Set the issuer to
  `http://<gateway>:7070`, and a library that starts from the issuer, such as
  Spring's `issuer-uri`, needs one line.
- **Upstreams in HTTPS signed by your own authority.** Infra, TLS, has a new section,
  Trusted authorities: what the gateway accepts when it calls, apart from the
  certificates it presents. Add authority opens one dialog: a name, and the
  authority's certificate chosen, dropped or pasted. Added to the public authorities,
  never instead of them.
- **Certificates from the file system, and HTTPS inside the cluster.** Add certificate,
  From the file system lists what the runtime mounted in the container - a Kubernetes
  Secret, a Swarm secret, a compose volume - with the key beside each certificate.
  Only the paths are kept, the key never enters the database, and a renewal by
  cert-manager or OpenShift is served without a restart. A third door, Internal, takes
  such a certificate and opens HTTPS on port 7443 for the services behind the gateway;
  the chart mounts the Secret with `internalTls.secretName` or
  `internalTls.openshiftServingCert`.
  The Internal door lists the names the runtime resolves to it and refuses a
  certificate that carries none of them; the import dialog says so as soon as a PEM
  is pasted.
- **The internal certificate, asked of cert-manager.** A spare way on Kubernetes, the usual
  one staying a Certificate declared with the deployment: with
  `internalTls.certManager=true`, Add certificate, Ask cert-manager has the gateway create
  its own Certificate for its internal names, signed by the issuer picked there. Its
  Secret is read through the API and watched, so there is no mount and no restart, and a
  renewal is served on every node when cert-manager writes it.

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
