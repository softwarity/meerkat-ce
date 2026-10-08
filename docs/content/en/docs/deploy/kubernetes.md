---
title: Kubernetes cluster
section: Deploying
order: 235
summary: Several gateways serving one installation: the Deployment, the two Services, the Ingress, and how not to turn the entry point into a single point of failure.
---

# Kubernetes cluster

Several gateways serving one installation, in front of the same applications. The whole shape follows from one property of the product: the nodes never talk to each other. All the coordination goes through the database they share, which is what lets a Kubernetes deployment be an ordinary one.

What you deploy is four objects and one dependency: a Deployment of three replicas, two ClusterIP Services (one per plane), an Ingress in front of the public one, and one external PostgreSQL. No StatefulSet, no headless Service, no PersistentVolumeClaim, no sidecar, no Redis.

The Helm chart in deploy/helm installs the single-gateway shape by default: one replica, one volume, Recreate. A cluster changes three things - no volume, a database URL, several replicas - which `values-ee-cluster.yaml` sets; they are also written out in full below rather than hidden behind values.

## Why a Deployment, and not a StatefulSet

A StatefulSet exists to give pods a stable name, a stable order and a volume of their own, so that they can find each other and keep their own state. Meerkat needs none of the three. A node never addresses another node: when one writes a route, it bumps a version in the database and sends a NOTIFY, and the others hear it on the connection they are already holding. That is PostgreSQL LISTEN/NOTIFY, and it is the reason PostgreSQL is the one external option - it was chosen for the transport, not for the storage.

So there is no peer-to-peer, no gossip, no Raft, no quorum, no seed list, and no node that has to learn another node's address. Nothing discovers pods. Scaling is one number, and a pod is interchangeable with any other.

The one thing a per-pod volume would carry is the embedded storage - and a database per pod is precisely what a cluster is not.

## No session affinity to ask for

Sessions live in the database, not in a pod. Each node keeps a five-second cache to answer fast, and that cache is invalidated through the same bus, so a revocation or an organisation just chosen on node A is not served stale by node B. A request can therefore land on any replica, which is what makes a rolling update uneventful: a pod that goes takes no session with it.

The brute-force counter is in the database too, one row per failed attempt, and that matters more than it looks. A limit of five attempts used to be five PER GATEWAY: an attacker spraying the load balancer got the limit multiplied by the very thing that was supposed to make the service sturdier. Five attempts are now five for the installation wherever they land, and a successful sign-in forgives on every node. A test says exactly that, and it is the reason nothing here asks your load balancer for sticky sessions.

What is relayed rather than shared: a live message published on one node reaches the pages held open by the others, so nobody sees half an announcement. What stays per node, and is worth knowing before you set a number: rate limits count in memory, per node, so N nodes allow up to N times the bound - the console says so where the number is typed. Two developer tools are per process for the same reason: the UI test mode and the tokens the embedded Swagger uses live in the node that issued them, so a simulated identity can jump from one request to the next across several nodes. That is said rather than fixed: it is a tool for one operator looking at one screen, and making it cluster-wide would be machinery for nobody.

## The prerequisite: one PostgreSQL they share

The embedded storage is a file one process owns. Point three pods at it and you have three independent gateways: three sets of routes, three sets of accounts, three admin passwords, and a console that shows whichever one your request happened to reach. Nothing would look broken from the binary side, which is what makes it a bad afternoon.

A cluster therefore starts with MEERKAT_DATABASE_URL pointing at a PostgreSQL every node reaches. It is the single external option, and there is nothing else to run: the notifications are the bus, and an advisory lock is the exclusion the product needs. No Redis, no message broker, no etcd.

> [!NOTE]
> This is an Enterprise capability, and the driver is what the edition is: the community binary links no PostgreSQL driver at all, so a database URL there gets a sentence naming what is available instead of a driver error. Absent code refuses on its own, and no licence is read.

## The data directory, and the one thing that is not in the database

The -data directory is still created with an external database, but it is no longer the source of truth: routes, accounts, sessions, audit trail and certificates are all rows. An emptyDir is enough, which is why there is no PersistentVolumeClaim here and therefore no ReadWriteOnce volume to hand from one machine to another.

> [!WARNING]
> One file is the exception, and it is the one that bites: the vault master key. It seals what is IN the database, so keeping it there would be sealing the door with the key in the lock. Left to itself, each pod generates its own key in its own emptyDir - and then a secret sealed by one node cannot be read by the others, which surfaces as an upstream password that works on one replica in three.

