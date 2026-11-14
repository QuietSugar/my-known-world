# Read-only MCP interface

MyKnownWorld exposes the official Go MCP SDK's **Streamable HTTP** transport at `/mcp` and a stdio executable for local agents. The tools use the same graph domain methods and PostgreSQL readers as the HTTP API; their results are typed structured JSON. No mutation tools are registered, and neither transport receives or echoes `MYKNOWNWORLD_WRITE_TOKEN`.

Available tools:

- `graph_query`: type, lifecycle, certainty, evidence-kind, and case-insensitive name/ID substring filters; stable ID order, bounded count, and explicit empty arrays.
- `graph_entity`: typed node details, direct relations, Self relation category/path, and assertion metadata with no value hashes.
- `graph_relations`: direct relations filtered by `out`, `in`, or `both` and an optional predicate.
- `graph_impact`: existing Self impact paths; the domain analysis excludes `knowsAbout`.
- `graph_explore`: existing direction/predicate/depth rules and 250-node truncation signal.
- `graph_history`: append-only node/relation events; requires PostgreSQL history storage.

The regular HTTP process serves `/mcp` on its configured HTTP listener. Local API defaults to `127.0.0.1:8080`; Compose publishes only on localhost by default. For local stdio use a database already initialized by the application:

```sh
DATABASE_URL='postgres://…' go run ./cmd/myknownworld-mcp
```

For the bundled Compose app, the binary is included in the container; a local MCP client can launch it as a subprocess with `docker compose exec -T app myknownworld-mcp` (run from the deployment directory). The container already has the private database URL; do not put credentials in MCP arguments or commit them. Stdio emits protocol messages only on stdout; diagnostics go to stderr. It does not migrate, seed, or mutate the graph. If there is no complete snapshot, startup fails closed. Keep the HTTP listener private as well: MCP has the same read access as the unauthenticated read-only API.
