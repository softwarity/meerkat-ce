---
title: Test platform
section: Quality
order: 33
summary: How the image proves itself where it will be deployed: compose, Swarm, Kubernetes, one instance and a PostgreSQL cluster, community and Enterprise.
---

# Test platform

Today's suites prove the **code**: 1,137 Go tests, a PostgreSQL suite, three real
directories (Dex, OpenLDAP, Samba AD) and 87 Playwright scenarios played against a
binary built from the tree. None of them proves the **image as shipped**, deployed by
**the files we publish**, on the targets it really runs on. The test platform closes
that gap. What each feature has to prove there is on [Test plan](/project/test-plan);
this page says how.

## The principle

1. **Test the image, not the tree.** One image per edition is built once, under an
   immutable tag (`sha-<short>`), and that digest is what goes to every target. The
   moving tags are set only once everything is green, and the verdict is read from
   marker files, not from `needs`: a skipped job satisfies a `needs`.
2. **Deploy with what we publish.** `deploy/docker-compose.yml`, `.ee.yml`,
   `stack.swarm.yml` and the Helm chart with its three values files, rewritten only
   for the image tag, and the rewrite is asserted before anything is applied. A copy
   made by hand for the tests drifts: plug paid for that over eight releases.
3. **One suite, several targets.** The Playwright suite and `scenarios.json` stay the
   source; the platform points them at a deployed gateway instead of starting one.
   What is not a scenario (killing a node, upgrading) is a **cell**: a script, a
   name, a step of its own in the job, red or green.
4. **Community and Enterprise play the same file.** Each edition expects its own
   answers: Enterprise succeeds where community answers 422 or shows the lock. An
   Enterprise feature that works on the community image is a failure, just like a
   community feature that breaks.

## The targets

| | Target | Topology | Editions | Database |
|---|---|---|---|---|
| `C1` | docker compose | one gateway + the test services | CE, EE | SQLite (volume) |
| `CC` | docker compose cluster | 3 gateways behind Traefik | EE | PostgreSQL 17 |
| `S1` | Docker Swarm | one replica, `stack deploy` | CE, EE | SQLite |
| `SC` | Docker Swarm | 3 replicas | EE | PostgreSQL |
| `K1` | kind + Helm | `values-ce-one-node` / `values-ee-one-node` | CE, EE | SQLite (PVC) |
| `KC` | kind + Helm | `values-ee-cluster`, 3 replicas | EE | PostgreSQL |
| `KO` | kind under OpenShift's rules | `values-*-one-node` unchanged, Pod Security `restricted`, arbitrary uid + group 0 | CE, EE | SQLite (PVC) |
| `OK` | MicroShift (OKD) in a container | the real `restricted-v2` SCC, exposed by Routes | CE, EE | SQLite (emptyDir) |

