# Write API: create, patch, and delete nodes and relations

The read API is public and read-only. Writes always require the independent
`MYKNOWNWORLD_WRITE_TOKEN` (at least 32 bytes) as a bearer credential, and they
never accept secrets: the typed model has no secret fields, and nothing you send
here is copied into Markdown, MCP responses, or logs.

There are two write styles:

| Style | Endpoints | Use it for |
| --- | --- | --- |
| Direct Write API (this document) | `POST /api/write/nodes`, `POST /api/write/relations`, `POST /api/write/batch`, `PATCH /api/write/nodes/{id}`, `POST /api/write/nodes/{id}/retire`, `DELETE /api/write/nodes/{id}`, `PATCH /api/write/relations/{id}`, `DELETE /api/write/relations/{id}` | scripts, automation, direct CRUD mutations |
| Preview-and-confirm | `POST /api/operations/preview` then `/api/operations/apply` | interactive review of a single change |

The legacy `POST /api/nodes` and `POST /api/relations` routes stay guarded: they
answer `428 Precondition Required` and never write, so an old client cannot skip
review by accident.

## Authentication

```sh
export MYKNOWNWORLD_WRITE_TOKEN='…at least 32 bytes…'
curl -H "Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN" …
```

Without a configured write token every write route answers
`503 writes_disabled`. A missing or wrong token answers `401 unauthorized`.
Health reporting exposes whether writes are enabled: `GET /healthz` →
`{"writesEnabled":true}`.

## Create one node

A published node must be connected to `Self`. You do not need to build that
relation: the server always creates it, defaulting to the `knowsAbout` predicate
and the id `anchor-<node id>`. Supply `anchor` only to choose the predicate or
the relation id.

```sh
curl -sS -X POST http://127.0.0.1:8080/api/write/nodes \
  -H "Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "node": {
      "id": "machine-home-nas",
      "type": "Machine",
      "name": "Home NAS",
      "lifecycle": "running",
      "modelVersion": 1,
      "machine": {"hostname": "nas.example.internal"}
    },
    "anchor": {"predicate": "owns"}
  }'
```

```json
{
  "created": {
    "nodeIds": ["machine-home-nas"],
    "relationIds": ["anchor-machine-home-nas"],
    "delta": {"addedNodes": ["machine-home-nas"], "addedRelations": ["anchor-machine-home-nas"]}
  },
  "revision": "…graph fingerprint…"
}
```

`201 Created` on success. `409 create_rejected` when the graph refuses the
object: duplicate id, unknown type, invalid typed details, or a predicate that is
not valid from `Self` to that node type.

## Create one relation

```sh
curl -sS -X POST http://127.0.0.1:8080/api/write/relations \
  -H "Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"relation": {"id": "nas-runs-backup", "from": "machine-home-nas", "predicate": "dependsOn", "to": "project-backup"}}'
```

Both endpoints must already exist; a relation cannot be created against a
dangling node, and `from == to` is rejected.

## Create a batch

One call, one transaction: either everything is applied or nothing is.

```sh
curl -sS -X POST http://127.0.0.1:8080/api/write/batch \
  -H "Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "nodes": [
      {"node": {"id": "machine-lab-01", "type": "Machine", "name": "Lab 01", "modelVersion": 1}},
      {"node": {"id": "project-lab", "type": "SoftwareProject", "name": "Lab project", "modelVersion": 1}, "anchor": {"predicate": "maintains"}}
    ],
    "relations": [
      {"id": "lab-runs-on-01", "from": "project-lab", "predicate": "dependsOn", "to": "machine-lab-01"}
    ]
  }'
```

```json
{
  "created": {
    "nodeIds": ["machine-lab-01", "project-lab"],
    "relationIds": ["anchor-machine-lab-01", "anchor-project-lab", "lab-runs-on-01"],
    "delta": {"addedNodes": ["machine-lab-01", "project-lab"], "addedRelations": ["anchor-machine-lab-01", "anchor-project-lab", "lab-runs-on-01"]}
  },
  "revision": "…graph fingerprint…"
}
```

Rules:

- Both arrays are optional, but at least one node or relation is required.
- Limits: 500 nodes and 500 relations per batch, 1 MiB request body.
- Ids must be unique inside the batch and must not already exist.
- Relations in the batch may reference nodes created earlier in the same batch
  or nodes that already exist.
- The whole batch shares one revision bump, one history entry per entity, and one
  reversible operation (`manual:createBatch`) that the existing undo endpoint can
  roll back.

## Patch an existing node

Updates editable fields of an existing node in place. Immutable properties
(`id` and `type`) cannot be changed, and the root `Self` node cannot be modified
via this route.

```sh
curl -sS -X PATCH http://127.0.0.1:8080/api/write/nodes/machine-home-nas \
  -H "Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "expectedRevision": "…graph fingerprint…",
    "node": {
      "name": "Home NAS Primary",
      "lifecycle": "running"
    }
  }'
```

