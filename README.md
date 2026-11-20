# MyKnownWorld

A personal cognitive graph for what I know, own, maintain, use, and operate.

MyKnownWorld distinguishes software projects from deployed instances, credentials from their secrets, and established models from provisional knowledge. Every published object is connected to `Self`.

## Documentation

- [Requirements](docs/requirements.md)
- [Technology recommendations](docs/technology.md)
- [Implementation plan](docs/implementation-plan.md)
- [Incremental sync protocol](docs/incremental-sync.md)
- [Synthetic test data and batch test](docs/test-data.md)
- [Markdown frontmatter adapter](docs/markdown-frontmatter.md)
- [Database-authoritative deployment and cutover runbook](docs/database-authoritative-cutover.md)
- [Read-only MCP interface](docs/read-only-mcp.md)
- [Write API: create nodes and relations](docs/write-api.md)
- [Secret providers and the CMDB credential boundary](docs/secret-providers.md)
- [Synthetic Markdown adapter configuration](docs/markdown-adapter-config.example.yaml)
- [Handover: state, settled decisions, and open work](docs/handover.md)

The current release is `v0.1.0`, the first official (non-pre-release) build. It adds the create and batch-create write API (`docs/write-api.md`), an application-layer CMDB catalog, a supported encrypted file secret provider, a vendor-neutral HTTP contract for operator-supplied secret services (`docs/secret-providers.md`), and the `myknownworld-materialize` command inside the runtime image. It is a `0.x` release: safe to run with real data and backups, but the API may still change in a minor release. Issues #32/#33 were closed as not planned for this line, with the reasoning and the minimal re-entry slice recorded on each issue; `docs/handover.md` tracks what is delivered, what is settled, and what is still open. See `docs/release-v0.1.0.md` for scope, setup, upgrade notes, and limitations. PostgreSQL/MyKnownWorld remains authoritative for the core graph; Markdown is only for initial or explicitly triggered, reviewed import/export, with no watcher or continuous synchronization.

## Docker deployment

Docker Compose is the recommended runnable package. Copy `.env.example` to `.env`, replace `POSTGRES_PASSWORD` with a long URL-safe secret, then run:

```sh
cp .env.example .env
# Edit .env and replace the example database password.
docker compose up --build -d
```

Before startup, also replace `CMDB_APP_TOKEN` in `.env` with a separate random value (do not reuse `MYKNOWNWORLD_WRITE_TOKEN`); it authorizes the app-layer CMDB. Credential routes stay disabled unless you configure a provider: set `CMDB_SECRET_PROVIDER=file` with `CMDB_VAULT_DIR` and `CMDB_VAULT_KEY_BASE64` (`openssl rand -base64 32`) or `CMDB_VAULT_KEY_FILE` for the built-in encrypted file provider, or set `CMDB_SECRET_PROVIDER=http` with `CMDB_SECRET_HTTP_URL` and `CMDB_SECRET_HTTP_TOKEN` to use your own service that speaks the small documented contract. The Compose vault directory is a tmpfs and is discarded when the container is recreated; mount a persistent, owner-only directory and keep the key outside it for anything you need to keep. See `docs/secret-providers.md`. Open `http://127.0.0.1:8080`. To enable authenticated core-graph batch-apply and provisional-review actions, set a stable random `MYKNOWNWORLD_WRITE_TOKEN` of at least 32 bytes in `.env` (leave it empty to keep all writes disabled). Generate one locally with `python3 -c 'import secrets; print(secrets.token_urlsafe(48))'`; do not publish it. The UI asks for the token when a protected write is submitted and stores it in the current browser session. A new installation seeds only a clearly synthetic demo graph by default so the UI is immediately usable. Set `MYKNOWNWORLD_SEED_DEMO=0` to disable this; the app then requires a valid existing snapshot. PostgreSQL is not published to the host; app and database share a Compose-scoped bridge network. The app port binds to localhost by default. Data persists in the `postgres-data` volume. `docker compose down` preserves it; `docker compose down -v` deletes it.

