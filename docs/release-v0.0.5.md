# MyKnownWorld v0.0.5

An early-access patch release that fixes six reported defects in the v0.0.4 workflows. It remains focused on single-person, local/private use.

## Fixes

- Identity-mapping renames no longer mutate the submitted sync batch during preview, so the exact previewed request can be applied with its plan hash.
- URL-encoded slash-containing object IDs work across node detail, assertion, resolution, path, exploration, and entity-history read routes. This supports IDs emitted by the Markdown adapter.
- MCP `graph_history` returns modeled snapshots as structured objects that satisfy the tool output schema.
- Manual-mutation plan MACs bind both mutation payload and expected graph revision. PostgreSQL receives and checks the canonical mutation digest after the API verifies the token-authenticated MAC.
- Provisional-review `evidenceReference` is persisted and survives reopening the PostgreSQL store (migration 017).
- Markdown SoftwareInstance imports accept singular `domain` as an alias for `domains`; files containing both keys are explicitly excluded to avoid silently choosing or merging values.

## Scope and operations

This remains an early-access, single-operator application; it does not add collaboration, multi-user access control, comprehensive import adapters, business-validity intervals, full point-in-time graph queries, or a hardened operational backup program. Run it locally or in a trusted private network, keep the write token private, and retain the authoritative source records. Do not store credentials/secrets, document bodies, private source configuration, or real personal data in the public repository or test fixtures.

The historical event timestamp is system-recorded time, not a claim about when a fact was true in the real world. The default Docker Compose configuration is local-only and uses the versioned `v0.0.5` image; synthetic demo data is enabled by default for a new installation.

## Verify

```sh
go test -race ./...
cd web && npm ci && npm run build
```

The GitHub Actions workflow additionally runs PostgreSQL integration tests, builds the hardened container, and checks the container API/UI smoke test. For the release image tar, verify `my-known-world-image.tar.sha256` before loading it, then use the documented Compose setup and smoke test from `README.md`.