```json
{
  "node": {
    "id": "machine-home-nas",
    "type": "Machine",
    "name": "Home NAS Primary",
    …
  },
  "revision": "…updated fingerprint…"
}
```

## Retire an existing node (Soft Decommission)

Retires an entity by setting `lifecycle: "retired"` and updating `lastConfirmed`
to the current UTC timestamp, without breaking existing graph relations or
disconnecting nodes.

Prefer retirement over physical deletion when an entity (such as hardware, a
machine, or a software service) has reached end-of-life or decommissioning: it
preserves history and historical context in the knowledge graph. The root `Self`
node cannot be retired.

```sh
curl -sS -X POST http://127.0.0.1:8080/api/write/nodes/machine-home-nas/retire \
  -H "Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"expectedRevision": "…graph fingerprint…"}'
```

```json
{
  "node": {
    "id": "machine-home-nas",
    "type": "Machine",
    "name": "Home NAS Primary",
    "lifecycle": "retired",
    …
  },
  "revision": "…updated fingerprint…"
}
```

Queries against `GET /api/nodes` support filtering out retired nodes via
`?includeRetired=false` (defaults to `true` to return the complete graph).

## Delete an existing node

Deletes an existing node and records audit history and an undoable snapshot
operation.

**Referential Integrity Guard:**
If the node has active incoming or outgoing relationships with other domain
entities (excluding its automatic `Self` anchor relationship), the request fails
with `409 write_rejected`. You must delete or redirect dependent relationships
first. When the node is isolated down to its `Self` anchor, deleting the node
automatically removes the anchor and asserts graph connectivity. The root `Self`
node cannot be deleted.

```sh
curl -sS -X DELETE http://127.0.0.1:8080/api/write/nodes/machine-home-nas \
  -H "Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"expectedRevision": "…graph fingerprint…"}'
```

```json
{
  "deleted": "machine-home-nas",
  "revision": "…updated fingerprint…"
}
```

## Patch an existing relation

Updates editable fields of an existing relation. Immutable properties (`id`,
`from`, `to`, and `predicate`) cannot be modified.

```sh
curl -sS -X PATCH http://127.0.0.1:8080/api/write/relations/nas-runs-backup \
  -H "Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "expectedRevision": "…graph fingerprint…",
    "relation": {
      "certainty": "confirmed"
    }
  }'
```

```json
{
  "relation": {
    "id": "nas-runs-backup",
    "from": "machine-home-nas",
    "predicate": "dependsOn",
    "to": "project-backup",
    "certainty": "confirmed",
    …
  },
  "revision": "…updated fingerprint…"
}
```

## Delete an existing relation

Deletes an existing relationship. If deleting this relationship disconnects any
active nodes from the graph (violating the reachability invariant from `Self`),
the deletion is rejected with `409 write_rejected`.

```sh
curl -sS -X DELETE http://127.0.0.1:8080/api/write/relations/nas-runs-backup \
  -H "Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"expectedRevision": "…graph fingerprint…"}'
```

```json
{
  "deleted": "nas-runs-backup",
  "revision": "…updated fingerprint…"
}
```

## Optimistic concurrency

Every response returns the resulting graph `revision`. Send it back as
`expectedRevision` to make a mutation conditional:

```json
{"expectedRevision": "…revision you read…", "node": {…}}
```

If the graph moved in between, the request fails with
`409 write_rejected` (or `409 create_rejected`) and the message `graph changed since this batch was
prepared; fetch the current revision and retry`; nothing is written. Omitting
`expectedRevision` accepts the current graph, which is fine for additive work but
means two writers can interleave.

## Status codes

| Code | Meaning |
| --- | --- |
| `200` | updated or deleted (PATCH or DELETE) |
| `201` | created (single or batch POST) |
| `400 invalid_request` | malformed JSON, unknown fields, more than one JSON value, body over the limit |
| `401 unauthorized` | missing or wrong bearer token |
| `404 not_found` | targeted node or relation does not exist |
| `409 write_rejected` / `create_rejected` | domain rejection: duplicate id, immutable field altered, referential integrity violation, dangling endpoint, stale revision |
| `503 writes_disabled` | `MYKNOWNWORLD_WRITE_TOKEN` is not configured |
| `503 write_unavailable` | the storage layer does not support writes (in-memory mode) |

Errors never echo submitted values back, so a caller cannot use the API to read
back an accidentally submitted secret.

## What this API does not do

- No dry run. Use `POST /api/operations/preview` when you want to inspect a
  change before it is applied.
- No field assertions, provisional objects, or sync policies. Those have their
  own guarded endpoints.
- No Markdown, Git, or watcher behaviour. The graph is authoritative.
