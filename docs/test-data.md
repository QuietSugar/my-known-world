# Maintained synthetic test world

`internal/fixtures/synthetic-world.json` is the canonical, public-safe fixture for testing batch synchronization. It contains **15 synthetic nodes and 16 relations**, covers all 12 currently modeled node types and all 12 predicates, and includes a Self-known-only object, nested machine hosting, provenance/certainty variants, a pending provisional object, populated typed details for Hardware, Machine, SoftwareProject, SoftwareInstance, AccountCredential, Product, Document, Runbook and WorkItem, a completed historical RunningPeriod, and four field-provenance assertions that contain only hashes and synthetic references. Identity remapping is covered separately by sync unit and PostgreSQL integration tests. It contains no real people, networks, credentials, document bodies, or production facts.

The `internal/fixtures` Go package embeds the JSON and supplies `SyntheticWorld()` to tests. Tests validate graph connectivity, predicate/type coverage, impact excluding `knowsAbout`, idempotent replay, provisional classification, and PostgreSQL batch apply/round-trip.

## Run the checks

```sh
go test ./internal/fixtures
POSTGRES_TEST_DSN='postgresql://postgres:@/postgres?host=/tmp/pgdata' \
  go test ./internal/store/postgres -run TestSyntheticWorldBatchAppliesAndRoundTrips -v
```

The database integration test creates a uniquely named temporary schema and drops only that schema afterward. It skips when `POSTGRES_TEST_DSN` is unset.

To inspect a non-mutating preview against a running local service:

```sh
curl -sS -X POST http://127.0.0.1:8080/api/sync/preview \
  -H 'Content-Type: application/json' \
  --data-binary @internal/fixtures/synthetic-world.json
```

The HTTP endpoint only previews. The `myknownworld-sync` command offers a guarded preview/apply path against PostgreSQL:

```sh
DATABASE_URL='postgresql://…' go run ./cmd/myknownworld-sync \
  -input internal/fixtures/synthetic-world.json

# Apply is explicit and requires an exact source confirmation:
DATABASE_URL='postgresql://…' go run ./cmd/myknownworld-sync \
  -input internal/fixtures/synthetic-world.json \
  -apply -confirm-source synthetic-world-v1
```

Applying uses the atomic `Store.ApplySync` transaction and does not replace unrelated sources. Always target a disposable test database, never your personal-data database. The integration test still independently covers apply and round-trip in an isolated schema.

## Maintenance rules

When adding a model type or predicate, update the JSON to exercise it and update the coverage assertions in `internal/fixtures/fixture_test.go`. Keep IDs stable except in tests explicitly exercising identity mappings. Add endpoint-invalid, missing/stale, conflict, rename, and atomic-failure cases to focused tests rather than polluting the valid fixture. Keep this dataset synthetic; never copy personal records, internal addresses, secrets, source configs, or document bodies into it. A value hash is not anonymization: never hash secrets or low-entropy sensitive values into graph assertions.