So in a cluster the key is supplied by the environment, identical on every pod, out of the same Secret. Generate it once and keep it wherever you keep keys - losing it loses every secret in the vault.

## The Secret

Three values, created rather than committed. A manifest with a base64 field is a secret in git, and base64 is not encryption.

```bash
# Three values, and none of them belongs in git.
kubectl create secret generic meerkat \
  --from-literal=database-url='postgres://meerkat:PASSWORD@postgres.data.svc:5432/meerkat?sslmode=require' \
  --from-literal=admin-password='the-first-admin-password' \
  --from-literal=vault-key="$(openssl rand -hex 32)"
```

MEERKAT_ADMIN_PASSWORD is read on the FIRST start only, to create the first admin account: whichever pod gets there first creates it, the others find it already there. Once you have signed in and made your own account, it can come out of the Secret.

## The Deployment

Both probes exist on both ports, and they are not interchangeable. /healthz is LIVENESS and answers UP unconditionally: this probe decides whether to kill the process, and a liveness probe that failed because the database was unreachable would turn a database blip into a restart of every node at once, each killed for a fault none of them has and none can fix by dying. /readyz is READINESS: it pings the store and checks that the routing table has been compiled, and answers 503 naming which of the two is missing - a node that accepted connections but has not compiled its table yet answers 404 to everything, and a rolling update would read that as a healthy instance.

The rule that follows: whatever sends traffic probes /readyz, and never /healthz.

::: details The full Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meerkat
  labels:
    app.kubernetes.io/name: meerkat
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      # Nothing local to hand over, so the new pod can be serving before the
      # old one leaves: no session is lost and no configuration is missed.
      maxUnavailable: 0
      maxSurge: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: meerkat
  template:
    metadata:
      labels:
        app.kubernetes.io/name: meerkat
    spec:
      # Three pods on one machine are one machine away from no gateway at all.
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: meerkat
      containers:
        - name: meerkat
          # The external database is an Enterprise capability: the community
          # image links no PostgreSQL driver at all. Pin a release rather than
          # riding "latest" on something that answers your front door. To try
          # the cluster first: docker.io/softwarity/meerkat:eval, public, the
          # Enterprise image with an evaluation notice - not for production.
          image: ghcr.io/softwarity/meerkat:latest
          ports:
            - name: app
              containerPort: 8080
            - name: admin
              containerPort: 9090
          env:
            # The shared database. This one line is what makes three pods one
            # gateway instead of three gateways.
            - name: MEERKAT_DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: database-url
            # The vault master key, THE SAME on every pod. It seals what is in
            # the database, so it is not kept there. Leave it out and each pod
            # generates its own, then reads none of the others' secrets.
            - name: MEERKAT_VAULT_KEY
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: vault-key
            # Read ONCE, on the first start, to create the admin account.
            - name: MEERKAT_ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: admin-password
            # This gateway is a production one: the developer surface stays
            # closed here whatever the database says.
            - name: MEERKAT_PRODUCTION
              value: "1"
          # STARTUP: the store is opened and migrated before the ports
          # open. Up to two minutes for that, before liveness counts.
          startupProbe:
            httpGet:
              path: /healthz
              port: app
            periodSeconds: 2
            failureThreshold: 60
          # LIVENESS: is this process still serving? It answers UP without
          # looking at anything, and that is the right answer - this probe
          # decides whether to KILL the process.
          livenessProbe:
            httpGet:
              path: /healthz
              port: app
            periodSeconds: 10
          # READINESS: can this node take a request right now? It checks the
          # store and the compiled routing table, and answers 503 naming which
          # of the two failed. This is the one a load balancer must probe.
          readinessProbe:
            httpGet:
              path: /readyz
              port: app
            periodSeconds: 5
            timeoutSeconds: 3
          volumeMounts:
            - name: data
              mountPath: /data
          resources:
            requests:
              cpu: 200m
              memory: 128Mi
            limits:
              memory: 512Mi
          securityContext:
            runAsNonRoot: true
            seccompProfile:
              type: RuntimeDefault
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
      volumes:
        # With a shared database this directory holds nothing that has to
        # outlive the pod: no PersistentVolumeClaim, and therefore no
        # ReadWriteOnce volume to hand from one machine to another.
        - name: data
          emptyDir: {}
