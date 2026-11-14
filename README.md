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
- [Read-only MCP interface](docs/read-only-mcp.md)
- [Synthetic Markdown adapter configuration](docs/markdown-adapter-config.example.yaml)

The first tester version (`v0.01`) was a read-only graph viewer. The current early-access target is `0.0.2` (Git tag `v0.0.2`): typed graph models, provenance and certainty, PostgreSQL persistence, authenticated review and sync writes, manual graph maintenance with preview-before-write, undo history, graph exploration, and a React UI. It serves synthetic sample data by default or an existing complete PostgreSQL snapshot.

## Docker deployment

Docker Compose is the recommended runnable package. Copy `.env.example` to `.env`, replace `POSTGRES_PASSWORD` with a long URL-safe secret, then run:

```sh
cp .env.example .env
# Edit .env and replace the example database password.
docker compose up --build -d
```

Open `http://127.0.0.1:8080`. To enable authenticated batch-apply and provisional-review actions, set a stable random `MYKNOWNWORLD_WRITE_TOKEN` of at least 32 bytes in `.env` (leave it empty to keep all writes disabled). Generate one locally with `python3 -c 'import secrets; print(secrets.token_urlsafe(48))'`; do not publish it. The UI asks for the token when a protected write is submitted and stores it in the current browser session. A new installation seeds only a clearly synthetic demo graph by default so the UI is immediately usable. Set `MYKNOWNWORLD_SEED_DEMO=0` to disable this; the app then requires a valid existing snapshot. PostgreSQL is not published to the host; app and database share a Compose-scoped bridge network. The app port binds to localhost by default. Data persists in the `postgres-data` volume. `docker compose down` preserves it; `docker compose down -v` deletes it.

The app container runs as UID 10001 with a read-only root filesystem, dropped capabilities, no-new-privileges, a small `/tmp` tmpfs, and a health check. The web UI is served from the same image as the API.

After startup, run the dependency-free read-only acceptance script with `python3 scripts/smoke_test.py --base-url http://127.0.0.1:8080`. It checks health, the unique Self anchor, node paths, provenance fields, impact-path boundaries, and API consistency without changing data. GitHub Actions runs the same script against the container deployment.

GitHub Actions builds and smoke-tests the container under the hardened Compose configuration. Each workflow run exports a versioned image tar artifact (90-day retention); pushes to `master` publish `ghcr.io/quietsugar/my-known-world:latest`, and version tag pushes publish the matching tag plus commit SHA. Pushing a version tag such as `v0.0.2` creates a GitHub release with the source archive, Docker image tar, checksum, and matching `docs/release-<tag>.md` notes. For a CI image tar, load it with `docker load -i my-known-world-image.tar`, set `IMAGE_TAG=ci` in `.env`, and run `docker compose up -d --no-build`. The current release candidate is described in `docs/release-v0.0.2.md`; use the [release checklist](docs/release-checklist-v0.0.2.md) before tagging.

## Local development

Run the API with `go run ./cmd/myknownworld` (defaults to `127.0.0.1:8080`; override with `ADDR`). The read-only MCP Streamable HTTP endpoint is `/mcp`; for local stdio, use `DATABASE_URL='postgres://…' go run ./cmd/myknownworld-mcp` after the database has been initialized. MCP registers query tools only and never receives the write token. Run the web UI with `cd web && npm ci && npm run dev`; its Vite proxy expects the API at `127.0.0.1:8081` by default, so start the API with `ADDR=127.0.0.1:8081 go run ./cmd/myknownworld` (set `API_PROXY_TARGET` to use a different backend).

Apply the schema to a PostgreSQL database with `DATABASE_URL='postgres://…' go run ./cmd/myknownworld-migrate`. Start the API with `DATABASE_URL='postgres://…' go run ./cmd/myknownworld`; it applies migrations and reads the complete stored snapshot. An empty database intentionally fails closed; set `MYKNOWNWORLD_SEED_DEMO=1` only to seed an intentionally synthetic database. The persistence package exposes atomic `SaveSnapshot` and validated `LoadSnapshot`. `SaveSnapshot` replaces the entire graph, so use it only with a complete authoritative snapshot. PostgreSQL integration tests run when `POSTGRES_TEST_DSN` is set. GitHub Actions runs formatting, vet, race tests, and PostgreSQL integration tests on a PostgreSQL 16 service.

This repository is being designed for public development. Real personal data, internal addresses, source configuration, credentials, documents, and graph snapshots must remain outside the public repository.
