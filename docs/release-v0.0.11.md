# MyKnownWorld v0.0.11 — tester preview

This is a **pre-release for functional testing and feedback**, not a production-ready CMDB or credential service. PostgreSQL remains authoritative for the core graph.

## What changed

- **Credential providers are no longer tied to a vendor.** The CMDB layer talks to one generic HTTP contract (`CMDB_SECRET_PROVIDER=http`, `CMDB_SECRET_HTTP_URL`, `CMDB_SECRET_HTTP_TOKEN`). Any secret service — Vault, Vaultwarden, KeePass, a cloud KMS gateway, or a small local daemon — can sit behind it; this repository carries no vendor SDK or vendor-specific protocol. See `docs/secret-providers.md`.
- **The built-in file provider is a supported implementation, not a test stub.** Values are AES-256-GCM encrypted, the store is owner-only, writes are serialized with an advisory lock so several processes cannot corrupt it, keys can come from `CMDB_VAULT_KEY_BASE64` or an owner-only `CMDB_VAULT_KEY_FILE`, `disable` destroys the ciphertext, and `myknownworld-cmdb -rekey-vault` rotates the master key offline with all-or-nothing rewrite.
- **Release packaging fixed (issue #37).** The downloadable image archive now carries the release version tag, so `docker load -i my-known-world-image.tar` plus the default `.env.example` starts Compose without manual retagging. The release job refuses to attach an archive that lacks that tag, and CI fails if the published GHCR tag is not anonymously pullable.
- **The materialization command is in the runtime image (issue #36).** `/usr/local/bin/myknownworld-materialize` ships alongside the app, MCP server, Markdown importer, and CMDB service.
- CMDB environment/service/account catalogs and deterministic Markdown materialization from v0.0.10 are unchanged.

## Test it

1. Copy `.env.example` to `.env`; replace `POSTGRES_PASSWORD` and `CMDB_APP_TOKEN` with independent random values. `CMDB_APP_TOKEN` must differ from `MYKNOWNWORLD_WRITE_TOKEN`.
2. Credential routes stay disabled until you choose a provider:
   - File provider: `CMDB_SECRET_PROVIDER=file` and `CMDB_VAULT_KEY_BASE64=$(openssl rand -base64 32)`. The Compose vault directory is a tmpfs and is discarded when the container is recreated; mount a persistent owner-only directory if it must survive.
   - Your own service: `CMDB_SECRET_PROVIDER=http`, `CMDB_SECRET_HTTP_URL`, `CMDB_SECRET_HTTP_TOKEN`. Loopback endpoints may use plaintext HTTP; anything else needs TLS.
3. `docker compose up -d --no-build` if you loaded the release archive, otherwise `docker compose up -d --build`. Open `http://127.0.0.1:8080`, choose **CMDB 环境目录**, and enter the CMDB application token.
4. For a local Markdown copy of the graph:

   ```sh
   docker compose exec app /usr/local/bin/myknownworld-materialize --api http://127.0.0.1:8080 --out /tmp/materialized
   ```

   The command only reads the public API, refuses to overwrite existing files without `--overwrite`, and reports unsupported content.

Use synthetic data only. Do not put real credentials, private addresses, or personal documents into test reports or public fixtures.

## Known limitations

- **Issue #32 remains open:** CMDB services and accounts are not yet linked to core graph entities, the complete environment → service → account workflow is not finished, and authorization is application-level (no fine-grained per-user grants or audit trail). The file provider is single-host and has no built-in audit log.
- **Issue #33 remains open:** markdown prose bodies were never stored and cannot be regenerated; unsupported relations, provenance, and history are reported rather than reconstructed. The adapter performs no Git operations and no continuous synchronization.
- **Issue #37:** if `ghcr.io/quietsugar/my-known-world:v0.0.11` is not anonymously pullable, use the attached archive (which is tagged correctly) or sign in to GHCR; CI reports the failure explicitly.
- No production or private-corpus validation is claimed.
