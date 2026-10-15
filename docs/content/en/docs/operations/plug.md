---
title: Plug
section: Operations
order: 228
summary: Open the developer tunnel, tell developers where it is, and what they run on macOS, Linux and Windows.
---

# Plug

[plug](https://github.com/softwarity/plug) runs a developer's local process as a
member of the cluster. Their service answers under its real name, and reaches
the others by theirs: no port-forward, no code change. A deployed service of the
same name is parked for the session and put back afterwards. What this is for,
and how everyone looking at the application is told who is serving it, is on
[Dev mode](/product/dev-mode). This page is how to open it and how to install it;
everything else plug does - profiles, several clusters at once, volumes,
environments - is in [plug's documentation](https://softwarity.github.io/plug/).

> [!NOTE] Enterprise edition
> The tunnel is built into the Enterprise image, and so is the plug client it
> hands out: plug **{{plug.version}}**.

## Two conditions, two owners

The tunnel runs only when **both** of these are on:

| switch | where | whose | what it decides |
|---|---|---|---|
| Developer mode | Application, General | application admin | what this installation offers its developers: the Developer menu, the API docs, the simulated sign-ins |
| Open the developer tunnel | **Infra, Plug** | infrastructure admin | a port into the cluster, and the deployment's rights over it (Docker socket, Kubernetes role) |

The tunnel's own switch **ships off**. These two switches decide, on a
production gateway as anywhere else.

While the tunnel is closed, a developer is not offered a key to deposit: the
page does not exist, and the user menu does not show it. A key for a door that
is not there is a key nobody can use.

The Infra, Plug page says what the tunnel is **doing**, not what was ticked:
listening on its port, closed and by which switch, or switched on but unable to
run here and why - a gateway that does not run in a container, for instance,
cannot provision a name in the cluster.

## The address developers use

The gateway cannot see what sits between it and a laptop: a NodePort, a
LoadBalancer, a published Docker port. So Infra, Plug records the **published**
host and port, and every command on that page and on the developer's profile
page carries them, ready to copy.

![The Plug screen: the tunnel switch, the published host and port, and who holds the developer capability](img/console/plug.webp)

The tunnel listens on **22222** in the container (`MEERKAT_PLUG_ADDR`).

- **Docker Compose**: publish it, `22222:22222`.
- **Kubernetes**: the Helm chart creates a `-plug` Service, ClusterIP by
  default. Set `plug.service.type` to `NodePort` or `LoadBalancer` for machines
  outside the cluster, then record that address on the page.
- **Docker Swarm**: not provided (see [One gateway](/docs/deploy/one-gateway)).

## On a developer's machine

On Infra, Plug, the **On a developer's machine** button opens these steps in a
drawer, for macOS, Linux or Windows, with the commands already carrying this
gateway's address. It is help, not configuration: nothing in it is saved.

![The drawer with the steps for a developer's machine: install, name the profile, the key pair, deposit it, plug a service in](img/console/plug-machine.webp)

A developer needs the **developer capability** on their account (Application,
Users). Then, once:

**1. Install plug.** It installs from the gateway itself, not from a package
manager, and prepares the machine once, so that later runs need no privilege.

macOS and Linux, from a terminal (it may ask for your password once):

```sh
ssh -p 22222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null get@<gateway-host> install | sh
```

Windows, from **Git Bash** (it comes with Git for Windows; it asks once to run
as Administrator):

```bash
ssh -n -p 22222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null get@<gateway-host> install-windows | bash -s -- <gateway-host> 22222
```

The installer creates a profile named after the host, which is what `-p` names
below. A name of your own reads better once a second cluster is installed, and
the key pair moves with it:

```sh
plug rn <gateway-host> my-cluster
```

`<profile>` below is that name: `my-cluster` once renamed, `<gateway-host>`
otherwise.

The commands on the Infra, Plug page follow the address and the profile name
typed there.

**2. Generate the key pair.** plug keeps it under `~/.plug/keys`, one pair per
cluster, so a key can be withdrawn from one cluster without touching the others.

```sh
plug keygen -p <profile>
plug pubkey -p <profile> | pbcopy                        # macOS
plug pubkey -p <profile> | xclip -selection clipboard    # Linux
plug pubkey -p <profile> | clip                          # Windows, Git Bash
```

**3. Deposit the public key** on your profile, signed in to the applications:
`/profile/dev/key`, or from any application, the user menu, Developer, plug key.
The page shows the SHA256 fingerprint, to compare with what `plug pubkey`
printed. It takes a public key, never a certificate.

One key per workstation: plug keeps a pair per profile, so a second machine
runs `plug keygen` there and deposits a second key beside the first. Each is
listed with its fingerprint and the comment it carries, and removing one closes
that machine alone. A key belongs to one account: the same key deposited twice,
or by somebody else, is refused.

**4. Plug a service in**, by prefixing the command that runs it:

```sh
plug -p <profile> -s my-service:8080:3000 npm run start
```

The cluster reaches it as `my-service:8080`, forwarded to your local `3000`,
for as long as the command runs.

## Beside Argo CD, Flux and other GitOps controllers

A cluster driven by a continuous-deployment controller - Argo CD, Flux, Rancher
Fleet, Crossplane - keeps every object equal to what a repository declares, and
undoes whatever differs. `-s` does the opposite on purpose: for the session, it
changes the Service's `spec.selector` and `spec.ports` so the name points at the
developer's machine. The controller sees drift and puts it back, usually within
minutes. Consuming the cluster's services is not affected: only `-s` is.

Nothing fails on either side, which is what makes it easy to miss. When the
deployed workload is running, the name goes back to it: the application
answers, the page loads, and it is simply not the developer's code answering.
A session that works for a few minutes and then stops reflecting changes is
the first sign. plug checks the path in the background and warns: the name *is
answered by something else*, or *is not reachable inside the cluster* when
nothing is deployed.

Who manages a Service is in its metadata (`kubectl describe svc <name>`):

| Controller | What to look for |
|---|---|
| Argo CD | annotation `argocd.argoproj.io/tracking-id` |
| Flux | `kustomize.toolkit.fluxcd.io/name`, `helm.toolkit.fluxcd.io/name` |
| Rancher Fleet | `fleet.cattle.io/bundle-name` |
| Crossplane, operators | `crossplane.io/composite`, or an `ownerReferences` entry with `controller: true` |

`app.kubernetes.io/managed-by: Helm` alone means nothing here: Helm acts on an
upgrade and does not reconcile, so a Service deployed by a chart can be taken.

**Argo CD**: tell the Application not to compare the two fields plug changes;
everything else about the Service keeps being reconciled. This goes in the
Application, not in the chart:

```yaml
spec:
  ignoreDifferences:
    - group: ""
      kind: Service
      name: my-service
      namespace: my-namespace
      jsonPointers:
        - /spec/selector
        - /spec/ports
```

On a cluster where plug is used routinely, set it once for every Service in the
`argocd-cm` ConfigMap:

```yaml
data:
  resource.customizations.ignoreDifferences._Service: |
    jsonPointers:
      - /spec/selector
      - /spec/ports
```

Turning Self Heal off also works for a one-off, but it suspends drift
correction for the whole Application, and it is easy to forget to turn back on.

**Flux** takes the instruction on the resource itself, so the chart is the place:
the annotation `kustomize.toolkit.fluxcd.io/reconcile: disabled` on the Service.
It stops reconciling the whole object, not two fields.

**Rancher Fleet** compares with `diff.comparePatches` in the bundle's
`fleet.yaml`, removing `/spec/selector` and `/spec/ports`. **Crossplane**: an
Object switched to `managementPolicies: ["Observe"]` is no longer re-applied.
An in-house operator rarely offers an equivalent: suspend it for the session, or
plug in a name it does not own.

> [!WARNING] End a session with Ctrl-C
> Ctrl-C is what puts the Service back, from a receipt plug stored on it. A
> session killed with `kill -9` leaves it parked until the agent's sweep, once a
> minute, finds the session gone and restores it: the Helm chart grants the
> `deployments` right that sweep needs. Under a controller, a Service can still
> keep an extra `app: plug` key in its selector, which the controller does not
> remove since it only manages the fields it knows: the Service then selects no
> pod, and the name answers nobody. If it still does after a couple of minutes,
> remove that key by hand. The chart also grants `endpoints`, so the agent
> writes the Endpoints itself when a selector cannot be repointed, rather than
> falling back to that key.

The details, and the other symptoms, are in plug's documentation:
[Continuous deployment & GitOps](https://softwarity.github.io/plug/#/continuous-deployment).

## Taking an access back

Removing the key on the profile, the developer capability on the account, or
either switch closes the door at the next connection: nothing was issued that
would outlive it. Infra, Plug lists who holds the capability, with the
fingerprint of their key or the fact that they have none yet.
