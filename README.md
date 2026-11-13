# MyKnownWorld

A personal cognitive graph for what I know, own, maintain, use, and operate.

MyKnownWorld distinguishes software projects from deployed instances, credentials from their secrets, and established models from provisional knowledge. Every published object is connected to `Self`.

## Documentation

- [Requirements](docs/requirements.md)
- [Technology recommendations](docs/technology.md)
- [Implementation plan](docs/implementation-plan.md)

The initial implementation includes a Go domain core with typed relationships, Self connectivity, provenance and certainty labels, impact-path analysis, PostgreSQL snapshot storage, and a read-only HTTP API with a React web UI. The first tester version is `0.01` (Git tag `v0.01`); it serves either synthetic sample data or an existing complete PostgreSQL snapshot.

## Docker deployment

Docker Compose is the recommended runnable package. Copy `.env.example` to `.env`, replace `POSTGRES_PASSWORD` with a long URL-safe secret, then run:

```sh
cp .env.example .env
# Edit .env and replace the example database password.
docker compose up --build -d
```

Open `http://127.0.0.1:8080`. A new installation seeds only a clearly synthetic demo graph by default so the UI is immediately usable. Set `MYKNOWNWORLD_SEED_DEMO=0` to disable this; the app then requires a valid existing snapshot. PostgreSQL is not published to the host; app and database share a Compose-scoped bridge network. The app port binds to localhost by default. Data persists in the `postgres-data` volume. `docker compose down` preserves it; `docker compose down -v` deletes it.

The app container runs as UID 10001 with a read-only root filesystem, dropped capabilities, no-new-privileges, a small `/tmp` tmpfs, and a health check. The web UI is served from the same image as the API.

After startup, run the dependency-free read-only acceptance script with `python3 scripts/smoke_test.py --base-url http://127.0.0.1:8080`. It checks health, the unique Self anchor, node paths, provenance fields, impact-path boundaries, and API consistency without changing data. GitHub Actions runs the same script against the container deployment.

GitHub Actions builds and smoke-tests the container under the hardened Compose configuration. Each workflow run exports a versioned image tar artifact; pushes to `master` publish `ghcr.io/quietsugar/my-known-world:latest`, and version tag pushes publish the matching tag plus commit SHA. Pushing `v0.01` creates a GitHub tester release with the source archive, Docker image tar, checksum, and test notes in `docs/release-v0.01.md`.

## Local development

Run the API with `go run ./cmd/myknownworld` (listens on `:8080`; override with `ADDR`). Endpoints: `GET /healthz`, `GET /api/nodes`, `GET /api/nodes/{id}`, `GET /api/path/{id}`, and `GET /api/impact`. Run the web UI with `cd web && npm ci && npm run dev`; its Vite proxy expects the API at `127.0.0.1:8081` by default, so start the API with `ADDR=:8081 go run ./cmd/myknownworld` (set `API_PROXY_TARGET` to use a different backend).

Apply the schema to a PostgreSQL database with `DATABASE_URL='postgres://…' go run ./cmd/myknownworld-migrate`. Start the API with `DATABASE_URL='postgres://…' go run ./cmd/myknownworld`; it applies migrations and reads the complete stored snapshot. An empty database intentionally fails closed; set `MYKNOWNWORLD_SEED_DEMO=1` only to seed an intentionally synthetic database. The persistence package exposes atomic `SaveSnapshot` and validated `LoadSnapshot`. `SaveSnapshot` replaces the entire graph, so use it only with a complete authoritative snapshot. PostgreSQL integration tests run when `POSTGRES_TEST_DSN` is set. GitHub Actions runs formatting, vet, race tests, and PostgreSQL integration tests on a PostgreSQL 16 service.

This repository is being designed for public development. Real personal data, internal addresses, source configuration, credentials, documents, and graph snapshots must remain outside the public repository.
