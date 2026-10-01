# Contributing to Meerkat

## Repository layout

| Path | License | Content |
|---|---|---|
| `/` (everything except `ee/`) | [FSL-1.1-Apache-2.0](./LICENSE.md) | The Meerkat gateway - community core |
| `ee/` | [Softwarity Commercial](./ee/LICENSE.md) | Enterprise features, source-visible, unlocked by license key |
| `cmd/meerkat/` | FSL | The single binary entry point |
| `internal/` | FSL | Core packages (not importable from outside the module) |
| `FEATURES.md` | FSL | What the product does and how far it is built (French) |

One repository, one binary: EE code compiles into every build and stays
absent from the community image entirely: the `ee` build tag decides what
the linker puts in (`internal/edition`), and the two images are published
separately.
Never gate features by build tags or separate artifacts.

Neither image is published by a push any more. A release publishes from
its tag; otherwise it is asked for, the day one is wanted:

```bash
gh workflow run CI --ref main     # builds and pushes both images
```

A push used to publish on every commit to main, which cost ten minutes
and left a stored image version behind each time - a hundred of them had
piled up before the storage bill mentioned it. The publication now prunes
the untagged versions it leaves, keeping the ten most recent.

## Conventions

- **Language**: code, comments, commit messages and public docs are in
  English. The requirements document is currently maintained in French.
- **Go**: version pinned in `go.mod`. Run `make fmt lint test` before pushing.
- **Commits**: imperative subject line ("Add route matcher"), body explains
  *why* when it is not obvious.
- **Secrets**: never commit credentials, keys or license files - no
  exceptions (lesson learned from V1).

## Development

```bash
make build   # bin/meerkat
make test    # go test -race ./...
make lint    # golangci-lint run
```
