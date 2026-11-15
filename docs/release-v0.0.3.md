# MyKnownWorld v0.0.3

An early-access release advancing MyKnownWorld from its typed graph and controlled sync foundation toward more practical, auditable maintenance workflows.

## Highlights

- Expanded structured detail editing for modeled entity types, including RunningPeriod lifecycle, version, and verification fields.
- Read-only Markdown/frontmatter import with provisional candidates for content that cannot be safely classified, explicit identity mappings, and source-scope validation.
- Read-only MCP interfaces over Streamable HTTP and stdio, exposing bounded structured query tools without graph write tools.
- Manual node and relationship maintenance with server-side preview, graph revision and plan-hash validation, and conditional undo; relation evidence metadata can be edited without changing relation identity or meaning.
- Continued improvements to provenance, review confirmation, source history, network/domain modeling, and synthetic test coverage.

## Safety and scope

This remains an early-access, single-operator application, not a multi-user service. Keep personal data, source configuration, internal addresses, credentials, document bodies, and graph snapshots private. Never store secrets or document contents in graph records; hashes are not anonymization. Writes remain disabled unless a sufficiently long `MYKNOWNWORLD_WRITE_TOKEN` is configured. Review deployment and backup guidance before using non-synthetic data.

Important limitations remain: full temporal graph queries and business-validity semantics are not implemented; real-source adapters are limited; field assertion and source-policy changes do not yet share the full server-side preview/plan-hash workflow; relation endpoints and predicates cannot be edited in place. See `docs/implementation-plan.md` for the current status.

## Test and run

Run Go tests (including PostgreSQL integration tests when `POSTGRES_TEST_DSN` is configured) and build the web UI:

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

Compose uses synthetic demo data by default. Set `MYKNOWNWORLD_SEED_DEMO=0` when deploying against an existing initialized database.
