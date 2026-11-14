# MyKnownWorld v0.0.2

A substantial early-access release that expands the initial read-only graph viewer into a private, typed knowledge-graph workspace with controlled synchronization and authenticated manual changes.

## Highlights

- Typed graph models and relationships for projects, deployed software, machines, networks, products, accounts, documents, runbooks, work items, and running periods; published nodes remain connected to the unique `Self` anchor.
- PostgreSQL migrations, graph exploration, impact/path queries, object and relationship history, field-level assertions and explicit conflict resolutions.
- Incremental sync preview/apply, source policies, identity mappings, provisional-object review, a synthetic CSV inventory importer, and sync-batch history/conditional undo.
- Authenticated manual node and relationship changes with preview-before-write, graph revision and signed plan validation; conditional undo for eligible manual operations.
- A refreshed web interface, hardened Docker Compose deployment, and synthetic fixtures/tests.

## Safety and scope

This is still an early-access, single-operator application, not a multi-user service. Keep real personal data, source configuration, internal addresses, credentials, document bodies, and graph snapshots private. Never put secrets or document contents into graph records; hashes are not anonymization. Writes are disabled unless a sufficiently long `MYKNOWNWORLD_WRITE_TOKEN` is configured. Review the deployment and backup guidance before using non-synthetic data.

Some advanced implementation-plan items remain incomplete, including full temporal graph queries, broad real-source adapters, friendlier typed forms, and evidence editing for every relation/field workflow. The release does not claim these features are complete.

## Test and run

Run the Go tests (including PostgreSQL integration tests when `POSTGRES_TEST_DSN` is configured) and build the web UI:

```sh
go test -race ./...
cd web && npm ci && npm run build
```

For the container, download `my-known-world-image.tar` from release assets, verify `my-known-world-image.tar.sha256`, then:

```sh
docker load -i my-known-world-image.tar
# Configure .env with a strong database password; select IMAGE_TAG=ci.
docker compose up -d --no-build
python3 scripts/smoke_test.py --base-url http://127.0.0.1:8080
```

The bundled Compose configuration uses synthetic demo data by default. Set `MYKNOWNWORLD_SEED_DEMO=0` when deploying against an existing initialized database.
