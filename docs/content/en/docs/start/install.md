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

Three images, built from the same commit and carrying the same name: the
**registry** and the tag are what say the edition.

| Image | Edition |
|---|---|
| `docker.io/softwarity/meerkat:latest` | community, public |
| `docker.io/softwarity/meerkat:eval` | evaluation, public: everything Enterprise does, with an evaluation notice - not for production |
| `ghcr.io/softwarity/meerkat:latest` | Enterprise, private registry |

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

## Verify the signature

All three images are signed when they are built, with
[cosign](https://docs.sigstore.dev/) and no key. There is no public key to go
and fetch: the signature carries the identity of the GitHub Actions workflow
that produced the image, and that identity is what you check.

```bash
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/softwarity/meerkat-ce/\.github/workflows/' \
  docker.io/softwarity/meerkat:latest
```

The same command verifies the evaluation image, `docker.io/softwarity/meerkat:eval`,
and the Enterprise image at `ghcr.io/softwarity/meerkat` once you are logged in
to that registry. All three are built by workflows living in
the public mirror, `softwarity/meerkat-ce`, which is the repository the
identity names - the Enterprise sources are private, the pipeline that builds
them is not. The command prints the exact identity it accepted, so a policy
that wants to pin one file rather than a directory can read it there.

What is signed is the **digest**, and recursively: the manifest list plus each
per-architecture image under it. Two consequences worth knowing. A client that
pulled the arm64 image verifies what it is actually running, not its sibling.
And a release, which re-tags an already tested digest under its version number
without rebuilding anything, carries the signature with it: nothing is
re-signed because nothing is rebuilt.

To refuse an unsigned image across a cluster rather than check one by hand, the
same two values go into a Kyverno policy:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: meerkat-signed
spec:
  validationFailureAction: Enforce
  rules:
    - name: verify-meerkat
      match:
        any:
          - resources:
              kinds: [Pod]
      verifyImages:
        - imageReferences:
            - "docker.io/softwarity/meerkat*"
            - "ghcr.io/softwarity/meerkat*"
          attestors:
            - entries:
                - keyless:
                    issuer: https://token.actions.githubusercontent.com
                    subject: "https://github.com/softwarity/meerkat-ce/.github/workflows/*"
```

Check `validationFailureAction` against your own Kyverno version: recent ones
moved that switch into the rule, and a policy that lands in audit mode reports
instead of refusing.

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
| `MEERKAT_CONFIG_FILE` (`-config`) | a YAML or JSON configuration seeding an **empty** gateway. A configured one does not apply it: when the file differs from what runs, it is shelved as a **saved configuration** (named after the file), to compare and set as current from the Configuration screen |
| `MEERKAT_VAULT_FILE` (`-vault`) | an encrypted vault file, ingested once |
| `MEERKAT_VAULT_PASSPHRASE`, `MEERKAT_VAULT_PASSPHRASE_FILE` | the passphrase for that file |
| `MEERKAT_TENANCY` (`-tenancy`) | `single` or `multi`, settled on the first start; afterwards the console owns it |

Without a configuration file the gateway starts empty - no route - and the
administrator's account is the only thing it creates.

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

### Logs and traces

| Flag | Variable | Default | What |
|---|---|---|---|
| `-log-level` | `MEERKAT_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`; a typo falls back to `info` rather than stopping the start |
| `-log-format` | `MEERKAT_LOG_FORMAT` | empty | `json` or `text`; empty picks JSON on a production gateway and text elsewhere |
| `-access-log` | `MEERKAT_ACCESS_LOG` | off | one line per request crossing the front door, on standard output - see [Logs](/docs/operations/logs) |
| `-otlp-endpoint` | `MEERKAT_OTLP_ENDPOINT` | empty | a collector to export traces to from the first second, such as `http://otel-collector:4318` (Enterprise) |
| `-otlp-sample` | `MEERKAT_OTLP_SAMPLE` | `0.1` | the share of the journeys this gateway opens that get recorded, 0 to 1 |

The level is set at startup; changing it means a restart.

The two `otlp` settings are for a gateway that must trace before anybody has
opened the console - an image started by a pipeline, say. The console's own
setting (**Infra, OpenTelemetry**) is the one to use otherwise: it adds the
metrics, the credential in the vault, the per-route choice and the Test button.
While it has never been switched on, the startup exporter runs; once it is, it
takes over, and switching it off again stops the export until the next start. See [Traces](/docs/operations/tracing).

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