```
:::

## Two Services, because there are two planes

Port 8080 is the data plane - your applications, the sign-in pages, the live channel. Port 9090 is the control plane - the admin console, the admin API and the MCP endpoint an agent talks to. Only the first belongs on the internet, and they are two Services for exactly that reason: an Ingress in front of your applications must not be able to reach the console by accident, and a single Service with two ports is one annotation away from doing so.

::: details Both Services
```yaml
# The data plane: what your users reach.
apiVersion: v1
kind: Service
metadata:
  name: meerkat
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: meerkat
  ports:
    - name: app
      port: 8080
      targetPort: app
---
# The control plane: console, admin API and the MCP endpoint. A SECOND
# Service, on purpose: an Ingress put in front of the applications has then no
# way of reaching the console by accident.
apiVersion: v1
kind: Service
metadata:
  name: meerkat-admin
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: meerkat
  ports:
    - name: admin
      port: 9090
      targetPort: admin
```
:::

How an operator reaches the console then, in order of preference: kubectl port-forward service/meerkat-admin 9090:9090 for occasional work; a second Ingress on an internal-only controller, or on the same one restricted by source address and client certificate, when a team needs it daily. What you should not do is put it on the same public host as the applications.

## The Ingress

Two things a default Ingress gets wrong for this gateway. The gateway proxies WebSockets, and it holds a live channel open on /meerkat/events - both are long-lived responses, so a controller that buffers the body or closes the connection at its default sixty seconds breaks pages that were working, intermittently and only for some users.

And the scheme. Meerkat can terminate TLS itself (-tls-addr, -admin-tls-addr, with ACME issuance serialised by an advisory lock so N nodes ask for one certificate rather than N, the material and the challenge being rows every node can answer from). In Kubernetes you usually terminate at the Ingress instead, and both work. But whatever terminates in front must send X-Forwarded-Proto: the gateway reads the scheme from it as much as from the connection, and without it three things go wrong at once - session cookies lose their Secure attribute, the console announces http:// URLs, and the redirect to HTTPS loops for ever. nginx and Traefik both set it by default; anything hand-rolled in front has to be checked.

::: details The full Ingress
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: meerkat
  annotations:
    # The gateway proxies WebSockets and holds a live channel open
    # (/meerkat/events). Both are long-lived responses: an Ingress that
    # buffers them, or closes at its default 60 s, breaks pages that worked.
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-buffering: "off"
spec:
  ingressClassName: nginx
  tls:
    - hosts: ["apps.example.com"]
      secretName: meerkat-tls
  rules:
    - host: apps.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: meerkat
                port:
                  name: app
# Nothing here points at meerkat-admin, and that is the point.
```
:::

## What sits in front

A ClusterIP Service is only reachable from inside the cluster: something in
front has to carry the address your users type. That address then becomes what
can take the installation down - three replicas behind a name that resolves to
one machine is a one-machine cluster with extra steps.

On a managed cluster (EKS, GKE, AKS): `type: LoadBalancer` on the data-plane
Service, and the provider hands back a stable, load-balanced address.

On bare metal that provider does not exist and `type: LoadBalancer` stays
pending. MetalLB (in L2 or BGP mode) or a keepalived VIP in front of an Ingress
controller answer that need. It is an infrastructure decision, not a Meerkat
setting: the gateway is reached the same way whichever answer you pick.

> [!NOTE] Traefik only replaces half of it
> Traefik - like nginx, HAProxy or Envoy - **is** an Ingress controller: it
> replaces the Ingress object above, and you would write an IngressRoute
> instead. But its pods are pods: on bare metal it needs exactly the same
> highly available entry address. Two Traefik replicas do not give you two
> entry addresses.

## The fragile part is the database

Replicas make the gateway survive a machine. They do nothing for the thing all of them read: there is one database, and no number of pods makes it two. Once the entry address can move, the database is the single point of failure of the installation, and that is where the remaining work belongs.

What happens when it goes is at least honest. /readyz answers 503 with the reason - the store is not answering - so Kubernetes takes those pods out of the Service endpoints while /healthz keeps saying UP and nothing gets restarted. The gateway stops serving rather than serving wrong answers, and it comes back by itself when the database does: no restart, no manual step, nothing to unwedge.

So point `MEERKAT_DATABASE_URL` at a name that follows the primary rather than
at a host, and measure your database's failover window: that window is the one
in which no node is ready.

## Optional: let the gateway see the namespace

