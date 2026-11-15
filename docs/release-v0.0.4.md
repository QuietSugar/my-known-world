# MyKnownWorld v0.0.4

An early-access release focused on making write paths explicit, reviewable, and safe against stale previews or stale undo state.

## Highlights

- Field assertion resolutions now use a read-only server preview, a signed plan hash, and an atomic revision check. Decisions remain append-only; preview responses expose no field values or value hashes.
- Source-policy allow-list updates preview the before/after policy and reject stale concurrent edits.
- Sync batch apply is bound to the exact submitted batch and a revision covering the graph, source state, identity mappings, and source policies.
- Provisional classification/rejection, manual graph edits, and conditional undo use preview-and-apply plans.
- Legacy direct-write HTTP routes are retired with HTTP 428 `preview_required`; they do not silently bypass the protected workflow.

## Safety and scope

This is still an early-access, single-operator application, not a multi-user service. Keep personal data, source configuration, internal addresses, credentials, document bodies, and graph snapshots private. Never store secrets or document contents in graph records; hashes are not anonymization. Writes remain disabled unless a sufficiently long `MYKNOWNWORLD_WRITE_TOKEN` is configured.

Full business-validity intervals and whole-graph historical queries are not implemented. System-recorded event times must not be interpreted as when a fact was true in the real world. Real-source adapters remain limited. See `docs/implementation-plan.md` for current status.

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
