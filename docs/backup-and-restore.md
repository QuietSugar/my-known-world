# Backup and Restore Procedure

MyKnownWorld relies on PostgreSQL as the authoritative source of truth for the
entire cognitive graph, change history, and CMDB catalog. This guide defines
the tested backup and restore lifecycle.

## 1. Creating a Backup

Use `scripts/backup.sh` to generate an atomic custom-format archive (`pg_dump -Fc`),
a SHA-256 checksum, and a sanitized metadata manifest.

### Docker Compose deployment

```sh
# Run from the deployment directory containing compose.yaml
scripts/backup.sh --compose --out-dir ./backups
```

### Direct connection via DSN

```sh
scripts/backup.sh --url "postgres://myknownworld:password@127.0.0.1:5432/myknownworld?sslmode=disable" --out-dir ./backups
```

### Generated Artifacts

Every backup produces three files in the output directory:
- `myknownworld_backup_<timestamp>.pg_dump`: Compressed custom-format PostgreSQL dump (`--format=custom --no-owner --no-privileges`).
- `myknownworld_backup_<timestamp>.sha256`: SHA-256 checksum for archive verification.
- `myknownworld_backup_<timestamp>_manifest.json`: Sanitized metadata record containing timestamp, version, database name, duration, and boolean flags for configuration settings. **No secret tokens or private keys are ever stored in the manifest.**

---

## 2. Restoring into a Target Database

Restoring requires a reachable PostgreSQL database with appropriate schema permissions.

### Restoring via Docker Compose

```sh
scripts/restore.sh --compose \
  --dump ./backups/myknownworld_backup_<timestamp>.pg_dump \
  --compose-db myknownworld
```

### Restoring via DSN

```sh
scripts/restore.sh \
  --dump ./backups/myknownworld_backup_<timestamp>.pg_dump \
  --url "postgres://myknownworld:password@127.0.0.1:5432/myknownworld?sslmode=disable"
```

The restore script:
1. Automatically validates the `.sha256` checksum before restoring.
2. Runs `pg_restore --list` to inspect table-of-contents integrity.
3. Executes `pg_restore --clean --if-exists --no-owner --no-privileges` to rebuild the schema cleanly.
4. Outputs the duration in milliseconds upon completion.

---

## 3. Important: CMDB Encrypted Vault Key Boundary

If your deployment uses the CMDB application layer with `CMDB_SECRET_PROVIDER=file`:

> [!WARNING]
> Credential values stored in `cmdb_secrets` are encrypted using AES-256-GCM keyed by `CMDB_VAULT_KEY_BASE64`. The database backup contains ciphertext only.
> 
> When restoring into a new host or fresh container stack, **you must configure the exact same `CMDB_VAULT_KEY_BASE64`** in your environment. Without this key, the restored graph and metadata will load, but attempts to read CMDB secrets will fail with decryption errors.

---

## 4. Verification and Benchmark

The backup and restore procedure is validated continuously by the `restore-drill` job in CI. On the synthetic corpus (~60–100 nodes and ~150 relations):
- **Backup duration**: ~40–80 ms
- **Restore duration**: ~70–120 ms
- **Verification check**: `scripts/verify_restore.py` asserts that the restored graph's `/api/nodes`, `/api/relations`, and fingerprint revision are byte-for-byte identical to the source deployment.