The app container runs as UID 10001 with a read-only root filesystem, dropped capabilities, no-new-privileges, a small `/tmp` tmpfs, and a health check. The web UI is served from the same image as the API.

The runtime image also bundles `myknownworld-materialize`, so `docker compose exec app /usr/local/bin/myknownworld-materialize --api http://127.0.0.1:8080 --out /tmp/materialized` writes a read-only Markdown copy of the graph to the container filesystem. To run a bundled CLI tool with plain `docker run`, override the image entrypoint and use a writable output directory; see [Running the bundled CLI tools in the container](docs/release-v0.1.0.md#running-the-bundled-cli-tools-in-the-container).

After startup, run the dependency-free read-only acceptance script with `python3 scripts/smoke_test.py --base-url http://127.0.0.1:8080`. It checks health, the unique Self anchor, node paths, provenance fields, impact-path boundaries, and API consistency without changing data. GitHub Actions runs the same script against the container deployment.

GitHub Actions builds and tests the hardened Compose deployment, including the CMDB synthetic credential lifecycle. Each workflow run exports a versioned image tar artifact (90-day retention); pushes to `master` publish `ghcr.io/quietsugar/my-known-world:latest`, and version tag pushes publish the matching tag plus commit SHA. Pushing a version tag creates a GitHub release with the source archive, Docker image tar, checksum, and matching `docs/release-<tag>.md` notes; a `VERSION` with a suffix such as `0.2.0-rc.1` is published as a pre-release, a plain version such as `0.1.0` as an official release. Release archives are tagged with the release version, so `docker load -i my-known-world-image.tar` followed by `docker compose up -d --no-build` works with the default `.env.example` settings. Per-run CI artifacts are tagged `ci`; for those, set `IMAGE_TAG=ci` in `.env` before starting. The v0.0.10 archive predates this and only contains the `ci` tag (see issue #37). The current release scope and setup are described in `docs/release-v0.1.0.md`.

## Local development

Run the API with `go run ./cmd/myknownworld` (defaults to `127.0.0.1:8080`; override with `ADDR`). Reads are public; writes need `MYKNOWNWORLD_WRITE_TOKEN`, and the single-call create endpoints (`POST /api/write/nodes`, `/api/write/relations`, `/api/write/batch`) are described in [docs/write-api.md](docs/write-api.md). The read-only MCP Streamable HTTP endpoint is `/mcp` (its `initialize` handshake reports the binary version, taken from the build or from `MYKNOWNWORLD_VERSION`); for local stdio, use `DATABASE_URL='postgres://…' go run ./cmd/myknownworld-mcp` after the database has been initialized. MCP registers query tools only and never receives the write token. Run the web UI with `cd web && npm ci && npm run dev`; its Vite proxy expects the API at `127.0.0.1:8081` by default, so start the API with `ADDR=127.0.0.1:8081 go run ./cmd/myknownworld` (set `API_PROXY_TARGET` to use a different backend).

Apply the schema to a PostgreSQL database with `DATABASE_URL='postgres://…' go run ./cmd/myknownworld-migrate`. Start the API with `DATABASE_URL='postgres://…' go run ./cmd/myknownworld`; it applies migrations and reads the complete stored snapshot. An empty database intentionally fails closed; set `MYKNOWNWORLD_SEED_DEMO=1` only to seed an intentionally synthetic database. The persistence package exposes atomic `SaveSnapshot` and validated `LoadSnapshot`. `SaveSnapshot` replaces the entire graph, so use it only with a complete authoritative snapshot. PostgreSQL integration tests run when `POSTGRES_TEST_DSN` is set. GitHub Actions runs formatting, vet, race tests, and PostgreSQL integration tests on a PostgreSQL 16 service.

This repository is being designed for public development. Real personal data, internal addresses, source configuration, credentials, documents, and graph snapshots must remain outside the public repository.
