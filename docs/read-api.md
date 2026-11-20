# Read API: pagination and projections

The MyKnownWorld HTTP read API is public, stateless, and read-only.
The two core endpoints for reading the graph are:

- `GET /api/nodes`
- `GET /api/relations`

## Forward compatibility

When invoked with no query parameters, both endpoints return the entire graph
as a single JSON document, matching existing caller expectations:

```json
{
  "items": [
    {"id": "self", "type": "Self", "name": "Self", …},
    …
  ]
}
```

No existing caller needs to change to stay functional.

## Pagination

Both `/api/nodes` and `/api/relations` support optional pagination parameters:

| Parameter | Type | Description |
| --- | --- | --- |
| `limit` | integer | Maximum number of items to return per page. Max allowed value is `500`. |
| `cursor` | string | Return items with an identifier strictly greater than `cursor`. Items are sorted stably by `ID`. |
| `offset` | integer | Number of items to skip. Ignored if `cursor` is specified. Must be non-negative. |

### Deterministic ordering

Nodes and relations are always ordered by their identifier (`ID`) in ascending
lexicographical order. Using `cursor` provides stable, drift-free pagination even
if mutations occur concurrently.

### Paginated response shape

When any query parameter (`limit`, `cursor`, `offset`, or `view`) is present,
the response contains pagination metadata:

```json
{
  "items": [ … ],
  "totalCount": 26,
  "count": 10,
  "hasMore": true,
  "nextCursor": "node-09"
}
```

In addition, standard headers are included on paginated responses:
- `X-Total-Count`: Total number of entities in the graph matching the scope.
- `X-Next-Cursor`: Identifier of the last item in the page (present when `hasMore` is true).

### Example: cursor-based traversal

```sh
# Page 1
curl -sS "http://127.0.0.1:8080/api/nodes?limit=10"

# Page 2 (using nextCursor from page 1)
curl -sS "http://127.0.0.1:8080/api/nodes?limit=10&cursor=node-09"
```

## Summary projection (`view=summary`)

For interactive or overview consumers that do not need detailed typed blocks
(e.g., `machine`, `softwareProject`, `hardware`, `network`), pass `view=summary`.

```sh
curl -sS "http://127.0.0.1:8080/api/nodes?view=summary&limit=50"
```

Summary items omit all detail blocks and only carry baseline identity,
provenance, and certainty fields:

```json
{
  "items": [
    {
      "id": "machine-nas",
      "type": "Machine",
      "name": "Home NAS",
      "lifecycle": "running",
      "source": "wiki",
      "evidenceKind": "observed",
      "certainty": "confirmed"
    }
  ],
  "totalCount": 1,
  "count": 1,
  "hasMore": false
}
```

This shape directly aligns with the output returned by the MCP `graph_query` tool.

Valid values for `view` are `summary` and `full` (default). Any other value
results in `400 invalid_request`.

## Materialization (`myknownworld-materialize`)

The bundled export CLI `myknownworld-materialize` uses this pagination API
internally. It traverses the graph in chunks (200 entities per request) using
cursor paging until exhaustion, ensuring that exports never run out of memory or
hit HTTP response limits regardless of graph size. The materialized Markdown
output remains 100% byte-identical.
