# Write API: create nodes and relations

The read API is public and read-only. Writes always require the independent
`MYKNOWNWORLD_WRITE_TOKEN` (at least 32 bytes) as a bearer credential, and they
never accept secrets: the typed model has no secret fields, and nothing you send
here is copied into Markdown, MCP responses, or logs.

There are two write styles:

| Style | Endpoints | Use it for |
| --- | --- | --- |
| Create API (this document) | `POST /api/write/nodes`, `/api/write/relations`, `/api/write/batch` | scripts, automation, one-shot additions |
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

## Optimistic concurrency

Every response returns the resulting graph `revision`. Send it back as
`expectedRevision` to make a create conditional:

```json
{"expectedRevision": "…revision you read…", "node": {…}}
```

If the graph moved in between, the request fails with
`409 create_rejected` and the message `graph changed since this batch was
prepared; fetch the current revision and retry`; nothing is written. Omitting
`expectedRevision` accepts the current graph, which is fine for additive work but
means two writers can interleave.

## Status codes

| Code | Meaning |
| --- | --- |
| `201` | created (single or batch) |
| `400 invalid_request` | malformed JSON, unknown fields, more than one JSON value, body over the limit |
| `401 unauthorized` | missing or wrong bearer token |
| `409 create_rejected` | domain rejection: duplicate id, unknown type, invalid details, invalid predicate, dangling endpoint, stale revision |
| `503 writes_disabled` | `MYKNOWNWORLD_WRITE_TOKEN` is not configured |
| `503 write_unavailable` | the storage layer does not support writes (in-memory mode) |

Errors never echo submitted values back, so a caller cannot use the API to read
back an accidentally submitted secret.

## What this API does not do

- No update, upsert, or delete. Use the preview-and-confirm operation routes for
  edits and removals; the create API only adds.
- No dry run. Use `POST /api/operations/preview` when you want to inspect a
  change before it is applied.
- No field assertions, provisional objects, or sync policies. Those have their
  own guarded endpoints.
- No Markdown, Git, or watcher behaviour. The graph is authoritative.
