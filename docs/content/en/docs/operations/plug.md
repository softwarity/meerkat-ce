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
> hands out.

## Two conditions, two owners

The tunnel runs only when **both** of these are on:

| switch | where | whose | what it decides |
|---|---|---|---|
| Developer mode | Application, General | application admin | what this installation offers its developers: the Developer menu, the API docs, the simulated sign-ins |
| Open the developer tunnel | **Infra, Plug** | infrastructure admin | a port into the cluster, and the deployment's rights over it (Docker socket, Kubernetes role) |

The tunnel's own switch **ships off**. A gateway declared production
(`MEERKAT_PRODUCTION`) keeps the whole developer surface closed whatever either
switch says.

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

![The Plug screen: the tunnel switch, the published host and port, and the commands for a developer's machine](img/console/plug.webp)

The tunnel listens on **22222** in the container (`MEERKAT_PLUG_ADDR`).

- **Docker Compose**: publish it, `22222:22222`.
- **Kubernetes**: the Helm chart creates a `-plug` Service, ClusterIP by
  default. Set `plug.service.type` to `NodePort` or `LoadBalancer` for machines
  outside the cluster, then record that address on the page.
- **Docker Swarm**: not provided (see [One gateway](/docs/deploy/one-gateway)).

## On a developer's machine

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

## Taking an access back

Removing the key on the profile, the developer capability on the account, or
either switch closes the door at the next connection: nothing was issued that
would outlive it. Infra, Plug lists who holds the capability, with the
fingerprint of their key or the fact that they have none yet.
