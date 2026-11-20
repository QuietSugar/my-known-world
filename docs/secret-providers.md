# Secret providers

This document defines the credential-provider boundary for the CMDB layer
(Issue #32) and how an operator supplies a provider.

**Scope decision:** running a specific third-party vault product is *not* a
requirement of this project. The project ships one supported first-party
provider (encrypted local files) and defines the contract an operator's own
provider must satisfy. Nothing in the application requires Vaultwarden, a cloud
KMS, or any other particular product.

The application only ever stores provider name, an opaque reference, a safe
access reference, and non-secret rotation metadata. It never writes secret
values to the graph, graph history, field assertions, MCP responses, logs, or
Markdown materializations.

## Provider contract

Any provider must satisfy these rules. They are enforced by the application
where it can enforce them, and are obligations of the provider where it cannot.

1. **Five operations.** `create`, `read`, `update`, `rotate`, and `disable`,
   each bounded by the caller's context and returning within the configured
   timeout.
2. **Opaque references.** `create` returns a reference that is stable, unique,
   URL-safe after escaping, at most 2048 bytes, free of CR/LF, and meaningless
   outside the provider (no secret value, no plaintext account name).
3. **Metadata is non-secret.** Returned metadata may contain only provider
   name, opaque ID, access reference, rotation timestamp, and status. It must
   never contain the secret, a reversible encoding of it, or a key.
4. **Values are transient.** A provider must not cache values server-side
   beyond the request, log them, include them in error messages, or return them
   from any operation other than `read`.
5. **Value limits.** Values are 1 byte to 64 KiB. `read` returns exactly the
   current value; `update` and `rotate` replace it atomically (no partial
   writes, no version gaps).
6. **Rotation is explicit.** `rotate` records the rotation time and marks the
   credential rotated; `update` changes the value without claiming rotation.
   A failed rotation must leave the previous value and metadata intact.
7. **Disable is revocation.** After `disable`, `read` must fail and the stored
   ciphertext must be destroyed (or cryptographically unrecoverable). Metadata
   survives as a tombstone so the reference stays resolvable as disabled.
8. **Authorization is separate.** Provider access must not be granted by
   possession of the core graph write token. Only the CMDB application bearer
   credential (or a stronger per-user mechanism you add) may reach a provider.
9. **Failure isolation.** Provider downtime must not block core graph reads or
   writes, must not leave misleading rotation metadata in the core graph, and
   must not corrupt the store.
10. **No default provider.** Starting the service without explicit provider
    configuration leaves credential routes reporting `503 secret provider
    unavailable`; the application never falls back to plaintext or memory-only
    storage.
11. **Secret hygiene in your own operations.** Whoever operates a provider owns
    key custody, access auditing, encrypted backup/restore, and revocation of
    copies that were already disclosed. The repository never stores keys.

## Selecting a provider

| Variable | Meaning |
| --- | --- |
| `CMDB_SECRET_PROVIDER` | `file`, `http`, or empty (credential routes disabled). |
| `CMDB_VAULT_DIR` | File provider store directory, owner-only. |
| `CMDB_VAULT_KEY_BASE64` | File provider key: 32 random bytes, Base64. |
| `CMDB_VAULT_KEY_FILE` | File provider key file (owner-only). Alternative to the variable above; not both. |
| `CMDB_SECRET_PROVIDER_URL` | `http` provider base URL. `https` required; `http` only with the opt-in below. |
| `CMDB_SECRET_PROVIDER_TOKEN` | `http` provider bearer credential, at least 32 characters. |
| `CMDB_SECRET_PROVIDER_ALLOW_HTTP` | Set to `1` to allow plaintext HTTP for loopback testing only. |

Unknown values fail at startup instead of silently disabling credential
operations.

## File provider (built in)

`internal/cmdb.FileVault` is the supported first-party provider. It is a real
provider, not a test double: values are AES-256-GCM encrypted, the store is
written atomically with mode `0600` into an owner-only directory, and mutations
are serialized with an advisory file lock so several processes on one host
cannot interleave read-modify-write cycles.

```
CMDB_SECRET_PROVIDER=file
CMDB_VAULT_DIR=/var/lib/myknownworld/vault     # owner-only, mode 0700
CMDB_VAULT_KEY_BASE64=$(openssl rand -base64 32)
```

Store layout:

```
vault.json   # metadata + nonce + ciphertext per reference; no plaintext values
vault.lock   # empty advisory lock file
```

Key handling:

- Generate the key once with a CSPRNG, keep it outside the store directory and
  outside the repository, and inject it through the environment or a mounted
  owner-only key file.
- A key file must be a regular, non-symlink file with mode `0600`, containing
  32 raw bytes or their Base64/hex encoding.
- Losing the key makes every stored value unrecoverable by design.
- The store directory is hardened to `0700` on startup; an existing
  group/world-accessible directory is tightened automatically, while a symlink
  or non-directory is refused. Data or key files with loose permissions are
  refused rather than used.

### Operations

- **Backup:** back up the whole directory *and* the key together, as one
  encrypted unit. `vault.json` without the key is useless; the key without the
  store is useless. Use an encrypted destination and verify restores.
- **Restore:** stop the service, restore the directory with owner-only
  permissions, restore the key, start the service, then read a known reference
  to confirm the restore.
- **Revocation:** `disable` destroys the ciphertext for that reference and
  leaves a tombstone. To revoke every value at once, destroy the key
  (crypto-shredding) and start a fresh store. Revocation cannot recall secret
  copies that were already handed to a caller; rotate the upstream system.
- **Key rotation:** stop the service and run

  ```sh
  CMDB_SECRET_PROVIDER=file CMDB_VAULT_DIR=/var/lib/myknownworld/vault \
  CMDB_VAULT_KEY_FILE=/run/secrets/vault.key \
  CMDB_VAULT_NEW_KEY_FILE=/run/secrets/vault-new.key \
    myknownworld-cmdb -rekey-vault
  ```

  The rekey rewrites every record only after all of them decrypt with the
  current key; then replace the mounted key and restart. A failure leaves the
  store untouched. Retire the old key only after a successful read.
- **Host loss:** the file provider is single-host. It does not replicate.
  Backups are the only recovery path.

Limitations, stated plainly: the file provider has no built-in audit trail, no
per-user authorization, no multi-host sharing (advisory locks and atomic rename
are not reliable on network filesystems), and no automatic key management. Add
those with your own provider when you need them.

## Bring your own provider

### Option A — HTTP provider protocol (any language)

Set `CMDB_SECRET_PROVIDER=http` and point `CMDB_SECRET_PROVIDER_URL` at your
service. The adapter is `internal/cmdb.HTTPSecretProvider`; the wire protocol is:

```
POST   {base}/v1/secrets               {"name":"...","value":"<base64>"}      -> 2xx {"metadata":{...}}
GET    {base}/v1/secrets/{ref}                                                -> 200 {"metadata":{...},"value":"<base64>"}
PUT    {base}/v1/secrets/{ref}         {"name":"...","value":"<base64>"}      -> 2xx {"metadata":{...}}
POST   {base}/v1/secrets/{ref}/rotate  {"name":"...","value":"<base64>"}      -> 2xx {"metadata":{...}}
POST   {base}/v1/secrets/{ref}/disable                                        -> 2xx {"metadata":{...}}
```

`metadata` is `{"provider": "...", "opaqueId": "...", "accessReference": "...",
"rotationAt": "...", "status": "..."}` where only `provider` and `opaqueId` are
required. Values are Base64 so any language can implement the server.

Adapter behavior an implementation must match:

- `Authorization: Bearer <CMDB_SECRET_PROVIDER_TOKEN>` on every request, over
  TLS. Plaintext HTTP is refused unless `CMDB_SECRET_PROVIDER_ALLOW_HTTP=1`,
  which exists for loopback testing only.
- Redirects are not followed, so the credential is never forwarded to another
  origin.
- Responses are bounded (256 KiB) and strictly validated: the returned
  `opaqueId` must equal the requested reference, metadata must be non-secret
  and CR/LF-free, and values must be 1 byte to 64 KiB.
- Status mapping: `404`/`410` → secret unavailable (`404` to callers);
  `401`/`403` → application credential rejected; any other non-2xx, transport
  error, or malformed body → provider unavailable (`503`). Provider response
  bodies are never echoed to callers or logs.
- Idempotency: `disable` on an already-disabled reference should succeed and
  return the tombstone; `update`/`rotate` on an unknown or disabled reference
  should return `404`.

### Option B — compiled-in Go provider

Implement `cmdb.SecretProvider`:

```go
Create(ctx context.Context, cmdb.SecretRequest) (cmdb.Metadata, error)
Read(ctx context.Context, ref string) (cmdb.SecretResult, error)
Update(ctx context.Context, ref string, cmdb.SecretRequest) (cmdb.Metadata, error)
Rotate(ctx context.Context, ref string, cmdb.SecretRequest) (cmdb.Metadata, error)
Disable(ctx context.Context, ref string) (cmdb.Metadata, error)
```

Then add a case to `cmdb.NewSecretProvider` (or construct your provider in your
own `main`) and pass it to `cmdb.SecretService{Provider: ..., Authorizer:
cmdb.RequestSecretAuthorizer{}}`. Keep the authorizer: it refuses any call that
did not pass the independent CMDB bearer check. Do not wire provider calls to
the graph write token, and do not return secret bytes from anything other than
`Read`.

### Review checklist for an operator-supplied provider

- Values at rest encrypted with a key held outside the application database.
- No secret in provider logs, error strings, metrics labels, or traces.
- `disable` provably destroys or crypto-shreds the value.
- Backup/restore and key rotation are documented and tested by you.
- Provider downtime yields `503` without mutating core graph records.
- Rotation metadata is only written after the provider confirms the operation.

## Failure semantics seen by callers

| Situation | HTTP response | Core graph effect |
| --- | --- | --- |
| No provider configured | `503 secret provider unavailable` | none |
| Provider down/timeout/malformed | `503 secret provider unavailable` | none |
| Unknown, disabled, or undecryptable reference | `404 secret unavailable` | none |
| Bad token / not authorized | `401` at the handler, before the provider | none |
| Request validation failure | `400` | none |

Rotation metadata is only recorded in the CMDB layer after the provider returns
success, so a failed operation cannot leave a misleading "rotated" claim.