Swarm and kind run on the runner's single node (`swarm init --advertise-addr
127.0.0.1`, kind with `extraPortMappings` to fixed host ports): what is tested is the
orchestrator and our files, not a multi-host network. kind is pinned and checked by
SHA256, the image loaded with `kind load` and `imagePullPolicy: Never`; each
deployment is awaited by its own `kubectl rollout status`, and `describe` plus the
logs are dumped on failure.

### OKD / OpenShift

OpenShift does not start a pod under the image's uid: the `restricted` SCC draws one
from the namespace's range, with group 0, and rejects a pod that names its own. That
is why the chart sets neither `runAsUser` nor `fsGroup`. Two ways to prove it, from
the cheapest to the most faithful:

1. **`KO`, on kind, every night**: the namespace labelled
   `pod-security.kubernetes.io/enforce=restricted`, and a test overlay doing what the
   SCC does (uid `1000680000`, group 0). If the gateway writes `/data`, migrates,
   serves and hands out the snapshot, it will run on OpenShift. Plus a static check of
   the rendered chart on every push: no pinned uid, nothing privileged, no port below
   1024.
2. **`OK`, MicroShift, every night**: OpenShift's small distribution, built from OKD,
   one node in a privileged container - it fits a standard GitHub runner. It is what
   kind cannot imitate: the `restricted-v2` SCC that ADMITS the pod and hands it its
   uid, and the router behind Routes. `e2e/platform/okd/run.sh <image> <ce|ee>`
   installs the published chart there, checks the SCC, the uid, the write to `/data`,
   both planes behind their Routes, and - on Enterprise - a mount helper as plug
   creates it, with a real SMB client. The `okd.yml` workflow plays it for both
   images. One thing is turned off: the volume claim, because MicroShift's storage
   driver wants an LVM volume group on the host.

## The test services

Everything lives on the platform network, nothing is reached on the internet: a
verdict about somebody else's website is not one about the product.

| Service | Image | For |
|---|---|---|
| httpbin | `mccutchen/go-httpbin` | the routes' upstream: headers, delays, statuses, streams |
| WebSocket and gRPC echo | a small Go binary in `e2e/platform/services` | ROUTE-13, ROUTE-20 (unary and streaming, h2c and TLS) |
| Mailpit | `axllent/mailpit` | the SMTP sink, read through its HTTP API (e-mail code, forgotten password, digest) |
| Dex | `dexidp/dex` | OIDC |
| OpenLDAP, Samba AD | the Enterprise CI's images | LDAP / AD and group rules (EE) |
| PostgreSQL | `postgres:17` | the cluster targets |
| Traefik | `traefik` | the load balancer in front of the compose cluster, and X-17's reverse proxy |
| Pebble + challtestsrv | `ghcr.io/letsencrypt/pebble` | ACME (EE), see below |
| Gitea | `gitea/gitea` | git locations (EE) |
| OpenTelemetry Collector | `otel/opentelemetry-collector-contrib`, file exporter | audit, traces, metrics (EE) |

## The phases

Each phase delivers something that runs and already catches regressions; the next
one starts only once the previous one has been green two nights in a row.

| Phase | Delivers | Targets | Estimated cost |
|---|---|---|---|
| **0. A target-independent suite** | `DATA_URL`, `ADMIN_URL`, the root password and the mail sink read from the environment; the `webServer` entries skipped when a target is given; a seed that no longer assumes a local disk | `I` | half a day |
| **1. The image on compose** | `e2e/platform/compose/` (test services as an overlay on the published files), cells `smoke`, `edition`, `matrix` (the whole suite against the image), community and Enterprise | `C1` | two days |
| **2. The cluster** | 3 gateways + PostgreSQL + Traefik; cells `sticky-free` (a session on A, served by B), `bus` (a route saved on A, served by C), `once` (schedules, digest, purge), `kill-node`, `pg-restart`, `vault-key-mismatch` | `CC` | three days |
| **3. Kubernetes** | kind + chart, the three values files; `helm upgrade --reuse-values`, rolling update under traffic, Service discovery; OpenShift's rules on kind and the static chart check | `K1`, `KC`, `KO` | four days |
| **4. Swarm** | `stack deploy` of the published file, secrets and configs, convergence, start-first update | `S1`, `SC` | one day |
| **5. Upgrade** | the previous release found (not pinned), deployed and populated, then replaced by the commit's image; a reference database per release; the downgrade refused | `C1`, `CC`, `K1` | two days |
| **6. The Enterprise integrations** | ACME against Pebble, git against Gitea, OpenTelemetry to the Collector | `C1`, `CC` | three days |
| **7. Soak and arm64** | 4 hours of mixed traffic every week; the smoke suite on a native arm64 runner | `CC`, `C1` | two days |

### Why phase 0 comes first

`e2e/playwright.config.ts` hard-codes `localhost:18082` and `localhost:19092`, starts
the gateway, httpbin and the SMTP sink, and the sign-up flow reads its mail from
`.tmp/mail` on the disk. As long as that holds, the suite can only aim at a local
process. Once those four points are read from the environment, the same suite,
without one more line, plays against any target: most of the value for the least
work.

## ACME, the hardest phase

Pebble is Let's Encrypt's test ACME server: it issues real certificates signed by a
throwaway CA, and `pebble-challtestsrv` acts as its DNS. Three obstacles, all of them
passable:

1. **Trust.** Pebble's directory is HTTPS under its own CA. The gateway has to trust
   it: `SSL_CERT_FILE` on the container is enough for Go on Linux, with no change to
   the product.
2. **The challenge.** Meerkat validates with TLS-ALPN-01, so Pebble has to reach the
   application's HTTPS door **on port 443** for every name asked for. challtestsrv
   resolves those names to the gateway, and the target publishes the application on
   443 inside the platform network (`PEBBLE_VA_ALWAYS_VALID=1` lets the cell be
   written before the network is right; then it comes off).
3. **The cluster.** Three nodes, one order: the certificate must be issued **once**,
   under the advisory lock, and served by all three. That is the test that matters,
   and the one no unit test can do.

The cell goes: save the Pebble authority, create an order placed on the application,
wait for the issued status, read the certificate each node serves, then force a
renewal (short lifetime on Pebble's side). On the community image, the same cell
checks the refusal.

## CI

| When | What | Where |
|---|---|---|
| every push | phase 0 and the `C1` smoke, both editions | `ci.yml` (meerkat-ce) and `ci-ee.yml` |
| every night on `main` | every target, the whole suite, the upgrade | a scheduled `platform.yml`, also dispatchable |
| before a release | the previous night must be green, or it is run again | the `ci-ee.yml` gate reads the markers |
| every week | the 4-hour soak | `soak.yml` |

What we keep from plug:

- each cell is a step of the job, with its own red;
- a script checks that the order of the cells, their files and the job names agree,
  and that none carries `continue-on-error`;
- a skipped cell is counted and raised as a warning, never shown as a pass;
- every call is bounded by a watchdog capped at the job's remaining time (a job
  killed by its timeout loses its log);
- an `abort-on-fail` job cancels the rest as soon as one leg fails, so the red stays
  red instead of turning "cancelled";
- log in to Docker Hub even to pull, because of the rate limits.

What we do not take: the tailnet. Meerkat's targets run inside the very job that
tests them; there is no client on other systems to connect.

## The layout in the repository

```
e2e/platform/
  plan.json            the plan, one entry per FEATURES.md row + the X-xx checks
  run.sh               <target> <edition> <cell...>
  lib.sh               waits, bounds, job summary
  cells/               one cell per file: smoke.sh, edition.sh, kill-node.sh...
  compose/             overlays on the published files: test services, cluster
  kind/                kind-config.yaml, test values
  swarm/               stack overlay
  services/            WebSocket and gRPC echo
```

## Keeping the plan true

`plan.json` is maintained beside `FEATURES.md`. **A feature added there adds its
entry here in the same commit**; if it is forgotten, the [Test plan](/project/test-plan)
page lists it at the bottom, under "In FEATURES.md and not in the plan yet". The
"Tests naming it today" column is read from the test sources: a feature no test names
is a feature nobody has checked, which is exactly what it is there to show.
