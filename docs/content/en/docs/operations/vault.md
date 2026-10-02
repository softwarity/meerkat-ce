---
title: The vault
section: Operations
order: 227
summary: Encrypted secrets and plain values under one namespace, the $name references, and the master key trap on a cluster.
---

# The vault

The vault is the one place the rest of the configuration points at instead of carrying a
value inline. It holds two kinds of entry under **one namespace**:

| Kind | At rest | Read back |
|---|---|---|
| **secret** | encrypted, AES-256-GCM | never in plain. The API says whether one is set, not what it is |
| **value** | plain text | readable - a hostname, a header name, an account name |

Holding both in one place is the point: one screen for everything the configuration refers
to, and a visible answer to "what is actually used". Only the encryption differs, and the
reference syntax is the same - so promoting a value to a secret never touches the objects
that point at it.

The screen is **Vault**, a transverse entry of its own.

![The vault screen](img/console/vault.webp)

## References

A value in the configuration refers to an entry by name:

```yaml
upstream: "http://${checkout-host}:8080"
password: "$smtp-password"
```

- `$name` and `${name}` are the same reference; the braces let it sit flush against what
 follows.
- `$$` is a literal `$`.
- A name that does not resolve is left **verbatim** and reported, so a typo shows up as
 itself. Turning it silently into an empty string would produce an empty upstream or an
 empty password, which fail in far more confusing ways.

For a **secret** there is one extra rule: only a value that is *entirely* one reference
counts as a reference. An upstream is built around its references, so a fragment is normal
there; a password is not a fragment. `${a}${b}` and `x-$token` are therefore literals - the
safe way round, since a value that cannot be certified as a reference is treated as a secret
to protect.

## Scopes

An entry belongs to a scope, and **a name is unique per scope** - so the same
`db-password` can mean a different thing for two organisations.

| Scope | Who administers it | What resolves against it |
|---|---|---|
| `infra` | gateway-admin | routes, filter arguments, upstream credentials |
| `app` | app-admin | the mail relay, the identity providers |
| an organisation | that organisation's administrators | its own entries |

Resolution honours the same split, which is what stops the administrator of one plane
quietly changing what another plane resolves: a route only ever expands against `infra`
entries. An organisation resolves its own first and falls back to the application's, so it
may **use** a global value without being able to edit it, and may shadow it by declaring its
own under the same name.

## A sensitive field always goes through the vault

A field that holds a secret has four states in the console, and typing a value blocks the
save until it has been filed. A literal inherited from a bootstrap file or an
older save is moved **server-side**: the browser sends a name, not a value - it never
received that literal and could not file it itself.

A reference is public. A literal never is.

## The master key

| Where it comes from | How |
|---|---|
| `MEERKAT_VAULT_KEY` | thirty-two bytes, as sixty-four hex characters or base64 |
| otherwise | a file `vault.key` in the data directory, mode `0600`, generated on first start |

The generated file sits next to the database, so it protects a **stolen or copied
database** - a backup, an export - and not a stolen data directory. Supplying the
key by environment keeps it off the disk entirely, and the console says which of the
two your installation does.

The same key seals more than the vault: **certificate private keys**, the **ACME account
key**, the developer tunnel's **host key** and every account's **TOTP secret** go through it
too. A TOTP secret is a password that never changes - whoever reads it computes every code the
account will ever accept - so a copied database holds none in clear. Those written before are
sealed at the next start.

> [!WARNING]
> **The key stays a local file even with an external database.** That is deliberate - it
> seals what is in the database, so keeping it *in* the database would seal the door with the
> key in the lock. The consequence on a cluster is the trap: each node generates its own key
> on first start, and a node then cannot open what another sealed. The error names it - *the
> private key cannot be unsealed - is this the vault key it was written with?* Set
> `MEERKAT_VAULT_KEY` to the same value on every node, before the first start.

### Rotating it

A restart with two keys:

1. Set the new key as `MEERKAT_VAULT_KEY` and the one it replaces as
   `MEERKAT_VAULT_KEY_PREVIOUS` (with the generated file, its content is the old key).
   The Helm chart has `vault.previousKey` for this.
2. Restart the nodes. Each one reads both keys, and the first to start seals
   everything again under the new one. The log says how many values it moved.
3. Once every node runs with the new key, remove `MEERKAT_VAULT_KEY_PREVIOUS`.

Nothing is unreadable in between. A node started with the new key alone, before
the others moved, fails on the first sealed value it reads, with the same error
as a wrong key.

## The vault as a file

The exact opposite of a configuration export, in every way. It holds the values
themselves, it is encrypted with a passphrase the gateway never stores, and it is **not**
something to version: it exists to bootstrap an environment or to move a gateway, then to be
deleted.

The file writes its own recipe in clear - format version, key derivation, its parameters, the
salt, the nonce - because it will outlive the binary that produced it by years, and a parameter
changed in a later release must not make an old export unreadable. The derivation is Argon2id
and the passphrase has a floor of twelve characters; the console offers to generate one, precisely
so nobody types the product name followed by the year.

A gateway can ingest one at startup: `-vault` with the passphrase in `MEERKAT_VAULT_PASSPHRASE`
or `MEERKAT_VAULT_PASSPHRASE_FILE`.

## Reminder dates

An entry can carry a **reminder date**: the day its secret is known to lapse at its source - a
token, a certificate.

It is purely a reminder and changes nothing. The gateway cannot tell whether a token was renewed
at the provider, so the `$name` reference goes on resolving. The date only feeds the daily digest,
which lists what is coming and what has just passed, never the value.

## What is missing

- **An external backend**: no HashiCorp Vault, no Kubernetes or Docker secrets as an
 alternative source.
