# MyKnownWorld v0.0.10 — tester preview

This is a **pre-release for functional testing and feedback**, not a production-ready CMDB or vault deployment. PostgreSQL remains authoritative for the core graph. This preview adds the application-owned CMDB environment/service/account catalogs and an opt-in encrypted local file vault provider.

## Test the CMDB and local vault

1. Copy `.env.example` to `.env`; replace `POSTGRES_PASSWORD` and `CMDB_APP_TOKEN` with independent random values. `CMDB_APP_TOKEN` must be distinct from `MYKNOWNWORLD_WRITE_TOKEN`.
2. For local secret lifecycle testing only, set `CMDB_SECRET_PROVIDER=file` and set `CMDB_VAULT_KEY_BASE64` to the output of `openssl rand -base64 32` (or an equivalent secure random 32-byte key encoded as Base64). Keep this key outside the repository and back it up separately if needed. The Compose preview uses an ephemeral tmpfs vault: its contents are discarded when the CMDB container is recreated.
3. Start the preview with `docker compose up -d`. Open `http://localhost:8080`, choose **CMDB 环境目录**, and enter the CMDB application token. Try synthetic environment, service, and account entries. The account credential reference field is an opaque reference only; never paste a real secret there.
4. Secret lifecycle API routes (`POST /cmdb/credentials`, `GET|PUT /cmdb/credentials/{ref}`, `POST /cmdb/credentials/{ref}/rotate`, and `POST /cmdb/credentials/{ref}/disable`) are available only when the file provider is enabled. Use disposable synthetic values only; the CMDB bearer token is required.

## Known issue in this archive

The `my-known-world-image.tar` attached to v0.0.10 contains only the `ci` image tag, while `.env.example` defaults to `IMAGE_TAG=v0.0.10`. After `docker load`, either set `IMAGE_TAG=ci` in `.env` or retag:

```sh
docker tag ghcr.io/quietsugar/my-known-world:ci ghcr.io/quietsugar/my-known-world:v0.0.10
```

Later releases tag the archive with the release version, and CI now verifies both the archive tag and anonymous pullability of the published GHCR tag (issue #37).

## Scope and limitations

- The file vault is encrypted with AES-256-GCM and intended for single-process local development/functional testing. It is not a managed, shared, or production vault. There is no production provider integration or fine-grained per-user authorization yet.
- The new app-owned environment/resource tables are separate from core graph tables. Core entity identity validation and the complete environment/account/service operational workflows are not included.
- Markdown materialization is deterministic, refuses overwrites by default, reports omitted relations and unsupported content, and does not recover original Markdown bodies or full graph history/provenance. It remains an explicit export/import workflow; no watcher or continuous synchronization is added.
- No production/private corpus or cutover validation is claimed. Keep real credentials, private data, and Markdown bodies out of test reports and public fixtures.

Please report usability issues and missing workflow details against the open CMDB/materialization issues. This preview should not be used with production secrets or relied on for operational continuity.