The gateway reads its OWN namespace for two things. The route editor offers its Services, so an upstream becomes a pick rather than a typed URL. And the Routes list shows, for each service, how many of its replicas are ready and which image they run - green when all are, orange when some are, red when none is - kept current by the API server's events rather than by asking on a timer (see [Upstream health](/docs/operations/upstream-health)).

There is no switch for either: what opens them is what the deployment grants. The chart grants both (`rbac.read` and `rbac.watch`, on by default). By hand, the ServiceAccount needs `list` and `watch` on services and pods in its own namespace and nothing else - no secret, no other namespace. Without the pods, the editor still offers the Services and the targets are checked by a connection; without anything, the console says which right it lacks while free typing stays exactly as it was.

A route whose upstream lives in ANOTHER namespace (`grafana.monitoring.svc`) is read the same way once that namespace is listed in `rbac.watchNamespaces`: the chart creates a Role in each one named, never a ClusterRole, so the gateway reads the namespaces somebody chose and not the whole cluster. A namespace not listed is checked by a connection.

Reading the pods reads the environment variables their manifests declare. Where a manifest carries a secret in clear, set `rbac.watch: false`, or better, move the secret into a Secret.

The developer tunnel asks for more than this, and what it grants is set out on [One gateway](/docs/deploy/one-gateway).

::: details The Role and its binding
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: meerkat
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: meerkat-services
rules:
  # Read the Services and pods of THIS namespace, and nothing else: no
  # secret, no other namespace. watch is what keeps the routes screen current
  # without asking on a timer.
  - apiGroups: [""]
    resources: ["services", "pods"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: meerkat-services
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: meerkat-services
subjects:
  - kind: ServiceAccount
    name: meerkat
# And name it in the pod spec:
#   serviceAccountName: meerkat
```
:::

## OpenShift and OKD

The same image and the same chart install on OpenShift and OKD under the default
`restricted-v2` SCC: no extra SCC to grant, no value to change, nothing run as root.
Three choices make that true, and they are worth knowing before overriding anything:

- **No uid in the pod.** OpenShift does not run the image's user: it draws a uid from
  the namespace's range, adds group 0, and rejects a pod that names its own -
  `runAsUser: Invalid value: 65532: must be in the ranges: [...]`. So neither the chart
  nor the manifest above sets `runAsUser`, `runAsGroup` or `fsGroup`. **Do not add them
  back** in your values: it buys nothing anywhere else, since the image already carries
  its uid.
- **`/data` belongs to the root group, and the group writes.** Whatever uid OpenShift
  picks, it is in group 0, so the store can write its first file even where `/data` is
  a plain directory rather than a mounted volume.
- **The image's `USER` is numeric (65532).** On a plain Kubernetes namespace enforcing
  Pod Security `restricted`, the kubelet refuses an image whose user is a name, since it
  cannot prove it is not root. A number passes both there and on OpenShift.

Everything else already fits: every port the gateway listens on is above 1024, it
needs no capability, no privilege escalation and no host path, and the read-only access
the route editor asks for (above) is an ordinary Role.

**Exposing it.** A **Route** replaces the Ingress, one per plane, with the same rule:
nothing points at the control plane unless you mean it to.

```yaml
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: meerkat
spec:
  host: apps.example.com
  to:
    kind: Service
    name: meerkat
  port:
    targetPort: app
  tls:
    # edge: the router holds the certificate and Meerkat sees HTTP with
    # X-Forwarded-Proto: https. passthrough instead when the gateway holds the
    # certificate itself - which ACME needs, since the TLS-ALPN challenge must
    # reach Meerkat untouched.
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
```

> [!NOTE] Checked every night
> The above is not a promise: every night the CI installs this chart on MicroShift
> (OpenShift built from OKD), under the `restricted-v2` SCC, and checks the uid it
> was given, the write to `/data` and both Routes. See the
> [test platform](/project/test-platform).

## Before calling it done

- Whatever sends traffic probes /readyz, not /healthz.
- MEERKAT_VAULT_KEY is set, and is the same value on every pod.
- Port 9090 is published nowhere: no Ingress rule, no LoadBalancer, no NodePort.
- X-Forwarded-Proto arrives, and the long-lived responses are neither buffered nor cut.
- More than one machine can answer the public address, and you have seen it happen.
- The database has a failover path, and the restore has been tried.
- Kill one pod during a load test and watch the figures: this is the cheapest rehearsal you will get.

One gateway on one volume is a simpler shape, and it has its own page: [One gateway](/docs/deploy/one-gateway).
