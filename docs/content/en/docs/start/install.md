---
title: Install
section: Getting started
order: 4
summary: The Docker images, building the binary yourself, and the settings the gateway reads at startup.
---

# Install

Meerkat is one Go binary with no CGO and no external dependency. Its embedded
storage is a file in a directory; an external database is an option, not a
prerequisite.

## Docker image

Two images, built from the same commit:

| Image | Edition |
|---|---|
| `docker.io/softwarity/meerkat:latest` | community |
| `ghcr.io/softwarity/meerkat-ee:latest` | Enterprise (private registry) |

```yaml
services:
  meerkat:
    image: docker.io/softwarity/meerkat:latest
    ports:
      - "8080:8080" # data plane
      - "9090:9090" # control plane - keep this one internal
    environment:
      MEERKAT_ADMIN_PASSWORD: ${MEERKAT_ADMIN_PASSWORD:?set a password for the first admin}
    volumes:
      - meerkat-data:/data
    restart: unless-stopped

volumes:
  meerkat-data:
```

The image sets `MEERKAT_DATA=/data` and declares that volume. It runs as a
non-root user (uid 65532), and the entry point is the binary itself, so
anything after the image name is a flag.

## Build it yourself

What is published is the images; a binary is built from the source.

```bash
git clone https://github.com/softwarity/meerkat-ce.git
cd meerkat-ce
make ui      # builds the console and stages it for embedding (needs Node)
make build   # -> bin/meerkat
./bin/meerkat --help
```

`make ui` is what puts the console **inside** the binary. Skip it and the
gateway still routes, but the control plane answers a JSON status page instead
of a console.

Go comes from `go.mod`, Node from `.node-version`. That repository is the
community tree: the Enterprise sources are not in it, so what it builds is the
community edition. See [Editions](/product/editions).

## What it reads at startup

Every flag has a `MEERKAT_*` equivalent, and the flag wins. `./bin/meerkat
--help` prints the list; this is it.

### Ports

| Flag | Variable | Default | What |
|---|---|---|---|
| `-addr` | `MEERKAT_ADDR` | `:8080` | data plane, plain HTTP |
| `-admin-addr` | `MEERKAT_ADMIN_ADDR` | `:9090` | control plane, plain HTTP |
| `-tls-addr` | `MEERKAT_TLS_ADDR` | `:8443` | data plane HTTPS, opened when TLS is switched on |
| `-admin-tls-addr` | `MEERKAT_ADMIN_TLS_ADDR` | `:9443` | control plane HTTPS, same |

The HTTPS ports are opened and closed while the gateway runs, from the console.
A failure there is logged and does not stop the plain ports - a typo in an
address costs one door, not the installation.

### Storage

| Flag | Variable | Default | What |
|---|---|---|---|
| `-data` | `MEERKAT_DATA` | `data` | directory holding the embedded storage |
| `-database-url` | `MEERKAT_DATABASE_URL` | empty | PostgreSQL URL for several gateways sharing one installation |

> [!WARNING]
> The default for `-data` is **relative**. Under Docker the image already sets
> it to `/data`; anywhere else, give it an absolute path, or the database lands
> wherever the process happened to start.

> [!NOTE]
> Enterprise edition. The PostgreSQL driver is compiled into the Enterprise
> image only. The community binary has no driver registered, so
> `MEERKAT_DATABASE_URL` there answers with a sentence saying what is
> available rather than a driver error.

### First start only

| Variable | What |
|---|---|
| `MEERKAT_ADMIN_PASSWORD` | the first administrator's password, read only while there is no account at all |
| `MEERKAT_CONFIG_FILE` (`-config`) | a YAML or JSON configuration seeding an **empty** gateway; a configured one ignores it |
| `MEERKAT_VAULT_FILE` (`-vault`) | an encrypted vault file, ingested once |
| `MEERKAT_VAULT_PASSPHRASE`, `MEERKAT_VAULT_PASSPHRASE_FILE` | the passphrase for that file |
| `MEERKAT_TENANCY` (`-tenancy`) | `single` or `multi`, settled on the first start; afterwards the console owns it |

A gateway seeded from a configuration file does not get the demonstration
routes: the operator has said what this gateway serves.

`-tenancy` is for bootstrap - a first boot, a seeded install, GitOps. Once the
mode has been chosen the flag is ignored, with a line in the log saying so
rather than a silent override. Asking for `multi` on a community image starts
single-tenant, and says that too.

### Operating switches

| Flag | Variable | What |
|---|---|---|
| `-production` | `MEERKAT_PRODUCTION` | declares this gateway production: the whole developer surface stays closed whatever the stored settings say |
| `-plug-addr` | `MEERKAT_PLUG_ADDR` | developer tunnel listen address, default `:22222`, Enterprise only |
| `-console-url` | `MEERKAT_CONSOLE_URL` | development override: proxy the console to a front dev server |
| `-version` | - | print version and exit |

`MEERKAT_PRODUCTION` is an environment decision rather than a stored one for a
reason: the stored switch travels. A configuration export, a restored backup or
a database copied from staging can each carry developer mode into production,
and nobody notices until there is a tunnel port in front of customers.

## Probes

Both planes answer both probes.

| Path | Answers |
|---|---|
| `/healthz` | liveness - UP unconditionally, because liveness decides whether to kill the process |
| `/readyz` | readiness - 503 with a reason when the store is not answering, or when the routing table has not been compiled yet |

> [!WARNING]
> Point your load balancer at `/readyz`, never `/healthz`. A liveness probe
> that failed on an unreachable database would turn a database blip into a
> restart of every node at once.

## Signals

- `SIGHUP` reloads the routes.
- `SIGINT` and `SIGTERM` stop the gateway, draining for up to ten seconds.
