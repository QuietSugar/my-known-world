# Layered adapter work (Issues #32 and #33)

This is an implementation in progress. PostgreSQL remains authoritative. No watcher, automatic Git operation, or reverse synchronization is introduced.

## Database-to-Markdown materialization

`myknownworld-materialize` reads `/api/nodes` and `/api/relations` only, then writes deterministic Markdown copies, a manifest, and an unsupported-data report to an explicitly supplied `--out` directory. It refuses to overwrite existing generated files unless `--overwrite` is explicitly supplied; it never removes unrelated files. Stable graph IDs are represented by `manifest.yaml` identity mappings. The manifest retains relation IDs/endpoints even when a relation has no Markdown reference representation.

The initial adapter maps only hosting (`host`), Product instances (`instances`), and WorkItem references (`related`) into Markdown frontmatter. Other relations are retained in the manifest and disclosed as unsupported. Original Markdown bodies, history, assertion hashes, and secret-like fields are not emitted. The importer now decodes all modeled type-specific detail structs and generic typed relations from generated frontmatter. This does not recreate source provenance, model revisions, original evidence references, historical timestamps, or original Markdown prose; those omissions are surfaced in the report/manifest. Source-specific field validation and semantics still require configuration review before applying any generated batch.

Example:

```sh
go run ./cmd/myknownworld-materialize --api http://127.0.0.1:8080 --out ./materialized-copy
```

## CMDB and credential-provider boundaries

`internal/cmdb` defines generic environment grouping metadata and a repository-backed catalog service with search/filter operations; it does not extend the core graph schema. Its HTTP handler exposes environment list/create/update/delete and secret create/read/update/rotate/disable operations behind a dedicated bearer credential (minimum 32 characters), separate from the core write token. Secret responses use `Cache-Control: no-store`; create/update operations return provider metadata only, while read returns secret bytes only to the separately authorized caller. It has no default plaintext or in-memory provider, adds no secret fields to graph entities, and never connects provider calls to the core graph write token. The `cmd/myknownworld-cmdb` process supplies a separate loopback-by-default application API with a PostgreSQL-owned `cmdb_app_environments` table. Set `CMDB_DATABASE_URL` and a distinct 32+ character `CMDB_APP_TOKEN`; put it behind a TLS reverse proxy before any network exposure. The web app includes basic CMDB environment, service, and account catalog screens; configure its same-origin `/cmdb` reverse-proxy path to the CMDB listener (development Vite proxy defaults to port 8082). The screen stores the application token in session storage and uses it only for CMDB requests. Account entries may hold an opaque external credential reference, never a secret value.

Credential routes stay unavailable unless a provider is explicitly configured. Two supported paths exist: the built-in encrypted file provider (`CMDB_SECRET_PROVIDER=file`) and an operator-supplied provider reached over the documented HTTP protocol (`CMDB_SECRET_PROVIDER=http`). A Go implementation of `cmdb.SecretProvider` can also be compiled in. See [`secret-providers.md`](secret-providers.md) for the provider contract, the wire protocol, the environment variables, and the backup/restore, revocation, rekey, and failure semantics of each option. A specific third-party vault product is not required by this project. Core entity lookup integration and the complete graph-linked environment/service/account workflow remain outstanding.
